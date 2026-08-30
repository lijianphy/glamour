package latex

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	inlineClose  = []byte(`\)`)
	displayOpen  = []byte(`\[`)
	displayClose = []byte(`\]`)
	dollarFence  = []byte(`$$`)
)

type inlineParser struct{}

func newInlineParser() parser.InlineParser {
	return &inlineParser{}
}

func (p *inlineParser) Trigger() []byte {
	return []byte{'\\', '$'}
}

func (p *inlineParser) Parse(_ ast.Node, reader text.Reader, _ parser.Context) ast.Node {
	line, segment := reader.PeekLine()
	end := lineContentEnd(line)
	if end < 2 {
		return nil
	}
	line = line[:end]
	switch line[0] {
	case '\\':
		return parseBackslashMath(reader, line, segment)
	case '$':
		return parseDollarMath(reader, line, segment)
	default:
		return nil
	}
}

func parseBackslashMath(reader text.Reader, line []byte, segment text.Segment) ast.Node {
	var closer []byte
	switch line[1] {
	case '(':
		closer = inlineClose
	case '[':
		closer = displayClose
	case ')', ']':
		reader.Advance(2)
		return newInlineMath(segment.WithStop(segment.Start + 2))
	default:
		return nil
	}

	stop := findUnescapedDelimiter(line, closer, 2)
	if stop < 0 {
		// Keeping an incomplete expression raw avoids delimiter flicker while
		// Markdown is being streamed. Inline expressions do not consume a
		// subsequent source line.
		stop = len(line)
	}
	reader.Advance(stop)
	return newInlineMath(segment.WithStop(segment.Start + stop))
}

func parseDollarMath(reader text.Reader, line []byte, segment text.Segment) ast.Node {
	if line[1] == '$' {
		stop := findUnescapedDelimiter(line, dollarFence, 2)
		if stop < 0 || len(bytes.TrimSpace(line[2:stop-2])) == 0 {
			return nil
		}
		reader.Advance(stop)
		return newInlineMath(segment.WithStop(segment.Start + stop))
	}
	if !validDollarOpener(line) {
		return nil
	}

	stop := findSingleDollarClose(line)
	if stop < 0 {
		return nil
	}
	reader.Advance(stop)
	return newInlineMath(segment.WithStop(segment.Start + stop))
}

func findUnescapedDelimiter(line, delimiter []byte, start int) int {
	for start+len(delimiter) <= len(line) {
		offset := bytes.Index(line[start:], delimiter)
		if offset < 0 {
			return -1
		}
		index := start + offset
		if !escapedBackslash(line, index) {
			return index + len(delimiter)
		}
		start = index + 1
	}
	return -1
}

func validDollarOpener(line []byte) bool {
	return len(line) > 1 && line[0] == '$' && line[1] != '$' && !util.IsSpace(line[1])
}

func findSingleDollarClose(line []byte) int {
	for i := 1; i < len(line); i++ {
		if line[i] != '$' || escapedBackslash(line, i) {
			continue
		}
		// An invalid candidate may be the opener of a later expression. Do not
		// let an earlier currency or shell marker consume that expression.
		if !validDollarCloser(line, i) {
			return -1
		}
		return i + 1
	}
	return -1
}

func validDollarCloser(line []byte, index int) bool {
	if index <= 0 || util.IsSpace(line[index-1]) {
		return false
	}
	return index+1 >= len(line) || line[index+1] < '0' || line[index+1] > '9'
}

func escapedBackslash(line []byte, index int) bool {
	count := 0
	for index--; index >= 0 && line[index] == '\\'; index-- {
		count++
	}
	return count%2 != 0
}

func lineContentEnd(line []byte) int {
	end := len(line)
	if end > 0 && line[end-1] == '\n' {
		end--
	}
	if end > 0 && line[end-1] == '\r' {
		end--
	}
	return end
}

type blockParser struct{}

type blockState struct {
	node       *MathBlock
	indent     int
	closer     []byte
	singleLine bool
}

var blockStateKey = parser.NewContextKey()

func newBlockParser() parser.BlockParser {
	return &blockParser{}
}

func (p *blockParser) Trigger() []byte {
	return []byte{'\\', '$'}
}

func (p *blockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	offset := pc.BlockOffset()
	closer, singleLine, ok := mathBlockOpening(line, offset)
	if !ok {
		return nil, parser.NoChildren
	}

	node := newMathBlock()
	pc.Set(blockStateKey, &blockState{
		node:       node,
		indent:     pc.BlockIndent(),
		closer:     closer,
		singleLine: singleLine,
	})
	start := segment.Start - segment.Padding + offset
	node.Lines().Append(text.NewSegment(start, segment.Stop))
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

func (p *blockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	state, ok := pc.Get(blockStateKey).(*blockState)
	if !ok || state.node != node || state.singleLine {
		return parser.Close
	}

	line, segment := reader.PeekLine()
	if line == nil {
		return parser.Close
	}

	math := state.node
	closed := delimiterLine(line, state.closer)
	math.Lines().Append(mathLineSegment(line, segment, reader, state.indent))
	reader.AdvanceToEOL()
	if closed {
		return parser.Close
	}
	return parser.Continue | parser.NoChildren
}

func (p *blockParser) Close(node ast.Node, _ text.Reader, pc parser.Context) {
	state, ok := pc.Get(blockStateKey).(*blockState)
	if ok && state.node == node {
		pc.Set(blockStateKey, nil)
	}
}

func (p *blockParser) CanInterruptParagraph() bool {
	return true
}

func (p *blockParser) CanAcceptIndentedLine() bool {
	return false
}

func delimiterLine(line, delimiter []byte) bool {
	return bytes.Equal(bytes.TrimSpace(line), delimiter)
}

func mathBlockOpening(line []byte, offset int) (closer []byte, singleLine bool, ok bool) {
	if offset < 0 || offset >= len(line) {
		return nil, false, false
	}
	value := bytes.TrimSpace(line[offset:])
	switch {
	case bytes.Equal(value, displayOpen):
		return displayClose, false, true
	case bytes.Equal(value, dollarFence):
		return dollarFence, false, true
	case singleLineDollarBlock(value):
		return dollarFence, true, true
	default:
		return nil, false, false
	}
}

func singleLineDollarBlock(value []byte) bool {
	return len(value) > 4 &&
		bytes.HasPrefix(value, dollarFence) &&
		bytes.HasSuffix(value, dollarFence) &&
		len(bytes.TrimSpace(value[2:len(value)-2])) > 0 &&
		!escapedBackslash(value, len(value)-2)
}

func mathLineSegment(line []byte, segment text.Segment, reader text.Reader, indent int) text.Segment {
	pos, padding := util.IndentPositionPadding(line, reader.LineOffset(), segment.Padding, indent)
	if pos < 0 {
		pos = max(0, util.FirstNonSpacePosition(line)) - segment.Padding
		padding = 0
	}
	return text.NewSegmentPadding(segment.Start+pos, segment.Stop, padding)
}

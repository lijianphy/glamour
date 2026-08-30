package latex

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	displayOpen  = []byte(`\[`)
	displayClose = []byte(`\]`)
)

type inlineParser struct{}

// NewInlineParser returns a parser for parenthesized and bracketed LaTeX math.
func NewInlineParser() parser.InlineParser {
	return &inlineParser{}
}

func (p *inlineParser) Trigger() []byte {
	return []byte{'\\'}
}

func (p *inlineParser) Parse(_ ast.Node, reader text.Reader, _ parser.Context) ast.Node {
	line, segment := reader.PeekLine()
	end := lineContentEnd(line)
	if end < 2 || line[0] != '\\' {
		return nil
	}

	var closer byte
	display := false
	switch line[1] {
	case '(':
		closer = ')'
	case '[':
		closer = ']'
		display = true
	case ')', ']':
		reader.Advance(2)
		return NewInlineMath(segment.WithStop(segment.Start+2), line[1] == ']')
	default:
		return nil
	}

	stop := findInlineClose(line[:end], closer)
	if stop < 0 {
		// Keeping an incomplete expression raw avoids delimiter flicker while
		// Markdown is being streamed. Inline expressions do not consume a
		// subsequent source line.
		stop = end
	}
	reader.Advance(stop)
	return NewInlineMath(segment.WithStop(segment.Start+stop), display)
}

func findInlineClose(line []byte, closer byte) int {
	for i := 2; i+1 < len(line); i++ {
		if line[i] != '\\' || line[i+1] != closer || escapedBackslash(line, i) {
			continue
		}
		return i + 2
	}
	return -1
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

// NewBlockParser returns a parser for display LaTeX math on standalone lines.
func NewBlockParser() parser.BlockParser {
	return &blockParser{}
}

func (p *blockParser) Trigger() []byte {
	return []byte{'\\'}
}

func (p *blockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	offset := pc.BlockOffset()
	if offset < 0 || !delimiterLine(line[offset:], displayOpen) {
		return nil, parser.NoChildren
	}

	node := NewMathBlock(pc.BlockIndent())
	start := segment.Start - segment.Padding + offset
	node.Lines().Append(text.NewSegment(start, segment.Stop))
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

func (p *blockParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if line == nil {
		return parser.Close
	}

	math := node.(*MathBlock)
	closed := delimiterLine(line, displayClose)
	math.Lines().Append(mathLineSegment(line, segment, reader, math.Indent))
	reader.AdvanceToEOL()
	if closed {
		return parser.Close
	}
	return parser.Continue | parser.NoChildren
}

func (p *blockParser) Close(_ ast.Node, _ text.Reader, _ parser.Context) {}

func (p *blockParser) CanInterruptParagraph() bool {
	return true
}

func (p *blockParser) CanAcceptIndentedLine() bool {
	return false
}

func delimiterLine(line, delimiter []byte) bool {
	return bytes.Equal(bytes.TrimSpace(line), delimiter)
}

func mathLineSegment(line []byte, segment text.Segment, reader text.Reader, indent int) text.Segment {
	pos, padding := util.IndentPositionPadding(line, reader.LineOffset(), segment.Padding, indent)
	if pos < 0 {
		pos = max(0, util.FirstNonSpacePosition(line)) - segment.Padding
		padding = 0
	}
	return text.NewSegmentPadding(segment.Start+pos, segment.Stop, padding)
}

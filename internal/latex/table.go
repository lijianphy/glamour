package latex

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	gfm "github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// TableParserOptions installs the GFM table parser with span-aware row splitting.
func TableParserOptions(math, code bool) []parser.Option {
	return []parser.Option{
		parser.WithParagraphTransformers(util.Prioritized(tableTransformer{math: math, code: code}, 200)),
		parser.WithASTTransformers(util.Prioritized(gfm.NewTableASTTransformer(), 0)),
	}
}

type tableTransformer struct{ math, code bool }

var tableSourceKey = parser.NewContextKey()

func (t tableTransformer) Transform(p *ast.Paragraph, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	lines := p.Lines()
	// Only scan potential table rows. Goldmark still validates the delimiter,
	// matches the header width, and handles alignment and escaped code pipes.
	for i := 1; i < lines.Len(); i++ {
		seg := lines.At(i)
		line := seg.Value(source)
		if !bytes.ContainsRune(line, '-') || len(bytes.Trim(line, " \t\r\n\v\f|:-")) != 0 {
			continue
		}
		var masked []byte
		for j := i - 1; j < lines.Len(); j++ {
			row := lines.At(j)
			rowBytes := source[row.Start:row.Stop]
			if !bytes.ContainsRune(rowBytes, '|') {
				continue
			}
			t.visitSpanPipes(rowBytes, func(pos int) {
				// Give only the row splitter a same-length view with span pipes
				// hidden. Its AST segments still index the untouched input.
				if masked == nil {
					masked = pc.ComputeIfAbsent(tableSourceKey, func() any { return bytes.Clone(source) }).([]byte)
				}
				masked[row.Start+pos] = 'x'
			})
		}
		if masked != nil {
			reader = text.NewReader(masked)
		}
		break
	}
	gfm.NewTableParagraphTransformer().Transform(p, reader, pc)
}

func (t tableTransformer) visitSpanPipes(line []byte, visit func(int)) {
	line = line[:lineContentEnd(line)]
	for i := 0; i < len(line); {
		var length int
		switch line[i] {
		case '`':
			// An unmatched run is returned as text, not a CodeSpan.
			r := text.NewReader(line[i:])
			n := parser.NewCodeSpanParser().Parse(nil, r, nil)
			_, pos := r.Position()
			if !t.code || n.Kind() != ast.KindCodeSpan {
				i += untilCellEnd(line[i : i+pos.Start])
				continue
			}
			length = pos.Start
		case '\\', '$':
			if t.math {
				stop, width := matchInlineMath(line[i:])
				if stop != 0 && width == 0 {
					// An incomplete expression stays raw only within its cell.
					i += untilCellEnd(line[i : i+stop])
					continue
				}
				length = stop
			}
			if length == 0 && line[i] == '\\' {
				// A Markdown escape cannot start another span.
				i += 2
				continue
			}
		}
		if length == 0 {
			i++
			continue
		}
		for j := i + 1; j < i+length; j++ {
			// Leave escaped pipes in code visible to GFM's unescaper.
			if line[j] == '|' && (line[i] != '`' || line[j-1] != '\\') {
				visit(j)
			}
		}
		i += length
	}
}

// Even if a backtick or an incomplete expression opens in one cell, resume
// looking for spans in the next. Match Goldmark's escaped-pipe check here.
func untilCellEnd(span []byte) int {
	for i, c := range span {
		if c == '|' && (i == 0 || span[i-1] != '\\') {
			return i
		}
	}
	return len(span)
}

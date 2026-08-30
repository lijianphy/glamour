package latex

import (
	"slices"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func TestExtensionParsesMathAndLeavesCodeRaw(t *testing.T) {
	source := []byte(strings.Join([]string{
		"Inline \\(x_*\\) and $y_*$.",
		"",
		"\\[",
		"E_* = mc^2",
		"\\]",
		"",
		"$$",
		"F_* = ma",
		"$$",
		"",
		"`\\(code\\) and $code$`",
		"",
		"```",
		"\\[code\\] and $$code$$",
		"```",
	}, "\n"))
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))

	var inline []string
	var blocks []string
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *InlineMath:
			inline = append(inline, string(node.Text(source)))
		case *MathBlock:
			blocks = append(blocks, string(node.Text(source)))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(inline) != 2 || inline[0] != `\(x_*\)` || inline[1] != `$y_*$` {
		t.Fatalf("inline math = %#v, want two expressions", inline)
	}
	if len(blocks) != 2 ||
		blocks[0] != "\\[\nE_* = mc^2\n\\]\n" ||
		blocks[1] != "$$\nF_* = ma\n$$\n" {
		t.Fatalf("display math = %#v, want two expressions", blocks)
	}
}

func TestInlineParserUsesUnescapedClosingDelimiter(t *testing.T) {
	source := []byte(`\(a \\) b \)`)
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))
	expression := document.FirstChild().FirstChild().(*InlineMath)
	if got := string(expression.Text(source)); got != string(source) {
		t.Fatalf("inline math = %q, want %q", got, source)
	}
}

func TestExtensionDoesNotParseEscapedMathOpeners(t *testing.T) {
	source := []byte(`Literal \\(not math\\) and \$not math$`)
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))

	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && (node.Kind() == KindInlineMath || node.Kind() == KindMathBlock) {
			t.Fatalf("unexpected math node for escaped delimiter: %T", node)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExtensionDoesNotParseCurrencyOrShellVariables(t *testing.T) {
	source := []byte("Costs $10 and $20. Shell $HOME. Spaced $ not math $.")
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))

	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && node.Kind() == KindInlineMath {
			t.Fatalf("unexpected inline math node for ordinary dollars: %q", node.Text(source))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDollarMathDoesNotStartAtEarlierCurrency(t *testing.T) {
	source := []byte("Cost $10 and equation $x + 1$.")
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))

	var expressions []string
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && node.Kind() == KindInlineMath {
			expressions = append(expressions, string(node.Text(source)))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(expressions) != 1 || expressions[0] != `$x + 1$` {
		t.Fatalf("dollar math = %#v, want only the equation", expressions)
	}
}

func TestExtensionParsesDollarMathWithEscapedDollar(t *testing.T) {
	source := []byte(`Math $x + \$5$ and $$E = mc^2$$.`)
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))

	var expressions []string
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && node.Kind() == KindInlineMath {
			expressions = append(expressions, string(node.Text(source)))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(expressions) != 2 ||
		expressions[0] != `$x + \$5$` ||
		expressions[1] != `$$E = mc^2$$` {
		t.Fatalf("dollar math = %#v, want inline and single-line display expressions", expressions)
	}
}

func TestExtensionParsesSingleLineDollarMathBlock(t *testing.T) {
	source := []byte("$$E_* = mc^2 + **raw**$$\n\nAfter.")
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))

	block, ok := document.FirstChild().(*MathBlock)
	if !ok {
		t.Fatalf("first node = %T, want *MathBlock", document.FirstChild())
	}
	if got := string(block.Text(source)); got != "$$E_* = mc^2 + **raw**$$\n" {
		t.Fatalf("single-line display math = %q", got)
	}
	if next := block.NextSibling(); next == nil || next.Kind() != ast.KindParagraph {
		t.Fatalf("node after single-line display math = %T, want paragraph", next)
	}
}

func TestDollarInlineRules(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantInline []string
	}{
		{
			name:       "simple",
			source:     "$x$",
			wantInline: []string{"$x$"},
		},
		{
			name:   "space after opener",
			source: "$ x$",
		},
		{
			name:   "space before closer",
			source: "$x $",
		},
		{
			name:   "digit after closer",
			source: "$x$5",
		},
		{
			name:       "adjacent expressions",
			source:     "$x$$y$",
			wantInline: []string{"$x$", "$y$"},
		},
		{
			name:   "empty double dollar",
			source: "$$$$",
		},
		{
			name:   "blank double dollar",
			source: "$$ $$",
		},
		{
			name:   "escaped opener",
			source: `\$x$`,
		},
		{
			name:       "inline double dollar",
			source:     "Math $$x_* + **raw**$$ here.",
			wantInline: []string{"$$x_* + **raw**$$"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inline, blocks := parseMathNodes(t, []byte(test.source))
			if !slices.Equal(inline, test.wantInline) {
				t.Fatalf("inline math = %#v, want %#v", inline, test.wantInline)
			}
			if len(blocks) != 0 {
				t.Fatalf("display math = %#v, want none", blocks)
			}
		})
	}
}

func TestExtensionParsesCRLFDisplayBlock(t *testing.T) {
	source := []byte("$$\r\nE_* = mc^2\r\n$$\r\n")
	_, blocks := parseMathNodes(t, source)
	if len(blocks) != 1 || blocks[0] != string(source) {
		t.Fatalf("CRLF display math = %#v, want %q", blocks, source)
	}
}

func FuzzExtensionParser(f *testing.F) {
	for _, source := range [][]byte{
		nil,
		[]byte("$x$"),
		[]byte("$$\nE = mc^2\n$$"),
		[]byte("\\[\nE = mc^2\n\\]"),
		[]byte(`Cost $10 and math $x$.`),
		[]byte{0xff, '$', '$', '\\'},
	} {
		f.Add(source)
	}

	f.Fuzz(func(t *testing.T, source []byte) {
		document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))
		if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch node := node.(type) {
			case *InlineMath:
				assertValidSegment(t, source, node.segment)
			case *MathBlock:
				for index := range node.Lines().Len() {
					assertValidSegment(t, source, node.Lines().At(index))
				}
			}
			return ast.WalkContinue, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
}

func parseMathNodes(t *testing.T, source []byte) (inline, blocks []string) {
	t.Helper()
	document := goldmark.New(goldmark.WithExtensions(Extension)).Parser().Parse(text.NewReader(source))
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *InlineMath:
			inline = append(inline, string(node.Text(source)))
		case *MathBlock:
			blocks = append(blocks, string(node.Text(source)))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
	return inline, blocks
}

func assertValidSegment(t *testing.T, source []byte, segment text.Segment) {
	t.Helper()
	if segment.Start < 0 || segment.Stop < segment.Start || segment.Stop > len(source) {
		t.Fatalf("math segment [%d:%d] is outside source length %d", segment.Start, segment.Stop, len(source))
	}
}

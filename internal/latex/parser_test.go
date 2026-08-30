package latex

import (
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func TestExtensionParsesMathAndLeavesCodeRaw(t *testing.T) {
	source := []byte("Inline \\(x_*\\).\n\n\\[\nE_* = mc^2\n\\]\n\n`\\(code\\)`\n\n```\n\\[code\\]\n```")
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

	if len(inline) != 1 || inline[0] != `\(x_*\)` {
		t.Fatalf("inline math = %#v, want one expression", inline)
	}
	if len(blocks) != 1 || blocks[0] != "\\[\nE_* = mc^2\n\\]\n" {
		t.Fatalf("display math = %#v, want one expression", blocks)
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
	source := []byte(`Literal \\(not math\\)`)
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

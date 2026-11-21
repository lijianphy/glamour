package glamour

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/glamour/v2/internal/latex"
	"github.com/yuin/goldmark/ast"
	table "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

func TestMathTableCells(t *testing.T) {
	const density = `$n(\mathbf{r}) = \sum_i |\phi_i|^2$`
	for _, tc := range []struct {
		name   string
		markup string
		math   bool
		want   [3]string
	}{
		{"density", "| Density | " + density + " | sum over occupied states |", true, [3]string{"Density", density, "sum over occupied states"}},
		{"parentheses", `a | \(x|y\) | z`, true, [3]string{"a", `\(x|y\)`, "z"}},
		{"display brackets", `a | \[x|y\] | z`, true, [3]string{"a", `\[x|y\]`, "z"}},
		{"display dollars", `a | $$x|y$$ | z`, true, [3]string{"a", `$$x|y$$`, "z"}},
		{"unclosed dollars", `a | $x | z`, false, [3]string{"a", "$x", "z"}},
		{"unclosed brackets", `a | \[x | z`, false, [3]string{"a", `\[x`, "z"}},
		{"math after incomplete expression", `a \[ | $x|y$ | z`, true, [3]string{`a \[`, `$x|y$`, "z"}},
		{"escaped opener", `a | \$x|y$`, false, [3]string{"a", `\$x`, "y$"}},
		{"escaped closer", `a | $x\$|y$ | z`, true, [3]string{"a", `$x\$|y$`, "z"}},
		{"currency", `a | $5 | $10`, false, [3]string{"a", "$5", "$10"}},
		{"escaped pipe in code", "a | ``a`\\|b`` | z", false, [3]string{"a", "a`|b", "z"}},
		{"raw pipe in code", "a | `$x|y$`", false, [3]string{"a", "`$x", "y$`"}},
		{"code then math", "`$` | $x|y$ | z", true, [3]string{"$", "$x|y$", "z"}},
		{"backticks in separate cells", "a` | $x|y$ | z`", true, [3]string{"a`", "$x|y$", "z`"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewTermRenderer(WithLatexMath())
			if err != nil {
				t.Fatal(err)
			}
			// Exercise identical syntax in a header and a body row. Use CRLF
			// to also check the source offsets.
			input := []byte(tc.markup + "\r\n:---|:---:|---:\r\n" + tc.markup)
			original := bytes.Clone(input)
			doc := r.md.Parser().Parse(text.NewReader(input))
			tbl, ok := doc.FirstChild().(*table.Table)
			if !ok || tbl.ChildCount() != 2 {
				t.Fatalf("expected a table with header and body, got %T", doc.FirstChild())
			}
			for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
				if row.ChildCount() != len(tc.want) {
					t.Fatalf("got %d cells, want %d", row.ChildCount(), len(tc.want))
				}
				cell := row.FirstChild()
				for i, want := range tc.want {
					if got := string(cell.Text(input)); got != want {
						t.Errorf("cell %d: got %q, want %q", i, got, want)
					}
					if i == 1 && tc.math {
						m, ok := cell.FirstChild().(*latex.InlineMath)
						if !ok || m.Body(input) == nil || cell.ChildCount() != 1 {
							t.Errorf("expected a single complete math node, got %T", cell.FirstChild())
						}
					}
					cell = cell.NextSibling()
				}
			}
			if !bytes.Equal(input, original) {
				t.Fatal("parser modified its input")
			}
		})
	}
}

func TestMathTableOptIn(t *testing.T) {
	r, err := NewTermRenderer()
	if err != nil {
		t.Fatal(err)
	}
	input := []byte("| H | I |\n|---|---|\n| $x|y$ | after |")
	doc := r.md.Parser().Parse(text.NewReader(input))
	row := doc.FirstChild().LastChild()
	if got := string(row.LastChild().Text(input)); got != "y$" {
		t.Errorf("default renderer changed GFM pipe splitting: got %q", got)
	}
}

func TestTableCodePipes(t *testing.T) {
	code := []TermRendererOption{WithTableCodePipes()}
	math := []TermRendererOption{WithLatexMath()}
	both := []TermRendererOption{WithTableCodePipes(), WithLatexMath()}
	for _, tc := range []struct {
		name string
		opts []TermRendererOption
		cell string
		want []string // cells before the final Notes cell
		kind ast.NodeKind
	}{
		{"default", nil, "`sort | uniq`", []string{"`sort", "uniq`"}, ast.KindText},
		{"math only", math, "`sort | uniq`", []string{"`sort", "uniq`"}, ast.KindText},
		{"code only", code, "`sort | uniq`", []string{"sort | uniq"}, ast.KindCodeSpan},
		{"both", both, "`sort | uniq`", []string{"sort | uniq"}, ast.KindCodeSpan},
		{"code only leaves math off", code, "$x|y$", []string{"$x", "y$"}, ast.KindText},
		{"raw and escaped", code, "`a|b\\|c`", []string{"a|b|c"}, ast.KindCodeSpan},
		{"longer fence", code, "``a`|b``", []string{"a`|b"}, ast.KindCodeSpan},
		{"mismatched fence", code, "``a|b`", []string{"``a", "b`"}, ast.KindText},
		{"escaped opener", code, "\\`a|b`", []string{"\\`a", "b`"}, ast.KindText},
		{"code opens first", both, "`$a|b$`", []string{"$a|b$"}, ast.KindCodeSpan},
		{"math opens first", both, "$a`|`b$", []string{"$a`|`b$"}, latex.KindInlineMath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewTermRenderer(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			rowText := "| " + tc.cell + " | Notes |"
			want := append(tc.want, "Notes")
			input := []byte(rowText + "\n" + strings.Repeat("|---", len(want)) + "|\n" + rowText)
			doc := r.md.Parser().Parse(text.NewReader(input))
			tbl, ok := doc.FirstChild().(*table.Table)
			if !ok || tbl.ChildCount() != 2 {
				t.Fatalf("expected header and body, got %T", doc.FirstChild())
			}
			for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
				if row.ChildCount() != len(want) {
					t.Fatalf("got %d cells, want %d", row.ChildCount(), len(want))
				}
				cell := row.FirstChild()
				for i, s := range want {
					if got := string(cell.Text(input)); got != s {
						t.Errorf("cell %d: got %q, want %q", i, got, s)
					}
					if i == 0 && (cell.FirstChild() == nil || cell.FirstChild().Kind() != tc.kind) {
						t.Errorf("cell parsed as %T, want %s", cell.FirstChild(), tc.kind)
					}
					cell = cell.NextSibling()
				}
			}
		})
	}
}

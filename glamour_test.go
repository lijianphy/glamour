package glamour

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"charm.land/glamour/v2/styles"
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
)

const markdown = "testdata/readme.markdown.in"

func TestTermRendererWriter(t *testing.T) {
	r, err := NewTermRenderer(
		WithStandardStyle(styles.DarkStyle),
	)
	if err != nil {
		t.Fatal(err)
	}

	in, err := os.ReadFile(markdown)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Write(in)
	if err != nil {
		t.Fatal(err)
	}
	err = r.Close()
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	golden.RequireEqual(t, b)
}

func TestTermRenderer(t *testing.T) {
	r, err := NewTermRenderer(
		WithStandardStyle("dark"),
	)
	if err != nil {
		t.Fatal(err)
	}

	in, err := os.ReadFile(markdown)
	if err != nil {
		t.Fatal(err)
	}

	b, err := r.Render(string(in))
	if err != nil {
		t.Fatal(err)
	}

	golden.RequireEqual(t, []byte(b))
}

func TestWithEmoji(t *testing.T) {
	r, err := NewTermRenderer(
		WithEmoji(),
	)
	if err != nil {
		t.Fatal(err)
	}

	b, err := r.Render(":+1:")
	if err != nil {
		t.Fatal(err)
	}
	b = strings.TrimSpace(b)

	// Thumbs up unicode character
	td := "\U0001f44d"

	if td != b {
		t.Errorf("Rendered output doesn't match!\nExpected: `\n%s`\nGot: `\n%s`\n", td, b)
	}
}

func TestWithPreservedNewLines(t *testing.T) {
	r, err := NewTermRenderer(
		WithPreservedNewLines(),
	)
	if err != nil {
		t.Fatal(err)
	}

	in, err := os.ReadFile("testdata/preserved_newline.in")
	if err != nil {
		t.Fatal(err)
	}

	b, err := r.Render(string(in))
	if err != nil {
		t.Fatal(err)
	}

	golden.RequireEqual(t, []byte(b))
}

func TestWithLatexMathTypesetsInlineAndDisplayExpressions(t *testing.T) {
	r, err := NewTermRenderer(
		WithLatexMath(),
		WithWordWrap(80),
	)
	if err != nil {
		t.Fatal(err)
	}

	source := "Inline \\(\\alpha\\) and $x^2$.\n\n\\[\n\\frac{a}{b}\n\\]\n\n$$y_1$$"
	want := "Inline α and x².\n\n a\n───\n b\n\ny₁"
	rendered, err := r.Render(source)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimRenderedRows(xansi.Strip(rendered)); got != want {
		t.Fatalf("rendered LaTeX math =\n%q\nwant\n%q", got, want)
	}
}

func TestTallInlineMathPromotion(t *testing.T) {
	const frac = `$\dfrac{a}{b}$`
	r, err := NewTermRenderer(WithLatexMath(), WithWordWrap(40))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ source, want string }{
		{"before " + frac + " after", "before\n\n a\n───\n b\n\nafter"},
		{frac + " " + frac, " a\n───\n b\n\n a\n───\n b"},
		{"> before " + frac + " after", "before\n\n a\n───\n b\n\nafter"},
	} {
		out, err := r.Render(tt.source)
		if err != nil || trimRenderedRows(xansi.Strip(out)) != tt.want {
			t.Errorf("Render(%q) = %q, %v; want %q", tt.source, out, err, tt.want)
		}
	}
	wide := `$\dfrac{\text{` + strings.Repeat("a", 45) + `}}{b}$`
	out, err := r.Render(wide)
	if err != nil || !strings.Contains(strings.Join(strings.Fields(xansi.Strip(out)), ""), wide) {
		t.Errorf("overwide equation must retain source: %q, %v", out, err)
	}
	r, err = NewTermRenderer(WithLatexMath(), WithStandardStyle(styles.DarkStyle), WithWordWrap(40))
	if err != nil {
		t.Fatal(err)
	}
	out, err = r.Render("- before " + frac + " after")
	rows := strings.Split(trimRenderedRows(xansi.Strip(out)), "\n")
	if err != nil || len(rows) != 7 || rows[1] != "" || rows[5] != "" ||
		strings.Index(rows[0], "before") != strings.Index(rows[3], "───") ||
		strings.Index(rows[2], "a") != strings.Index(rows[3], "───")+1 || !strings.Contains(rows[6], "after") {
		t.Errorf("list equation lost its content column: %q, %v", rows, err)
	}
}

func TestTableMathLayout(t *testing.T) {
	const source = `| $\dfrac{a}{b}$ | $\frac{a}{b}$ |
| ---: | :---: |
| before $$\frac{x}{y}$$ after | **\[\frac{m}{n}\]** |
| left $$z$$ right | $x^2$ |`
	// The header rule follows the denominator. Fractions stay rectangular
	// under right/center alignment, even with prose on either side.
	const want = `        a  │  a/b
       ─── │
        b  │
───────────┼────────
    before │   m
        x  │  ───
       ─── │   n
        y  │
     after │
      left │   x²
         z │
     right │`
	for _, wrap := range []bool{true, false} {
		r, err := NewTermRenderer(WithLatexMath(), WithTableWrap(wrap), WithWordWrap(20))
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(source)
		if got := trimRenderedRows(xansi.Strip(out)); err != nil || got != want {
			t.Errorf("wrap=%t:\n%s\nwant:\n%s\nerr: %v", wrap, got, want, err)
		}
	}
}

func TestTableMathFitBoundary(t *testing.T) {
	const source = "| h |\n|---|\n" + `| $$\frac{a}{b}$$ |`
	for _, width := range []int{4, 5} { // two margin columns + 2 or 3 content columns
		r, err := NewTermRenderer(WithLatexMath(), WithWordWrap(width))
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(source)
		rows := strings.Split(trimRenderedRows(xansi.Strip(out)), "\n")
		if err != nil || len(rows) < 3 {
			t.Fatalf("width=%d: %q, %v", width, out, err)
		}
		for _, row := range rows {
			if xansi.StringWidth(row) > width {
				t.Fatalf("width=%d: overwide row %q", width, row)
			}
		}
		body := strings.Join(strings.Fields(strings.Join(rows[2:], "")), "")
		want := `$$\frac{a}{b}$$`
		if width == 5 {
			want = "a───b"
		}
		if body != want {
			t.Errorf("width=%d: body=%q, want %q", width, body, want)
		}
	}
}

func TestTableMathWithTabbedProse(t *testing.T) {
	const source = "| h | v |\n|---|---|\n| A\tB | " + `$$\frac{x}{y}$$ |`
	for _, tt := range []struct {
		width int
		want  string
	}{
		// With no width limit each column uses its natural width, including
		// the four spaces that Lip Gloss uses for a tab.
		{0, " h      │ v\n────────┼─────\n A    B │  x\n        │ ───\n        │  y"},
		// Every physical row keeps its separator, even when the prose wraps.
		{12, " h    │ v\n──────┼─────\n A    │  x\n B    │ ───\n      │  y"},
	} {
		r, err := NewTermRenderer(WithLatexMath(), WithWordWrap(tt.width))
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(source)
		if got := trimRenderedRows(xansi.Strip(out)); err != nil || got != tt.want {
			t.Errorf("width=%d: got\n%s\nwant\n%s\nerr: %v", tt.width, got, tt.want, err)
		}
	}
}

func TestTableMathWithExhaustedListWidth(t *testing.T) {
	// List indentation uses the whole budget. Zero remaining columns
	// must not be mistaken for an unconstrained table.
	r, err := NewTermRenderer(WithLatexMath(), WithStandardStyle(styles.DarkStyle), WithWordWrap(2))
	if err != nil {
		t.Fatal(err)
	}
	const equation = `$$\frac{x}{y}$$`
	out, err := r.Render("- item\n\n  | h |\n  |---|\n  | " + equation + " |")
	if err != nil || !strings.Contains(xansi.Strip(out), equation) {
		t.Errorf("expected source fallback, got %q, %v", out, err)
	}
}

func TestTableDisplayMathBreaksAtOperators(t *testing.T) {
	r, err := NewTermRenderer(WithLatexMath(), WithWordWrap(9), WithTableWrap(false))
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render("| h |\n|---|\n" + `| $$a+b+c+d$$ |`)
	rows := strings.Split(trimRenderedRows(xansi.Strip(out)), "\n")
	if err != nil || len(rows) < 4 {
		t.Fatalf("expected multiple typeset lines: %q, %v", out, err)
	}
	for _, row := range rows {
		if xansi.StringWidth(row) > 9 {
			t.Fatalf("overwide line: %q", row)
		}
	}
	if got := strings.Join(strings.Fields(strings.Join(rows[2:], "")), ""); got != "a+b+c+d" {
		t.Fatalf("equation lost during bounded typesetting: %q", rows)
	}
}

func TestTableMathLeavesLiteralCellsAlone(t *testing.T) {
	r, err := NewTermRenderer(WithLatexMath(), WithWordWrap(64))
	if err != nil {
		t.Fatal(err)
	}
	const source = "| formula | literal |\n|---|---|\n" +
		"| $$z$$ | `$$x^2$$` and $x^{2$ |"
	out, err := r.Render(source)
	if err != nil || !strings.Contains(xansi.Strip(out), `$$x^2$$ and $x^{2$`) {
		t.Errorf("literal or malformed math changed: %q, %v", out, err)
	}
}

func TestTableMatrixInList(t *testing.T) {
	const width = 40
	r, err := NewTermRenderer(WithLatexMath(), WithWordWrap(width), WithStandardStyle(styles.DarkStyle))
	if err != nil {
		t.Fatal(err)
	}
	const source = `- Matrices:

  | Matrix | Reference |
  | --- | --- |
  | $$\begin{pmatrix}\mathbf{x} & b \\ c & d\end{pmatrix}$$ | [notes](https://example.com) |`
	out, err := r.Render(source)
	if err != nil {
		t.Fatal(err)
	}
	plain := xansi.Strip(out)
	if !strings.Contains(out, "\x1b[1mx\x1b[22m") || !strings.Contains(plain, "[1]:") {
		t.Fatalf("missing native math face or table footer: %q", out)
	}
	seen := 0
	seam := -1
	for line := range strings.SplitSeq(plain, "\n") {
		if xansi.StringWidth(line) > width {
			t.Fatalf("list table exceeds width: %q", line)
		}
		if left, _, ok := strings.Cut(line, "│"); ok {
			if seam == -1 {
				seam = xansi.StringWidth(left)
			}
			if !strings.HasPrefix(left, "  ") || xansi.StringWidth(left) != seam {
				t.Fatalf("list indent or separator shifted:\n%s", plain)
			}
			seen++
		}
	}
	if seen < 3 || strings.Contains(plain, `\begin`) {
		t.Fatalf("matrix not laid out within its list cell:\n%s", plain)
	}
}

func TestDisplayMathInOrderedList(t *testing.T) {
	const source = `3. **Dirac Spinor:**
   Before the matrix:
   $$\Psi = \begin{pmatrix} \chi_L \\ \chi_R \end{pmatrix}$$
   Its 4 components account for **2 spin states** (up/down) $\times$ **2 particle/antiparticle states** (e.g., electron and positron).
4. **Majorana Spinor:**`
	for _, theme := range []string{styles.LightStyle, styles.DarkStyle} {
		r, err := NewTermRenderer(WithLatexMath(), WithStandardStyle(theme), WithWordWrap(48))
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(source)
		if err != nil {
			t.Fatal(err)
		}
		rows := strings.Split(trimRenderedRows(xansi.Strip(out)), "\n")
		var before, after, next int
		for i, row := range rows {
			switch strings.TrimSpace(row) {
			case "Before the matrix:":
				before = i
			case "4. Majorana Spinor:":
				next = i
			}
			if strings.HasPrefix(strings.TrimSpace(row), "Its 4 components") {
				after = i
			}
		}
		if before == 0 || after <= before+3 || next <= after+1 || rows[before+1] != "" || rows[after-1] != "" {
			t.Fatalf("%s: missing matrix spacing or wrapped prose:\n%s", theme, strings.Join(rows, "\n"))
		}
		column := strings.Index(rows[before], "Before")
		for _, row := range rows[after:next] {
			if len(row)-len(strings.TrimLeft(row, " ")) != column {
				t.Errorf("%s: expected indent %d on %q", theme, column, row)
			}
		}
	}
}

func TestPromotedMathPreservesLineBoundaries(t *testing.T) {
	r, err := NewTermRenderer(WithLatexMath(), WithPreservedNewLines())
	if err != nil {
		t.Fatal(err)
	}
	const frac = `$\dfrac{a}{b}$`
	const grid = " a\n───\n b"
	for _, sep := range []string{"\n", "\n\n"} {
		out, err := r.Render("Before" + sep + frac + sep + "After")
		if got, want := trimRenderedRows(xansi.Strip(out)), "Before\n\n"+grid+"\n\nAfter"; err != nil || got != want {
			t.Errorf("separator %q: got %q, %v; want %q", sep, got, err, want)
		}
	}
}

func TestLatexMathNativeFaces(t *testing.T) {
	const input = "**before $\\mathbf{x}y$ after**\n\n$$\\sout{z}$$"
	for _, theme := range []string{styles.DarkStyle, styles.NoTTYStyle} {
		r, err := NewTermRenderer(WithLatexMath(), WithStandardStyle(theme))
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(input)
		if err != nil {
			t.Fatal(err)
		}
		if theme == styles.DarkStyle {
			for _, span := range []string{"\x1b[1mx\x1b[22m\x1b[1my", "\x1b[9mz\x1b[29m"} {
				if !strings.Contains(out, span) {
					t.Errorf("missing %q in %q", span, out)
				}
			}
		} else if strings.Contains(out, "\x1b[") || !strings.Contains(out, "𝐱y") {
			t.Errorf("unexpected plain output: %q", out)
		}
	}
}

func TestWithLatexMathPreservesIncompleteSource(t *testing.T) {
	r, err := NewTermRenderer(WithLatexMath())
	if err != nil {
		t.Fatal(err)
	}

	for _, source := range []string{
		"Streaming \\(x_*",
		"Closing \\) and \\] delimiters",
		"\\[\nE_* = mc^2",
		"Streaming $x_*",
		"$$\nF_* = ma",
	} {
		rendered, err := r.Render(source)
		if err != nil {
			t.Fatal(err)
		}
		if got := trimRenderedRows(xansi.Strip(rendered)); got != source {
			t.Errorf("rendered incomplete LaTeX math = %q, want %q", got, source)
		}
	}
}

func TestWithLatexMathLeavesOrdinaryDollarsUnchanged(t *testing.T) {
	r, err := NewTermRenderer(WithLatexMath())
	if err != nil {
		t.Fatal(err)
	}

	source := "Costs $10 and $20. Shell $HOME. Math $x + 1$."
	rendered, err := r.Render(source)
	if err != nil {
		t.Fatal(err)
	}
	want := "Costs $10 and $20. Shell $HOME. Math x + 1."
	if got := trimRenderedRows(xansi.Strip(rendered)); got != want {
		t.Fatalf("rendered dollar text = %q, want %q", got, want)
	}
}

func trimRenderedRows(value string) string {
	lines := strings.Split(strings.Trim(value, "\r\n"), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func TestStyles(t *testing.T) {
	_, err := NewTermRenderer()
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewTermRenderer(
		WithStandardStyle(styles.DarkStyle),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewTermRenderer(
		WithEnvironmentConfig(),
	)
	if err != nil {
		t.Fatal(err)
	}
}

// TestCustomStyle checks the expected errors with custom styling. We need to
// support built-in styles and custom style sheets.
func TestCustomStyle(t *testing.T) {
	md := "testdata/example.md"
	tests := []struct {
		name      string
		stylePath string
		err       error
		expected  string
	}{
		{name: "style exists", stylePath: "testdata/custom.style", err: nil, expected: "testdata/custom.style"},
		{name: "style doesn't exist", stylePath: "testdata/notfound.style", err: os.ErrNotExist, expected: styles.DarkStyle},
		{name: "style is empty", stylePath: "", err: nil, expected: styles.DarkStyle},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GLAMOUR_STYLE", tc.stylePath)
			g, err := NewTermRenderer(
				WithEnvironmentConfig(),
			)
			if !errors.Is(err, tc.err) {
				t.Fatal(err)
			}
			if !errors.Is(tc.err, os.ErrNotExist) {
				w, err := NewTermRenderer(WithStylePath(tc.expected))
				if err != nil {
					t.Fatal(err)
				}
				text, _ := os.ReadFile(md)
				want, err := w.RenderBytes(text)
				got, err := g.RenderBytes(text)
				if !bytes.Equal(want, got) {
					t.Error("Wrong style used")
				}
			}
		})
	}
}

func TestRenderHelpers(t *testing.T) {
	in, err := os.ReadFile(markdown)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Render(string(in), "dark")
	if err != nil {
		t.Error(err)
	}

	golden.RequireEqual(t, []byte(b))
}

func TestCapitalization(t *testing.T) {
	p := true
	style := styles.DarkStyleConfig
	style.H1.Upper = &p
	style.H2.Title = &p
	style.H3.Lower = &p

	r, err := NewTermRenderer(
		WithStyles(style),
	)
	if err != nil {
		t.Fatal(err)
	}

	b, err := r.Render("# everything is uppercase\n## everything is titled\n### everything is lowercase")
	if err != nil {
		t.Fatal(err)
	}

	golden.RequireEqual(t, []byte(b))
}

func FuzzData(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		func() int {
			_, err := RenderBytes(data, styles.DarkStyle)
			if err != nil {
				return 0
			}
			return 1
		}()
	})
}

func TestTableAscii(t *testing.T) {
	markdown := strings.TrimSpace(`
| Header A  | Header B  |
| --------- | --------- |
| Cell 1    | Cell 2    |
| Cell 3    | Cell 4    |
| Cell 5    | Cell 6    |
`)

	renderer, err := NewTermRenderer(
		WithStyles(styles.ASCIIStyleConfig),
		WithWordWrap(80),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := renderer.Render(markdown)
	if err != nil {
		t.Fatal(err)
	}

	nonAsciiRegexp := regexp.MustCompile(`[^\x00-\x7f]+`)
	nonAsciiChars := nonAsciiRegexp.FindAllString(result, -1)
	if len(nonAsciiChars) > 0 {
		t.Errorf("Non-ASCII characters found in output: %v", nonAsciiChars)
	}
}

func ExampleASCIIStyleConfig() {
	markdown := strings.TrimSpace(`
| Header A  | Header B  |
| --------- | --------- |
| Cell 1    | Cell 2    |
| Cell 3    | Cell 4    |
| Cell 5    | Cell 6    |
`)

	renderer, err := NewTermRenderer(
		WithStyles(styles.ASCIIStyleConfig),
		WithWordWrap(80),
	)
	if err != nil {
		return
	}

	result, err := renderer.Render(markdown)
	if err != nil {
		return
	}
	result = strings.ReplaceAll(result, " ", ".")
	fmt.Println(result)

	// Output:
	// ..............................................................................
	// ...Header.A.............................|.Header.B............................
	// ..--------------------------------------|-------------------------------------
	// ...Cell.1...............................|.Cell.2..............................
	// ...Cell.3...............................|.Cell.4..............................
	// ...Cell.5...............................|.Cell.6..............................
}

func TestWithChromaFormatterDefault(t *testing.T) {
	r, err := NewTermRenderer(
		WithStandardStyle(styles.DarkStyle),
	)
	if err != nil {
		t.Fatal(err)
	}

	in, err := os.ReadFile("testdata/TestWithChromaFormatter.md")
	if err != nil {
		t.Fatal(err)
	}

	b, err := r.Render(string(in))
	if err != nil {
		t.Fatal(err)
	}

	golden.RequireEqual(t, []byte(b))
}

func TestWithChromaFormatterCustom(t *testing.T) {
	r, err := NewTermRenderer(
		WithStandardStyle(styles.DarkStyle),
		WithChromaFormatter("terminal16"),
	)
	if err != nil {
		t.Fatal(err)
	}

	in, err := os.ReadFile("testdata/TestWithChromaFormatter.md")
	if err != nil {
		t.Fatal(err)
	}

	b, err := r.Render(string(in))
	if err != nil {
		t.Fatal(err)
	}

	golden.RequireEqual(t, []byte(b))
}

func TestWithConcreteChromaStyle(t *testing.T) {
	const styleName = "glamour-concrete-test"
	style := chroma.MustNewStyle(styleName, chroma.StyleEntries{
		chroma.Text:       "#010203",
		chroma.Keyword:    "#040506",
		chroma.Background: "bg:#070809",
	})
	if _, registered := chromastyles.Registry[styleName]; registered {
		t.Fatalf("test style %q is already globally registered", styleName)
	}

	renderer, err := NewTermRenderer(
		WithStandardStyle(styles.DarkStyle),
		WithChromaFormatter("terminal16m"),
		WithChromaStyle(style),
	)
	if err != nil {
		t.Fatalf("create renderer: %v", err)
	}
	rendered, err := renderer.Render("```go\npackage main\n```\n\n```\nplain text\n```")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for name, sequence := range map[string]string{
		"keyword": "\x1b[38;2;4;5;6m",
		"text":    "\x1b[38;2;1;2;3m",
	} {
		if !strings.Contains(rendered, sequence) {
			t.Fatalf("rendered code missing concrete %s style %q:\n%q", name, sequence, rendered)
		}
	}
	if _, registered := chromastyles.Registry[styleName]; registered {
		t.Fatalf("concrete style %q was added to the global registry", styleName)
	}
}

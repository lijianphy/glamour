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
	out, err := r.Render("| value | next |\n|---|---|\n| " + frac + " | $x^2$ |")
	if err != nil || !strings.Contains(out, frac) || !strings.Contains(out, "x²") {
		t.Errorf("table must retain tall source and render one-line math: %q, %v", out, err)
	}
	wide := `$\dfrac{\text{` + strings.Repeat("a", 45) + `}}{b}$`
	out, err = r.Render(wide)
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

package ansi

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

const (
	examplesDir = "../styles/examples/"
	issuesDir   = "../testdata/issues/"
)

func TestRenderer(t *testing.T) {
	files, err := filepath.Glob(examplesDir + "*.md")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		bn := strings.TrimSuffix(filepath.Base(f), ".md")
		t.Run(bn, func(t *testing.T) {
			sn := filepath.Join(examplesDir, bn+".style")

			in, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(sn)
			if err != nil {
				t.Fatal(err)
			}

			options := Options{
				WordWrap: 80,
			}
			err = json.Unmarshal(b, &options.Styles)
			if err != nil {
				t.Fatal(err)
			}

			switch bn {
			case "table_wrap":
				tableWrap := true
				options.TableWrap = &tableWrap
			case "table_truncate":
				tableWrap := false
				options.TableWrap = &tableWrap
			case "table_with_inline_links":
				options.InlineTableLinks = true
			case "table_with_footer_links", "table_with_footer_links_no_color":
				options.InlineTableLinks = false
			}

			md := goldmark.New(
				goldmark.WithExtensions(
					extension.GFM,
					extension.DefinitionList,
					emoji.Emoji,
				),
				goldmark.WithParserOptions(
					parser.WithAutoHeadingID(),
				),
			)

			ar := NewRenderer(options)
			md.SetRenderer(
				renderer.NewRenderer(
					renderer.WithNodeRenderers(util.Prioritized(ar, 1000))))

			var buf bytes.Buffer
			if err := md.Convert(in, &buf); err != nil {
				t.Error(err)
			}

			golden.RequireEqual(t, buf.Bytes())
		})
	}
}

func TestRendererListItemsUseHangingIndent(t *testing.T) {
	indent := uint(2)
	options := Options{
		WordWrap: 54,
		Styles: StyleConfig{
			List: StyleList{
				StyleBlock: StyleBlock{
					Indent: &indent,
				},
				LevelIndent: 2,
			},
			Item:        StylePrimitive{BlockPrefix: "• "},
			Enumeration: StylePrimitive{BlockPrefix: ". "},
			Task: StyleTask{
				Ticked:   "[x] ",
				Unticked: "[ ] ",
			},
		},
	}
	source := strings.Join([]string{
		"1. Capture: Both stdout and stderr are written to a single temp file.",
		"2. Truncation: Output is read back and truncated to 2000 lines or 50000 Unicode code points, whichever is hit first. If truncated, the full output is saved to `.sciagent/batch_shell_output_timestamp_rand.txt` and a system message points you to it.",
		"3. Streaming deltas: During execution, output is batched.",
		"",
		"- Prepare `run.in`",
		"  - Capture stdout",
		"  - Save stderr",
	}, "\n")

	got := renderMarkdownForTest(t, source, options)
	stripped := xansi.Strip(got)
	for _, want := range []string{"  1. Capture", "  2. Truncation", "  3. Streaming", "  • Prepare", "    • Capture", "    • Save"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("rendered list missing %q:\n%s", want, stripped)
		}
	}

	inSecondItem := false
	continuations := 0
	for line := range strings.SplitSeq(stripped, "\n") {
		if width := xansi.StringWidth(line); width > options.WordWrap {
			t.Fatalf("rendered line width = %d, want <= %d: %q\n%s", width, options.WordWrap, line, stripped)
		}
		switch {
		case strings.HasPrefix(line, "  2. "):
			inSecondItem = true
			continue
		case strings.HasPrefix(line, "  3. "):
			inSecondItem = false
			continue
		}
		if !inSecondItem || strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "     ") {
			t.Fatalf("list continuation line is not aligned under item text: %q\n%s", line, stripped)
		}
		continuations++
	}
	if continuations == 0 {
		t.Fatalf("test did not produce list continuation lines:\n%s", stripped)
	}
}

func TestRendererCodeBlockLongLinesWrapInsideBlockIndent(t *testing.T) {
	margin := uint(2)
	background := "#272822"
	options := Options{
		WordWrap: 30,
		Styles: StyleConfig{
			CodeBlock: StyleCodeBlock{
				StyleBlock: StyleBlock{
					StylePrimitive: StylePrimitive{
						BackgroundColor: &background,
					},
					Margin: &margin,
				},
				Theme: "monokai",
			},
		},
	}
	source := "```python\nprint(\"" + strings.Repeat("x", 60) + "\")\n```"

	got := renderMarkdownForTest(t, source, options)
	stripped := xansi.Strip(got)
	nonBlank := 0
	for line := range strings.SplitSeq(stripped, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		nonBlank++
		if width := xansi.StringWidth(line); width > options.WordWrap {
			t.Fatalf("code block line width = %d, want <= %d: %q\n%s", width, options.WordWrap, line, stripped)
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("wrapped code block line lost block indent: %q\n%s", line, stripped)
		}
	}
	if nonBlank < 2 {
		t.Fatalf("test did not produce wrapped code rows:\n%s", stripped)
	}
}

func renderMarkdownForTest(t *testing.T, source string, options Options) string {
	t.Helper()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.DefinitionList,
			emoji.Emoji,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	ar := NewRenderer(options)
	md.SetRenderer(
		renderer.NewRenderer(
			renderer.WithNodeRenderers(util.Prioritized(ar, 1000))))

	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRendererIssues(t *testing.T) {
	files, err := filepath.Glob(issuesDir + "*.md")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		bn := strings.TrimSuffix(filepath.Base(f), ".md")
		t.Run(bn, func(t *testing.T) {
			in, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile("../styles/dark.json")
			if err != nil {
				t.Fatal(err)
			}

			options := Options{
				WordWrap: 80,
			}
			err = json.Unmarshal(b, &options.Styles)
			if err != nil {
				t.Fatal(err)
			}
			if bn == "493" {
				tableWrap := false
				options.TableWrap = &tableWrap
			}

			md := goldmark.New(
				goldmark.WithExtensions(
					extension.GFM,
					extension.DefinitionList,
					emoji.Emoji,
				),
				goldmark.WithParserOptions(
					parser.WithAutoHeadingID(),
				),
			)

			ar := NewRenderer(options)
			md.SetRenderer(
				renderer.NewRenderer(
					renderer.WithNodeRenderers(util.Prioritized(ar, 1000))))

			var buf bytes.Buffer
			if err := md.Convert(in, &buf); err != nil {
				t.Error(err)
			}

			golden.RequireEqual(t, buf.Bytes())
		})
	}
}

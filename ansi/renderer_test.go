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
	for _, want := range []string{"  1. Capture", "  2. Truncation", "  3. Streaming", "  • Prepare", "      • Capture", "      • Save"} {
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

func TestRendererNestedListsAlignToParentContentColumn(t *testing.T) {
	indent := uint(2)
	options := Options{
		WordWrap: 80,
		Styles: StyleConfig{
			List: StyleList{
				StyleBlock: StyleBlock{
					Indent: &indent,
				},
				LevelIndent: 0,
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
		"- [ ] Prepare simulation inputs",
		"  - create `run.in`",
		"  - create `nep.in`",
		"- [x] Validate output",
		"  - check `thermo.out`",
		"",
		"9. Nine parent",
		"   - child nine",
		"10. Ten parent",
		"    - child ten",
	}, "\n")

	got := renderMarkdownForTest(t, source, options)
	stripped := xansi.Strip(got)
	for _, want := range []string{
		"  [ ] Prepare simulation inputs",
		"      • create run.in",
		"      • create nep.in",
		"  [x] Validate output",
		"      • check thermo.out",
		"  9. Nine parent",
		"     • child nine",
		"  10. Ten parent",
		"      • child ten",
	} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("rendered nested list missing %q:\n%s", want, stripped)
		}
	}
}

func TestRendererNestedListBlocksUseListLevelIndent(t *testing.T) {
	indent := uint(2)
	codeMargin := uint(5)
	levelIndent := uint(0)
	tableMargin := uint(0)
	quoteIndent := uint(1)
	quoteToken := "> "
	options := Options{
		WordWrap: 60,
		Styles: StyleConfig{
			BlockQuote: StyleBlock{
				Indent:      &quoteIndent,
				IndentToken: &quoteToken,
			},
			List: StyleList{
				StyleBlock: StyleBlock{
					Indent: &indent,
				},
				LevelIndent: levelIndent,
			},
			Item:        StylePrimitive{BlockPrefix: "- "},
			Enumeration: StylePrimitive{BlockPrefix: ". "},
			Task: StyleTask{
				Ticked:   "[x] ",
				Unticked: "[ ] ",
			},
			CodeBlock: StyleCodeBlock{
				StyleBlock: StyleBlock{
					Margin: &codeMargin,
				},
			},
			Table: StyleTable{
				StyleBlock: StyleBlock{
					Margin: &tableMargin,
				},
			},
		},
	}
	source := strings.Join([]string{
		"- [ ] Parent",
		"  - Child",
		"",
		"    ```sh",
		"    echo hi",
		"    ```",
		"",
		"    | A | B |",
		"    | - | - |",
		"    | x | y |",
		"",
		"    > Quote",
	}, "\n")

	got := renderMarkdownForTest(t, source, options)
	stripped := xansi.Strip(got)
	for _, want := range []string{
		"  [ ] Parent",
		"      - Child",
	} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("rendered nested block fixture missing %q:\n%s", want, stripped)
		}
	}
	for line := range strings.SplitSeq(stripped, "\n") {
		if width := xansi.StringWidth(line); width > options.WordWrap {
			t.Fatalf("rendered line width = %d, want <= %d: %q\n%s", width, options.WordWrap, line, stripped)
		}
	}

	assertLineIndent := func(contains string, want int) {
		t.Helper()
		for line := range strings.SplitSeq(stripped, "\n") {
			if strings.Contains(line, contains) {
				if got := leadingSpaceWidth(line); got != want {
					t.Fatalf("line %q indent = %d, want %d\n%s", contains, got, want, stripped)
				}
				return
			}
		}
		t.Fatalf("rendered nested block fixture missing line containing %q:\n%s", contains, stripped)
	}
	assertLineIndent("echo hi", 8)
	assertLineIndent("┼", 8)
	assertLineIndent("> Quote", 8)
}

func TestRendererNestedListBlocksHonorNonzeroListLevelIndent(t *testing.T) {
	indent := uint(2)
	codeMargin := uint(5)
	levelIndent := uint(2)
	tableMargin := uint(0)
	quoteIndent := uint(1)
	quoteToken := "> "
	options := Options{
		WordWrap: 80,
		Styles: StyleConfig{
			BlockQuote: StyleBlock{
				Indent:      &quoteIndent,
				IndentToken: &quoteToken,
			},
			List: StyleList{
				StyleBlock: StyleBlock{
					Indent: &indent,
				},
				LevelIndent: levelIndent,
			},
			Item: StylePrimitive{BlockPrefix: "- "},
			Task: StyleTask{
				Ticked:   "[x] ",
				Unticked: "[ ] ",
			},
			CodeBlock: StyleCodeBlock{
				StyleBlock: StyleBlock{
					Margin: &codeMargin,
				},
			},
			Table: StyleTable{
				StyleBlock: StyleBlock{
					Margin: &tableMargin,
				},
			},
		},
	}
	source := strings.Join([]string{
		"- [ ] Parent",
		"  - Child",
		"",
		"    ```sh",
		"    echo hi",
		"    ```",
		"",
		"    | A | B |",
		"    | - | - |",
		"    | x | y |",
		"",
		"    > Quote",
	}, "\n")

	got := renderMarkdownForTest(t, source, options)
	stripped := xansi.Strip(got)
	if !strings.Contains(stripped, "        - Child") {
		t.Fatalf("rendered child list did not honor level indent:\n%s", stripped)
	}
	for line := range strings.SplitSeq(stripped, "\n") {
		if width := xansi.StringWidth(line); width > options.WordWrap {
			t.Fatalf("rendered line width = %d, want <= %d: %q\n%s", width, options.WordWrap, line, stripped)
		}
	}

	assertLineIndent := func(contains string, want int) {
		t.Helper()
		for line := range strings.SplitSeq(stripped, "\n") {
			if strings.Contains(line, contains) {
				if got := leadingSpaceWidth(line); got != want {
					t.Fatalf("line %q indent = %d, want %d\n%s", contains, got, want, stripped)
				}
				return
			}
		}
		t.Fatalf("rendered nested block fixture missing line containing %q:\n%s", contains, stripped)
	}
	assertLineIndent("echo hi", 12)
	assertLineIndent("┼", 12)
	assertLineIndent("> Quote", 12)
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

func TestRendererCodeBlockLongLinesHardWrap(t *testing.T) {
	options := Options{
		WordWrap: 20,
		Styles: StyleConfig{
			CodeBlock: StyleCodeBlock{
				Theme: "monokai",
			},
		},
	}
	source := "```sh\necho alpha beta gamma delta\n```"

	stripped := xansi.Strip(renderMarkdownForTest(t, source, options))
	for line := range strings.SplitSeq(stripped, "\n") {
		if strings.Contains(line, "echo alpha") {
			if got := strings.TrimRight(line, " "); got != "echo alpha beta gamm" {
				t.Fatalf("first code row = %q, want hard wrap:\n%s", got, stripped)
			}
			return
		}
	}
	t.Fatalf("rendered code is missing first row:\n%s", stripped)
}

func TestRendererListTableAvoidsSilentRightEdgeClipping(t *testing.T) {
	options := listTableOptions(50)
	source := strings.Join([]string{
		"2. **Step 2: Convergence tests**",
		"",
		"   | Parameter  | Test range      | Chosen value |",
		"   |------------|-----------------|-------------:|",
		"   | ENCUT      | 400-800 eV      |       520 eV |",
		"   | KPOINTS    | 2x2x2 - 12x12x12 |      8x8x8  |",
		"   | SIGMA      | 0.01-0.2 eV     |       0.05 eV |",
	}, "\n")

	stripped := xansi.Strip(renderMarkdownForTest(t, source, options))
	for _, unwanted := range []string{"Chosen valu\n", "520 e\n", "8x8x\n", "0.05 e\n"} {
		if strings.Contains(stripped, unwanted) {
			t.Fatalf("list table silently clipped %q:\n%s", unwanted, stripped)
		}
	}
	for _, want := range []string{"Chosen value", "520 eV", "8x8x8", "12x12x12", "0.05 eV"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("list table render missing %q:\n%s", want, stripped)
		}
	}
}

func TestRendererWideListTablesDoNotInsertBlankRows(t *testing.T) {
	options := listTableOptions(120)
	source := strings.Join([]string{
		"- **MD Run #1 (NVT equilibration)**",
		"  ",
		"  | Phase | Steps | Ensemble |",
		"  |-------|-------|----------|",
		"  | Heat | 10000 | NVT |",
		"  | Equil | 50000 | NVT |",
		"  | Sample | 100000 | NVE |",
		"",
		"- **MD Run #2 (NPT production)**",
		"  ",
		"  | Phase | Steps | Ensemble |",
		"  |-------|-------|----------|",
		"  | Press | 50000 | NPT |",
		"  | Sample | 200000 | NPT |",
	}, "\n")

	stripped := xansi.Strip(renderMarkdownForTest(t, source, options))
	if strings.Contains(stripped, "Ensemble\n\n") {
		t.Fatalf("wide list table inserted a blank row between header and separator:\n%s", stripped)
	}
	for _, want := range []string{"Sample", "100000", "200000", "NVE", "NPT"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("wide list table render missing %q:\n%s", want, stripped)
		}
	}
}

func listTableOptions(width int) Options {
	return Options{
		WordWrap: width,
		Styles: StyleConfig{
			Item:        StylePrimitive{BlockPrefix: "• "},
			Enumeration: StylePrimitive{BlockPrefix: ". "},
			Task: StyleTask{
				Ticked:   "[x] ",
				Unticked: "[ ] ",
			},
		},
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

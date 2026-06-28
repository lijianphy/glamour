package ansi

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var (
	benchmarkMargin uint = 1
	benchmarkIndent uint = 2
)

const benchmarkParagraph = "The quick brown fox jumps over the lazy dog while ANSI-aware wrapping, indentation, and padding keep terminal output stable.\n" +
	"A second line includes punctuation, commas, semicolons; and break-points for the wrapper.\n"

const benchmarkGoCode = `package main

import "fmt"

func main() {
	for i := 0; i < 24; i++ {
		fmt.Printf("value=%d with a very long code line that wraps repeatedly inside a fixed width terminal window\n", i)
	}
}
`

func benchmarkContext() RenderContext {
	return NewRenderContext(Options{
		WordWrap: 80,
		Styles: StyleConfig{
			Document: StyleBlock{},
			CodeBlock: StyleCodeBlock{
				StyleBlock: StyleBlock{
					Margin: &benchmarkMargin,
				},
				Theme: "monokai",
			},
		},
	})
}

func BenchmarkBlockElementFinish(b *testing.B) {
	style := StyleBlock{Margin: &benchmarkMargin}
	source := strings.Repeat(benchmarkParagraph, 8)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		ctx := benchmarkContext()
		element := BlockElement{
			Block:  &bytes.Buffer{},
			Style:  style,
			Margin: true,
		}
		ctx.blockStack.Push(element)
		ctx.blockStack.Current().Block.WriteString(source)
		if err := element.Finish(io.Discard, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarginWriter(b *testing.B) {
	rules := StyleBlock{Margin: &benchmarkMargin, Indent: &benchmarkIndent}
	source := strings.Repeat(benchmarkParagraph, 16)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		ctx := benchmarkContext()
		ctx.blockStack.Push(BlockElement{Block: &bytes.Buffer{}, Style: StyleBlock{}})
		var out bytes.Buffer
		w := NewMarginWriter(ctx, &out, rules)
		if _, err := io.WriteString(w, source); err != nil {
			b.Fatal(err)
		}
		if err := w.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIndentWriter(b *testing.B) {
	source := strings.Repeat("alpha beta gamma\n", 128)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		w := NewIndentWriter(&out, 4, nil)
		if _, err := io.WriteString(w, source); err != nil {
			b.Fatal(err)
		}
		if err := w.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPaddingWriter(b *testing.B) {
	source := strings.Repeat("\x1b[38;5;42mwide 世界 text\x1b[m\n", 128)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		w := NewPaddingWriter(&out, 80, nil)
		if _, err := io.WriteString(w, source); err != nil {
			b.Fatal(err)
		}
		if err := w.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHighlightedCodeBlockLongLines(b *testing.B) {
	ctx := benchmarkContext()
	ctx.blockStack.Push(BlockElement{Block: &bytes.Buffer{}, Style: StyleBlock{}})
	element := &CodeBlockElement{Code: benchmarkGoCode, Language: "go"}
	b.SetBytes(int64(len(benchmarkGoCode)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		if err := element.Render(&out, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlainCodeBlock(b *testing.B) {
	ctx := benchmarkContext()
	ctx.blockStack.Push(BlockElement{Block: &bytes.Buffer{}, Style: StyleBlock{}})
	source := strings.Repeat("plain code block with no language and a long line that still wraps inside the block\n", 12)
	element := &CodeBlockElement{Code: source}
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		if err := element.Render(&out, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNestedCodeBlocks(b *testing.B) {
	source := `- item before
  ` + "```go" + `
  fmt.Println("list nested code block with a long enough line to wrap and pad")
  ` + "```" + `

> quote before
> ` + "```text" + `
> quoted plain code block
> ` + "```" + `
`
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.DefinitionList),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	ar := NewRenderer(benchmarkContext().options)
	md.SetRenderer(renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(ar, 1000))))
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		if err := md.Convert([]byte(source), &out); err != nil {
			b.Fatal(err)
		}
	}
}

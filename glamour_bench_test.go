package glamour

import (
	"strings"
	"testing"
)

const benchmarkMarkdownDocument = `
# Glamour benchmark

This fixture covers **emphasis**, _italics_, [links](https://example.com/a/very/long/path?with=query), inline ` + "`code`" + `,
and enough prose to exercise word wrapping over several terminal rows. The quick brown fox jumps over the lazy dog
while a second sentence keeps the paragraph long enough for repeated wrapping work.

> A blockquote with a nested list:
>
> - quoted item one with enough text to wrap across rows
> - quoted item two
>
> ` + "```go" + `
> fmt.Println("quoted code")
> ` + "```" + `

1. ordered item with a long continuation line that should keep hanging indentation stable across wraps
2. another item
   - nested item
   - nested item with more words and punctuation, semicolons; commas, and pipes |

| Parameter | Range | Chosen value |
| --- | ---: | --- |
| alpha | 0.0 - 1.0 | 0.25 |
| beta | 100 - 200 | 160 |
| gamma | long descriptive value | wraps in the table cell |

` + "```go" + `
package main

import "fmt"

func main() {
	for i := 0; i < 12; i++ {
		fmt.Printf("value=%d and a long tail that should wrap inside the code block\n", i)
	}
}
` + "```" + `

` + "```text" + `
plain text code block with no lexer work
and a second line with tabs    already expanded by the renderer
` + "```" + `
`

func BenchmarkMarkdownDocumentRender(b *testing.B) {
	renderer, err := NewTermRenderer(WithStandardStyle("dark"), WithWordWrap(80))
	if err != nil {
		b.Fatal(err)
	}
	source := []byte(benchmarkMarkdownDocument)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := renderer.RenderBytes(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownStreamingPrefixRender(b *testing.B) {
	renderer, err := NewTermRenderer(WithStandardStyle("dark"), WithWordWrap(80))
	if err != nil {
		b.Fatal(err)
	}
	source := []byte(strings.Repeat(benchmarkMarkdownDocument, 3))
	prefixes := make([][]byte, 0, 96)
	for end := 256; end < len(source); end += 256 {
		prefixes = append(prefixes, source[:end])
	}
	prefixes = append(prefixes, source)
	b.SetBytes(int64(len(source) * len(prefixes)))
	b.ReportAllocs()
	for b.Loop() {
		for _, prefix := range prefixes {
			if _, err := renderer.RenderBytes(prefix); err != nil {
				b.Fatal(err)
			}
		}
	}
}

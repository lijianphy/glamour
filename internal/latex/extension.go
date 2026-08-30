package latex

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

type extension struct{}

// Extension adds LaTeX math parsing to Goldmark. Glamour's ANSI renderer owns
// rendering for the resulting nodes.
var Extension = &extension{}

// Extend implements goldmark.Extender.
func (e *extension) Extend(markdown goldmark.Markdown) {
	markdown.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(newBlockParser(), 650),
		),
		parser.WithInlineParsers(
			util.Prioritized(newInlineParser(), 50),
		),
	)
}

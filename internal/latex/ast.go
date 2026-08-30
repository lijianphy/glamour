// Package latex provides Goldmark nodes and parsers for LaTeX math delimiters.
package latex

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// KindInlineMath is the node kind for inline LaTeX math.
var KindInlineMath = ast.NewNodeKind("InlineMath")

// InlineMath is raw LaTeX delimited by \( and \), or an individual math
// delimiter retained while an expression is still being streamed.
type InlineMath struct {
	ast.BaseInline

	Segment text.Segment
	Display bool
}

// Kind implements ast.Node.
func (n *InlineMath) Kind() ast.NodeKind {
	return KindInlineMath
}

// IsRaw prevents further Markdown parsing within the expression.
func (n *InlineMath) IsRaw() bool {
	return true
}

// Text returns the original delimited LaTeX source.
func (n *InlineMath) Text(source []byte) []byte {
	return n.Segment.Value(source)
}

// Dump implements ast.Node.
func (n *InlineMath) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Text": string(n.Text(source)),
	}, nil)
}

// NewInlineMath returns an inline math node for a source segment.
func NewInlineMath(segment text.Segment, display bool) *InlineMath {
	return &InlineMath{
		Segment: segment,
		Display: display,
	}
}

// KindMathBlock is the node kind for display LaTeX math.
var KindMathBlock = ast.NewNodeKind("MathBlock")

// MathBlock is raw, multiline LaTeX delimited by standalone \[ and \] lines.
type MathBlock struct {
	ast.BaseBlock

	Indent int
}

// Kind implements ast.Node.
func (n *MathBlock) Kind() ast.NodeKind {
	return KindMathBlock
}

// IsRaw prevents inline Markdown parsing within the expression.
func (n *MathBlock) IsRaw() bool {
	return true
}

// Text returns the original delimited LaTeX source.
func (n *MathBlock) Text(source []byte) []byte {
	return n.Lines().Value(source)
}

// Dump implements ast.Node.
func (n *MathBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Text": string(n.Text(source)),
	}, nil)
}

// NewMathBlock returns an empty display-math block.
func NewMathBlock(indent int) *MathBlock {
	return &MathBlock{Indent: indent}
}

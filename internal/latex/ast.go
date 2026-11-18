// Package latex provides Goldmark nodes and parsers for LaTeX math delimiters.
package latex

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// KindInlineMath is the node kind for inline LaTeX math.
var KindInlineMath = ast.NewNodeKind("InlineMath")

// InlineMath is raw delimited LaTeX.
type InlineMath struct {
	ast.BaseInline

	segment      text.Segment
	delimiterLen int // zero for incomplete expressions and standalone closers
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
	return n.segment.Value(source)
}

// Dump implements ast.Node.
func (n *InlineMath) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Text": string(n.Text(source)),
	}, nil)
}

// Body returns the delimiter-free expression, or nil until it is closed.
func (n *InlineMath) Body(source []byte) []byte {
	if n.delimiterLen == 0 {
		return nil
	}
	raw := n.Text(source)
	return raw[n.delimiterLen : len(raw)-n.delimiterLen]
}

func newInlineMath(segment text.Segment, delimiterLen int) *InlineMath {
	return &InlineMath{segment: segment, delimiterLen: delimiterLen}
}

// KindMathBlock is the node kind for display LaTeX math.
var KindMathBlock = ast.NewNodeKind("MathBlock")

// MathBlock is raw display LaTeX delimited by standalone \[ and \] or $$ lines.
type MathBlock struct {
	ast.BaseBlock
	// Closed reports whether the matching closing delimiter was consumed.
	Closed bool
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

func newMathBlock() *MathBlock {
	return &MathBlock{}
}

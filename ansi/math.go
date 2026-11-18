package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/doug/termtex"
)

// MathBlockElement renders display LaTeX as width-bounded terminal text.
type MathBlockElement struct {
	Text   string
	Style  StyleBlock
	closed bool
}

// Render renders a MathBlockElement.
func (e *MathBlockElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	indentation := uintValue(e.Style.Indent)
	margin := uintValue(e.Style.Margin)
	layout := childBlockLayout(ctx, indentation, margin)

	var target io.Writer
	var blockBuffer bytes.Buffer
	var iw *IndentWriter
	if layout.inList {
		target = &blockBuffer
	} else {
		iw = NewIndentWriter(w, layout.indent, func(_ io.Writer) {
			_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
		})
		target = iw
	}

	_, _ = renderText(target, bs.Current().Style.StylePrimitive, e.Style.BlockPrefix)
	value := e.Text
	if e.closed {
		// Both block fence forms have two bytes at each end.
		raw := strings.TrimSpace(value)
		value = renderMath(raw[2:len(raw)-2], value, false, layout.width)
	}
	if value != "" && !strings.HasSuffix(value, "\n") {
		value += "\n"
	}
	element := &BaseElement{
		Token: wrapCodeBlockLines(value, layout.width),
		Style: e.Style.StylePrimitive,
		Raw:   true,
	}
	if err := element.Render(target, ctx); err != nil {
		return fmt.Errorf("glamour: error rendering math block: %w", err)
	}
	_, _ = renderText(target, bs.Current().Style.StylePrimitive, e.Style.BlockSuffix)
	return finishListChildBlock(w, blockBuffer.String(), layout, iw)
}

func renderMath(body, raw string, inline bool, width int) string {
	out, err := termtex.Render(body, termtex.Style{Inline: inline, Width: width})
	if err != nil || out == "" || (inline && strings.ContainsRune(out, '\n')) {
		return raw
	}
	// Termtex breaks at operators, but an indivisible grid can still be
	// too wide. Wrap its source rather than tearing the diagram apart.
	if width > 0 {
		for line := range strings.SplitSeq(out, "\n") {
			if xansi.StringWidth(line) > width {
				return raw
			}
		}
	}
	return out
}

func uintValue(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

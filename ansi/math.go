package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// MathBlockElement renders display LaTeX as raw, width-bounded terminal text.
type MathBlockElement struct {
	Text  string
	Style StyleBlock
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

func uintValue(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

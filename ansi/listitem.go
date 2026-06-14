package ansi

import (
	"io"
	"strconv"
)

// An ItemElement is used to render items inside a list.
type ItemElement struct {
	IsOrdered   bool
	Enumeration uint
}

// Render renders an ItemElement.
func (e *ItemElement) Render(w io.Writer, ctx RenderContext) error {
	if e.IsOrdered {
		enumeration := ctx.options.Styles.Enumeration
		number := strconv.FormatInt(int64(e.Enumeration), 10) //nolint:gosec
		if err := renderListMarker(w, ctx, number, enumeration); err != nil {
			return err
		}
		return renderListMarker(w, ctx, enumeration.BlockPrefix, enumeration)
	}
	return renderListMarker(w, ctx, ctx.options.Styles.Item.BlockPrefix, ctx.options.Styles.Item)
}

func renderListMarker(w io.Writer, ctx RenderContext, marker string, style StylePrimitive) error {
	style.BlockPrefix = ""
	el := &BaseElement{
		Token: marker,
		Style: style,
	}
	return el.Render(w, ctx)
}

package ansi

import (
	"io"
	"strconv"

	xansi "github.com/charmbracelet/x/ansi"
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
		if err := renderListMarker(w, ctx, enumeration.BlockPrefix, enumeration); err != nil {
			return err
		}
		setListContinuationColumn(ctx, listMarkerWidth(number, enumeration)+listMarkerWidth(enumeration.BlockPrefix, enumeration))
		return nil
	}
	marker := ctx.options.Styles.Item.BlockPrefix
	if err := renderListMarker(w, ctx, marker, ctx.options.Styles.Item); err != nil {
		return err
	}
	setListContinuationColumn(ctx, listMarkerWidth(marker, ctx.options.Styles.Item))
	return nil
}

func renderListMarker(w io.Writer, ctx RenderContext, marker string, style StylePrimitive) error {
	style.BlockPrefix = ""
	el := &BaseElement{
		Token: marker,
		Style: style,
	}
	return el.Render(w, ctx)
}

func setListContinuationColumn(ctx RenderContext, column int) {
	if ctx.list == nil {
		return
	}
	ctx.list.setContinuationColumn(column)
}

func listMarkerWidth(marker string, style StylePrimitive) int {
	style.BlockPrefix = ""
	token := escapeReplacer.Replace(marker)
	if style.Format != "" {
		if formatted, err := formatToken(style.Format, token); err == nil {
			token = formatted
		}
	}
	return xansi.StringWidth(style.Prefix + token + style.Suffix)
}

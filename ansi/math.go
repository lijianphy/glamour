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
	indentation := uintValue(e.Style.Indent)
	margin := uintValue(e.Style.Margin)
	layout := childBlockLayout(ctx, indentation, margin)
	value := e.Text
	if e.closed {
		raw := strings.TrimSpace(value)
		if out, ok := typesetMath(raw[2:len(raw)-2], false, layout.width, ctx.options.Styles.NativeMathFaces); ok {
			value = out
		}
	}
	if !layout.inList {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return e.renderValue(w, ctx, value, layout)
}

func (e *MathBlockElement) renderValue(w io.Writer, ctx RenderContext, value string, layout listChildBlockLayout) error {
	bs := ctx.blockStack
	var target io.Writer
	var blockBuffer bytes.Buffer
	var iw *IndentWriter
	if layout.inList {
		if _, err := io.WriteString(w, blankLineSeparator(bs.Current().Block.String())); err != nil {
			return err
		}
		target = &blockBuffer
	} else {
		iw = NewIndentWriter(w, layout.indent, func(_ io.Writer) {
			_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
		})
		target = iw
	}

	_, _ = renderText(target, bs.Current().Style.StylePrimitive, e.Style.BlockPrefix)
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
	if layout.inList {
		_, err := io.WriteString(w, markListOpaqueLines(blockBuffer.String(), layout.indent)+"\n")
		return err
	}
	return iw.Close()
}

func typesetMath(body string, inline bool, width int, nativeFaces bool) (string, bool) {
	out, err := termtex.Render(body, termtex.Style{Inline: inline, Width: width, NativeFaces: nativeFaces})
	if err != nil || out == "" {
		return "", false
	}
	// Termtex breaks at operators, but an indivisible grid can still be
	// too wide. Wrap its source rather than tearing the diagram apart.
	if width > 0 {
		for line := range strings.SplitSeq(out, "\n") {
			if xansi.StringWidth(line) > width {
				return "", false
			}
		}
	}
	return out, true
}

type promotedMath struct {
	block  MathBlockElement
	value  string
	layout listChildBlockLayout
}

func inlineMath(ctx RenderContext, body string, raw *BaseElement, allowPromotion bool) ElementRenderer {
	native := ctx.options.Styles.NativeMathFaces
	out, ok := typesetMath(body, true, 0, native)
	switch {
	case !ok:
		return raw
	case !strings.ContainsRune(out, '\n'):
		raw.Token = out
		return raw
	case !allowPromotion:
		return raw
	}
	style := ctx.options.Styles.MathBlock
	layout := childBlockLayout(ctx, uintValue(style.Indent), uintValue(style.Margin))
	if layout.width > 0 {
		if grid, fits := typesetMath(body, false, layout.width, native); fits {
			return &promotedMath{block: MathBlockElement{Style: style}, value: grid, layout: layout}
		}
	}
	return raw
}

func (e *promotedMath) Render(w io.Writer, ctx RenderContext) error {
	if e.layout.inList {
		return e.block.renderValue(w, ctx, e.value, e.layout)
	}
	return writeParagraphBlock(ctx, func(w io.Writer) error {
		return e.block.renderValue(w, ctx, e.value, e.layout)
	})
}

func uintValue(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

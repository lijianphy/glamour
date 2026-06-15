package ansi

import (
	"bytes"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
)

// BlockElement provides a render buffer for children of a block element.
// After all children have been rendered into it, it applies indentation and
// margins around them and writes everything to the parent rendering buffer.
type BlockElement struct {
	Block   *bytes.Buffer
	Style   StyleBlock
	Margin  bool
	Newline bool
	List    bool
}

// Render renders a BlockElement.
func (e *BlockElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	bs.Push(*e)
	if e.List && ctx.list != nil {
		ctx.list.push()
	}

	_, _ = renderText(w, bs.Parent().Style.StylePrimitive, e.Style.BlockPrefix)
	_, _ = renderText(bs.Current().Block, bs.Current().Style.StylePrimitive, e.Style.Prefix)
	return nil
}

// Finish finishes rendering a BlockElement.
func (e *BlockElement) Finish(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	if e.List && ctx.list != nil {
		defer ctx.list.pop()
	}

	if e.Margin { //nolint: nestif
		width := int(bs.Width(ctx))
		s := lipgloss.Wrap(bs.Current().Block.String(), width, " ,.;-+|")
		if e.List {
			s = wrapListBlock(bs.Current().Block.String(), listWrapWidth(width, bs.Current().Style), ctx.options.Styles)
		}

		mw := NewMarginWriter(ctx, w, bs.Current().Style)
		defer mw.Close() //nolint:errcheck
		if _, err := io.WriteString(mw, s); err != nil {
			return fmt.Errorf("glamour: error writing to writer: %w", err)
		}

		if e.Newline {
			if _, err := io.WriteString(mw, "\n"); err != nil {
				return fmt.Errorf("glamour: error writing to writer: %w", err)
			}
		}
	} else {
		_, err := bs.Parent().Block.Write(bs.Current().Block.Bytes())
		if err != nil {
			return fmt.Errorf("glamour: error writing to writer: %w", err)
		}
	}

	_, _ = renderText(w, bs.Current().Style.StylePrimitive, e.Style.Suffix)
	_, _ = renderText(w, bs.Parent().Style.StylePrimitive, e.Style.BlockSuffix)

	bs.Current().Block.Reset()
	bs.Pop()
	return nil
}

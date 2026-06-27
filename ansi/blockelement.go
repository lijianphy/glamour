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

	blockPrefixTarget := w
	if bs.Parent().List {
		blockPrefixTarget = bs.Current().Block
	}
	_, _ = renderText(blockPrefixTarget, bs.Parent().Style.StylePrimitive, e.Style.BlockPrefix)
	_, _ = renderText(bs.Current().Block, bs.Current().Style.StylePrimitive, e.Style.Prefix)
	return nil
}

// Finish finishes rendering a BlockElement.
func (e *BlockElement) Finish(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	if e.List && ctx.list != nil {
		defer ctx.list.pop()
	}

	target := w
	var parentListBlock bytes.Buffer
	markForParentList := e.List && bs.Parent().List
	markForParentListChild := !e.List && bs.Parent().List
	markIndent := 0
	if markForParentList || markForParentListChild {
		target = &parentListBlock
	}

	if e.Margin { //nolint: nestif
		width := int(bs.Width(ctx))
		wrapWidth := width
		marginWidth := width
		if markForParentListChild {
			markIndent = listNestedBlockIndent(ctx)
			width -= markIndent
			if width < 0 {
				width = 0
			}
			marginWidth = max(width-marginIndentWidth(bs.Current().Style, 0), 0)
			wrapWidth = marginWidth
		}
		block := bs.Current().Block.String()
		var s string
		if e.List {
			// wrapListBlock already owns list wrapping and opaque child handling.
			// Keep this direct: pre-wrapping list buffers doubles ANSI scanning
			// and allocations on every streamed render.
			s = wrapListBlock(block, listWrapWidth(width, bs.Current().Style), ctx.options.Styles)
		} else {
			if markForParentListChild {
				block = trimTrailingANSIWhitespaceLines(block, wrapWidth)
			}
			s = lipgloss.Wrap(block, wrapWidth, " ,.;-+|")
		}

		mw := NewMarginWriterWithIndentOffsetAndWidth(ctx, target, bs.Current().Style, 0, marginWidth)
		if _, err := io.WriteString(mw, s); err != nil {
			return fmt.Errorf("glamour: error writing to writer: %w", err)
		}

		if e.Newline {
			if _, err := io.WriteString(mw, "\n"); err != nil {
				return fmt.Errorf("glamour: error writing to writer: %w", err)
			}
		}
		if err := mw.Close(); err != nil {
			return fmt.Errorf("glamour: error closing margin writer: %w", err)
		}
	} else {
		_, err := target.Write(bs.Current().Block.Bytes())
		if err != nil {
			return fmt.Errorf("glamour: error writing to writer: %w", err)
		}
	}

	_, _ = renderText(target, bs.Current().Style.StylePrimitive, e.Style.Suffix)
	_, _ = renderText(target, bs.Parent().Style.StylePrimitive, e.Style.BlockSuffix)

	if markForParentList || markForParentListChild {
		if _, err := io.WriteString(w, markListOpaqueLines(parentListBlock.String(), markIndent)); err != nil {
			return fmt.Errorf("glamour: error writing list child block: %w", err)
		}
	}

	bs.Current().Block.Reset()
	bs.Pop()
	return nil
}

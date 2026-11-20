package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
)

// A ParagraphElement is used to render individual paragraphs.
type ParagraphElement struct {
	First bool
}

// Render renders a ParagraphElement.
func (e *ParagraphElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	rules := ctx.options.Styles.Paragraph

	if !e.First {
		_, _ = io.WriteString(w, "\n")
	}
	be := BlockElement{
		Block: &bytes.Buffer{},
		Style: cascadeStyle(bs.Current().Style, rules, false),
	}
	bs.Push(be)

	_, _ = renderText(w, bs.Parent().Style.StylePrimitive, rules.BlockPrefix)
	_, _ = renderText(bs.Current().Block, bs.Current().Style.StylePrimitive, rules.Prefix)
	return nil
}

// Finish finishes rendering a ParagraphElement.
func (e *ParagraphElement) Finish(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	rules := bs.Current().Style

	mw := NewMarginWriter(ctx, w, rules)
	defer mw.Close() //nolint:errcheck
	if len(strings.TrimSpace(bs.Current().Block.String())) > 0 {
		current := bs.Current()
		flow := paragraphTail(&current, ctx)
		_, err := io.WriteString(mw, current.Block.String()[:current.flowStart]+flow)
		if err != nil {
			return fmt.Errorf("glamour: error writing to writer: %w", err)
		}
		if flow != "" || current.flowStart == 0 {
			_, _ = io.WriteString(mw, "\n")
		}
	}

	_, _ = renderText(w, bs.Current().Style.StylePrimitive, rules.Suffix)
	_, _ = renderText(w, bs.Parent().Style.StylePrimitive, rules.BlockSuffix)

	bs.Current().Block.Reset()
	bs.Pop()
	return nil
}

// Commit pending prose, then protect the block's already-laid-out rows from reflow.
func writeParagraphBlock(ctx RenderContext, writeBlock func(io.Writer) error) error {
	stack := *ctx.blockStack
	current := &stack[len(stack)-1]
	flow := paragraphTail(current, ctx)
	current.Block.Truncate(current.flowStart)
	plain := xansi.Strip(flow)
	if strings.TrimSpace(plain) != "" || strings.ContainsRune(plain, '\n') {
		current.Block.WriteString(flow)
		current.Block.WriteString(blankLineSeparator(plain))
	} else if current.flowStart != 0 {
		// Adjacent grids: the preceding grid already ended its row.
		current.Block.WriteByte('\n')
	}
	if err := writeBlock(current.Block); err != nil {
		return err
	}
	current.flowStart = current.Block.Len()
	return nil
}

func paragraphTail(current *BlockElement, ctx RenderContext) string {
	text := current.Block.String()[current.flowStart:]
	if !ctx.options.PreserveNewLines {
		text = strings.ReplaceAll(text, "\n", " ")
	}
	if current.flowStart == 0 {
		return wrapString(text, int(ctx.blockStack.Width(ctx)), "")
	}
	plain := xansi.Strip(text)
	trimmed := strings.TrimLeft(plain, " ")
	if n := len(plain) - len(trimmed); n > 0 {
		text = xansi.TruncateLeft(text, n, "")
	}
	if strings.TrimSpace(trimmed) != "" && !strings.HasPrefix(trimmed, "\n") {
		text = "\n" + text
	}
	return wrapString(text, int(ctx.blockStack.Width(ctx)), "")
}

// Return only the line breaks still needed to leave a blank row.
func blankLineSeparator(text string) string {
	for _, missing := range []string{"\n\n", "\n"} {
		var ok bool
		text, ok = strings.CutSuffix(strings.TrimRight(text, " \t"), "\n")
		if !ok {
			return missing
		}
	}
	return ""
}

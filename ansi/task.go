package ansi

import (
	"io"

	xansi "github.com/charmbracelet/x/ansi"
)

// A TaskElement is used to render tasks inside a todo-list.
type TaskElement struct {
	Checked bool
}

// Render renders a TaskElement.
func (e *TaskElement) Render(w io.Writer, ctx RenderContext) error {
	var el *BaseElement

	pre := ctx.options.Styles.Task.Unticked
	if e.Checked {
		pre = ctx.options.Styles.Task.Ticked
	}

	el = &BaseElement{
		Prefix: pre,
		Style:  ctx.options.Styles.Task.StylePrimitive,
	}

	if err := el.Render(w, ctx); err != nil {
		return err
	}
	setListContinuationColumn(ctx, xansi.StringWidth(pre))
	return nil
}

package ansi

import (
	"bytes"
	"io"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	xansi "github.com/charmbracelet/x/ansi"
)

// text is the normally styled cell, including verbatim source for each
// equation. A grid replaces its own byte range only if it fits.
type tableCell struct {
	text  string
	grids []cellGrid
}

type cellWriter struct {
	bytes.Buffer
	grids []cellGrid
}

type cellGrid struct {
	start, end int
	equation   *tableEquation
	style      StylePrimitive
	natural    string
}

type tableEquation struct {
	*BaseElement
	body, natural string
}

func tableMath(ctx RenderContext, body string, raw *BaseElement) ElementRenderer {
	display := strings.HasPrefix(raw.Token, "$$") || strings.HasPrefix(raw.Token, `\[`)
	native := ctx.options.Styles.NativeMathFaces
	out, ok := typesetMath(body, !display, 0, native)
	if ok && !display {
		if !strings.ContainsRune(out, '\n') {
			raw.Token = out
			return raw
		}
		out, ok = typesetMath(body, false, 0, native)
	}
	if !ok {
		return raw
	}
	return &tableEquation{BaseElement: raw, body: body, natural: out}
}

func (e *tableEquation) styled(ctx RenderContext, style StylePrimitive, value string) (string, error) {
	el := *e.BaseElement
	el.Token = value
	var b strings.Builder
	err := el.StyleOverrideRender(&b, ctx, style)
	return expandCellTabs(b.String()), err
}

func (e *tableEquation) StyleOverrideRender(w io.Writer, ctx RenderContext, style StylePrimitive) error {
	b, ok := w.(*cellWriter)
	if !ok {
		return e.BaseElement.StyleOverrideRender(w, ctx, style)
	}
	start := b.Len()
	if err := e.BaseElement.StyleOverrideRender(b, ctx, style); err != nil {
		return err
	}
	value, err := e.styled(ctx, style, e.natural)
	if err == nil {
		b.grids = append(b.grids, cellGrid{start, b.Len(), e, style, value})
	}
	return err
}

// Match Lip Gloss's four-space tabs before measuring and wrapping. Otherwise
// the painter can wrap a line that already fitted the allocated cell width.
func expandCellTabs(text string) string {
	return strings.ReplaceAll(text, "\t", "    ")
}

func (c tableCell) prose(start, end int) string {
	// Grid offsets refer to the original styled bytes, before tab expansion.
	return expandCellTabs(c.text[start:end])
}

func (c tableCell) naturalWidth() int {
	if len(c.grids) == 0 {
		return lipgloss.Width(c.prose(0, len(c.text)))
	}
	width, start := 0, 0
	for _, g := range c.grids {
		width = max(width, lipgloss.Width(trimCellProse(c.prose(start, g.start))), lipgloss.Width(g.natural))
		start = g.end
	}
	return max(width, lipgloss.Width(trimCellProse(c.prose(start, len(c.text)))))
}

// Spaces separating prose from an equation aren't part of either block.
// Cut by terminal columns so style and hyperlink escapes remain balanced.
func trimCellProse(text string) string {
	plain := xansi.Strip(text)
	left := len(plain) - len(strings.TrimLeft(plain, " "))
	right := len(plain) - len(strings.TrimRight(plain, " "))
	if left+right == 0 {
		return text
	}
	if left+right >= len(plain) {
		return ""
	}
	return xansi.Cut(text, left, xansi.StringWidth(plain)-right)
}

// gridWidths fits the natural widths in place, excluding framing. Cap the largest
// columns first; the binary search doesn't depend on the source text length.
func gridWidths(widths []int, budget int, unbounded bool, styles table.StyleFunc) []int {
	n := len(widths)
	total := 0
	for col, w := range widths {
		budget -= styles(table.HeaderRow, col).GetHorizontalFrameSize()
		widths[col] = max(1, w)
		total += widths[col]
	}
	if unbounded {
		return widths
	}
	if n == 0 || budget < n {
		return nil
	}
	if total <= budget {
		extra := budget - total
		for i := range widths {
			widths[i] += extra / n
			if i < extra%n {
				widths[i]++
			}
		}
		return widths
	}
	limit := sort.Search(budget+1, func(limit int) bool {
		sum := 0
		for _, w := range widths {
			sum += min(w, limit)
		}
		return sum > budget
	}) - 1
	for _, w := range widths {
		budget -= min(w, limit)
	}
	for i, w := range widths {
		widths[i] = min(w, limit)
		if w > limit && budget > 0 {
			widths[i]++
			budget--
		}
	}
	return widths
}

// fit returns the grid's padded lines, or nil if its source should stay in
// the surrounding prose. Padding keeps alignment from moving individual lines.
func (g cellGrid) fit(ctx RenderContext, width int) ([]string, error) {
	value := g.natural
	w := lipgloss.Width(value)
	if w > width {
		out, fits := typesetMath(g.equation.body, false, width, ctx.options.Styles.NativeMathFaces)
		if !fits {
			return nil, nil
		}
		var err error
		value, err = g.equation.styled(ctx, g.style, out)
		if err != nil {
			return nil, err
		}
		w = lipgloss.Width(value)
		if w > width {
			return nil, nil
		}
	}
	lines := strings.Split(wrapString(value, 0, ""), "\n")
	for i, line := range lines {
		lines[i] += strings.Repeat(" ", w-xansi.StringWidth(line))
	}
	return lines, nil
}

func (c tableCell) lines(ctx RenderContext, width int, wrap bool) ([]string, error) {
	var lines []string
	appendProse := func(text string) {
		if text == "" {
			return
		}
		if wrap {
			text = wrapString(text, width, "")
		} else {
			text = wrapString(text, 0, "") // isolate styles across explicit newlines
		}
		for line := range strings.SplitSeq(text, "\n") {
			lines = append(lines, xansi.Truncate(line, width, "…"))
		}
	}
	start := 0
	for _, g := range c.grids {
		grid, err := g.fit(ctx, width)
		if err != nil {
			return nil, err
		}
		if grid == nil {
			continue
		}
		appendProse(trimCellProse(c.prose(start, g.start)))
		lines = append(lines, grid...)
		start = g.end
	}
	tail := c.prose(start, len(c.text))
	if start != 0 {
		tail = trimCellProse(tail)
	}
	appendProse(tail)
	return lines, nil
}

func (s *tableRenderState) renderGridRows(ctx RenderContext, widths []int, styles table.StyleFunc, border lipgloss.Border, borderStyle lipgloss.Style, seam int) (string, error) {
	full := ctx.options.Styles.Table.FullGrid
	junction := borderStyle.Width(seam)
	separator := junction.Render(border.Left)
	var left, right string
	if full {
		left, right = separator, junction.Render(border.Right)
	}
	rule := func(start, cross, end, stroke string) string {
		var b strings.Builder
		if full {
			b.WriteString(junction.Render(start))
		}
		unit := lipgloss.Width(stroke)
		for j, w := range widths {
			if j != 0 {
				b.WriteString(junction.Render(cross))
			}
			size := w + styles(table.HeaderRow, j).GetHorizontalFrameSize()
			fill := strings.Repeat(" ", size)
			if unit > 0 {
				fill = strings.Repeat(stroke, size/unit) + strings.Repeat(" ", size%unit)
			}
			b.WriteString(borderStyle.Render(fill))
		}
		if full {
			b.WriteString(junction.Render(end))
		}
		return b.String()
	}
	middle := rule(border.MiddleLeft, border.Middle, border.MiddleRight, border.Top)
	wrap := ctx.options.TableWrap == nil || *ctx.options.TableWrap
	var output []string
	if full {
		output = append(output, rule(border.TopLeft, border.MiddleTop, border.TopRight, border.Top))
	}
	for i, row := range s.rows {
		cells := make([][]string, len(widths))
		cellStyles := make([]lipgloss.Style, len(widths))
		height := 1
		for j, w := range widths {
			st := styles(i-1, j) // HeaderRow == -1
			cellStyles[j] = st.Width(w + st.GetHorizontalPadding())
			if j < len(row) {
				lines, err := row[j].lines(ctx, w, i != 0 && wrap)
				if err != nil {
					return "", err
				}
				cells[j] = lines
				height = max(height, len(lines))
			}
		}
		for y := range height {
			var line strings.Builder
			line.WriteString(left)
			for j, cell := range cells {
				if j != 0 {
					line.WriteString(separator)
				}
				var value string
				if y < len(cell) {
					value = cell[y]
				}
				line.WriteString(cellStyles[j].Render(value))
			}
			line.WriteString(right)
			output = append(output, line.String())
		}
		if (!full && i == 0) || (full && i < len(s.rows)-1) {
			output = append(output, middle)
		}
	}
	if full {
		output = append(output, rule(border.BottomLeft, border.MiddleBottom, border.BottomRight, border.Bottom))
	}
	return strings.Join(output, "\n"), nil
}

package ansi

import (
	"bytes"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
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

// A wrapped column needs room for its widest indivisible prose grapheme,
// including in the source shown when a math grid doesn't fit.
type columnMeasure struct{ natural, minimum int }

func (m *columnMeasure) include(c columnMeasure) {
	m.natural = max(m.natural, c.natural)
	m.minimum = max(m.minimum, c.minimum)
}

func (c tableCell) measure() columnMeasure {
	m := columnMeasure{minimum: 1}
	if strings.IndexFunc(c.text, func(r rune) bool { return r >= utf8.RuneSelf }) >= 0 {
		g := uniseg.NewGraphemes(xansi.Strip(c.text))
		for g.Next() {
			m.minimum = max(m.minimum, g.Width())
		}
	}
	if len(c.grids) == 0 {
		m.natural = lipgloss.Width(c.prose(0, len(c.text)))
		return m
	}
	width, start := 0, 0
	for _, g := range c.grids {
		width = max(width, lipgloss.Width(trimCellProse(c.prose(start, g.start))), lipgloss.Width(g.natural))
		start = g.end
	}
	m.natural = max(width, lipgloss.Width(trimCellProse(c.prose(start, len(c.text)))))
	return m
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

// fitColumnWidths fits the natural widths, excluding framing. Cap the largest
// columns first; the binary search doesn't depend on the source text length.
func fitColumnWidths(columns []columnMeasure, budget int, unbounded bool) []int {
	n := len(columns)
	widths := make([]int, n)
	total, minimum := 0, 0
	for col, m := range columns {
		widths[col] = max(m.minimum, m.natural)
		total += widths[col]
		minimum += m.minimum
	}
	if unbounded {
		return widths
	}
	if n == 0 || budget < minimum {
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
		for j, w := range widths {
			sum += max(columns[j].minimum, min(w, limit))
		}
		return sum > budget
	}) - 1
	for j, w := range widths {
		budget -= max(columns[j].minimum, min(w, limit))
	}
	for i, w := range widths {
		widths[i] = max(columns[i].minimum, min(w, limit))
		if widths[i] == limit && w > limit && budget > 0 {
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
			if !wrap {
				line = xansi.Truncate(line, width, "…")
			}
			lines = append(lines, line)
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

func (s *tableRenderState) paintRows(ctx RenderContext, widths []int, styles cellStyleFunc, border lipgloss.Border, borderStyle lipgloss.Style, seam int) (string, error) {
	full := ctx.options.Styles.Table.FullGrid
	junction := borderStyle.Width(seam)
	separator := junction.Render(border.Left)
	var left, right string
	if full {
		left, right = separator, junction.Render(border.Right)
	}
	header := make([]lipgloss.Style, len(widths))
	body := make([]lipgloss.Style, len(widths))
	for j, w := range widths {
		header[j] = withContentWidth(styles(true, j), w)
		body[j] = withContentWidth(styles(false, j), w)
	}
	rule := func(start, cross, end, stroke string) string {
		var b strings.Builder
		if full {
			b.WriteString(junction.Render(start))
		}
		for j, w := range widths {
			if j != 0 {
				b.WriteString(junction.Render(cross))
			}
			size := w + header[j].GetHorizontalFrameSize()
			b.WriteString(borderStyle.Render(ruleFill(stroke, size)))
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
		cellStyles := body
		if i == 0 {
			cellStyles = header
		}
		lines, err := paintRow(ctx, row, widths, cellStyles, i != 0 && wrap, left, separator, right)
		if err != nil {
			return "", err
		}
		output = append(output, lines...)
		if (!full && i == 0) || (full && i < len(s.rows)-1) {
			output = append(output, middle)
		}
	}
	if full {
		output = append(output, rule(border.BottomLeft, border.MiddleBottom, border.BottomRight, border.Bottom))
	}
	return strings.Join(output, "\n"), nil
}

func ruleFill(stroke string, size int) string {
	if unit := lipgloss.Width(stroke); unit > 0 {
		return strings.Repeat(stroke, size/unit) + strings.Repeat(" ", size%unit)
	}
	return strings.Repeat(" ", size)
}

func withContentWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.Width(width + style.GetHorizontalPadding())
}

func paintRow(ctx RenderContext, row []tableCell, widths []int, styles []lipgloss.Style, wrap bool, left, separator, right string) ([]string, error) {
	cells := make([][]string, len(widths))
	height := 1
	for j, w := range widths {
		if j < len(row) {
			lines, err := row[j].lines(ctx, w, wrap)
			if err != nil {
				return nil, err
			}
			cells[j] = lines
			height = max(height, len(lines))
		}
	}
	lines := make([]string, height)
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
			line.WriteString(styles[j].Render(value))
		}
		line.WriteString(right)
		lines[y] = line.String()
	}
	return lines, nil
}

func (c tableCell) standalone(ctx RenderContext, width int) (string, error) {
	if width <= 0 { // A list indent can exhaust the entire width budget.
		return c.prose(0, len(c.text)), nil
	}
	lines, err := c.lines(ctx, width, true)
	return strings.Join(lines, "\n"), err
}

// Transpose each body row into header/value pairs without stretching short pairs.
func (s *tableRenderState) paintRecords(ctx RenderContext, record [2]columnMeasure, styles cellStyleFunc, border lipgloss.Border, borderStyle lipgloss.Style) (string, error) {
	header := s.rows[0]
	if len(s.rows) == 1 {
		labels := make([]string, len(header))
		for j, c := range header {
			label, err := c.standalone(ctx, s.width)
			if err != nil {
				return "", err
			}
			labels[j] = label
		}
		return strings.Join(labels, "\n"), nil
	}
	sep := border.Left
	if s.width >= lipgloss.Width(sep)+2+record[0].minimum+record[1].minimum {
		sep = " " + sep + " "
	}
	natural := max(record[0].natural, record[0].minimum) + max(record[1].natural, record[1].minimum)
	budget := min(natural, s.width-lipgloss.Width(sep))
	widths := fitColumnWidths(record[:], budget, false)
	var pairStyles [][2]lipgloss.Style
	recordSep := "\n\n"
	if widths != nil {
		pairStyles = make([][2]lipgloss.Style, len(header))
		for j := range header {
			for k := range widths {
				pairStyles[j][k] = styles(k == 0, j).Margin(0).Padding(0).Align(lipgloss.Left).Width(widths[k])
			}
		}
		rule := ruleFill(border.Top, widths[0]+lipgloss.Width(sep)+widths[1])
		recordSep = "\n" + borderStyle.Render(rule) + "\n"
	}
	sep = borderStyle.Render(sep)
	var records []string
	for _, row := range s.rows[1:] {
		var fields []string
		for j, label := range header {
			var value tableCell
			if j < len(row) {
				value = row[j]
			}
			pair := []tableCell{label, value}
			if widths == nil {
				for _, c := range pair {
					text, err := c.standalone(ctx, s.width)
					if err != nil {
						return "", err
					}
					fields = append(fields, text)
				}
				continue
			}
			lines, err := paintRow(ctx, pair, widths, pairStyles[j][:], true, "", sep, "")
			if err != nil {
				return "", err
			}
			fields = append(fields, lines...)
		}
		records = append(records, strings.Join(fields, "\n"))
	}
	return strings.Join(records, recordSep), nil
}

package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	xansi "github.com/charmbracelet/x/ansi"
	astext "github.com/yuin/goldmark/extension/ast"
)

// A TableElement is used to render tables.
type TableElement struct {
	table  *astext.Table
	source []byte
}

type tableRenderState struct {
	lipgloss *table.Table
	header   []string
	row      []string
	inList   bool
	indent   int
	width    int

	columnWidths    []int
	lastColumnCells []string
	tableImages     []tableLink
	tableLinks      []tableLink
}

// A TableRowElement is used to render a single row in a table.
type TableRowElement struct{}

// A TableHeadElement is used to render a table's head element.
type TableHeadElement struct{}

// A TableCellElement is used to render a single cell in a row.
type TableCellElement struct {
	Children []ElementRenderer
	Head     bool
}

// Render renders a TableElement.
func (e *TableElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack

	var indentation uint
	var margin uint
	rules := ctx.options.Styles.Table
	if rules.Indent != nil {
		indentation = *rules.Indent
	}
	if rules.Margin != nil {
		margin = *rules.Margin
	}

	prefixIndent := int(indentation + margin)
	tableIndent := 0
	width := int(ctx.blockStack.Width(ctx))
	ctx.table.inList = bs.Current().List
	if ctx.table.inList {
		tableIndent, width = indentedBlockWidth(ctx, indentation, margin)
		prefixIndent = tableIndent
	}
	ctx.table.indent = tableIndent
	ctx.table.width = width

	iw := NewIndentWriter(w, prefixIndent, func(_ io.Writer) {
		_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
	})
	defer iw.Close() //nolint:errcheck

	style := bs.With(rules.StylePrimitive)

	_, _ = renderText(iw, bs.Current().Style.StylePrimitive, rules.BlockPrefix)
	_, _ = renderText(iw, style, rules.Prefix)

	wrap := true
	if ctx.options.TableWrap != nil {
		wrap = *ctx.options.TableWrap
	}
	ctx.table.lipgloss = table.New().Width(width).Wrap(wrap)

	if err := e.collectLinksAndImages(ctx); err != nil {
		return err
	}

	return nil
}

func (e *TableElement) setStyles(ctx RenderContext) {
	docRules := ctx.options.Styles.Document
	if docRules.BackgroundColor != nil {
		baseStyle := lipgloss.NewStyle().Background(lipgloss.Color(*docRules.BackgroundColor))
		ctx.table.lipgloss.BaseStyle(baseStyle)
	}

	rules := ctx.options.Styles.Table
	compact := ctx.table.inList && rules.Margin == nil && ctx.table.compactTable(ctx.table.width)
	ctx.table.lipgloss = ctx.table.lipgloss.StyleFunc(func(row, col int) lipgloss.Style {
		st := lipglossStyleFromPrimitive(
			tableCellStylePrimitive(rules, row == table.HeaderRow),
		).Inline(false)
		// Default Styles
		if compact {
			st = st.Margin(0, 0)
		} else {
			st = st.Margin(0, 1)
		}

		// Override with custom styles
		if m := rules.Margin; m != nil {
			st = st.Padding(0, int(*m))
		}
		switch e.table.Alignments[col] {
		case astext.AlignLeft:
			st = st.Align(lipgloss.Left).PaddingRight(0)
		case astext.AlignCenter:
			st = st.Align(lipgloss.Center)
		case astext.AlignRight:
			st = st.Align(lipgloss.Right).PaddingLeft(0)
		case astext.AlignNone:
			// do nothing
		}

		return st
	})
}

func (e *TableElement) setBorders(ctx RenderContext) {
	rules := ctx.options.Styles.Table
	border := lipgloss.NormalBorder()

	if rules.RowSeparator != nil && rules.ColumnSeparator != nil {
		border = lipgloss.Border{
			Top:    *rules.RowSeparator,
			Bottom: *rules.RowSeparator,
			Left:   *rules.ColumnSeparator,
			Right:  *rules.ColumnSeparator,
			Middle: *rules.CenterSeparator,
		}
	}
	ctx.table.lipgloss.Border(border)
	ctx.table.lipgloss.BorderStyle(
		lipglossStyleFromPrimitive(cascadeStylePrimitives(rules.StylePrimitive, rules.Border)),
	)
	ctx.table.lipgloss.BorderTop(false)
	ctx.table.lipgloss.BorderLeft(false)
	ctx.table.lipgloss.BorderRight(false)
	ctx.table.lipgloss.BorderBottom(false)
}

// Finish finishes rendering a TableElement.
func (e *TableElement) Finish(_ io.Writer, ctx RenderContext) error {
	defer ctx.table.reset()

	rules := ctx.options.Styles.Table

	e.setStyles(ctx)
	e.setBorders(ctx)
	ctx.table.finishRows()

	ow := ctx.blockStack.Current().Block
	tableString := safeTableString(ctx.table.lipgloss, ctx.table.width, ctx.table.lastColumnCells)
	tw := io.Writer(ow)
	var iw *IndentWriter
	if ctx.table.indent > 0 {
		iw = NewIndentWriter(ow, ctx.table.indent, func(_ io.Writer) {
			_, _ = renderText(ow, ctx.blockStack.Current().Style.StylePrimitive, " ")
		})
		tw = iw
	}
	if _, err := io.WriteString(tw, tableString); err != nil {
		return fmt.Errorf("glamour: error writing to buffer: %w", err)
	}

	_, _ = renderText(tw, ctx.blockStack.With(rules.StylePrimitive), rules.Suffix)
	if iw != nil {
		if err := iw.Close(); err != nil {
			return fmt.Errorf("glamour: error closing table indentation: %w", err)
		}
	}
	_, _ = renderText(ow, ctx.blockStack.Current().Style.StylePrimitive, rules.BlockSuffix)

	e.printTableLinks(ctx)

	return nil
}

// Finish finishes rendering a TableRowElement.
func (e *TableRowElement) Finish(_ io.Writer, ctx RenderContext) error {
	if ctx.table.lipgloss == nil {
		return nil
	}

	ctx.table.rememberRow(ctx.table.row)
	ctx.table.lipgloss.Row(ctx.table.row...)
	ctx.table.row = []string{}
	return nil
}

// Finish finishes rendering a TableHeadElement.
func (e *TableHeadElement) Finish(_ io.Writer, ctx RenderContext) error {
	if ctx.table.lipgloss == nil {
		return nil
	}

	ctx.table.rememberRow(ctx.table.header)
	ctx.table.lipgloss.Headers(ctx.table.header...)
	ctx.table.header = []string{}
	return nil
}

// Render renders a TableCellElement.
func (e *TableCellElement) Render(_ io.Writer, ctx RenderContext) error {
	var b bytes.Buffer
	style := tableCellStylePrimitive(ctx.options.Styles.Table, e.Head)
	for _, child := range e.Children {
		if r, ok := child.(StyleOverriderElementRenderer); ok {
			if err := r.StyleOverrideRender(&b, ctx, style); err != nil {
				return fmt.Errorf("glamour: error rendering with style: %w", err)
			}
		} else {
			var bb bytes.Buffer
			if err := child.Render(&bb, ctx); err != nil {
				return fmt.Errorf("glamour: error rendering: %w", err)
			}
			el := &BaseElement{
				Token: bb.String(),
				Style: style,
			}
			if err := el.Render(&b, ctx); err != nil {
				return err
			}
		}
	}

	if e.Head {
		ctx.table.header = append(ctx.table.header, b.String())
	} else {
		ctx.table.row = append(ctx.table.row, b.String())
	}

	return nil
}

func (s *tableRenderState) rememberRow(row []string) {
	if len(row) == 0 {
		return
	}
	s.lastColumnCells = append(s.lastColumnCells, strings.TrimSpace(xansi.Strip(row[len(row)-1])))
	if len(row) > len(s.columnWidths) {
		s.columnWidths = append(s.columnWidths, make([]int, len(row)-len(s.columnWidths))...)
	}
	for column, cell := range row {
		s.columnWidths[column] = max(s.columnWidths[column], xansi.StringWidth(cell))
	}
}

func (s *tableRenderState) compactTable(width int) bool {
	if width <= 0 || len(s.columnWidths) == 0 {
		return false
	}
	naturalWidth := 0
	for _, columnWidth := range s.columnWidths {
		naturalWidth += columnWidth + 2
	}
	naturalWidth += max(0, len(s.columnWidths)-1)
	return width < naturalWidth
}

// safeTableString avoids a Lipgloss edge case where a borderless table at a
// width boundary gets hard-clipped by MaxWidth without an ellipsis.
func safeTableString(t *table.Table, width int, lastColumnCells []string) string {
	rendered := t.String()
	if width <= 1 || len(lastColumnCells) == 0 || !tableStringLooksClipped(rendered, width, lastColumnCells) {
		return rendered
	}

	t.Width(width - 1)
	narrower := t.String()
	t.Width(width)
	if tableStringEllipsisCount(narrower) > tableStringEllipsisCount(rendered) ||
		tableStringLineCount(narrower) > tableStringLineCount(rendered) {
		return narrower
	}
	return rendered
}

func tableStringLooksClipped(rendered string, width int, lastColumnCells []string) bool {
	plain := xansi.Strip(rendered)
	lines := fullWidthTableLines(plain, width)
	if len(lines) == 0 {
		return false
	}

	for _, cell := range lastColumnCells {
		if cell == "" || strings.Contains(plain, cell) {
			continue
		}
		if anyLineEndsWithCellPrefix(lines, cell) {
			return true
		}
	}
	return false
}

func fullWidthTableLines(rendered string, width int) []string {
	var lines []string
	for line := range strings.SplitSeq(rendered, "\n") {
		line = strings.TrimRight(line, " ")
		if line == "" || strings.ContainsRune(line, '…') || xansi.StringWidth(line) != width {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func anyLineEndsWithCellPrefix(lines []string, cell string) bool {
	runes := []rune(cell)
	for end := len(runes) - 1; end > 0; end-- {
		prefix := string(runes[:end])
		if xansi.StringWidth(prefix) < 3 {
			continue
		}
		for _, line := range lines {
			if strings.HasSuffix(line, prefix) {
				return true
			}
		}
	}
	return false
}

func tableStringLineCount(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}

func tableStringEllipsisCount(value string) int {
	return strings.Count(xansi.Strip(value), "…")
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := values[:0]
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func (s *tableRenderState) finishRows() {
	if len(s.lastColumnCells) > 1 {
		s.lastColumnCells = uniqueNonEmptyStrings(s.lastColumnCells)
	}
}

func (s *tableRenderState) reset() {
	s.lipgloss = nil
	s.header = nil
	s.row = nil
	s.tableImages = nil
	s.tableLinks = nil
	s.lastColumnCells = nil
	s.columnWidths = nil
	s.inList = false
	s.indent = 0
	s.width = 0
}

func tableCellStylePrimitive(rules StyleTable, head bool) StylePrimitive {
	style := cascadeStylePrimitives(rules.StylePrimitive, rules.Cell)
	if head {
		style = cascadeStylePrimitives(style, rules.Header)
	}
	return style
}

func lipglossStyleFromPrimitive(rules StylePrimitive) lipgloss.Style {
	style := lipgloss.NewStyle()
	if rules.Color != nil {
		style = style.Foreground(lipgloss.Color(*rules.Color))
	}
	if rules.BackgroundColor != nil {
		style = style.Background(lipgloss.Color(*rules.BackgroundColor))
	}
	if rules.Underline != nil {
		style = style.Underline(*rules.Underline)
	}
	if rules.Bold != nil {
		style = style.Bold(*rules.Bold)
	}
	if rules.Italic != nil {
		style = style.Italic(*rules.Italic)
	}
	if rules.CrossedOut != nil {
		style = style.Strikethrough(*rules.CrossedOut)
	}
	if rules.Faint != nil {
		style = style.Faint(*rules.Faint)
	}
	if rules.Inverse != nil {
		style = style.Reverse(*rules.Inverse)
	}
	if rules.Blink != nil {
		style = style.Blink(*rules.Blink)
	}
	return style
}

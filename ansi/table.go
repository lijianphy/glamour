package ansi

import (
	"bytes"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	astext "github.com/yuin/goldmark/extension/ast"
)

// A TableElement is used to render tables.
type TableElement struct {
	table  *astext.Table
	source []byte
}

type tableRenderState struct {
	buffer bytes.Buffer
	rows   [][]tableCell // first row is the GFM header
	row    []tableCell
	inList bool
	indent int
	width  int

	tableImages []tableLink
	tableLinks  []tableLink
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

	tableIndent := 0
	width := int(ctx.blockStack.Width(ctx))
	ctx.table.inList = bs.Current().List
	var prefixTarget io.Writer
	if ctx.table.inList {
		layout := childBlockLayout(ctx, indentation, margin)
		tableIndent = layout.indent
		width = layout.width
		prefixTarget = &ctx.table.buffer
	} else {
		prefixIndent := int(indentation + margin)
		iw := NewIndentWriter(w, prefixIndent, func(_ io.Writer) {
			_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
		})
		defer iw.Close() //nolint:errcheck
		prefixTarget = iw
	}
	ctx.table.indent = tableIndent
	ctx.table.width = width

	style := bs.With(rules.StylePrimitive)

	_, _ = renderText(prefixTarget, bs.Current().Style.StylePrimitive, rules.BlockPrefix)
	_, _ = renderText(prefixTarget, style, rules.Prefix)

	if err := e.collectLinksAndImages(ctx); err != nil {
		return err
	}

	return nil
}

func tableBaseStyle(ctx RenderContext) lipgloss.Style {
	baseStyle := lipgloss.NewStyle()
	docRules := ctx.options.Styles.Document
	if docRules.BackgroundColor != nil {
		baseStyle = baseStyle.Background(lipgloss.Color(*docRules.BackgroundColor))
	}
	return baseStyle
}

type cellStyleFunc func(header bool, col int) lipgloss.Style

func (e *TableElement) cellStyles(ctx RenderContext, compact bool) cellStyleFunc {
	baseStyle := tableBaseStyle(ctx)
	rules := ctx.options.Styles.Table
	return func(header bool, col int) lipgloss.Style {
		style := inheritCurrentFaint(ctx, tableCellStylePrimitive(rules, header))
		st := lipglossStyleFromPrimitive(style).Inline(false)
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

		return st.Inherit(baseStyle)
	}
}

func tableBorder(ctx RenderContext) (lipgloss.Border, lipgloss.Style) {
	rules := ctx.options.Styles.Table
	border := lipgloss.NormalBorder()

	if rules.RowSeparator != nil && rules.ColumnSeparator != nil {
		border.Top, border.Bottom = *rules.RowSeparator, *rules.RowSeparator
		border.Left, border.Right = *rules.ColumnSeparator, *rules.ColumnSeparator
		if rules.CenterSeparator != nil {
			border.Middle = *rules.CenterSeparator
		}
	}
	borderStyle := inheritCurrentFaint(ctx, cascadeStylePrimitives(rules.StylePrimitive, rules.Border))
	return border, lipglossStyleFromPrimitive(borderStyle).Inherit(tableBaseStyle(ctx))
}

// Finish finishes rendering a TableElement.
func (e *TableElement) Finish(_ io.Writer, ctx RenderContext) error {
	defer ctx.table.reset()

	rules := ctx.options.Styles.Table

	tableString, err := e.renderRows(ctx)
	if err != nil {
		return err
	}

	ow := ctx.blockStack.Current().Block
	tw := io.Writer(ow)
	var iw *IndentWriter
	if ctx.table.inList {
		tw = &ctx.table.buffer
	} else if ctx.table.indent > 0 {
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
	_, _ = renderText(tw, ctx.blockStack.Current().Style.StylePrimitive, rules.BlockSuffix)

	e.printTableLinks(ctx)
	if ctx.table.inList {
		if _, err := io.WriteString(ow, markListOpaqueLines(ctx.table.buffer.String(), ctx.table.indent)); err != nil {
			return fmt.Errorf("glamour: error writing table block to buffer: %w", err)
		}
	}

	return nil
}

// Finish finishes rendering a TableRowElement.
func (e *TableRowElement) Finish(_ io.Writer, ctx RenderContext) error {
	ctx.table.endRow()
	return nil
}

// Finish finishes rendering a TableHeadElement.
func (e *TableHeadElement) Finish(_ io.Writer, ctx RenderContext) error {
	ctx.table.endRow()
	return nil
}

// Render renders a TableCellElement.
func (e *TableCellElement) Render(_ io.Writer, ctx RenderContext) error {
	var b cellWriter
	style := inheritCurrentFaint(ctx, tableCellStylePrimitive(ctx.options.Styles.Table, e.Head))
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

	ctx.table.row = append(ctx.table.row, tableCell{text: b.String(), grids: b.grids})

	return nil
}

func (s *tableRenderState) endRow() {
	s.rows = append(s.rows, s.row)
	s.row = nil
}

func (e *TableElement) renderRows(ctx RenderContext) (string, error) {
	s := ctx.table
	canCompact := s.inList && ctx.options.Styles.Table.Margin == nil && s.width > 0
	columns := make([]columnMeasure, len(e.table.Alignments))
	record := [2]columnMeasure{}
	wrap := ctx.options.TableWrap == nil || *ctx.options.TableWrap
	for i, row := range s.rows {
		field := 1
		if i == 0 {
			field = 0
		}
		for j, cell := range row {
			m := cell.measure()
			record[field].include(m)
			if i == 0 || !wrap {
				m.minimum = 1 // headers and non-wrapping body cells may truncate
			}
			columns[j].include(m)
		}
	}
	border, borderStyle := tableBorder(ctx)
	full := ctx.options.Styles.Table.FullGrid
	seam := max(lipgloss.Width(border.Left), lipgloss.Width(border.Middle))
	separators := max(0, len(e.table.Alignments)-1)
	if full {
		seam = max(1, seam) // the corners and junctions are at least one column wide
		separators += 2
	}
	budget := s.width - separators*seam
	styles := e.cellStyles(ctx, canCompact && compactTable(budget, columns))
	for j := range columns {
		budget -= styles(true, j).GetHorizontalFrameSize()
	}
	if widths := fitColumnWidths(columns, budget, ctx.options.WordWrap == 0); widths != nil {
		return s.paintRows(ctx, widths, styles, border, borderStyle, seam)
	}
	return s.paintRecords(ctx, record, styles, border, borderStyle)
}

func compactTable(width int, columns []columnMeasure) bool {
	naturalWidth := 0
	for _, col := range columns {
		naturalWidth += col.natural + 2
	}
	return width < naturalWidth
}

func (s *tableRenderState) reset() {
	s.buffer.Reset()
	s.rows = nil
	s.row = nil
	s.tableImages = nil
	s.tableLinks = nil
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

func inheritCurrentFaint(ctx RenderContext, style StylePrimitive) StylePrimitive {
	current := ctx.blockStack.Current().Style.StylePrimitive
	if styleIsFaint(current) && style.Faint == nil {
		style.Faint = current.Faint
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

package ansi

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
)

const (
	// Internal markers identify already-rendered child block rows while a list
	// buffer is rewrapped. They are stripped before final output.
	listOpaqueLinePrefix    = "\x1eglamour-list-opaque:"
	listOpaqueLineSeparator = "\x1f"
)

type listChildBlockLayout struct {
	inList bool
	indent int
	width  int
}

type listRenderState struct {
	continuationColumns []int
}

func (s *listRenderState) push() {
	s.continuationColumns = append(s.continuationColumns, 0)
}

func (s *listRenderState) pop() {
	if len(s.continuationColumns) == 0 {
		return
	}
	s.continuationColumns = s.continuationColumns[:len(s.continuationColumns)-1]
}

func (s *listRenderState) setContinuationColumn(column int) {
	if len(s.continuationColumns) == 0 {
		return
	}
	s.continuationColumns[len(s.continuationColumns)-1] = column
}

func (s *listRenderState) continuationColumn() int {
	if len(s.continuationColumns) == 0 {
		return 0
	}
	return s.continuationColumns[len(s.continuationColumns)-1]
}

func wrapListBlock(value string, width int, styles StyleConfig) string {
	if value == "" {
		return ""
	}

	lines := strings.Split(value, "\n")
	wrapped := make([]string, 0, len(lines))
	continuationColumn := 0
	for _, line := range lines {
		if indent, content, ok := parseListOpaqueLine(line); ok {
			if indent > 0 {
				content = strings.Repeat(" ", indent) + content
			}
			wrapped = append(wrapped, content)
			continue
		}
		if strings.TrimSpace(xansi.Strip(line)) == "" {
			wrapped = append(wrapped, line)
			continue
		}
		if column, ok := listContentColumn(line, styles); ok {
			itemLines := wrapListItemLine(line, column, width)
			wrapped = append(wrapped, itemLines...)
			continuationColumn = column
			continue
		}
		if continuationColumn > 0 {
			wrapped = append(wrapped, wrapListContinuationLine(line, continuationColumn, width)...)
			continue
		}
		wrapped = append(wrapped, wrapString(line, width, " ,.;-+|"))
	}
	return strings.Join(wrapped, "\n")
}

func listWrapWidth(width int, style StyleBlock) int {
	if style.Indent != nil {
		width -= int(*style.Indent)
	}
	if style.Margin != nil {
		width -= int(*style.Margin)
	}
	return max(1, width)
}

func childBlockLayout(ctx RenderContext, indentation, margin uint) listChildBlockLayout {
	layout := listChildBlockLayout{
		indent: int(indentation + margin),
		width:  int(ctx.blockStack.Width(ctx)),
	}
	if ctx.blockStack.Current().List {
		layout.inList = true
		layout.width = listWrapWidth(layout.width, ctx.blockStack.Current().Style)
		layout.indent = listNestedBlockIndent(ctx)
	} else if ctx.blockStack.Parent().List && ctx.blockStack.Current().IndentOffset == 0 {
		layout.width -= listNestedBlockIndent(ctx)
		layout.width -= marginIndentWidth(ctx.blockStack.Current().Style, 0)
	}
	layout.width -= layout.indent
	if layout.width < 0 {
		layout.width = 0
	}
	return layout
}

func markListOpaqueLines(value string, indent int) string {
	if value == "" {
		return ""
	}
	if indent < 0 {
		indent = 0
	}
	prefix := listOpaqueLinePrefix + strconv.Itoa(indent) + listOpaqueLineSeparator
	// This is hit for every opaque child block during live Markdown rendering.
	// Build the marked output in one pass instead of SplitAfter+Join to avoid
	// per-line slice/string churn.
	var builder strings.Builder
	builder.Grow(len(value) + strings.Count(value, "\n")*len(prefix) + len(prefix))
	for value != "" {
		builder.WriteString(prefix)
		if index := strings.IndexByte(value, '\n'); index >= 0 {
			builder.WriteString(value[:index+1])
			value = value[index+1:]
		} else {
			builder.WriteString(value)
			break
		}
	}
	return builder.String()
}

func finishListChildBlock(w io.Writer, value string, layout listChildBlockLayout, closer *IndentWriter) error {
	if closer != nil {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	if !layout.inList {
		return nil
	}
	if _, err := io.WriteString(w, markListOpaqueLines(value, layout.indent)); err != nil {
		return fmt.Errorf("write list child block: %w", err)
	}
	return nil
}

func parseListOpaqueLine(line string) (int, string, bool) {
	if !strings.HasPrefix(line, listOpaqueLinePrefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(line, listOpaqueLinePrefix)
	before, after, ok := strings.Cut(rest, listOpaqueLineSeparator)
	if !ok {
		return 0, line, false
	}
	indent, err := strconv.Atoi(before)
	if err != nil {
		return 0, line, false
	}
	if indent < 0 {
		indent = 0
	}
	return indent, after, true
}

func trimTrailingANSIWhitespaceLines(value string, targetWidth int) string {
	if value == "" {
		return ""
	}
	parts := strings.SplitAfter(value, "\n")
	for index, part := range parts {
		if part == "" {
			continue
		}
		hasNewline := strings.HasSuffix(part, "\n")
		line := strings.TrimSuffix(part, "\n")
		plain := xansi.Strip(line)
		// Most nested child rows are already within the target width. Avoid the
		// extra ANSI Cut pass for those rows; besides being faster, this also
		// keeps intentionally padded backgrounds intact.
		if targetWidth >= 0 && xansi.StringWidth(plain) <= targetWidth {
			continue
		}
		trimmedWidth := xansi.StringWidth(strings.TrimRight(plain, " \t"))
		line = xansi.Cut(line, 0, trimmedWidth)
		if hasNewline {
			line += "\n"
		}
		parts[index] = line
	}
	return strings.Join(parts, "")
}

func currentListContinuationColumn(ctx RenderContext) int {
	if ctx.list == nil {
		return 0
	}
	return ctx.list.continuationColumn()
}

func listBlockContentIndent(ctx RenderContext) int {
	return max(currentListContinuationColumn(ctx), 0)
}

func listNestedBlockIndent(ctx RenderContext) int {
	return listBlockContentIndent(ctx) + int(ctx.options.Styles.List.LevelIndent)
}

// nestedListIndent returns the indentation for a child list relative to its
// parent list block. The child marker starts at the parent item's content
// column plus the configured level offset, so wide task or ordered markers do
// not collapse the visual hierarchy.
func nestedListIndent(ctx RenderContext) uint {
	return uint(listNestedBlockIndent(ctx))
}

func wrapListItemLine(line string, column, width int) []string {
	if width <= column || xansi.StringWidth(line) <= width {
		return []string{line}
	}

	marker := xansi.Cut(line, 0, column)
	content := xansi.Cut(line, column, xansi.StringWidth(line))
	parts := strings.Split(wrapString(content, width-column, " ,.;-+|"), "\n")
	for index, part := range parts {
		part = strings.TrimLeft(part, " \t")
		if index == 0 {
			parts[index] = marker + part
			continue
		}
		parts[index] = strings.Repeat(" ", column) + part
	}
	return parts
}

func wrapListContinuationLine(line string, column, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	if leadingSpaceWidth(xansi.Strip(line)) < column {
		line = strings.Repeat(" ", column-leadingSpaceWidth(xansi.Strip(line))) + line
	}
	if xansi.StringWidth(line) <= width {
		return []string{line}
	}

	parts := strings.Split(wrapString(line, width, " ,.;-+|"), "\n")
	for index := 1; index < len(parts); index++ {
		parts[index] = strings.Repeat(" ", column) + strings.TrimLeft(parts[index], " \t")
	}
	return parts
}

func listContentColumn(line string, styles StyleConfig) (int, bool) {
	plain := xansi.Strip(line)
	indent := leadingSpaceWidth(plain)
	rest := strings.TrimLeft(plain, " \t")
	if width, ok := orderedListMarkerWidth(rest, styles.Enumeration.BlockPrefix); ok {
		return indent + width, true
	}
	for _, marker := range []string{
		styles.Item.BlockPrefix,
		styles.Task.Ticked,
		styles.Task.Unticked,
	} {
		if marker == "" {
			continue
		}
		if strings.HasPrefix(rest, marker) {
			return indent + xansi.StringWidth(marker), true
		}
	}
	return 0, false
}

func orderedListMarkerWidth(line, markerSuffix string) (int, bool) {
	if markerSuffix == "" {
		return 0, false
	}

	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	if index == 0 || !strings.HasPrefix(line[index:], markerSuffix) {
		return 0, false
	}
	return xansi.StringWidth(line[:index+len(markerSuffix)]), true
}

func leadingSpaceWidth(line string) int {
	width := 0
	for _, r := range line {
		switch r {
		case ' ', '\t':
			width++
		default:
			return width
		}
	}
	return width
}

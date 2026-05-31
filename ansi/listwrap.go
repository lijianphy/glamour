package ansi

import (
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

func wrapListBlock(value string, width int, styles StyleConfig) string {
	if value == "" {
		return ""
	}

	lines := strings.Split(value, "\n")
	wrapped := make([]string, 0, len(lines))
	continuationColumn := 0
	for _, line := range lines {
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
		wrapped = append(wrapped, lipgloss.Wrap(line, width, " ,.;-+|"))
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

func wrapListItemLine(line string, column, width int) []string {
	if width <= column || xansi.StringWidth(line) <= width {
		return []string{line}
	}

	marker := xansi.Cut(line, 0, column)
	content := xansi.Cut(line, column, xansi.StringWidth(line))
	parts := strings.Split(lipgloss.Wrap(content, width-column, " ,.;-+|"), "\n")
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
	if leadingSpaceWidth(xansi.Strip(line)) < column {
		line = strings.Repeat(" ", column-leadingSpaceWidth(xansi.Strip(line))) + line
	}
	if width <= 0 || xansi.StringWidth(line) <= width {
		return []string{line}
	}

	parts := strings.Split(lipgloss.Wrap(line, width, " ,.;-+|"), "\n")
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

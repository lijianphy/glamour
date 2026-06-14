package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/alecthomas/chroma/v2/styles"
	xansi "github.com/charmbracelet/x/ansi"
)

const (
	// The chroma style theme name used for rendering.
	chromaStyleTheme = "charm"

	// The chroma formatter name used for rendering.
	chromaFormatter = "terminal256"
)

// mutex for synchronizing access to the chroma style registry.
// Related https://github.com/alecthomas/chroma/pull/650
var mutex = sync.Mutex{}

// A CodeBlockElement is used to render code blocks.
type CodeBlockElement struct {
	Code     string
	Language string
}

func chromaStyle(style StylePrimitive) string {
	var s string

	if style.Color != nil {
		s = *style.Color
	}
	if style.BackgroundColor != nil {
		if s != "" {
			s += " "
		}
		s += "bg:" + *style.BackgroundColor
	}
	if style.Italic != nil && *style.Italic {
		if s != "" {
			s += " "
		}
		s += "italic"
	}
	if style.Bold != nil && *style.Bold {
		if s != "" {
			s += " "
		}
		s += "bold"
	}
	if style.Underline != nil && *style.Underline {
		if s != "" {
			s += " "
		}
		s += "underline"
	}

	return s
}

// Render renders a CodeBlockElement.
func (e *CodeBlockElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack

	var indentation uint
	var margin uint
	formatter := chromaFormatter
	rules := ctx.options.Styles.CodeBlock
	if rules.Indent != nil {
		indentation = *rules.Indent
	}
	if rules.Margin != nil {
		margin = *rules.Margin
	}
	if len(ctx.options.ChromaFormatter) > 0 {
		formatter = ctx.options.ChromaFormatter
	}
	theme := rules.Theme

	if rules.Chroma != nil {
		theme = chromaStyleTheme
		mutex.Lock()
		// Don't register the style if it's already registered.
		_, ok := styles.Registry[theme]
		if !ok {
			styles.Register(chroma.MustNewStyle(theme,
				chroma.StyleEntries{
					chroma.Text:                chromaStyle(rules.Chroma.Text),
					chroma.Error:               chromaStyle(rules.Chroma.Error),
					chroma.Comment:             chromaStyle(rules.Chroma.Comment),
					chroma.CommentPreproc:      chromaStyle(rules.Chroma.CommentPreproc),
					chroma.Keyword:             chromaStyle(rules.Chroma.Keyword),
					chroma.KeywordReserved:     chromaStyle(rules.Chroma.KeywordReserved),
					chroma.KeywordNamespace:    chromaStyle(rules.Chroma.KeywordNamespace),
					chroma.KeywordType:         chromaStyle(rules.Chroma.KeywordType),
					chroma.Operator:            chromaStyle(rules.Chroma.Operator),
					chroma.Punctuation:         chromaStyle(rules.Chroma.Punctuation),
					chroma.Name:                chromaStyle(rules.Chroma.Name),
					chroma.NameBuiltin:         chromaStyle(rules.Chroma.NameBuiltin),
					chroma.NameTag:             chromaStyle(rules.Chroma.NameTag),
					chroma.NameAttribute:       chromaStyle(rules.Chroma.NameAttribute),
					chroma.NameClass:           chromaStyle(rules.Chroma.NameClass),
					chroma.NameConstant:        chromaStyle(rules.Chroma.NameConstant),
					chroma.NameDecorator:       chromaStyle(rules.Chroma.NameDecorator),
					chroma.NameException:       chromaStyle(rules.Chroma.NameException),
					chroma.NameFunction:        chromaStyle(rules.Chroma.NameFunction),
					chroma.NameOther:           chromaStyle(rules.Chroma.NameOther),
					chroma.Literal:             chromaStyle(rules.Chroma.Literal),
					chroma.LiteralNumber:       chromaStyle(rules.Chroma.LiteralNumber),
					chroma.LiteralDate:         chromaStyle(rules.Chroma.LiteralDate),
					chroma.LiteralString:       chromaStyle(rules.Chroma.LiteralString),
					chroma.LiteralStringEscape: chromaStyle(rules.Chroma.LiteralStringEscape),
					chroma.GenericDeleted:      chromaStyle(rules.Chroma.GenericDeleted),
					chroma.GenericEmph:         chromaStyle(rules.Chroma.GenericEmph),
					chroma.GenericInserted:     chromaStyle(rules.Chroma.GenericInserted),
					chroma.GenericStrong:       chromaStyle(rules.Chroma.GenericStrong),
					chroma.GenericSubheading:   chromaStyle(rules.Chroma.GenericSubheading),
					chroma.Background:          chromaStyle(rules.Chroma.Background),
				}))
		}
		mutex.Unlock()
	}

	iw := NewIndentWriter(w, int(indentation+margin), func(_ io.Writer) {
		_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
	})
	defer iw.Close() //nolint:errcheck

	width := int(bs.Width(ctx))
	if bs.Current().List {
		width = listWrapWidth(width, bs.Current().Style)
	}
	width -= int(indentation + margin)
	if width < 0 {
		width = 0
	}

	if len(theme) > 0 {
		_, _ = renderText(iw, bs.Current().Style.StylePrimitive, rules.BlockPrefix)

		var highlighted bytes.Buffer
		err := quick.Highlight(&highlighted, e.Code, e.Language, formatter, theme)
		if err != nil {
			return fmt.Errorf("glamour: error highlighting code: %w", err)
		}
		if _, err := io.WriteString(iw, renderCodeBlockBackground(highlighted.String(), rules, theme, width)); err != nil {
			return fmt.Errorf("glamour: error writing highlighted code: %w", err)
		}
		_, _ = renderText(iw, bs.Current().Style.StylePrimitive, rules.BlockSuffix)
		return nil
	}

	// fallback rendering
	el := &BaseElement{
		Token: wrapCodeBlockLines(e.Code, width),
		Style: rules.StylePrimitive,
	}

	return el.Render(iw, ctx)
}

// renderCodeBlockBackground paints and right-pads each highlighted row when the
// code block style opts in to a background. The caller's indentation writer adds
// leading indentation outside this background, matching Rich's Markdown code
// block rendering where list/quote indentation remains unpainted.
func renderCodeBlockBackground(value string, rules StyleCodeBlock, theme string, width int) string {
	value = wrapCodeBlockLines(value, width)
	if rules.BackgroundColor == nil {
		return value
	}
	backgroundSGR := codeBlockBackgroundSequence(rules, theme)
	if backgroundSGR == "" || width <= 0 {
		return value
	}

	// Rich's Markdown code blocks use Syntax(..., padding=1), so add the top
	// padding row here. Chroma already emits a final empty row for fenced blocks.
	value = "\n" + value
	parts := strings.SplitAfter(value, "\n")
	for index, part := range parts {
		if part == "" {
			continue
		}
		hasNewline := strings.HasSuffix(part, "\n")
		line := strings.TrimSuffix(part, "\n")
		line = backgroundSGR + reapplyBackgroundAfterReset(line, backgroundSGR)
		if padding := width - xansi.StringWidth(line); padding > 0 {
			line += strings.Repeat(" ", padding)
		}
		line += "\x1b[0m"
		if hasNewline {
			line += "\n"
		}
		parts[index] = line
	}
	rendered := strings.Join(parts, "")
	if !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}
	return rendered
}

func wrapCodeBlockLines(value string, width int) string {
	if value == "" || width <= 0 {
		return value
	}
	lines := strings.SplitAfter(value, "\n")
	for index, line := range lines {
		if line == "" {
			continue
		}
		hasNewline := strings.HasSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\n")
		if xansi.StringWidth(line) > width {
			line = wrapCodeBlockLine(line, width)
		}
		if hasNewline {
			line += "\n"
		}
		lines[index] = line
	}
	return strings.Join(lines, "")
}

func wrapCodeBlockLine(value string, width int) string {
	if codeBlockLineStartsWithWhitespace(value) {
		return hardWrapCodeBlockLine(value, width)
	}
	return lipgloss.Wrap(value, width, " ,.;-+|")
}

func codeBlockLineStartsWithWhitespace(value string) bool {
	for _, r := range xansi.Strip(value) {
		return unicode.IsSpace(r)
	}
	return false
}

func hardWrapCodeBlockLine(value string, width int) string {
	wrapped := xansi.Hardwrap(value, width, true)
	if wrapped == value {
		return value
	}
	var buffer bytes.Buffer
	ww := lipgloss.NewWrapWriter(&buffer)
	_, _ = io.WriteString(ww, wrapped)
	_ = ww.Close()
	return buffer.String()
}

// codeBlockBackgroundSequence returns the explicit code block background color,
// falling back to the Chroma theme background when the explicit color is empty
// or invalid.
func codeBlockBackgroundSequence(rules StyleCodeBlock, theme string) string {
	if rules.BackgroundColor != nil {
		if background := styleBackgroundSequence(*rules.BackgroundColor); background != "" {
			return background
		}
	}
	if style := styles.Get(theme); style != nil {
		for _, tokenType := range []chroma.TokenType{chroma.Background, chroma.Text} {
			if background := style.Get(tokenType).Background; background.IsSet() {
				return backgroundSequence(background)
			}
		}
	}
	return ""
}

// reapplyBackgroundAfterReset preserves the block background across Chroma's
// token reset sequences. Chroma may emit either ESC[0m or ESC[m.
func reapplyBackgroundAfterReset(value, backgroundSGR string) string {
	value = strings.ReplaceAll(value, "\x1b[0m", "\x1b[0m"+backgroundSGR)
	return strings.ReplaceAll(value, "\x1b[m", "\x1b[m"+backgroundSGR)
}

func backgroundSequence(colour chroma.Colour) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", colour.Red(), colour.Green(), colour.Blue())
}

func styleBackgroundSequence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if number, err := strconv.Atoi(value); err == nil && number >= 0 && number <= 255 {
		return fmt.Sprintf("\x1b[48;5;%dm", number)
	}
	if colour := chroma.ParseColour(value); colour.IsSet() {
		return backgroundSequence(colour)
	}
	if colour := chroma.ParseColour("#" + value); colour.IsSet() {
		return backgroundSequence(colour)
	}
	return ""
}

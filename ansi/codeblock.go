package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	xansi "github.com/charmbracelet/x/ansi"
)

const (
	// The chroma style theme name used for rendering.
	chromaStyleTheme = "charm"

	// The chroma formatter name used for rendering.
	chromaFormatter = "terminal256"

	codeBlockTabWidth = 4
)

// mutex for synchronizing access to the chroma style registry.
// Related https://github.com/alecthomas/chroma/pull/650
var mutex = sync.Mutex{}

// codeBlockLexerCache avoids Chroma's expensive registry glob matching on
// every live-stream re-render. Cache both recognized languages and misses:
// misses still analyse source so unknown fences keep quick.Highlight's
// best-effort behavior. Language-less fences are rendered as plain text.
var (
	codeBlockLexerCache     sync.Map
	codeBlockFormatterCache sync.Map
	codeBlockStyleCache     sync.Map
)

type cachedCodeBlockLexer struct {
	lexer chroma.Lexer
	found bool
}

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
		if !ok {
			codeBlockStyleCache.Delete(theme)
		}
	}

	layout := childBlockLayout(ctx, indentation, margin)
	var target io.Writer
	var blockBuffer bytes.Buffer
	var iw *IndentWriter
	if layout.inList {
		target = &blockBuffer
	} else {
		iw = NewIndentWriter(w, layout.indent, func(_ io.Writer) {
			_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
		})
		target = iw
	}

	code := expandCodeBlockTabs(e.Code)
	if len(theme) > 0 {
		_, _ = renderText(target, bs.Current().Style.StylePrimitive, rules.BlockPrefix)

		faint := styleIsFaint(cascadeStylePrimitives(bs.Current().Style.StylePrimitive, rules.StylePrimitive))
		rendered, err := renderCachedCodeBlock(ctx, code, e.Language, formatter, theme, rules, layout.width, faint)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(target, rendered); err != nil {
			return fmt.Errorf("glamour: error writing highlighted code: %w", err)
		}
		_, _ = renderText(target, bs.Current().Style.StylePrimitive, rules.BlockSuffix)
		return finishListChildBlock(w, blockBuffer.String(), layout, iw)
	}

	// fallback rendering
	el := &BaseElement{
		Token: wrapCodeBlockLines(code, layout.width),
		Style: rules.StylePrimitive,
	}

	if err := el.Render(target, ctx); err != nil {
		return err
	}
	return finishListChildBlock(w, blockBuffer.String(), layout, iw)
}

func renderCachedCodeBlock(ctx RenderContext, source, language, formatter, theme string, rules StyleCodeBlock, width int, faint bool) (string, error) {
	background := ""
	if rules.BackgroundColor != nil {
		background = *rules.BackgroundColor
	}
	language = strings.TrimSpace(language)
	key := newCodeBlockCacheKey(source, language, formatter, theme, background, width, faint)
	if rendered, ok := ctx.codeBlocks.Get(key); ok {
		return rendered, nil
	}

	var highlightedCode string
	if isPlainTextCodeBlockLanguage(language) {
		var highlighted bytes.Buffer
		if err := renderPlainCodeBlock(&highlighted, source, formatter, theme); err != nil {
			return "", fmt.Errorf("glamour: error rendering plain code: %w", err)
		}
		highlightedCode = highlighted.String()
	} else {
		var highlighted bytes.Buffer
		if err := highlightCodeBlock(&highlighted, source, language, formatter, theme); err != nil {
			return "", fmt.Errorf("glamour: error highlighting code: %w", err)
		}
		highlightedCode = highlighted.String()
	}
	if faint {
		highlightedCode = forceFaintANSI(highlightedCode)
	}
	rendered := renderCodeBlockBackground(highlightedCode, rules, theme, width)
	ctx.codeBlocks.Add(key, rendered)
	return rendered, nil
}

func renderPlainCodeBlock(w io.Writer, source, formatterName, theme string) error {
	formatter := cachedChromaFormatter(formatterName)
	style := cachedChromaStyle(theme)
	if err := formatter.Format(w, style, chroma.Literator(chroma.Token{Type: chroma.Text, Value: source})); err != nil {
		return fmt.Errorf("format plain code block: %w", err)
	}
	return nil
}

func highlightCodeBlock(w io.Writer, source, language, formatterName, theme string) error {
	// This intentionally mirrors quick.Highlight, but routes lexer selection
	// through codeBlockLexer so repeated streamed renders don't rescan Chroma's
	// lexer filename globs for the same fence language.
	lexer := codeBlockLexer(language, source)
	formatter := cachedChromaFormatter(formatterName)
	style := cachedChromaStyle(theme)

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return fmt.Errorf("tokenise code block: %w", err)
	}
	if err := formatter.Format(w, style, iterator); err != nil {
		return fmt.Errorf("format code block: %w", err)
	}
	return nil
}

func cachedChromaFormatter(formatterName string) chroma.Formatter {
	if cached, ok := codeBlockFormatterCache.Load(formatterName); ok {
		return cached.(chroma.Formatter)
	}
	formatter := formatters.Get(formatterName)
	if formatter == nil {
		formatter = formatters.Fallback
	}
	cached, _ := codeBlockFormatterCache.LoadOrStore(formatterName, formatter)
	return cached.(chroma.Formatter)
}

func cachedChromaStyle(theme string) *chroma.Style {
	if cached, ok := codeBlockStyleCache.Load(theme); ok {
		return cached.(*chroma.Style)
	}
	style := styles.Get(theme)
	if style == nil {
		style = styles.Fallback
	}
	cached, _ := codeBlockStyleCache.LoadOrStore(theme, style)
	return cached.(*chroma.Style)
}

func codeBlockLexer(language, source string) chroma.Lexer {
	language = strings.TrimSpace(language)
	if language == "" {
		return plainTextCodeBlockLexer()
	}

	entry := loadCodeBlockLexer(language)
	if entry.found {
		return entry.lexer
	}
	return analysedCodeBlockLexer(source)
}

func plainTextCodeBlockLexer() chroma.Lexer {
	entry := loadCodeBlockLexer("text")
	if entry.found {
		return entry.lexer
	}
	return chroma.Coalesce(lexers.Fallback)
}

func isPlainTextCodeBlockLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "text", "txt", "plain", "plaintext":
		return true
	default:
		return false
	}
}

func loadCodeBlockLexer(language string) cachedCodeBlockLexer {
	if cached, ok := codeBlockLexerCache.Load(language); ok {
		return cached.(cachedCodeBlockLexer)
	}
	lexer := lexers.Get(language)
	entry := cachedCodeBlockLexer{found: lexer != nil}
	if entry.found {
		entry.lexer = chroma.Coalesce(lexer)
	}
	cached, _ := codeBlockLexerCache.LoadOrStore(language, entry)
	return cached.(cachedCodeBlockLexer)
}

func analysedCodeBlockLexer(source string) chroma.Lexer {
	lexer := lexers.Analyse(source)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	return chroma.Coalesce(lexer)
}

func expandCodeBlockTabs(value string) string {
	if !strings.Contains(value, "\t") {
		return value
	}
	var out strings.Builder
	column := 0
	for _, r := range value {
		switch r {
		case '\t':
			spaces := codeBlockTabWidth - column%codeBlockTabWidth
			out.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		case '\n', '\r':
			out.WriteRune(r)
			column = 0
		default:
			out.WriteRune(r)
			column += xansi.StringWidth(string(r))
		}
	}
	return out.String()
}

// renderCodeBlockBackground paints and right-pads each highlighted row when the
// code block style opts in to a background. The caller applies leading block
// indentation outside this background, matching Rich's Markdown code block
// rendering where list/quote indentation remains unpainted.
func renderCodeBlockBackground(value string, rules StyleCodeBlock, theme string, width int) string {
	value = wrapCodeBlockLines(value, width)
	if rules.BackgroundColor == nil {
		return value
	}
	backgroundSGR := codeBlockBackgroundSequence(rules, theme)
	if backgroundSGR == "" || width <= 0 {
		return value
	}

	var builder strings.Builder
	builder.Grow(len(value) + strings.Count(value, "\n")*(len(backgroundSGR)+len("\x1b[0m")))
	lastNewline := false
	for len(value) > 0 {
		nl := strings.IndexByte(value, '\n')
		if nl < 0 {
			writeCodeBlockBackgroundLine(&builder, value, backgroundSGR, width, false)
			return builder.String() + "\n"
		}
		writeCodeBlockBackgroundLine(&builder, value[:nl], backgroundSGR, width, true)
		lastNewline = true
		value = value[nl+1:]
	}
	if builder.Len() == 0 || !lastNewline {
		builder.WriteByte('\n')
	}
	return builder.String()
}

func writeCodeBlockBackgroundLine(builder *strings.Builder, line, backgroundSGR string, width int, newline bool) {
	builder.WriteString(backgroundSGR)
	writeReapplyBackgroundAfterReset(builder, line, backgroundSGR)
	if padding := width - xansi.StringWidth(line); padding > 0 {
		builder.WriteString(strings.Repeat(" ", padding))
	}
	builder.WriteString("\x1b[0m")
	if newline {
		builder.WriteByte('\n')
	}
}

func wrapCodeBlockLines(value string, width int) string {
	if value == "" || width <= 0 {
		return value
	}
	var builder strings.Builder
	changed := false
	for len(value) > 0 {
		nl := strings.IndexByte(value, '\n')
		line := value
		hasNewline := false
		if nl >= 0 {
			line = value[:nl]
			hasNewline = true
		}
		wrapped := line
		if xansi.StringWidth(line) > width {
			wrapped = wrapCodeBlockLine(line, width)
			changed = true
		}
		builder.WriteString(wrapped)
		if hasNewline {
			builder.WriteByte('\n')
			value = value[nl+1:]
		} else {
			value = ""
		}
	}
	if !changed {
		return builder.String()
	}
	return builder.String()
}

func wrapCodeBlockLine(value string, width int) string {
	return hardWrapCodeBlockLine(value, width)
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
	if style := cachedChromaStyle(theme); style != nil {
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
func writeReapplyBackgroundAfterReset(builder *strings.Builder, value, backgroundSGR string) {
	for len(value) > 0 {
		reset0 := strings.Index(value, "\x1b[0m")
		reset := strings.Index(value, "\x1b[m")
		switch {
		case reset0 < 0 && reset < 0:
			builder.WriteString(value)
			return
		case reset >= 0 && (reset0 < 0 || reset < reset0):
			builder.WriteString(value[:reset+len("\x1b[m")])
			builder.WriteString(backgroundSGR)
			value = value[reset+len("\x1b[m"):]
		default:
			builder.WriteString(value[:reset0+len("\x1b[0m")])
			builder.WriteString(backgroundSGR)
			value = value[reset0+len("\x1b[0m"):]
		}
	}
}

func styleIsFaint(style StylePrimitive) bool {
	return style.Faint != nil && *style.Faint
}

func forceFaintANSI(value string) string {
	const faintSGR = "\x1b[2m"
	if value == "" {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value) + strings.Count(value, "\x1b[")*len(faintSGR) + len(faintSGR))
	builder.WriteString(faintSGR)
	for {
		start := strings.Index(value, "\x1b[")
		if start < 0 {
			builder.WriteString(value)
			return builder.String()
		}
		builder.WriteString(value[:start])
		value = value[start:]
		end := strings.IndexByte(value, 'm')
		if end < 0 {
			builder.WriteString(value)
			return builder.String()
		}
		builder.WriteString(value[:end+1])
		builder.WriteString(faintSGR)
		value = value[end+1:]
	}
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

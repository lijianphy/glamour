package ansi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// MarginWriter is a Writer that applies indentation and padding around
// whatever you write to it.
type MarginWriter struct {
	iw *IndentWriter
}

// NewMarginWriter returns a new MarginWriter.
func NewMarginWriter(ctx RenderContext, w io.Writer, rules StyleBlock) *MarginWriter {
	return NewMarginWriterWithIndentOffset(ctx, w, rules, 0)
}

// NewMarginWriterWithIndentOffset returns a new MarginWriter whose indentation
// starts after an already established parent block content column.
func NewMarginWriterWithIndentOffset(ctx RenderContext, w io.Writer, rules StyleBlock, indentOffset int) *MarginWriter {
	return NewMarginWriterWithIndentOffsetAndWidth(ctx, w, rules, indentOffset, int(ctx.blockStack.Width(ctx)))
}

// NewMarginWriterWithIndentOffsetAndWidth returns a new MarginWriter with an
// explicit target width for callers that render inside a parent content box.
func NewMarginWriterWithIndentOffsetAndWidth(ctx RenderContext, w io.Writer, rules StyleBlock, indentOffset int, width int) *MarginWriter {
	bs := ctx.blockStack

	var indentation uint
	var margin uint
	if rules.Indent != nil {
		indentation = *rules.Indent
	}
	if rules.Margin != nil {
		margin = *rules.Margin
	}

	var padding bytes.Buffer
	_, _ = renderText(&padding, rules.StylePrimitive, " ")
	paddingText := padding.String()
	pw := NewPaddingWriterWithBatchPadding(w, max(width, 0), nil, func(_ io.Writer, count int) {
		_, _ = io.WriteString(w, strings.Repeat(paddingText, count))
	})

	ic := " "
	if rules.IndentToken != nil {
		ic = *rules.IndentToken
	}
	baseIndent := max(indentOffset, 0)
	styleIndent := int(indentation + margin)
	totalIndent := baseIndent + styleIndent
	indentUnit := 0
	iw := NewIndentWriter(pw, totalIndent, func(_ io.Writer) {
		token := ic
		if indentUnit < baseIndent {
			token = " "
		}
		style := bs.Parent().Style.StylePrimitive
		if rules.IndentTokenStyle != nil {
			style = cascadeStylePrimitives(style, *rules.IndentTokenStyle)
		}
		_, _ = renderText(w, style, token)
		indentUnit++
		if indentUnit >= totalIndent {
			indentUnit = 0
		}
	})

	return &MarginWriter{
		iw: iw,
	}
}

func marginIndentWidth(rules StyleBlock, indentOffset int) int {
	if indentOffset < 0 {
		indentOffset = 0
	}
	var indentation uint
	var margin uint
	if rules.Indent != nil {
		indentation = *rules.Indent
	}
	if rules.Margin != nil {
		margin = *rules.Margin
	}
	styleIndent := int(indentation + margin)
	token := " "
	if rules.IndentToken != nil {
		token = *rules.IndentToken
	}
	return indentOffset + styleIndent*ansi.StringWidth(token)
}

// Write writes to the margin writer and implements [io.Writer].
func (w *MarginWriter) Write(b []byte) (int, error) {
	n, err := w.iw.Write(b)
	if err != nil {
		return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
	}
	return n, nil
}

// Close closes the [MarginWriter].
func (w *MarginWriter) Close() error {
	return w.iw.Close()
}

// PaddingFunc is a function that applies padding around whatever you write to it.
type PaddingFunc = func(w io.Writer)

// BatchPaddingFunc is a function that applies count columns of padding around
// whatever you write to it.
type BatchPaddingFunc = func(w io.Writer, count int)

// PaddingWriter is a writer that applies padding around whatever you write to
// it.
type PaddingWriter struct {
	Padding      int
	PadFunc      PaddingFunc
	BatchPadFunc BatchPaddingFunc
	w            *ansiStateWriter
	lineWidth    int
	// widthPending carries an incomplete ANSI escape sequence across Write calls.
	widthPending []byte
}

// NewPaddingWriter returns a new PaddingWriter.
func NewPaddingWriter(w io.Writer, padding int, padFunc PaddingFunc) *PaddingWriter {
	return NewPaddingWriterWithBatchPadding(w, padding, padFunc, nil)
}

// NewPaddingWriterWithBatchPadding returns a new PaddingWriter using a batch
// padding callback when available.
func NewPaddingWriterWithBatchPadding(w io.Writer, padding int, padFunc PaddingFunc, batchPadFunc BatchPaddingFunc) *PaddingWriter {
	return &PaddingWriter{
		Padding:      padding,
		PadFunc:      padFunc,
		BatchPadFunc: batchPadFunc,
		w:            newANSIStateWriter(w),
	}
}

// Write writes to the padding writer.
func (w *PaddingWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		nl := bytes.IndexByte(p, '\n')
		if nl < 0 {
			w.lineWidth += w.visibleSegmentWidth(p)
			if _, err := w.w.Write(p); err != nil {
				return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
			}
			return total, nil
		}

		if nl > 0 {
			chunk := p[:nl]
			w.lineWidth += w.visibleSegmentWidth(chunk)
			if _, err := w.w.Write(chunk); err != nil {
				return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
			}
		}
		if err := w.padLine(); err != nil {
			return 0, err
		}
		if _, err := w.w.Write(p[nl : nl+1]); err != nil {
			return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
		}
		w.lineWidth = 0
		w.widthPending = w.widthPending[:0]
		p = p[nl+1:]
	}

	return total, nil
}

func (w *PaddingWriter) padLine() error {
	padding := w.Padding - w.lineWidth
	if padding <= 0 {
		return nil
	}
	if w.BatchPadFunc != nil {
		w.BatchPadFunc(w.w, padding)
		return nil
	}
	if w.PadFunc != nil {
		for range padding {
			w.PadFunc(w.w)
		}
		return nil
	}
	if _, err := io.WriteString(w.w, strings.Repeat(" ", padding)); err != nil {
		return fmt.Errorf("glamour: error writing padding: %w", err)
	}
	return nil
}

// Close closes the [PaddingWriter].
func (w *PaddingWriter) Close() error {
	return w.w.Close()
}

// IndentFunc is a function that applies indentation around whatever you write to
// it.
type IndentFunc = func(w io.Writer)

// IndentWriter is a writer that applies indentation around whatever you write to
// it.
type IndentWriter struct {
	Indent     int
	IndentFunc IndentFunc
	w          io.Writer
	pw         *ansiStateWriter
	skipIndent bool
}

// NewIndentWriter returns a new IndentWriter.
func NewIndentWriter(w io.Writer, indent int, indentFunc IndentFunc) *IndentWriter {
	return &IndentWriter{
		Indent:     indent,
		IndentFunc: indentFunc,
		pw:         newANSIStateWriter(w),
		w:          w,
	}
}

func (w *IndentWriter) resetPen() {
	style := w.pw.Style()
	link := w.pw.Link()
	if !style.IsZero() {
		_, _ = io.WriteString(w.w, ansi.ResetStyle)
	}
	if !link.IsZero() {
		_, _ = io.WriteString(w.w, ansi.ResetHyperlink())
	}
}

func (w *IndentWriter) restorePen() {
	style := w.pw.Style()
	link := w.pw.Link()
	if !style.IsZero() {
		_, _ = io.WriteString(w.w, style.String())
	}
	if !link.IsZero() {
		_, _ = io.WriteString(w.w, ansi.SetHyperlink(link.URL, link.Params))
	}
}

// Write writes to the indentation writer.
func (w *IndentWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		if !w.skipIndent {
			if err := w.writeIndent(); err != nil {
				return 0, err
			}
			w.skipIndent = true
		}

		nl := bytes.IndexByte(p, '\n')
		if nl < 0 {
			if _, err := w.pw.Write(p); err != nil {
				return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
			}
			return total, nil
		}

		if _, err := w.pw.Write(p[:nl+1]); err != nil {
			return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
		}
		w.skipIndent = false
		p = p[nl+1:]
	}

	return total, nil
}

func (w *IndentWriter) writeIndent() error {
	w.resetPen()
	defer w.restorePen()

	indent := max(w.Indent, 0)
	if w.IndentFunc != nil {
		for range indent {
			w.IndentFunc(w.pw)
		}
		return nil
	}
	if _, err := io.WriteString(w.pw, strings.Repeat(" ", indent)); err != nil {
		return fmt.Errorf("glamour: error writing indentation: %w", err)
	}
	return nil
}

// Close closes the [IndentWriter].
func (w *IndentWriter) Close() error {
	// Close the wrap writer (w.pw) before the downstream writer (w.w). w.pw
	// wraps w.w, so its Close flushes a trailing style/link reset back through
	// w.w. Closing w.w first would return its parser to the pool and nil it
	// out, turning that flush into a write on a closed writer.
	werr := w.pw.Close()

	if c, ok := w.w.(io.WriteCloser); ok {
		werr = errors.Join(werr, c.Close())
	}

	return werr
}

type ansiStateWriter struct {
	w      io.Writer
	p      *ansi.Parser
	style  uv.Style
	link   uv.Link
	closed bool
}

func newANSIStateWriter(w io.Writer) *ansiStateWriter {
	return &ansiStateWriter{w: w}
}

func (w *ansiStateWriter) Style() uv.Style {
	return w.style
}

func (w *ansiStateWriter) Link() uv.Link {
	return w.link
}

func (w *ansiStateWriter) ensureParser() *ansi.Parser {
	if w.p != nil {
		return w.p
	}
	w.p = ansi.GetParser()
	w.p.SetHandler(ansi.Handler{
		HandleCsi: func(cmd ansi.Cmd, params ansi.Params) {
			if cmd == 'm' {
				uv.ReadStyle(params, &w.style)
			}
		},
		HandleOsc: func(cmd int, data []byte) {
			if cmd == 8 {
				uv.ReadLink(data, &w.link)
			}
		},
	})
	return w.p
}

func (w *ansiStateWriter) advance(p []byte) {
	if w.p == nil && bytes.IndexByte(p, '\x1b') < 0 {
		return
	}
	parser := w.ensureParser()
	for _, b := range p {
		parser.Advance(b)
	}
}

func (w *ansiStateWriter) Write(p []byte) (int, error) {
	if w.closed {
		return len(p), nil
	}
	total := len(p)
	for len(p) > 0 {
		nl := bytes.IndexByte(p, '\n')
		if nl < 0 {
			if err := w.writeChunk(p); err != nil {
				return 0, err
			}
			return total, nil
		}
		if err := w.writeChunk(p[:nl]); err != nil {
			return 0, err
		}
		w.advance(p[nl : nl+1])
		if err := w.resetPen(); err != nil {
			return 0, err
		}
		if _, err := w.w.Write(p[nl : nl+1]); err != nil {
			return 0, fmt.Errorf("glamour: error writing newline: %w", err)
		}
		if err := w.restorePen(); err != nil {
			return 0, err
		}
		p = p[nl+1:]
	}
	return total, nil
}

func (w *ansiStateWriter) writeChunk(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	w.advance(p)
	if _, err := w.w.Write(p); err != nil {
		return fmt.Errorf("glamour: error writing bytes: %w", err)
	}
	return nil
}

func (w *ansiStateWriter) resetPen() error {
	if !w.style.IsZero() {
		if _, err := io.WriteString(w.w, ansi.ResetStyle); err != nil {
			return fmt.Errorf("glamour: error resetting style: %w", err)
		}
	}
	if !w.link.IsZero() {
		if _, err := io.WriteString(w.w, ansi.ResetHyperlink()); err != nil {
			return fmt.Errorf("glamour: error resetting hyperlink: %w", err)
		}
	}
	return nil
}

func (w *ansiStateWriter) restorePen() error {
	if !w.link.IsZero() {
		if _, err := io.WriteString(w.w, ansi.SetHyperlink(w.link.URL, w.link.Params)); err != nil {
			return fmt.Errorf("glamour: error restoring hyperlink: %w", err)
		}
	}
	if !w.style.IsZero() {
		if _, err := io.WriteString(w.w, w.style.String()); err != nil {
			return fmt.Errorf("glamour: error restoring style: %w", err)
		}
	}
	return nil
}

func (w *ansiStateWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	err := w.resetPen()
	if w.p != nil {
		ansi.PutParser(w.p)
		w.p = nil
	}
	return err
}

func (w *PaddingWriter) visibleSegmentWidth(p []byte) int {
	if len(p) == 0 {
		return 0
	}
	if len(w.widthPending) > 0 {
		combined := make([]byte, 0, len(w.widthPending)+len(p))
		combined = append(combined, w.widthPending...)
		combined = append(combined, p...)
		w.widthPending = w.widthPending[:0]
		p = combined
	}
	if width, pendingStart, ok := asciiANSIWidth(p); ok {
		if pendingStart >= 0 {
			w.widthPending = append(w.widthPending, p[pendingStart:]...)
		}
		return width
	}
	if pendingStart := trailingIncompleteANSIStart(p); pendingStart >= 0 {
		w.widthPending = append(w.widthPending, p[pendingStart:]...)
		p = p[:pendingStart]
	}
	return ansi.StringWidth(string(p))
}

func asciiANSIWidth(p []byte) (int, int, bool) {
	width := 0
	for i := 0; i < len(p); {
		b := p[i]
		if b >= 0x80 {
			return 0, -1, false
		}
		if b == '\x1b' {
			next := i + 1
			if next >= len(p) {
				return width, i, true
			}
			start := i
			var complete bool
			switch p[next] {
			case '[':
				i, complete = skipCSI(p, next+1)
			case ']':
				i, complete = skipStringTerminatedSequence(p, next+1)
			case 'P', '^', '_':
				i, complete = skipStringTerminatedSequence(p, next+1)
			default:
				i, complete = next+1, true
			}
			if !complete {
				return width, start, true
			}
			continue
		}
		if b >= 0x20 && b < 0x7f {
			width++
		}
		i++
	}
	return width, -1, true
}

func skipCSI(p []byte, i int) (int, bool) {
	for i < len(p) {
		if p[i] >= 0x40 && p[i] <= 0x7e {
			return i + 1, true
		}
		i++
	}
	return i, false
}

func skipStringTerminatedSequence(p []byte, i int) (int, bool) {
	for i < len(p) {
		if p[i] == '\a' {
			return i + 1, true
		}
		if p[i] == '\x1b' && i+1 < len(p) && p[i+1] == '\\' {
			return i + 2, true
		}
		i++
	}
	return i, false
}

func trailingIncompleteANSIStart(p []byte) int {
	for i := 0; i < len(p); i++ {
		if p[i] != '\x1b' {
			continue
		}
		next := i + 1
		if next >= len(p) {
			return i
		}
		var end int
		var complete bool
		switch p[next] {
		case '[':
			end, complete = skipCSI(p, next+1)
		case ']':
			end, complete = skipStringTerminatedSequence(p, next+1)
		case 'P', '^', '_':
			end, complete = skipStringTerminatedSequence(p, next+1)
		default:
			end, complete = next+1, true
		}
		if !complete {
			return i
		}
		i = end - 1
	}
	return -1
}

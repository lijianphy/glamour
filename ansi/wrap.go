package ansi

import (
	"bytes"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

func wrapString(value string, width int, breakpoints string) string {
	wrapped := strictWrap(value, width, breakpoints)
	if strings.IndexByte(value, '\x1b') < 0 {
		return wrapped
	}
	var out bytes.Buffer
	w := newANSIStateWriter(&out)
	_, _ = io.WriteString(w, wrapped)
	_ = w.Close()
	return out.String()
}

// strictWrap decides breaks on visible graphemes, not on ANSI style runs.
// Byte offsets refer to plain text; emit advances through the original styled
// text only when a row is committed.
func strictWrap(value string, limit int, breakpoints string) string {
	if limit < 1 {
		return value
	}
	plain := value
	if strings.Contains(value, "\x1b") {
		plain = xansi.Strip(value)
	}
	raw := value
	var out strings.Builder
	pos := 0
	emit := func(end int, keep bool) {
		for pos < end {
			n := min(end-pos, len(raw))
			if n == 0 {
				break
			}
			if i := strings.IndexByte(raw[:n], '\x1b'); i >= 0 {
				n = i
			}
			if n == 0 {
				seq, _, size, _ := xansi.DecodeSequence(raw, 0, nil)
				out.WriteString(seq)
				raw = raw[size:]
				continue
			}
			if keep {
				out.WriteString(raw[:n])
			}
			raw = raw[n:]
			pos += n
		}
	}
	var start, cells, lastBreak, breakCells, trimAt int
	// Spaces between words disappear only when they become the wrap point.
	spaceStart := -1
	g := uniseg.NewGraphemes(plain)
	for g.Next() {
		from, to := g.Positions()
		cluster := g.Str()
		r, _ := utf8.DecodeRuneInString(cluster)
		if cluster == "\n" || cluster == "\r\n" {
			if spaceStart >= 0 && cells > limit {
				emit(spaceStart, true)
				emit(from, false)
			}
			emit(to, true)
			start, cells, lastBreak, spaceStart = to, 0, to, -1
			continue
		}
		w := g.Width()
		if r != '\u00a0' && unicode.IsSpace(r) {
			if spaceStart < 0 {
				spaceStart = from
			}
			cells += max(1, w)
			trimAt, lastBreak, breakCells = spaceStart, to, cells
			continue
		}
		spaceStart = -1
		if cells+w > limit && lastBreak > start {
			emit(trimAt, true)
			emit(lastBreak, false)
			if trimAt > start {
				out.WriteByte('\n')
			}
			start, cells = lastBreak, cells-breakCells
		}
		if cells+w > limit && from > start {
			emit(from, true)
			out.WriteByte('\n')
			start, cells = from, 0
		}
		cells += w
		// Keep ASCII word and configured punctuation behavior. Elsewhere use
		// Unicode line breaks, including the boundary from ASCII into CJK.
		unicodeBoundary := g.LineBreak() != uniseg.LineDontBreak && (r >= utf8.RuneSelf || (to < len(plain) && plain[to] >= utf8.RuneSelf))
		if unicodeBoundary || r == '-' || strings.ContainsRune(breakpoints, r) {
			trimAt, lastBreak, breakCells = to, to, cells
		}
	}
	if spaceStart >= 0 && cells > limit {
		emit(spaceStart, true)
		emit(len(plain), false)
	}
	if pos == 0 {
		return value
	}
	out.WriteString(raw)
	return out.String()
}

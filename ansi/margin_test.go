package ansi

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// TestIndentWriterCloseOrder guards against a use-after-close crash in the
// writer chain. IndentWriter.pw wraps IndentWriter.w, so closing w before pw
// used to tear down the writer that pw's closing style/link reset flushes
// through, panicking on a nil ANSI parser. Closing pw first avoids that.
func TestIndentWriterCloseOrder(t *testing.T) {
	var buf bytes.Buffer

	// pw stands in for the downstream PaddingWriter: a passthrough whose own
	// underlying WrapWriter gets closed when pw is closed.
	pw := newWrapCloser(&buf)
	iw := NewIndentWriter(pw, 2, nil)

	// Write content carrying an open SGR style so closing flushes a reset.
	if _, err := io.WriteString(iw, "\x1b[1mhello\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Must not panic.
	if err := iw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// wrapCloser models glamour's PaddingWriter: it forwards writes through an
// inner lipgloss WrapWriter and closes that writer on Close.
type wrapCloser struct {
	w *lipgloss.WrapWriter
}

func newWrapCloser(w *bytes.Buffer) *wrapCloser {
	return &wrapCloser{w: lipgloss.NewWrapWriter(w)}
}

func (c *wrapCloser) Write(p []byte) (int, error) { return c.w.Write(p) }

func (c *wrapCloser) Close() error { return c.w.Close() }

func TestPaddingWriterUnicodeAndANSIWidth(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPaddingWriter(&buf, 6, nil)
	if _, err := io.WriteString(pw, "\x1b[31mA界e\u0301\x1b[m\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	line := strings.TrimSuffix(xansi.Strip(buf.String()), "\n")
	if width := xansi.StringWidth(line); width != 6 {
		t.Fatalf("width = %d, want 6 in %q", width, line)
	}
}

func TestPaddingWriterHandlesSplitANSISequences(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPaddingWriter(&buf, 6, nil)
	for _, chunk := range []string{"\x1b[", "31mred", "\x1b[", "0m\n"} {
		if _, err := io.WriteString(pw, chunk); err != nil {
			t.Fatalf("write %q: %v", chunk, err)
		}
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	line := strings.TrimSuffix(xansi.Strip(buf.String()), "\n")
	if width := xansi.StringWidth(line); width != 6 {
		t.Fatalf("width = %d, want 6 in %q", width, line)
	}
}

func TestIndentWriterRestoresStyleAndHyperlinkAcrossLines(t *testing.T) {
	var buf bytes.Buffer
	iw := NewIndentWriter(&buf, 2, nil)
	link := "\x1b]8;;https://example.com\a"
	resetLink := "\x1b]8;;\a"
	if _, err := io.WriteString(iw, link+"\x1b[31mred\nnext\x1b[0m"+resetLink); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "  \x1b[31m"+link+"next") {
		t.Fatalf("style/link were not restored after newline: %q", got)
	}
}

func TestCodeBlockRenderCacheBounds(t *testing.T) {
	cache := newCodeBlockRenderCache()
	for i := range codeBlockRenderCacheMaxEntries + 32 {
		key := newCodeBlockCacheKey(string(rune('a'+i%26)), "go", "terminal256", "monokai", "", 80, false)
		key.length = i
		cache.Add(key, strings.Repeat("x", 128))
	}
	if len(cache.entries) > codeBlockRenderCacheMaxEntries {
		t.Fatalf("cache entries = %d, want <= %d", len(cache.entries), codeBlockRenderCacheMaxEntries)
	}
	if cache.bytes > codeBlockRenderCacheMaxBytes {
		t.Fatalf("cache bytes = %d, want <= %d", cache.bytes, codeBlockRenderCacheMaxBytes)
	}
}

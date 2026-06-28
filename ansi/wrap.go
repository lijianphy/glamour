package ansi

import (
	"bytes"
	"io"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
)

func wrapString(value string, width int, breakpoints string) string {
	if strings.IndexByte(value, '\x1b') < 0 {
		return xansi.Wrap(value, width, breakpoints)
	}
	var out bytes.Buffer
	w := newANSIStateWriter(&out)
	_, _ = io.WriteString(w, xansi.Wrap(value, width, breakpoints))
	_ = w.Close()
	return out.String()
}

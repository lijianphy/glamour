package ansi

import (
	"bytes"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/parser"
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

// strictWrap is based on x/ansi.Wrap, but moves a buffered word to the next
// line when its trailing breakpoint would exceed the requested width.
func strictWrap(value string, limit int, breakpoints string) string {
	if limit < 1 {
		return value
	}

	const nonBreakingSpace = '\u00a0'
	var (
		cluster    string
		out        bytes.Buffer
		word       bytes.Buffer
		space      bytes.Buffer
		spaceWidth int
		lineWidth  int
		wordWidth  int
		state      = parser.GroundState
	)

	addSpace := func() {
		if spaceWidth == 0 && space.Len() == 0 {
			return
		}
		lineWidth += spaceWidth
		out.Write(space.Bytes())
		space.Reset()
		spaceWidth = 0
	}
	addWord := func() {
		if word.Len() == 0 {
			return
		}
		addSpace()
		lineWidth += wordWidth
		out.Write(word.Bytes())
		word.Reset()
		wordWidth = 0
	}
	addNewline := func() {
		out.WriteByte('\n')
		lineWidth = 0
		space.Reset()
		spaceWidth = 0
	}
	prepareBreakpoint := func(width int) {
		if lineWidth+spaceWidth+wordWidth+width > limit {
			if lineWidth > 0 {
				addNewline()
			} else {
				space.Reset()
				spaceWidth = 0
			}
		}
		if wordWidth+width > limit {
			addWord()
			addNewline()
		}
		addSpace()
		addWord()
	}

	for index := 0; index < len(value); {
		nextState, action := parser.Table.Transition(state, value[index])
		if nextState == parser.Utf8State {
			var width int
			cluster, width = xansi.FirstGraphemeCluster(value[index:], xansi.GraphemeWidth)
			index += len(cluster)

			r, _ := utf8.DecodeRuneInString(cluster)
			switch {
			case r != utf8.RuneError && unicode.IsSpace(r) && r != nonBreakingSpace:
				addWord()
				space.WriteRune(r)
				spaceWidth += width
			case strings.ContainsAny(cluster, breakpoints):
				prepareBreakpoint(width)
				out.WriteString(cluster)
				lineWidth += width
			default:
				if wordWidth+width > limit {
					addWord()
				}

				word.WriteString(cluster)
				wordWidth += width

				if lineWidth+wordWidth+spaceWidth > limit {
					addNewline()
				}
				if wordWidth == limit {
					addWord()
				}
			}

			state = parser.GroundState
			continue
		}

		switch action {
		case parser.PrintAction, parser.ExecuteAction:
			switch r := rune(value[index]); {
			case r == '\n':
				if wordWidth == 0 {
					if lineWidth+spaceWidth > limit {
						lineWidth = 0
					} else {
						out.Write(space.Bytes())
					}
					space.Reset()
					spaceWidth = 0
				}
				addWord()
				addNewline()
			case unicode.IsSpace(r):
				addWord()
				space.WriteRune(r)
				spaceWidth++
			case r == '-', runeContainsAny(r, breakpoints):
				prepareBreakpoint(1)
				out.WriteByte(value[index])
				lineWidth++
			default:
				if lineWidth == limit {
					addNewline()
				}

				word.WriteRune(r)
				wordWidth++

				if wordWidth == limit {
					addWord()
				}
				if lineWidth+wordWidth+spaceWidth > limit {
					addNewline()
				}
			}
		default:
			word.WriteByte(value[index])
		}

		if state != parser.Utf8State {
			state = nextState
		}
		index++
	}

	if wordWidth == 0 {
		if lineWidth+spaceWidth > limit {
			lineWidth = 0
		} else {
			out.Write(space.Bytes())
		}
		space.Reset()
		spaceWidth = 0
	}
	addWord()
	return out.String()
}

func runeContainsAny(r rune, value string) bool {
	for _, candidate := range value {
		if r == candidate {
			return true
		}
	}
	return false
}

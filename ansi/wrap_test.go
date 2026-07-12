package ansi

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

func TestStrictWrapKeepsBreakpointWithinWidth(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		width       int
		breakpoints string
		want        string
	}{
		{
			name:        "configured punctuation",
			value:       "CapabilityBundle.Execute runs in a goroutine without panic recovery.",
			width:       67,
			breakpoints: listWrapBreakpoints,
			want:        "CapabilityBundle.Execute runs in a goroutine without panic\nrecovery.",
		},
		{
			name:  "mandatory hyphen",
			value: "1234 abc-",
			width: 8,
			want:  "1234\nabc-",
		},
		{
			name:        "ANSI styled punctuation",
			value:       "\x1b[36mCapabilityBundle.Execute\x1b[0m runs in a goroutine without panic recovery.",
			width:       67,
			breakpoints: listWrapBreakpoints,
			want:        "CapabilityBundle.Execute runs in a goroutine without panic\nrecovery.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := wrapString(test.value, test.width, test.breakpoints)
			if stripped := xansi.Strip(got); stripped != test.want {
				t.Fatalf("wrapped text = %q, want %q", stripped, test.want)
			}
			for line := range strings.SplitSeq(got, "\n") {
				if width := xansi.StringWidth(line); width > test.width {
					t.Fatalf("wrapped line width = %d, want <= %d: %q", width, test.width, xansi.Strip(line))
				}
			}
		})
	}
}

func TestStrictWrapMatchesXANSIWithoutBreakpointOverflow(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		width       int
		breakpoints string
	}{
		{
			name:  "plain words",
			value: "the quick brown fox jumped over the lazy dog",
			width: 16,
		},
		{
			name:  "long word",
			value: "the quick foxxxxxxxxxxxxxxxx jumped",
			width: 12,
		},
		{
			name:  "ANSI styles",
			value: "\x1b[31mred text\x1b[0m and plain text",
			width: 12,
		},
		{
			name:  "wide graphemes",
			value: "hello 世界 and 😀 emoji",
			width: 10,
		},
		{
			name:  "preserved newlines",
			value: "first line\nsecond line",
			width: 20,
		},
		{
			name:        "configured breakpoints",
			value:       "alpha,beta gamma;delta",
			width:       14,
			breakpoints: ",;",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := strictWrap(test.value, test.width, test.breakpoints)
			want := xansi.Wrap(test.value, test.width, test.breakpoints)
			if got != want {
				t.Fatalf("strict wrap = %q, want x/ansi behavior %q", got, want)
			}
		})
	}
}

package glamour

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEntityDecodingCannotInjectTerminalCommands(t *testing.T) {
	renderer, err := NewTermRenderer(WithStandardStyle("dark"))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"before &#27;[2Jafter",
		"before &#x1b;]52;c;ZXZpbA==&#7;after",
		"before &#27;[38;2;1;2;3mafter",
		"<p>before &#27;[2Jafter</p>",
		"~~before &#27;[2Jafter~~",
	} {
		output, err := renderer.Render(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"\x1b[2J", "\x1b]52;", "\x1b[38;2;1;2;3m"} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("source %q injected %q", source, forbidden)
			}
		}
		// Parsing can separate the introducer from its printable suffix.
		plain := ansi.Strip(output)
		if !strings.Contains(plain, "before") || !strings.Contains(plain, "after") {
			t.Fatalf("safe content lost: %q", output)
		}
	}
	output, err := renderer.Render("`&#27;[2J` **bold** [link](https://example.test)")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ansi.Strip(output), "&#27;[2J") || !strings.Contains(output, "\x1b]8;") {
		t.Fatalf("literal code or generated hyperlink lost: %q", output)
	}
}

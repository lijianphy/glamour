package ansi

import "testing"

func TestDecodedSourceTextCannotIntroduceTerminalControls(t *testing.T) {
	for _, source := range []string{
		"before &#27;[2Jafter",
		"before &#x1b;]52;c;ZXZpbA==&#7;after",
		"before &#27;[38;2;1;2;3mafter",
		"before &#27;[0mafter",
		"before \x9b2Jafter",
	} {
		if got := decodeSourceText(source); got != "before after" {
			t.Errorf("decodeSourceText(%q) = %q", source, got)
		}
	}
	if got := decodeSourceText("a &amp; b\n\t\u754c"); got != "a & b\n\t\u754c" {
		t.Fatalf("safe source changed: %q", got)
	}
	ctx := NewRenderContext(Options{})
	if got := ctx.SanitizeHTML("<p>before &#27;[2Jafter</p>", false); got != "before after" {
		t.Fatalf("HTML decode reintroduced controls: %q", got)
	}
}

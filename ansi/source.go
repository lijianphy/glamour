package ansi

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// decodeSourceText filters after entity decoding, before trusted styling.
// Filtering BaseElement.Token instead would also strip generated hyperlinks
// and nested styles, which share that compositional representation.
func decodeSourceText(value string) string {
	return safeSourceText(html.UnescapeString(value))
}

func safeSourceText(value string) string {
	for _, character := range value {
		// Invalid UTF-8 can contain raw C1 escape introducers.
		if character == utf8.RuneError || unicode.IsControl(character) && character != '\n' && character != '\t' {
			return strings.Map(func(character rune) rune {
				if unicode.IsControl(character) && character != '\n' && character != '\t' {
					return -1
				}
				return character
			}, ansi.Strip(value))
		}
	}
	return value
}

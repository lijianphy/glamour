package ansi

import (
	"regexp"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TemplateFuncMap contains a few useful template helpers.
var (
	TemplateFuncMap = template.FuncMap{
		"Left": func(values ...any) string {
			s := values[0].(string)
			n := min(values[1].(int), len(s))

			return s[:n]
		},
		"Matches": func(values ...any) bool {
			ok, _ := regexp.MatchString(values[1].(string), values[0].(string))
			return ok
		},
		"Mid": func(values ...any) string {
			s := values[0].(string)
			l := min(values[1].(int), len(s))

			if len(values) > 2 { //nolint:mnd
				r := min(values[2].(int), len(s))
				return s[l:r]
			}
			return s[l:]
		},
		"Right": func(values ...any) string {
			s := values[0].(string)
			n := max(len(s)-values[1].(int), 0)

			return s[n:]
		},
		"Last": func(values ...any) string {
			return values[0].([]string)[len(values[0].([]string))-1]
		},
		// strings functions
		"Compare":      strings.Compare, // 1.5+ only
		"Contains":     strings.Contains,
		"ContainsAny":  strings.ContainsAny,
		"Count":        strings.Count,
		"EqualFold":    strings.EqualFold,
		"HasPrefix":    strings.HasPrefix,
		"HasSuffix":    strings.HasSuffix,
		"Index":        strings.Index,
		"IndexAny":     strings.IndexAny,
		"Join":         strings.Join,
		"LastIndex":    strings.LastIndex,
		"LastIndexAny": strings.LastIndexAny,
		"Repeat":       strings.Repeat,
		"Replace":      strings.Replace,
		"Split":        strings.Split,
		"SplitAfter":   strings.SplitAfter,
		"SplitAfterN":  strings.SplitAfterN,
		"SplitN":       strings.SplitN,
		"Title":        cases.Title(language.English).String,
		"ToLower":      cases.Lower(language.English).String,
		"ToTitle":      cases.Upper(language.English).String,
		"ToUpper":      strings.ToUpper,
		"Trim":         strings.Trim,
		"TrimLeft":     strings.TrimLeft,
		"TrimPrefix":   strings.TrimPrefix,
		"TrimRight":    strings.TrimRight,
		"TrimSpace":    strings.TrimSpace,
		"TrimSuffix":   strings.TrimSuffix,
	}
)

// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

// NormalizeInput cleans raw text before parsing: CRLF→LF, collapse blank lines, strip control chars.
func NormalizeInput(raw string) (string, error) {
	maxChars := appLimits.Input.MaxInputChars
	if utf8.RuneCountInString(raw) > maxChars {
		return "", NewError("ENGINE_ERR_INPUT_TOO_LARGE", StageInput,
			msg("ENGINE_ERR_INPUT_TOO_LARGE", "max_chars", maxChars))
	}

	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse 3+ blank lines → single blank line
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	// Strip control characters except tab and newline
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r == '\t' || r == '\n' || r >= 0x20 {
			b.WriteRune(r)
		}
	}

	return strings.TrimSpace(b.String()), nil
}

// SanitizeHTML removes dangerous tags and attributes using bluemonday.
func SanitizeHTML(html string) string {
	p := bluemonday.NewPolicy()
	p.AllowStandardURLs()
	p.AllowElements(
		"h1", "h2", "h3", "h4", "h5", "h6",
		"p", "br", "hr",
		"ul", "ol", "li",
		"table", "thead", "tbody", "tr", "th", "td",
		"pre", "code",
		"blockquote",
		"strong", "b", "em", "i", "del", "s", "strike",
		"a", "img",
		"div", "span", "section", "article", "main", "header", "footer", "nav",
	)
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "alt").OnElements("img")
	p.AllowAttrs("class").Globally()
	return p.Sanitize(html)
}

package session

import (
	"strings"
	"time"
	"unicode"
)

const (
	maxASCIITokens = 6
	maxCJKChars    = 12
	scanRunes      = 80
)

// isCJK reports whether r is a CJK ideograph (Han) we keep verbatim in slugs.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// keepRune reports whether r is a slug-significant character.
func keepRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	return isCJK(r)
}

// Slug converts a user prompt into a deterministic short slug.
//
// Rules (from plan): scan first 80 runes, drop paths/punctuation/newlines,
// lowercase ASCII, keep at most 6 ASCII tokens or 12 CJK characters.
func Slug(prompt string) string {
	runes := []rune(strings.TrimSpace(prompt))
	if len(runes) > scanRunes {
		runes = runes[:scanRunes]
	}

	var b strings.Builder
	lastByte := byte(0)
	cjkCount := 0
	tokenCount := 0
	inToken := false

	write := func(s string) {
		b.WriteString(s)
		if len(s) > 0 {
			lastByte = s[len(s)-1]
		}
	}

	for _, r := range runes {
		switch {
		case isCJK(r):
			if cjkCount >= maxCJKChars {
				continue
			}
			write(string(r))
			cjkCount++
			inToken = false // each CJK char is standalone
		case keepRune(r):
			if !inToken {
				if tokenCount >= maxASCIITokens {
					continue
				}
				if b.Len() > 0 && lastByte != '-' {
					write("-")
				}
				tokenCount++
				inToken = true
			}
			write(string(unicode.ToLower(r)))
		default:
			// separator: '/', whitespace, punctuation, etc.
			// The next kept ASCII token emits the boundary dash.
			inToken = false
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}

// ProvisionalTitle returns a short human-readable title for the first prompt.
// It is the first non-empty line, trimmed to a reasonable length.
func ProvisionalTitle(prompt string) string {
	line := prompt
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	r := []rune(line)
	if len(r) > 60 {
		line = strings.TrimSpace(string(r[:60]))
	}
	if line == "" {
		return "Untitled"
	}
	return line
}

// FolderName builds the post-title folder name: S-YYYYMMDD-HHMM__<slug>.
func FolderName(t time.Time, slug string) string {
	if slug == "" {
		slug = "untitled"
	}
	return "S-" + t.Format("20060102-1504") + "__" + slug
}

// UntitledFolderName builds the initial folder name before the first prompt.
func UntitledFolderName(t time.Time) string {
	return "CC Session - Untitled - " + t.Format("20060102-150405")
}

package rag

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeForEmbed canonicalizes text at the embedding boundary so queries,
// seed descriptors and RAG chunks inhabit the same normalized space.
func NormalizeForEmbed(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(value)))
	var out strings.Builder
	var previousLetter rune
	for _, current := range decomposed {
		if unicode.Is(unicode.Mn, current) {
			continue
		}
		if unicode.IsLetter(current) {
			if current == previousLetter {
				continue
			}
			previousLetter = current
		} else {
			previousLetter = 0
		}
		out.WriteRune(current)
	}
	return norm.NFC.String(out.String())
}

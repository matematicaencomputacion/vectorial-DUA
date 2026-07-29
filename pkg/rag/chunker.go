package rag

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Chunk is a retrievable fragment of the knowledge base.
type Chunk struct {
	ID        string
	Source    string
	Title     string
	Text      string
	Embedding []float32
}

// ChunkMarkdown splits markdown into heading/paragraph oriented chunks.
func ChunkMarkdown(source, content string) []Chunk {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var title string
	var buf strings.Builder
	flush := func() {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		if text == "" {
			return
		}
		if title == "" {
			title = deriveTitle(source, text)
		}
		id := fmt.Sprintf("%s#%d", filepath.ToSlash(source), len(chunks)+1)
		chunks = append(chunks, Chunk{
			ID:     id,
			Source: filepath.ToSlash(source),
			Title:  title,
			Text:   text,
		})
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flush()
			title = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			buf.WriteString(trimmed)
			buf.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			// paragraph break
			if buf.Len() > 400 {
				flush()
			} else {
				buf.WriteByte('\n')
			}
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
		if buf.Len() >= 900 {
			flush()
		}
	}
	flush()
	return chunks
}

func deriveTitle(source, text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 80)
		}
	}
	return filepath.Base(source)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Tokenize returns lowercased alphanumeric tokens for overlap metrics.
func Tokenize(s string) []string {
	s = strings.ToLower(s)
	var b strings.Builder
	var out []string
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		if len(tok) >= 3 {
			out = append(out, tok)
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

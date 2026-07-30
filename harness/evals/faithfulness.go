package evals

import (
	"sort"
	"strings"

	"github.com/vectorial-dua/avlp/pkg/rag"
)

// RAGCase evaluates faithfulness / context relevance of a live station.
type RAGCase struct {
	CaseID               string `json:"case_id"`
	Description          string `json:"description"`
	DoubtText            string `json:"doubt_text"`
	ExpectedSourceSubstr string `json:"expected_source_substr"`
}

// RAGSignals are faithfulness metrics.
type RAGSignals struct {
	Faithfulness      float64 `json:"faithfulness"`
	ContextRelevance  float64 `json:"context_relevance"`
	GroundedPrecision float64 `json:"grounded_precision,omitempty"`
	ContextCoverage   float64 `json:"context_coverage,omitempty"`
	SourceAttribution float64 `json:"source_attribution,omitempty"`
	Aggregate         float64 `json:"aggregate"`
}

// FaithfulnessMode selects the judge appropriate to extractive or generated text.
type FaithfulnessMode string

const (
	FaithfulnessExtractive FaithfulnessMode = "extractive"
	FaithfulnessGenerative FaithfulnessMode = "generative"
)

// RAGCaseResult is one RAG eval outcome.
type RAGCaseResult struct {
	CaseID         string     `json:"case_id"`
	Passed         bool       `json:"passed"`
	Signals        RAGSignals `json:"signals"`
	Sources        []string   `json:"sources"`
	ContentPreview string     `json:"content_preview,omitempty"`
	Message        string     `json:"message,omitempty"`
}

// ScoreRAGFaithfulness measures extractive grounding: chunk texts must appear in the answer.
func ScoreRAGFaithfulness(answer string, chunks []rag.ScoredChunk, expectedSourceSubstr string) RAGSignals {
	return ScoreRAGFaithfulnessMode(answer, chunks, expectedSourceSubstr, FaithfulnessExtractive)
}

// ScoreRAGFaithfulnessMode evaluates exact extraction or lexical generative grounding.
func ScoreRAGFaithfulnessMode(
	answer string,
	chunks []rag.ScoredChunk,
	expectedSourceSubstr string,
	mode FaithfulnessMode,
) RAGSignals {
	if mode == FaithfulnessGenerative {
		return scoreGenerativeFaithfulness(answer, chunks, expectedSourceSubstr)
	}
	return scoreExtractiveFaithfulness(answer, chunks, expectedSourceSubstr)
}

func scoreExtractiveFaithfulness(answer string, chunks []rag.ScoredChunk, expectedSourceSubstr string) RAGSignals {
	if len(chunks) == 0 {
		return RAGSignals{}
	}
	contained := 0
	sourceHit := 0.0
	var ctx strings.Builder
	for _, h := range chunks {
		text := strings.TrimSpace(h.Chunk.Text)
		ctx.WriteString(text)
		ctx.WriteByte(' ')
		if text != "" && strings.Contains(answer, text) {
			contained++
		}
		if expectedSourceSubstr != "" && strings.Contains(strings.ToLower(h.Chunk.Source), strings.ToLower(expectedSourceSubstr)) {
			sourceHit = 1
		}
	}
	faith := float64(contained) / float64(len(chunks))
	// relevance: expected source present + lexical overlap of answer with context
	overlap := tokenOverlap(answer, ctx.String())
	rel := 0.5*sourceHit + 0.5*overlap
	agg := 0.60*faith + 0.40*rel
	return RAGSignals{Faithfulness: faith, ContextRelevance: rel, Aggregate: agg}
}

func scoreGenerativeFaithfulness(answer string, chunks []rag.ScoredChunk, expectedSourceSubstr string) RAGSignals {
	if len(chunks) == 0 {
		return RAGSignals{}
	}
	var context strings.Builder
	for _, chunk := range chunks {
		context.WriteString(chunk.Chunk.Title)
		context.WriteByte(' ')
		context.WriteString(chunk.Chunk.Text)
		context.WriteByte(' ')
	}
	answerTerms := significantTerms(stripSourcesSection(answer))
	contextTerms := significantTerms(context.String())
	grounded := setIntersectionSize(answerTerms, contextTerms)
	precision := ratio(grounded, len(answerTerms))

	keyTerms := topContextTerms(context.String(), 12)
	covered := 0
	for term := range keyTerms {
		if _, ok := answerTerms[term]; ok {
			covered++
		}
	}
	coverage := ratio(covered, len(keyTerms))
	source := 0.0
	if expectedSourceSubstr != "" &&
		strings.Contains(strings.ToLower(answer), strings.ToLower(expectedSourceSubstr)) {
		source = 1
	}
	relevance := 0.5*source + 0.5*coverage
	aggregate := 0.6*precision + 0.4*relevance
	return RAGSignals{
		Faithfulness:      precision,
		ContextRelevance:  relevance,
		GroundedPrecision: precision,
		ContextCoverage:   coverage,
		SourceAttribution: source,
		Aggregate:         aggregate,
	}
}

func tokenOverlap(a, b string) float64 {
	ta := uniqueTokens(rag.Tokenize(a))
	tb := uniqueTokens(rag.Tokenize(b))
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for t := range ta {
		if _, ok := tb[t]; ok {
			inter++
		}
	}
	// precision-oriented: fraction of answer tokens grounded in context
	return float64(inter) / float64(len(ta))
}

func uniqueTokens(toks []string) map[string]struct{} {
	m := make(map[string]struct{}, len(toks))
	for _, t := range toks {
		m[t] = struct{}{}
	}
	return m
}

var generativeStopwords = map[string]struct{}{
	"al": {}, "como": {}, "con": {}, "de": {}, "del": {}, "el": {}, "en": {},
	"es": {}, "esa": {}, "ese": {}, "esta": {}, "este": {}, "la": {}, "las": {},
	"le": {}, "lo": {}, "los": {}, "me": {}, "mi": {}, "mis": {}, "ni": {},
	"no": {}, "o": {}, "para": {}, "por": {}, "que": {}, "se": {}, "sin": {},
	"su": {}, "sus": {}, "te": {}, "tu": {}, "tus": {}, "un": {}, "una": {},
	"unos": {}, "unas": {}, "y": {},
}

func significantTerms(text string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, token := range rag.Tokenize(rag.NormalizeForEmbed(text)) {
		if len(token) < 4 {
			continue
		}
		if _, stopword := generativeStopwords[token]; stopword {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func topContextTerms(text string, limit int) map[string]struct{} {
	counts := make(map[string]int)
	for _, token := range rag.Tokenize(rag.NormalizeForEmbed(text)) {
		if len(token) < 4 {
			continue
		}
		if _, stopword := generativeStopwords[token]; stopword {
			continue
		}
		counts[token]++
	}
	terms := make([]string, 0, len(counts))
	for term := range counts {
		terms = append(terms, term)
	}
	sort.Slice(terms, func(i, j int) bool {
		if counts[terms[i]] == counts[terms[j]] {
			return terms[i] < terms[j]
		}
		return counts[terms[i]] > counts[terms[j]]
	})
	if len(terms) > limit {
		terms = terms[:limit]
	}
	out := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		out[term] = struct{}{}
	}
	return out
}

func stripSourcesSection(answer string) string {
	lines := strings.Split(answer, "\n")
	for i, line := range lines {
		heading := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if strings.EqualFold(heading, "Fuentes") {
			return strings.Join(lines[:i], "\n")
		}
	}
	return answer
}

func setIntersectionSize(a, b map[string]struct{}) int {
	intersection := 0
	for term := range a {
		if _, ok := b[term]; ok {
			intersection++
		}
	}
	return intersection
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

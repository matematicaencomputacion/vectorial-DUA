package evals

import (
	"strings"

	"github.com/vectorial-dua/avlp/pkg/rag"
)

// RAGCase evaluates faithfulness / context relevance of a live station.
type RAGCase struct {
	CaseID              string  `json:"case_id"`
	Description         string  `json:"description"`
	DoubtText           string  `json:"doubt_text"`
	ExpectedSourceSubstr string `json:"expected_source_substr"`
}

// RAGSignals are faithfulness metrics.
type RAGSignals struct {
	Faithfulness     float64 `json:"faithfulness"`
	ContextRelevance float64 `json:"context_relevance"`
	Aggregate        float64 `json:"aggregate"`
}

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

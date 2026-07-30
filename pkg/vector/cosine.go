package vector

import (
	"math"
	"os"
	"strings"
)

// DefaultSimilarityThreshold is the cosine cutoff for static DUA node matches
// in offline HashEmbedder / plumbing mode.
const DefaultSimilarityThreshold float32 = 0.85

// CosineSimilarity returns the cosine similarity between a and b.
// Returns 0 when either vector is empty, dimension-mismatched, or zero-norm.
func CosineSimilarity(a, b []float32) float32 {
	n := len(a)
	if n == 0 || n != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := 0; i < n; i++ {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// EffectiveDefaultThreshold returns AVLP_SIMILARITY_THRESHOLD when set to a
// valid float in (0, 1], otherwise DefaultSimilarityThreshold (0.85).
//
// 0.85 is calibrated for hash/plumbing. Semantic embedders (e.g. bge-m3)
// typically need a lower cutoff (~0.6 as a starting point); validate empirically.
func EffectiveDefaultThreshold() float32 {
	return ResolveEffectiveThreshold(strings.TrimSpace(os.Getenv("AVLP_CONFIG_PATH"))).Value
}

// ResolveThreshold returns the query threshold when in (0, 1]; otherwise the
// effective default (env override or DefaultSimilarityThreshold).
func ResolveThreshold(requested float32) float32 {
	if requested > 0 && requested <= 1 {
		return requested
	}
	return EffectiveDefaultThreshold()
}

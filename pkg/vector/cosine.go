package vector

import "math"

// DefaultSimilarityThreshold is the cosine cutoff for static DUA node matches.
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

// ResolveThreshold returns the query threshold or the default when unset/invalid.
func ResolveThreshold(requested float32) float32 {
	if requested <= 0 || requested > 1 {
		return DefaultSimilarityThreshold
	}
	return requested
}

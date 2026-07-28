package vector

import "fmt"

// ContentEmbedDims is the shared dimensionality of pedagogical content embeddings
// (routing index, RAG chunks, query_embedding, live-button vector_delta).
//
// This is NOT the learner preference profile V_e (5 axes: Dominio, Sensorial,
// Frustracion, Ritmo, Autonomia). Content space and preference space must never
// be mixed via silent truncation.
const ContentEmbedDims = 64

// FitContentEmbedding projects a vector into content embedding space.
//
// Rules (explicit; no silent truncate-to-5):
//   - len == ContentEmbedDims: defensive copy
//   - 0 < len < ContentEmbedDims: zero-pad (legacy short seeds / queries)
//   - len == 0 or len > ContentEmbedDims: error
//
// Callers that need preference-axis updates must use a documented V_e path
// (preference_delta), not this function.
func FitContentEmbedding(v []float32) ([]float32, error) {
	n := len(v)
	switch {
	case n == 0:
		return nil, fmt.Errorf("content embedding is empty; want %d dims", ContentEmbedDims)
	case n > ContentEmbedDims:
		return nil, fmt.Errorf("content embedding length %d exceeds ContentEmbedDims=%d (refusing silent truncate)", n, ContentEmbedDims)
	case n == ContentEmbedDims:
		return append([]float32(nil), v...), nil
	default:
		out := make([]float32, ContentEmbedDims)
		copy(out, v)
		return out, nil
	}
}

// MustFitContentEmbedding is FitContentEmbedding for static seed literals.
func MustFitContentEmbedding(v []float32) []float32 {
	out, err := FitContentEmbedding(v)
	if err != nil {
		panic(err)
	}
	return out
}

package vector

import "fmt"

// ContentEmbedDims is the shared dimensionality of pedagogical content embeddings
// (routing index, RAG chunks, query_embedding, live-button vector_delta).
//
// This is NOT the learner preference profile V_e (5 axes: Dominio, Sensorial,
// Frustracion, Ritmo, Autonomia). Content space and preference space must never
// be mixed via silent truncation.
const ContentEmbedDims = 64

// FitContentEmbedding projects a vector into the default hash offline space
// (ContentEmbedDims). Prefer FitIndexEmbedding when the active index may use
// remote embedder dimensionality.
func FitContentEmbedding(v []float32) ([]float32, error) {
	return FitIndexEmbedding(v, ContentEmbedDims)
}

// FitIndexEmbedding validates or projects v into an index embedding space.
//
// Rules (explicit; no silent truncate):
//   - len == wantDims: defensive copy
//   - 0 < len < wantDims and wantDims == ContentEmbedDims: zero-pad (legacy hash seeds)
//   - len == 0 or any other mismatch: error
func FitIndexEmbedding(v []float32, wantDims int) ([]float32, error) {
	if wantDims <= 0 {
		return nil, fmt.Errorf("index dims must be positive")
	}
	n := len(v)
	switch {
	case n == 0:
		return nil, fmt.Errorf("embedding is empty; want %d dims", wantDims)
	case n == wantDims:
		return append([]float32(nil), v...), nil
	case n < wantDims && wantDims == ContentEmbedDims:
		out := make([]float32, wantDims)
		copy(out, v)
		return out, nil
	case n > wantDims:
		return nil, fmt.Errorf("embedding length %d exceeds index dims %d (refusing silent truncate)", n, wantDims)
	default:
		return nil, fmt.Errorf("embedding length %d does not match index dims %d (refusing silent pad)", n, wantDims)
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

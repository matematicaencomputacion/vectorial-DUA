package dua

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/vectorial-dua/avlp/pkg/rag"
)

// Mutator appends live botonera entries from novel doubts via RAG.
type Mutator struct {
	Registry  *Registry
	Retriever *rag.Retriever
}

// MutateRequest is the input for "tengo una duda diferente".
type MutateRequest struct {
	NodeID         string
	StudentID      string
	DoubtText      string
	QueryEmbedding []float32
	Frustration    float32
}

// MutateResult is the new button and updated node snapshot.
type MutateResult struct {
	Button InteractiveButton
	Node   *InteractiveVideoNode
}

// Mutate retrieves RAG context and appends a live PLAY_CLIP button grounded in sources.
func (m *Mutator) Mutate(ctx context.Context, req MutateRequest) (MutateResult, error) {
	if m.Registry == nil {
		return MutateResult{}, fmt.Errorf("registry is nil")
	}
	if strings.TrimSpace(req.DoubtText) == "" {
		return MutateResult{}, fmt.Errorf("doubt_text is required")
	}
	if _, ok := m.Registry.Get(req.NodeID); !ok {
		return MutateResult{}, fmt.Errorf("interactive node not found: %s", req.NodeID)
	}

	var sources []string
	if m.Retriever != nil {
		hits, err := m.Retriever.RetrieveText(ctx, req.DoubtText)
		if err != nil {
			return MutateResult{}, err
		}
		sources = rag.Sources(hits)
	}

	btnID, err := newButtonULID()
	if err != nil {
		return MutateResult{}, err
	}

	delta := req.QueryEmbedding
	if len(delta) == 0 && m.Retriever != nil {
		delta, err = m.Retriever.Embedder.Embed(ctx, req.DoubtText)
		if err != nil {
			return MutateResult{}, err
		}
		if len(delta) > 5 {
			delta = delta[:5]
		}
	}
	if len(delta) == 0 {
		delta = []float32{0.1, 0.1, 0.2, 0.1, 0.1}
	}

	label := truncateLabel(req.DoubtText, 64)
	if len(sources) > 0 {
		label = truncateLabel(req.DoubtText, 48) + " [" + filepathBase(sources[0]) + "]"
	}
	mediaURL := "live://interactive/" + req.NodeID + "/" + btnID

	btn := InteractiveButton{
		IDBtn:           "live_" + btnID,
		Label:           "LIVE: " + label,
		ActionType:      ActionPlayClip,
		MediaURL:        mediaURL,
		TimestampStart:  0,
		TimestampEnd:    60,
		VectorDelta:     append([]float32(nil), delta...),
		IsLiveGenerated: true,
	}

	node, err := m.Registry.AppendButton(req.NodeID, btn)
	if err != nil {
		return MutateResult{}, err
	}
	return MutateResult{Button: btn, Node: node}, nil
}

func newButtonULID() (string, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), ulid.Monotonic(rand.Reader, 0))
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func truncateLabel(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func filepathBase(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

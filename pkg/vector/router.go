package vector

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LiveGenerator materializes a station when no static node matches.
// Implemented by pkg/livestation.Generator (interface avoids import cycles).
type LiveGenerator interface {
	GenerateLive(ctx context.Context, req LiveRequest) (LiveResult, error)
}

// LiveRequest is the miss-path payload for live station generation.
type LiveRequest struct {
	StudentID      string
	DoubtText      string
	QueryEmbedding []float32
	Frustration    float32
	Dimension      string
	Format         string
	TrackingULID   string
}

// LiveResult is a generated pedagogical station.
type LiveResult struct {
	Node         Node
	Content      string
	Sources      []string
	TrackingULID string
}

// RouteOutcome is either a static/live DUA match or a pending live station.
type RouteOutcome struct {
	Matched           bool
	IsLiveGenerated   bool
	Node              Node
	Similarity        float32
	TrackingULID      string
	LiveStatus        string
	LiveMessage       string
	LiveContent       string
	RetrievedSources  []string
	NodeNotFoundEvent *NodeNotFoundEvent
}

// Router resolves pedagogical nodes from student query embeddings.
type Router struct {
	Index   *Index
	Bus     *EventBus
	Live    LiveGenerator // optional RAG pipeline
	Enabled bool          // when false, miss stays pending-only
}

// NewRouter wires an index and event bus.
func NewRouter(index *Index, bus *EventBus) *Router {
	if bus == nil {
		bus = NewEventBus()
	}
	return &Router{Index: index, Bus: bus, Enabled: RAGEnabledFromEnv()}
}

// RAGEnabledFromEnv reads AVLP_RAG_ENABLED (default true).
func RAGEnabledFromEnv() bool {
	v := strings.TrimSpace(os.Getenv("AVLP_RAG_ENABLED"))
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return true
	}
	return b
}

// QueryOptions carries optional text/session hints for RAG.
type QueryOptions struct {
	DoubtText   string
	Frustration float32
	Dimension   string
	Format      string
}

// QueryNearest finds a static node above threshold or triggers live-station fallback.
func (r *Router) QueryNearest(studentID string, query []float32, threshold float32) (RouteOutcome, error) {
	return r.QueryNearestWithOptions(context.Background(), studentID, query, threshold, QueryOptions{})
}

// QueryNearestWithOptions is the full miss→RAG path.
func (r *Router) QueryNearestWithOptions(ctx context.Context, studentID string, query []float32, threshold float32, opt QueryOptions) (RouteOutcome, error) {
	th := ResolveThreshold(threshold)
	fitted, err := FitContentEmbedding(query)
	if err != nil {
		return RouteOutcome{}, fmt.Errorf("query embedding: %w", err)
	}
	match := r.Index.Nearest(fitted)

	if match.Found && match.Similarity >= th {
		return RouteOutcome{
			Matched:    true,
			Node:       match.Node,
			Similarity: match.Similarity,
		}, nil
	}

	tracking, err := NewTrackingULID()
	if err != nil {
		return RouteOutcome{}, err
	}
	eventID, err := NewEventID()
	if err != nil {
		return RouteOutcome{}, err
	}

	best := float32(0)
	if match.Found {
		best = match.Similarity
	}

	evt := NodeNotFoundEvent{
		EventID:        eventID,
		TrackingULID:   tracking,
		StudentID:      studentID,
		QueryEmbedding: append([]float32(nil), fitted...),
		BestSimilarity: best,
		Threshold:      th,
		Timestamp:      time.Now().UTC(),
	}
	r.Bus.EmitNodeNotFound(evt)

	if r.Enabled && r.Live != nil {
		live, err := r.Live.GenerateLive(ctx, LiveRequest{
			StudentID:      studentID,
			DoubtText:      opt.DoubtText,
			QueryEmbedding: fitted,
			Frustration:    opt.Frustration,
			Dimension:      opt.Dimension,
			Format:         opt.Format,
			TrackingULID:   tracking,
		})
		if err == nil {
			return RouteOutcome{
				Matched:          true,
				IsLiveGenerated:  true,
				Node:             live.Node,
				Similarity:       best,
				TrackingULID:     live.TrackingULID,
				LiveContent:      live.Content,
				RetrievedSources: append([]string(nil), live.Sources...),
				NodeNotFoundEvent: &evt,
			}, nil
		}
		// fall through to pending on RAG failure
	}

	return RouteOutcome{
		Matched:           false,
		Similarity:        best,
		TrackingULID:      tracking,
		LiveStatus:        "in_progress",
		LiveMessage:       "No static DUA node met the similarity threshold; live station generation requested",
		NodeNotFoundEvent: &evt,
	}, nil
}

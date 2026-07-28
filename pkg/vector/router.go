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
	Live    LiveGenerator  // optional RAG pipeline
	Ledger  *StationLedger // pending station registry
	Enabled bool           // when false, miss stays pending-only
}

// NewRouter wires an index and event bus with a station ledger.
func NewRouter(index *Index, bus *EventBus) *Router {
	if bus == nil {
		bus = NewEventBus()
	}
	return &Router{
		Index:   index,
		Bus:     bus,
		Ledger:  NewStationLedger(StationTTLFromEnv()),
		Enabled: RAGEnabledFromEnv(),
	}
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
	fitted, err := FitIndexEmbedding(query, r.Index.Dims())
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

	liveReq := LiveRequest{
		StudentID:      studentID,
		DoubtText:      opt.DoubtText,
		QueryEmbedding: fitted,
		Frustration:    opt.Frustration,
		Dimension:      opt.Dimension,
		Format:         opt.Format,
		TrackingULID:   tracking,
	}
	if r.Ledger != nil {
		r.Ledger.RegisterInProgress(tracking, studentID, liveReq)
	}

	if r.Enabled && r.Live != nil {
		// Acquire Retrying before GenerateLive so concurrent LookupStation polls
		// do not start a second generation for the same ULID.
		if r.Ledger != nil {
			if _, should, _ := r.Ledger.TryBeginRetry(tracking); !should {
				status := StationInProgress
				if rec := r.Ledger.Get(tracking); rec != nil {
					status = rec.Status
				}
				return RouteOutcome{
					Matched:           false,
					Similarity:        best,
					TrackingULID:      tracking,
					LiveStatus:        status,
					LiveMessage:       pendingStudentMessage(status),
					NodeNotFoundEvent: &evt,
				}, nil
			}
		}
		live, err := r.Live.GenerateLive(ctx, liveReq)
		if err == nil {
			if r.Ledger != nil {
				r.Ledger.MarkReady(tracking, live)
			}
			return RouteOutcome{
				Matched:           true,
				IsLiveGenerated:   true,
				Node:              live.Node,
				Similarity:        best,
				TrackingULID:      live.TrackingULID,
				LiveContent:       live.Content,
				RetrievedSources:  append([]string(nil), live.Sources...),
				NodeNotFoundEvent: &evt,
			}, nil
		}
		if r.Ledger != nil {
			r.Ledger.MarkFailed(tracking, err.Error())
		}
		// fall through to pending on RAG failure
	}

	status := StationInProgress
	if r.Ledger != nil {
		if rec := r.Ledger.Get(tracking); rec != nil {
			status = rec.Status
		}
	}

	return RouteOutcome{
		Matched:           false,
		Similarity:        best,
		TrackingULID:      tracking,
		LiveStatus:        status,
		LiveMessage:       pendingStudentMessage(status),
		NodeNotFoundEvent: &evt,
	}, nil
}

func pendingStudentMessage(status string) string {
	switch status {
	case StationFailed:
		return "No encontré material verificado todavía; probemos reformular la duda juntos."
	default:
		return "Estamos preparando una estación para tu duda; en un momento va a estar lista. Tu pregunta vale la pena."
	}
}

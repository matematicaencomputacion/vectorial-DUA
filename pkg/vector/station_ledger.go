package vector

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Station lifecycle statuses for pending live stations.
const (
	StationInProgress = "in_progress"
	StationReady      = "ready"
	StationFailed     = "failed"
)

const defaultStationTTL = 24 * time.Hour

// StationRecord is one tracked live-station request.
type StationRecord struct {
	TrackingULID string
	StudentID    string
	Status       string
	Request      LiveRequest // retained for lazy retry
	Result       *LiveResult // set when ready
	FailCause    string      // internal only; never expose raw to students
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Clone returns a deep-ish copy safe for callers.
func (r *StationRecord) Clone() *StationRecord {
	if r == nil {
		return nil
	}
	out := *r
	out.Request.QueryEmbedding = append([]float32(nil), r.Request.QueryEmbedding...)
	if r.Result != nil {
		res := *r.Result
		res.Sources = append([]string(nil), r.Result.Sources...)
		res.Node.Embedding = append([]float32(nil), r.Result.Node.Embedding...)
		out.Result = &res
	}
	return &out
}

// StationLedger tracks pending/ready/failed live stations by tracking_ulid.
type StationLedger struct {
	mu   sync.Mutex
	byID map[string]*StationRecord
	ttl  time.Duration
	now  func() time.Time
}

// NewStationLedger creates an empty ledger with the given TTL (≤0 → 24h).
func NewStationLedger(ttl time.Duration) *StationLedger {
	if ttl <= 0 {
		ttl = defaultStationTTL
	}
	return &StationLedger{
		byID: make(map[string]*StationRecord),
		ttl:  ttl,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// StationTTLFromEnv reads AVLP_STATION_TTL (Go duration, default 24h).
func StationTTLFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("AVLP_STATION_TTL"))
	if v == "" {
		return defaultStationTTL
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return defaultStationTTL
	}
	return d
}

// RegisterInProgress inserts a new in_progress record (overwrites same ULID).
func (l *StationLedger) RegisterInProgress(trackingULID, studentID string, req LiveRequest) {
	if l == nil || trackingULID == "" {
		return
	}
	now := l.now()
	req.TrackingULID = trackingULID
	req.StudentID = studentID
	req.QueryEmbedding = append([]float32(nil), req.QueryEmbedding...)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(now)
	l.byID[trackingULID] = &StationRecord{
		TrackingULID: trackingULID,
		StudentID:    studentID,
		Status:       StationInProgress,
		Request:      req,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// MarkReady stores a successful generation result.
func (l *StationLedger) MarkReady(trackingULID string, result LiveResult) {
	if l == nil || trackingULID == "" {
		return
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(now)
	rec, ok := l.byID[trackingULID]
	if !ok {
		rec = &StationRecord{
			TrackingULID: trackingULID,
			CreatedAt:    now,
		}
		l.byID[trackingULID] = rec
	}
	res := result
	res.Sources = append([]string(nil), result.Sources...)
	res.Node.Embedding = append([]float32(nil), result.Node.Embedding...)
	res.TrackingULID = trackingULID
	rec.Status = StationReady
	rec.Result = &res
	rec.FailCause = ""
	rec.UpdatedAt = now
}

// MarkFailed records an internal failure cause (not student-facing).
func (l *StationLedger) MarkFailed(trackingULID, cause string) {
	if l == nil || trackingULID == "" {
		return
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(now)
	rec, ok := l.byID[trackingULID]
	if !ok {
		return
	}
	rec.Status = StationFailed
	rec.FailCause = cause
	rec.Result = nil
	rec.UpdatedAt = now
}

// Get returns a clone of the record or nil if missing/expired.
func (l *StationLedger) Get(trackingULID string) *StationRecord {
	if l == nil || trackingULID == "" {
		return nil
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(now)
	rec, ok := l.byID[trackingULID]
	if !ok {
		return nil
	}
	return rec.Clone()
}

// Len returns the number of live entries (tests).
func (l *StationLedger) Len() int {
	if l == nil {
		return 0
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(now)
	return len(l.byID)
}

func (l *StationLedger) purgeLocked(now time.Time) {
	if l.ttl <= 0 {
		return
	}
	for id, rec := range l.byID {
		if now.Sub(rec.CreatedAt) > l.ttl {
			delete(l.byID, id)
		}
	}
}

// LookupStation returns the station record, optionally retrying generation when
// status is failed or still in_progress and a LiveGenerator is available.
func (r *Router) LookupStation(ctx context.Context, trackingULID, studentID string) (*StationRecord, error) {
	if r == nil || r.Ledger == nil {
		return nil, fmt.Errorf("station ledger unavailable")
	}
	rec := r.Ledger.Get(trackingULID)
	if rec == nil {
		return nil, nil
	}
	if studentID != "" && rec.StudentID != "" && studentID != rec.StudentID {
		// Hide existence from wrong student (PR 5.2 will map to NotFound).
		return nil, nil
	}
	if rec.Status == StationReady {
		return rec, nil
	}
	if !r.Enabled || r.Live == nil {
		return rec, nil
	}
	if rec.Status != StationFailed && rec.Status != StationInProgress {
		return rec, nil
	}

	live, err := r.Live.GenerateLive(ctx, rec.Request)
	if err != nil {
		r.Ledger.MarkFailed(trackingULID, err.Error())
		return r.Ledger.Get(trackingULID), nil
	}
	r.Ledger.MarkReady(trackingULID, live)
	return r.Ledger.Get(trackingULID), nil
}

package knowledge

import (
	"context"
	"strings"
	"sync"
	"time"
)

// ConceptVisitStore records which concepts a student has met.
type ConceptVisitStore interface {
	RecordVisit(ctx context.Context, studentID string, id ConceptID) error
	Visited(ctx context.Context, studentID string) (map[ConceptID]time.Time, error)
	HasVisited(ctx context.Context, studentID string, id ConceptID) (bool, error)
}

// MemoryConceptVisitStore is an in-process visit ledger (tests / no path).
type MemoryConceptVisitStore struct {
	mu     sync.RWMutex
	visits map[string]map[ConceptID]time.Time // student → concept → when
	now    func() time.Time
}

// NewMemoryConceptVisitStore builds an empty in-memory store.
func NewMemoryConceptVisitStore() *MemoryConceptVisitStore {
	return &MemoryConceptVisitStore{
		visits: make(map[string]map[ConceptID]time.Time),
		now:    time.Now,
	}
}

// RecordVisit implements ConceptVisitStore.
func (s *MemoryConceptVisitStore) RecordVisit(_ context.Context, studentID string, id ConceptID) error {
	if s == nil {
		return nil
	}
	studentID = strings.TrimSpace(studentID)
	if studentID == "" || id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.visits[studentID] == nil {
		s.visits[studentID] = make(map[ConceptID]time.Time)
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	s.visits[studentID][id] = now
	return nil
}

// Visited implements ConceptVisitStore.
func (s *MemoryConceptVisitStore) Visited(_ context.Context, studentID string) (map[ConceptID]time.Time, error) {
	out := make(map[ConceptID]time.Time)
	if s == nil {
		return out, nil
	}
	studentID = strings.TrimSpace(studentID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, at := range s.visits[studentID] {
		out[id] = at
	}
	return out, nil
}

// HasVisited implements ConceptVisitStore.
func (s *MemoryConceptVisitStore) HasVisited(ctx context.Context, studentID string, id ConceptID) (bool, error) {
	visited, err := s.Visited(ctx, studentID)
	if err != nil {
		return false, err
	}
	_, ok := visited[id]
	return ok, nil
}

package dua

import (
	"fmt"
	"sync"
)

const (
	// VeDims is learner preference vector dimensionality:
	// [Dominio, Sensorial, Frustracion, Ritmo, Autonomia].
	VeDims = 5
)

// ProfileRepository is the consumer-facing contract for learner V_e.
// Get/Apply are the minimum surface used by InteractionStore and router handlers.
type ProfileRepository interface {
	Get(studentID string) []float32
	Apply(studentID string, delta []float32) ([]float32, error)
}

// DefaultVe returns the neutral learner profile baseline.
func DefaultVe() []float32 {
	return []float32{0.5, 0.5, 0.4, 0.5, 0.5}
}

// ApplyDelta applies a preference delta in V_e space.
//
// Guard rule for Ola 1:
//   - len(delta) MUST equal VeDims, otherwise returns error.
//   - No blending with other sources is performed here.
func ApplyDelta(ve, delta []float32) ([]float32, error) {
	if len(delta) != VeDims {
		return nil, fmt.Errorf("invalid preference delta dims: got %d want %d", len(delta), VeDims)
	}

	out := DefaultVe()
	if len(ve) >= VeDims {
		copy(out, ve[:VeDims])
	}
	for i := 0; i < VeDims; i++ {
		out[i] = clamp01(out[i] + delta[i])
	}
	return out, nil
}

func clamp01(v float32) float32 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// ProfileStore keeps V_e in memory per student.
// It implements ProfileRepository.
type ProfileStore struct {
	mu   sync.RWMutex
	byID map[string][]float32
}

// NewProfileStore creates an empty in-memory profile store.
func NewProfileStore() *ProfileStore {
	return &ProfileStore{byID: make(map[string][]float32)}
}

// Get returns a defensive copy of the student profile or DefaultVe when absent.
func (s *ProfileStore) Get(studentID string) []float32 {
	if s == nil {
		return DefaultVe()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cur, ok := s.byID[studentID]
	if !ok {
		return DefaultVe()
	}
	out := make([]float32, VeDims)
	copy(out, cur)
	return out
}

// Apply applies a preference delta to the stored V_e and returns the updated copy.
// Returns error when delta dimensionality does not match VeDims.
func (s *ProfileStore) Apply(studentID string, delta []float32) ([]float32, error) {
	if s == nil {
		return nil, fmt.Errorf("profile store is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.byID[studentID]
	if !ok {
		cur = DefaultVe()
	}
	next, err := ApplyDelta(cur, delta)
	if err != nil {
		return nil, err
	}
	s.byID[studentID] = append([]float32(nil), next...)
	return append([]float32(nil), next...), nil
}

// Snapshot returns a deep copy of all stored profiles.
// Used by FileProfileStore for persistence; not part of ProfileRepository
// so consumers stay on the Get/Apply minimum contract.
func (s *ProfileStore) Snapshot() map[string][]float32 {
	if s == nil {
		return map[string][]float32{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]float32, len(s.byID))
	for id, ve := range s.byID {
		out[id] = append([]float32(nil), ve...)
	}
	return out
}

// ReplaceAll replaces in-memory profiles with the provided map (defensive copies).
// Invalid entries (wrong dims) are skipped. Used when loading a snapshot.
func (s *ProfileStore) ReplaceAll(profiles map[string][]float32) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = make(map[string][]float32, len(profiles))
	for id, ve := range profiles {
		if len(ve) != VeDims {
			continue
		}
		s.byID[id] = append([]float32(nil), ve...)
	}
}

var _ ProfileRepository = (*ProfileStore)(nil)

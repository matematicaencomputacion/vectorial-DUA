package dua

import "sync"

// InteractionStore tracks which subtopics a student opened (in-memory).
type InteractionStore struct {
	mu       sync.RWMutex
	seen     map[string]map[string]struct{} // key: student|parent → set of subtopic ids
	profiles ProfileRepository              // optional; nil => tracking only
	Logf     Logf                           // optional; nil = silent
}

// NewInteractionStore creates an empty store.
func NewInteractionStore() *InteractionStore {
	return &InteractionStore{seen: make(map[string]map[string]struct{})}
}

// NewInteractionStoreWithProfiles creates a store with optional profile updates.
// Passing nil keeps tracking-only behavior.
func NewInteractionStoreWithProfiles(profiles ProfileRepository) *InteractionStore {
	return &InteractionStore{
		seen:     make(map[string]map[string]struct{}),
		profiles: profiles,
	}
}

func interactionKey(studentID, parentNodeID string) string {
	return studentID + "|" + parentNodeID
}

// Record marks a subtopic as opened and optionally applies preference delta.
// Touch tracking always succeeds even when delta application fails.
func (s *InteractionStore) Record(studentID, parentNodeID, subtopicID string, delta []float32) {
	if s == nil {
		return
	}

	// Always track the touch first.
	s.mu.Lock()
	k := interactionKey(studentID, parentNodeID)
	if s.seen[k] == nil {
		s.seen[k] = make(map[string]struct{})
	}
	s.seen[k][subtopicID] = struct{}{}
	profiles := s.profiles
	s.mu.Unlock()

	// Empty delta is valid: no profile update needed.
	if profiles == nil || len(delta) == 0 {
		return
	}
	if _, err := profiles.Apply(studentID, delta); err != nil {
		s.Logf.printf("interaction profile delta skipped student=%s parent=%s subtopic=%s: %v",
			studentID, parentNodeID, subtopicID, err)
	}
}

// HasOpened reports whether the student opened a subtopic under a parent node.
func (s *InteractionStore) HasOpened(studentID, parentNodeID, subtopicID string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := s.seen[interactionKey(studentID, parentNodeID)]
	if set == nil {
		return false
	}
	_, ok := set[subtopicID]
	return ok
}

// OpenedList returns all opened subtopic ids for a student/node.
func (s *InteractionStore) OpenedList(studentID, parentNodeID string) []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := s.seen[interactionKey(studentID, parentNodeID)]
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

package dua

import (
	"fmt"
	"strings"
	"sync"
)

const DefaultMaxHierarchyDepth = 3

// SubtopicDepthLevel is the fractal depth of a hierarchical subtopic.
type SubtopicDepthLevel string

const (
	DepthMacro     SubtopicDepthLevel = "macro"
	DepthComponent SubtopicDepthLevel = "component"
	DepthMicro     SubtopicDepthLevel = "micro"
)

// SubtopicNode is a recursive optional branch in the accordion navigator.
type SubtopicNode struct {
	SubtopicID      string             `json:"subtopic_id"`
	Title           string             `json:"title"`
	DepthLevel      SubtopicDepthLevel `json:"depth_level"`
	IsOptional      bool               `json:"is_optional"`
	MediaURL        string             `json:"media_url,omitempty"`
	DurationSeconds int32              `json:"duration_seconds,omitempty"`
	OrbitDelta      []float32          `json:"orbit_delta,omitempty"`
	ChildSubtopics  []SubtopicNode     `json:"child_subtopics,omitempty"`
}

// DUAHierarchicalTree is the optional fractal navigator under a root node.
type DUAHierarchicalTree struct {
	MainTopicTitle string         `json:"main_topic_title"`
	MacroMediaURL  string         `json:"macro_media_url"`
	Subtopics      []SubtopicNode `json:"subtopics"`
}

// Validate checks the hierarchical tree (unique ids, max depth).
func (t *DUAHierarchicalTree) Validate() error {
	return t.ValidateMaxDepth(DefaultMaxHierarchyDepth)
}

// ValidateMaxDepth validates with an explicit depth budget.
func (t *DUAHierarchicalTree) ValidateMaxDepth(maxDepth int) error {
	if t == nil {
		return fmt.Errorf("hierarchy is nil")
	}
	if strings.TrimSpace(t.MainTopicTitle) == "" {
		return fmt.Errorf("main_topic_title is required")
	}
	if strings.TrimSpace(t.MacroMediaURL) == "" {
		return fmt.Errorf("macro_media_url is required")
	}
	if len(t.Subtopics) == 0 {
		return fmt.Errorf("subtopics must not be empty")
	}
	seen := map[string]struct{}{}
	for i, s := range t.Subtopics {
		if err := validateSubtopic(&s, 1, maxDepth, seen); err != nil {
			return fmt.Errorf("subtopics[%d]: %w", i, err)
		}
	}
	return nil
}

func validateSubtopic(s *SubtopicNode, level, maxDepth int, seen map[string]struct{}) error {
	if s == nil {
		return fmt.Errorf("nil subtopic")
	}
	if level > maxDepth {
		return fmt.Errorf("hierarchy exceeds max depth %d at %s", maxDepth, s.SubtopicID)
	}
	if strings.TrimSpace(s.SubtopicID) == "" || strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("subtopic_id and title required")
	}
	if _, ok := seen[s.SubtopicID]; ok {
		return fmt.Errorf("duplicate subtopic_id: %s", s.SubtopicID)
	}
	seen[s.SubtopicID] = struct{}{}
	switch s.DepthLevel {
	case DepthMacro, DepthComponent, DepthMicro, "":
		// empty allowed → inferred by level
	default:
		return fmt.Errorf("invalid depth_level: %s", s.DepthLevel)
	}
	if strings.TrimSpace(s.MediaURL) == "" && len(s.ChildSubtopics) == 0 {
		return fmt.Errorf("leaf subtopic requires media_url")
	}
	for i := range s.ChildSubtopics {
		if err := validateSubtopic(&s.ChildSubtopics[i], level+1, maxDepth, seen); err != nil {
			return fmt.Errorf("child[%d]: %w", i, err)
		}
	}
	return nil
}

// FindByID returns a subtopic by id (non-linear access).
func (t *DUAHierarchicalTree) FindByID(id string) (*SubtopicNode, bool) {
	if t == nil {
		return nil, false
	}
	for i := range t.Subtopics {
		if n, ok := findIn(&t.Subtopics[i], id); ok {
			return n, true
		}
	}
	return nil, false
}

func findIn(s *SubtopicNode, id string) (*SubtopicNode, bool) {
	if s.SubtopicID == id {
		return s, true
	}
	for i := range s.ChildSubtopics {
		if n, ok := findIn(&s.ChildSubtopics[i], id); ok {
			return n, true
		}
	}
	return nil, false
}

// PathTo returns the id path from a root subtopic to the target (non-linear).
func (t *DUAHierarchicalTree) PathTo(id string) ([]string, bool) {
	if t == nil {
		return nil, false
	}
	for i := range t.Subtopics {
		if path, ok := pathIn(&t.Subtopics[i], id, nil); ok {
			return path, true
		}
	}
	return nil, false
}

func pathIn(s *SubtopicNode, id string, prefix []string) ([]string, bool) {
	cur := append(append([]string{}, prefix...), s.SubtopicID)
	if s.SubtopicID == id {
		return cur, true
	}
	for i := range s.ChildSubtopics {
		if path, ok := pathIn(&s.ChildSubtopics[i], id, cur); ok {
			return path, true
		}
	}
	return nil, false
}

// Clone deep-copies the tree.
func (t *DUAHierarchicalTree) Clone() *DUAHierarchicalTree {
	if t == nil {
		return nil
	}
	out := &DUAHierarchicalTree{
		MainTopicTitle: t.MainTopicTitle,
		MacroMediaURL:  t.MacroMediaURL,
		Subtopics:      cloneSubtopics(t.Subtopics),
	}
	return out
}

func cloneSubtopics(in []SubtopicNode) []SubtopicNode {
	if in == nil {
		return nil
	}
	out := make([]SubtopicNode, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].OrbitDelta = append([]float32(nil), in[i].OrbitDelta...)
		out[i].ChildSubtopics = cloneSubtopics(in[i].ChildSubtopics)
	}
	return out
}

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

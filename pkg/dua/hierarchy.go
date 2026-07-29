package dua

import (
	"fmt"
	"strings"
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

// Package knowledge models a directed curriculum graph of stable concepts
// (not node ULIDs) with typed edges for prerequisites and sequencing.
package knowledge

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// SchemaVersion is the only supported curriculum JSON version.
	SchemaVersion = 1
	// MaxTraversalDepth caps Path / neighbourhood walks.
	MaxTraversalDepth = 4
	conceptPrefix     = "concept:"
)

var slugRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ConceptID is concept:<slug> with a stable curriculum identity.
type ConceptID string

// ParseConceptID validates and normalizes a concept id.
func ParseConceptID(raw string) (ConceptID, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, conceptPrefix) {
		return "", fmt.Errorf("concept id must start with %q: %q", conceptPrefix, raw)
	}
	slug := strings.TrimPrefix(raw, conceptPrefix)
	if !slugRE.MatchString(slug) {
		return "", fmt.Errorf("invalid concept slug %q (want ^[a-z0-9]+(-[a-z0-9]+)*$)", slug)
	}
	return ConceptID(conceptPrefix + slug), nil
}

// NormalizeConceptRef accepts concept:<slug> or bare slug.
func NormalizeConceptRef(raw string) (ConceptID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty concept ref")
	}
	if !strings.HasPrefix(raw, conceptPrefix) {
		raw = conceptPrefix + raw
	}
	return ParseConceptID(raw)
}


// Track is a program lane; edges MAY cross tracks.
type Track string

const (
	TrackIngles     Track = "ingles"
	TrackPython     Track = "python"
	TrackMatematica Track = "matematica"
	TrackPlataforma Track = "plataforma"
)

// ValidTrack reports whether t is a known lane.
func ValidTrack(t Track) bool {
	switch t {
	case TrackIngles, TrackPython, TrackMatematica, TrackPlataforma:
		return true
	default:
		return false
	}
}

// EdgeKind is the typed relation between concepts.
// Direction invariant: the arrow always points to the more foundational /
// earlier concept. from --requires--> to means "from rests on to".
type EdgeKind string

const (
	EdgeRequires    EdgeKind = "requires"
	EdgeDeepens     EdgeKind = "deepens"
	EdgeContinues   EdgeKind = "continues"
	EdgeAlternative EdgeKind = "alternative"
)

// ValidEdgeKind reports whether k is known.
func ValidEdgeKind(k EdgeKind) bool {
	switch k {
	case EdgeRequires, EdgeDeepens, EdgeContinues, EdgeAlternative:
		return true
	default:
		return false
	}
}

// CycleCheckedKinds are directed kinds that must be acyclic.
func CycleCheckedKinds() []EdgeKind {
	return []EdgeKind{EdgeRequires, EdgeDeepens, EdgeContinues}
}

// Concept is a curriculum atom with stable identity across nodes/restarts.
type Concept struct {
	ID      ConceptID
	Title   string
	Summary string
	Track   Track
	Tags    []string
	Source  string
}

// Edge is a typed directed (or symmetric for alternative) relation.
type Edge struct {
	From        ConceptID
	To          ConceptID
	Kind        EdgeKind
	Strength    float64 // (0,1]
	RationaleES string
	Source      string
}

// Health is a snapshot of graph binding coverage.
type Health struct {
	Concepts           int
	Edges              int
	ResourcesUnbound   int // resources without any declared concept
	ConceptsUntaught   int // concepts with no teaching resource
	Warnings           []string
}

// KnowledgeGraph is the query surface for curriculum structure.
// No studentID: the graph models curriculum; learner evidence is a later PR.
type KnowledgeGraph interface {
	Concept(id ConceptID) (Concept, bool)
	Prerequisites(id ConceptID) []ConceptID
	Dependents(id ConceptID) []ConceptID
	Neighbors(id ConceptID, kind EdgeKind) []Edge
	Path(from, to ConceptID) ([]ConceptID, bool)
	Health() Health
}

// ResourceBinder resolves concept ↔ teaching resource at runtime.
type ResourceBinder interface {
	ResourcesFor(id ConceptID) []string
	ConceptsForNode(nodeID string) []ConceptID
}

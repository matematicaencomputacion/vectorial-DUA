package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// fileDocument is the on-disk curriculum schema.
type fileDocument struct {
	Version  int            `json:"version"`
	Concepts []fileConcept  `json:"concepts"`
	Edges    []fileEdge     `json:"edges"`
}

type fileConcept struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Track   string   `json:"track"`
	Tags    []string `json:"tags"`
	Source  string   `json:"source"`
}

type fileEdge struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Kind        string  `json:"kind"`
	Strength    float64 `json:"strength"`
	RationaleES string  `json:"rationale_es"`
	Source      string  `json:"source"`
}

// Report collects soft warnings from loading / binding.
type Report struct {
	Warnings []string
}

// Logf is an optional injectable logger (Ola 2.d style).
type Logf func(format string, args ...any)

// LoadOptions controls LoadFile behaviour.
type LoadOptions struct {
	Strict bool // AVLP_KNOWLEDGE_STRICT
	Logf   Logf
	Binder ResourceBinder // optional; enables resource-binding warnings
}

// StrictFromEnv reads AVLP_KNOWLEDGE_STRICT.
func StrictFromEnv() bool {
	v := strings.TrimSpace(os.Getenv("AVLP_KNOWLEDGE_STRICT"))
	return strings.EqualFold(v, "true") || v == "1"
}

// LoadFile loads and validates a versioned curriculum JSON.
func LoadFile(path string, opt LoadOptions) (*MemoryGraph, Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Report{}, err
	}
	var doc fileDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, Report{}, fmt.Errorf("knowledge: decode %s: %w", path, err)
	}
	if doc.Version != SchemaVersion {
		return nil, Report{}, fmt.Errorf("knowledge: unsupported version %d (want %d)", doc.Version, SchemaVersion)
	}

	g := newMemoryGraph()
	seenID := map[ConceptID]struct{}{}
	for i, fc := range doc.Concepts {
		id, err := ParseConceptID(fc.ID)
		if err != nil {
			return nil, Report{}, fmt.Errorf("knowledge: concepts[%d]: %w", i, err)
		}
		if _, ok := seenID[id]; ok {
			return nil, Report{}, fmt.Errorf("knowledge: duplicate concept %s", id)
		}
		track := Track(strings.TrimSpace(fc.Track))
		if !ValidTrack(track) {
			return nil, Report{}, fmt.Errorf("knowledge: concept %s: invalid track %q", id, fc.Track)
		}
		if strings.TrimSpace(fc.Title) == "" {
			return nil, Report{}, fmt.Errorf("knowledge: concept %s: title is required", id)
		}
		c := Concept{
			ID:      id,
			Title:   strings.TrimSpace(fc.Title),
			Summary: strings.TrimSpace(fc.Summary),
			Track:   track,
			Tags:    append([]string(nil), fc.Tags...),
			Source:  strings.TrimSpace(fc.Source),
		}
		sort.Strings(c.Tags)
		seenID[id] = struct{}{}
		g.concepts[id] = c
		g.conceptOrder = append(g.conceptOrder, id)
	}

	type edgeKey struct {
		from, to ConceptID
		kind     EdgeKind
	}
	seenEdge := map[edgeKey]struct{}{}
	for i, fe := range doc.Edges {
		from, err := ParseConceptID(fe.From)
		if err != nil {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d].from: %w", i, err)
		}
		to, err := ParseConceptID(fe.To)
		if err != nil {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d].to: %w", i, err)
		}
		if _, ok := g.concepts[from]; !ok {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d]: unknown from %s", i, from)
		}
		if _, ok := g.concepts[to]; !ok {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d]: unknown to %s", i, to)
		}
		if from == to {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d]: self-edge on %s", i, from)
		}
		kind := EdgeKind(strings.TrimSpace(fe.Kind))
		if !ValidEdgeKind(kind) {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d]: invalid kind %q", i, fe.Kind)
		}
		if fe.Strength <= 0 || fe.Strength > 1 {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d]: strength %v out of (0,1]", i, fe.Strength)
		}
		if kind == EdgeRequires && strings.TrimSpace(fe.RationaleES) == "" {
			return nil, Report{}, fmt.Errorf("knowledge: edges[%d]: requires edge needs non-empty rationale_es", i)
		}
		key := edgeKey{from: from, to: to, kind: kind}
		if _, ok := seenEdge[key]; ok {
			return nil, Report{}, fmt.Errorf("knowledge: duplicate edge (%s,%s,%s)", from, to, kind)
		}
		if kind == EdgeAlternative {
			rev := edgeKey{from: to, to: from, kind: kind}
			if _, ok := seenEdge[rev]; ok {
				return nil, Report{}, fmt.Errorf("knowledge: duplicate edge (%s,%s,%s)", from, to, kind)
			}
		}
		seenEdge[key] = struct{}{}
		e := Edge{
			From:        from,
			To:          to,
			Kind:        kind,
			Strength:    fe.Strength,
			RationaleES: strings.TrimSpace(fe.RationaleES),
			Source:      strings.TrimSpace(fe.Source),
		}
		g.edges = append(g.edges, e)
	}

	g.rebuildAdjacency()
	for _, kind := range CycleCheckedKinds() {
		if cycle := g.findCycle(kind); len(cycle) > 0 {
			return nil, Report{}, fmt.Errorf("knowledge: cycle in %s: %s", kind, formatCycle(cycle))
		}
	}

	rep := Report{}
	degree := map[ConceptID]int{}
	for _, e := range g.edges {
		degree[e.From]++
		degree[e.To]++
	}
	for _, id := range g.conceptOrder {
		if degree[id] == 0 {
			msg := fmt.Sprintf("concept without edges: %s", id)
			rep.Warnings = append(rep.Warnings, msg)
			if opt.Logf != nil {
				opt.Logf("%s", msg)
			}
		}
	}

	if opt.Binder != nil {
		if ib, ok := opt.Binder.(*IndexBinder); ok {
			bindRep, err := ApplyBindingWarnings(g, ib, opt.Strict, opt.Logf)
			rep.Warnings = append(rep.Warnings, bindRep.Warnings...)
			if err != nil {
				return nil, rep, err
			}
		} else {
			ctx := context.Background()
			for _, id := range g.conceptOrder {
				res, err := opt.Binder.ResourcesFor(ctx, id)
				if err != nil {
					return nil, rep, err
				}
				if len(res) == 0 {
					msg := fmt.Sprintf("concept without teaching resource: %s", id)
					rep.Warnings = append(rep.Warnings, msg)
					if opt.Logf != nil {
						opt.Logf("%s", msg)
					}
					if opt.Strict {
						return nil, rep, fmt.Errorf("knowledge: %s", msg)
					}
				}
			}
		}
	}

	return g, rep, nil
}

func formatCycle(ids []ConceptID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, " -> ")
}

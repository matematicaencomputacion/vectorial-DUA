package knowledge

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// IndexBinder derives concept ↔ resource bindings from the live index and
// interactive registry at process startup.
type IndexBinder struct {
	Index    *vector.Index
	Registry *dua.Registry

	// caches built on Bind()
	byConcept map[ConceptID][]string
	byNode    map[string][]ConceptID
}

// Bind (re)builds the maps from Index + Registry. Safe to call once after both
// are loaded.
func (b *IndexBinder) Bind() {
	if b == nil {
		return
	}
	b.byConcept = make(map[ConceptID][]string)
	b.byNode = make(map[string][]ConceptID)

	add := func(nodeID string, concepts []string, resourceKey string) {
		var ids []ConceptID
		for _, raw := range concepts {
			id, err := NormalizeConceptRef(raw)
			if err != nil {
				continue
			}
			ids = append(ids, id)
			b.byConcept[id] = appendUniqueString(b.byConcept[id], resourceKey)
		}
		if len(ids) > 0 {
			sortConceptIDs(ids)
			b.byNode[nodeID] = ids
		}
	}

	if b.Registry != nil {
		b.Registry.ForEach(func(node *dua.InteractiveVideoNode) {
			if node == nil {
				return
			}
			key := "interactive://" + node.NodeID
			add(node.NodeID, node.Concepts, key)
		})
	}
	if b.Index != nil {
		for _, n := range b.Index.Nodes() {
			if n.IsLiveGenerated {
				continue
			}
			key := n.ResourceURL
			if key == "" {
				key = n.ID
			}
			add(n.ID, n.Concepts, key)
		}
	}
	for id := range b.byConcept {
		sort.Strings(b.byConcept[id])
	}
}

// ResourcesFor implements ResourceBinder.
func (b *IndexBinder) ResourcesFor(_ context.Context, id ConceptID) ([]string, error) {
	if b == nil || b.byConcept == nil {
		return nil, nil
	}
	return append([]string(nil), b.byConcept[id]...), nil
}

// ConceptsForNode implements ResourceBinder.
func (b *IndexBinder) ConceptsForNode(_ context.Context, nodeID string) ([]ConceptID, error) {
	if b == nil || b.byNode == nil {
		return nil, nil
	}
	return append([]ConceptID(nil), b.byNode[nodeID]...), nil
}

// UnboundResourceCount returns curated resources that declare no concepts.
func (b *IndexBinder) UnboundResourceCount() int {
	if b == nil || b.Index == nil {
		return 0
	}
	n := 0
	seen := map[string]struct{}{}
	if b.Registry != nil {
		b.Registry.ForEach(func(node *dua.InteractiveVideoNode) {
			if node == nil || len(node.Concepts) > 0 {
				return
			}
			seen[node.NodeID] = struct{}{}
			n++
		})
	}
	for _, node := range b.Index.Nodes() {
		if node.IsLiveGenerated || len(node.Concepts) > 0 {
			continue
		}
		if _, ok := seen[node.ID]; ok {
			continue
		}
		if strings.HasPrefix(node.ResourceURL, "interactive://") {
			continue
		}
		n++
	}
	return n
}

// MissingConceptRefs lists concept ids declared on resources but absent from g.
func (b *IndexBinder) MissingConceptRefs(g *MemoryGraph) []ConceptID {
	if b == nil || g == nil || b.byConcept == nil {
		return nil
	}
	var missing []ConceptID
	seen := map[ConceptID]struct{}{}
	for id := range b.byConcept {
		if _, ok := g.concepts[id]; ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		missing = append(missing, id)
	}
	sortConceptIDs(missing)
	return missing
}

func appendUniqueString(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}

// ApplyBindingWarnings appends binder warnings and optionally fails in strict mode.
func ApplyBindingWarnings(g *MemoryGraph, binder *IndexBinder, strict bool, logf Logf) (Report, error) {
	rep := Report{}
	if g == nil || binder == nil {
		return rep, nil
	}
	binder.Bind()
	g.SetBinder(binder)
	ctx := context.Background()

	for _, id := range g.conceptOrder {
		res, err := binder.ResourcesFor(ctx, id)
		if err != nil {
			return rep, err
		}
		if len(res) == 0 {
			msg := "concept without teaching resource: " + string(id)
			rep.Warnings = append(rep.Warnings, msg)
			if logf != nil {
				logf("%s", msg)
			}
			if strict {
				return rep, errStrict(msg)
			}
		}
	}
	for _, id := range binder.MissingConceptRefs(g) {
		msg := "resource declares absent concept: " + string(id)
		rep.Warnings = append(rep.Warnings, msg)
		if logf != nil {
			logf("%s", msg)
		}
		if strict {
			return rep, errStrict(msg)
		}
	}
	if n := binder.UnboundResourceCount(); n > 0 {
		msg := formatUnbound(n)
		rep.Warnings = append(rep.Warnings, msg)
		if logf != nil {
			logf("%s", msg)
		}
	}
	return rep, nil
}

type strictError string

func (e strictError) Error() string { return string(e) }

func errStrict(msg string) error { return strictError("knowledge: " + msg) }

func formatUnbound(n int) string {
	return "resources without concept: " + strconv.Itoa(n)
}

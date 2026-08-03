package knowledge

import (
	"sort"
)

// MemoryGraph is an in-memory KnowledgeGraph with precomputed adjacency.
type MemoryGraph struct {
	concepts     map[ConceptID]Concept
	conceptOrder []ConceptID
	edges        []Edge
	// out[kind][from] = edges leaving from
	out map[EdgeKind]map[ConceptID][]Edge
	// in[kind][to] = edges arriving at to
	in map[EdgeKind]map[ConceptID][]Edge
	// binder optional for Health
	binder ResourceBinder
}

func newMemoryGraph() *MemoryGraph {
	return &MemoryGraph{
		concepts: make(map[ConceptID]Concept),
		out:      make(map[EdgeKind]map[ConceptID][]Edge),
		in:       make(map[EdgeKind]map[ConceptID][]Edge),
	}
}

// SetBinder attaches a resource binder for Health snapshots.
func (g *MemoryGraph) SetBinder(b ResourceBinder) {
	if g != nil {
		g.binder = b
	}
}

func (g *MemoryGraph) rebuildAdjacency() {
	g.out = make(map[EdgeKind]map[ConceptID][]Edge)
	g.in = make(map[EdgeKind]map[ConceptID][]Edge)
	for _, e := range g.edges {
		if g.out[e.Kind] == nil {
			g.out[e.Kind] = make(map[ConceptID][]Edge)
		}
		if g.in[e.Kind] == nil {
			g.in[e.Kind] = make(map[ConceptID][]Edge)
		}
		g.out[e.Kind][e.From] = append(g.out[e.Kind][e.From], e)
		g.in[e.Kind][e.To] = append(g.in[e.Kind][e.To], e)
	}
	for kind := range g.out {
		for from := range g.out[kind] {
			g.out[kind][from] = sortEdges(g.out[kind][from])
		}
	}
	for kind := range g.in {
		for to := range g.in[kind] {
			g.in[kind][to] = sortEdges(g.in[kind][to])
		}
	}
}

func sortEdges(edges []Edge) []Edge {
	out := append([]Edge(nil), edges...)
	sort.SliceStable(out, func(i, j int) bool {
		// depth proxy unused here; order: strength desc, then peer id asc
		if out[i].Strength != out[j].Strength {
			return out[i].Strength > out[j].Strength
		}
		// for out-lists compare To; for in-lists From — use both sides
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].From < out[j].From
	})
	return out
}

// Concept returns a concept by id.
func (g *MemoryGraph) Concept(id ConceptID) (Concept, bool) {
	if g == nil {
		return Concept{}, false
	}
	c, ok := g.concepts[id]
	return c, ok
}

// Prerequisites returns foundational concepts that id requires (edge id→prereq).
func (g *MemoryGraph) Prerequisites(id ConceptID) []ConceptID {
	return g.neighborIDs(id, EdgeRequires, true)
}

// Dependents returns concepts that require id (incoming requires).
func (g *MemoryGraph) Dependents(id ConceptID) []ConceptID {
	return g.neighborIDs(id, EdgeRequires, false)
}

func (g *MemoryGraph) neighborIDs(id ConceptID, kind EdgeKind, outgoing bool) []ConceptID {
	if g == nil {
		return nil
	}
	var edges []Edge
	if outgoing {
		edges = g.out[kind][id]
	} else {
		edges = g.in[kind][id]
	}
	// edges already sorted: strength desc, peer id asc — preserve that order.
	seen := map[ConceptID]struct{}{}
	var out []ConceptID
	for _, e := range edges {
		peer := e.To
		if !outgoing {
			peer = e.From
		}
		if _, ok := seen[peer]; ok {
			continue
		}
		seen[peer] = struct{}{}
		out = append(out, peer)
	}
	return out
}

// Neighbors returns edges of the given kind incident to id (deterministic).
// For alternative, both stored directions are treated as undirected.
func (g *MemoryGraph) Neighbors(id ConceptID, kind EdgeKind) []Edge {
	if g == nil {
		return nil
	}
	var edges []Edge
	edges = append(edges, g.out[kind][id]...)
	if kind == EdgeAlternative {
		for _, e := range g.in[kind][id] {
			// Present as leaving id for a stable API.
			edges = append(edges, Edge{
				From:        id,
				To:          e.From,
				Kind:        e.Kind,
				Strength:    e.Strength,
				RationaleES: e.RationaleES,
				Source:      e.Source,
			})
		}
	}
	return sortEdges(edges)
}

// Path finds a learning path from → to following requires/deepens/continues
// toward foundational concepts, then reverses so the result is study order
// (foundational first). Limited to MaxTraversalDepth hops.
func (g *MemoryGraph) Path(from, to ConceptID) ([]ConceptID, bool) {
	if g == nil {
		return nil, false
	}
	if _, ok := g.concepts[from]; !ok {
		return nil, false
	}
	if _, ok := g.concepts[to]; !ok {
		return nil, false
	}
	if from == to {
		return []ConceptID{from}, true
	}

	// Walk with the arrow (toward foundational). Goal is to reach `to`
	// starting at `from` within MaxTraversalDepth.
	type frame struct {
		id   ConceptID
		path []ConceptID
	}
	q := []frame{{id: from, path: []ConceptID{from}}}
	visited := map[ConceptID]struct{}{from: {}}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if len(cur.path)-1 >= MaxTraversalDepth {
			continue
		}
		var next []Edge
		for _, kind := range CycleCheckedKinds() {
			next = append(next, g.out[kind][cur.id]...)
		}
		next = sortEdges(next)
		for _, e := range next {
			peer := e.To
			if _, ok := visited[peer]; ok {
				continue
			}
			np := append(append([]ConceptID{}, cur.path...), peer)
			if peer == to {
				return reverseIDs(np), true
			}
			visited[peer] = struct{}{}
			q = append(q, frame{id: peer, path: np})
		}
	}
	return nil, false
}

func reverseIDs(ids []ConceptID) []ConceptID {
	out := make([]ConceptID, len(ids))
	for i := range ids {
		out[len(ids)-1-i] = ids[i]
	}
	return out
}

func sortConceptIDs(ids []ConceptID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
}

// Health summarizes graph size and binding coverage.
func (g *MemoryGraph) Health() Health {
	h := Health{}
	if g == nil {
		return h
	}
	h.Concepts = len(g.concepts)
	h.Edges = len(g.edges)
	if g.binder != nil {
		for _, id := range g.conceptOrder {
			if len(g.binder.ResourcesFor(id)) == 0 {
				h.ConceptsUntaught++
				h.Warnings = append(h.Warnings, "concept without teaching resource: "+string(id))
			}
		}
		if ib, ok := g.binder.(*IndexBinder); ok {
			h.ResourcesUnbound = ib.UnboundResourceCount()
		}
	}
	return h
}

// ConceptIDs returns all concept ids in file order.
func (g *MemoryGraph) ConceptIDs() []ConceptID {
	if g == nil {
		return nil
	}
	return append([]ConceptID(nil), g.conceptOrder...)
}

// Edges returns a copy of all stored edges.
func (g *MemoryGraph) Edges() []Edge {
	if g == nil {
		return nil
	}
	return append([]Edge(nil), g.edges...)
}

// findCycle returns a cycle path (including repeat of start) for kind, or nil.
func (g *MemoryGraph) findCycle(kind EdgeKind) []ConceptID {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[ConceptID]int{}
	parent := map[ConceptID]ConceptID{}
	var cycle []ConceptID

	var dfs func(ConceptID) bool
	dfs = func(u ConceptID) bool {
		color[u] = gray
		for _, e := range g.out[kind][u] {
			v := e.To
			switch color[v] {
			case white:
				parent[v] = u
				if dfs(v) {
					return true
				}
			case gray:
				// reconstruct cycle u -> ... -> v -> u
				cycle = []ConceptID{v}
				cur := u
				for cur != v {
					cycle = append(cycle, cur)
					cur = parent[cur]
					if cur == "" {
						break
					}
				}
				cycle = append(cycle, v)
				// reverse to start at v
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return true
			}
		}
		color[u] = black
		return false
	}

	for _, id := range g.conceptOrder {
		if color[id] == white {
			if dfs(id) {
				return cycle
			}
		}
	}
	return nil
}

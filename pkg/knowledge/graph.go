package knowledge

import (
	"context"
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
	// binder optional for Stats coverage
	binder ResourceBinder
}

func newMemoryGraph() *MemoryGraph {
	return &MemoryGraph{
		concepts: make(map[ConceptID]Concept),
		out:      make(map[EdgeKind]map[ConceptID][]Edge),
		in:       make(map[EdgeKind]map[ConceptID][]Edge),
	}
}

// SetBinder attaches a resource binder for Stats snapshots.
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
		if out[i].Strength != out[j].Strength {
			return out[i].Strength > out[j].Strength
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].From < out[j].From
	})
	return out
}

func sortRelations(rels []Relation) []Relation {
	out := append([]Relation(nil), rels...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Depth != out[j].Depth {
			return out[i].Depth < out[j].Depth
		}
		if out[i].Strength != out[j].Strength {
			return out[i].Strength > out[j].Strength
		}
		return out[i].Peer.ID < out[j].Peer.ID
	})
	return out
}

// Concept implements KnowledgeGraph.
func (g *MemoryGraph) Concept(_ context.Context, id ConceptID) (Concept, error) {
	if g == nil {
		return Concept{}, ErrConceptNotFound
	}
	c, ok := g.concepts[id]
	if !ok {
		return Concept{}, ErrConceptNotFound
	}
	return c, nil
}

// Prerequisites walks toward foundational concepts (outgoing edges).
// Zero-value opts: MaxDepth=1, Kinds=[requires].
func (g *MemoryGraph) Prerequisites(ctx context.Context, id ConceptID, opts TraverseOptions) ([]Relation, error) {
	return g.traverse(ctx, id, opts, true, []EdgeKind{EdgeRequires})
}

// Dependents walks toward concepts that rest on id (incoming edges).
// Zero-value opts: MaxDepth=1, Kinds=[requires].
func (g *MemoryGraph) Dependents(ctx context.Context, id ConceptID, opts TraverseOptions) ([]Relation, error) {
	return g.traverse(ctx, id, opts, false, []EdgeKind{EdgeRequires})
}

// Neighbors returns incident relations for the requested kinds (deterministic).
// Zero-value opts: MaxDepth=1, all EdgeKinds. alternative is treated undirected.
func (g *MemoryGraph) Neighbors(ctx context.Context, id ConceptID, opts TraverseOptions) ([]Relation, error) {
	defaults := []EdgeKind{EdgeRequires, EdgeDeepens, EdgeContinues, EdgeAlternative}
	return g.traverse(ctx, id, opts, true, defaults)
}

func (g *MemoryGraph) traverse(
	ctx context.Context,
	id ConceptID,
	opts TraverseOptions,
	outgoing bool,
	defaultKinds []EdgeKind,
) ([]Relation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrConceptNotFound
	}
	if _, ok := g.concepts[id]; !ok {
		return nil, ErrConceptNotFound
	}

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	kinds := opts.Kinds
	if len(kinds) == 0 {
		kinds = append([]EdgeKind(nil), defaultKinds...)
	}
	minStrength := opts.MinStrength
	limit := opts.Limit

	type frame struct {
		id    ConceptID
		depth int
	}
	q := []frame{{id: id, depth: 0}}
	seenPeer := map[ConceptID]struct{}{}
	var out []Relation

	for len(q) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cur := q[0]
		q = q[1:]
		if cur.depth >= maxDepth {
			continue
		}
		nextDepth := cur.depth + 1
		for _, kind := range kinds {
			edges := g.incidentEdges(cur.id, kind, outgoing)
			for _, e := range edges {
				if minStrength > 0 && e.Strength < minStrength {
					continue
				}
				peerID := e.To
				if !outgoing {
					peerID = e.From
				}
				// Neighbors with alternative: undirected presentation already in incidentEdges.
				if _, dup := seenPeer[peerID]; dup {
					continue
				}
				peer, ok := g.concepts[peerID]
				if !ok {
					continue
				}
				seenPeer[peerID] = struct{}{}
				out = append(out, Relation{
					Kind:        e.Kind,
					Strength:    e.Strength,
					RationaleES: e.RationaleES,
					Source:      e.Source,
					Peer:        peer,
					Depth:       nextDepth,
				})
				if limit > 0 && len(out) >= limit {
					return sortRelations(out), nil
				}
				if nextDepth < maxDepth {
					q = append(q, frame{id: peerID, depth: nextDepth})
				}
			}
		}
	}
	return sortRelations(out), nil
}

func (g *MemoryGraph) incidentEdges(id ConceptID, kind EdgeKind, outgoing bool) []Edge {
	var edges []Edge
	if outgoing {
		edges = append(edges, g.out[kind][id]...)
		if kind == EdgeAlternative {
			for _, e := range g.in[kind][id] {
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
	} else {
		edges = append(edges, g.in[kind][id]...)
		if kind == EdgeAlternative {
			for _, e := range g.out[kind][id] {
				edges = append(edges, Edge{
					From:        e.To,
					To:          id,
					Kind:        e.Kind,
					Strength:    e.Strength,
					RationaleES: e.RationaleES,
					Source:      e.Source,
				})
			}
		}
	}
	return sortEdges(edges)
}

// Path finds a learning path from → to following kinds toward foundations,
// then reverses so the result is study order (foundational first).
// Zero-value opts: MaxDepth=MaxTraversalDepth, Kinds=CycleCheckedKinds.
func (g *MemoryGraph) Path(ctx context.Context, from, to ConceptID, opts TraverseOptions) ([]ConceptID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrConceptNotFound
	}
	if _, ok := g.concepts[from]; !ok {
		return nil, ErrConceptNotFound
	}
	if _, ok := g.concepts[to]; !ok {
		return nil, ErrConceptNotFound
	}
	if from == to {
		return []ConceptID{from}, nil
	}

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = MaxTraversalDepth
	}
	kinds := opts.Kinds
	if len(kinds) == 0 {
		kinds = CycleCheckedKinds()
	}
	minStrength := opts.MinStrength

	type frame struct {
		id   ConceptID
		path []ConceptID
	}
	q := []frame{{id: from, path: []ConceptID{from}}}
	visited := map[ConceptID]struct{}{from: {}}
	for len(q) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cur := q[0]
		q = q[1:]
		if len(cur.path)-1 >= maxDepth {
			continue
		}
		var next []Edge
		for _, kind := range kinds {
			next = append(next, g.out[kind][cur.id]...)
		}
		next = sortEdges(next)
		for _, e := range next {
			if minStrength > 0 && e.Strength < minStrength {
				continue
			}
			peer := e.To
			if _, ok := visited[peer]; ok {
				continue
			}
			np := append(append([]ConceptID{}, cur.path...), peer)
			if peer == to {
				return reverseIDs(np), nil
			}
			visited[peer] = struct{}{}
			q = append(q, frame{id: peer, path: np})
		}
	}
	return nil, ErrNoPath
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

// Health implements KnowledgeGraph as an availability probe.
func (g *MemoryGraph) Health(_ context.Context) error {
	if g == nil {
		return ErrConceptNotFound
	}
	return nil
}

// Stats summarizes graph size and binding coverage (not connection health).
func (g *MemoryGraph) Stats() Stats {
	h := Stats{}
	if g == nil {
		return h
	}
	h.Concepts = len(g.concepts)
	h.Edges = len(g.edges)
	if g.binder != nil {
		ctx := context.Background()
		for _, id := range g.conceptOrder {
			res, err := g.binder.ResourcesFor(ctx, id)
			if err != nil || len(res) == 0 {
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

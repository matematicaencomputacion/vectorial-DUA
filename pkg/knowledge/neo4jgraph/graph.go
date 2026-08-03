package neo4jgraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

// Graph is a read-only Neo4j curriculum graph with breaker + TTL cache.
type Graph struct {
	driver neo4j.DriverWithContext
	cfg    Config
	br     *breaker
	cache  *ttlCache

	// run is overridable in tests (default: executeRead via driver).
	run func(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
}

// New opens a Bolt driver. URI must be non-empty.
func New(cfg Config) (*Graph, error) {
	if strings.TrimSpace(cfg.URI) == "" {
		return nil, fmt.Errorf("neo4jgraph: URI is required")
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = defaultCooldown
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = defaultCacheTTL
	}
	if cfg.MaxTransactionRetryTime <= 0 {
		cfg.MaxTransactionRetryTime = maxTxRetry
	}

	auth := neo4j.NoAuth()
	if cfg.User != "" || cfg.Password != "" {
		auth = neo4j.BasicAuth(cfg.User, cfg.Password, "")
	}
	driver, err := neo4j.NewDriverWithContext(cfg.URI, auth, func(c *neo4j.Config) {
		c.MaxTransactionRetryTime = cfg.MaxTransactionRetryTime
	})
	if err != nil {
		return nil, fmt.Errorf("neo4jgraph: driver: %w", err)
	}
	g := &Graph{
		driver: driver,
		cfg:    cfg,
		br:     newBreaker(breakerLimit, cfg.Cooldown, cfg.Logf),
		cache:  newTTLCache(cfg.CacheTTL),
	}
	g.run = g.executeRead
	return g, nil
}

// NewFromEnv returns (nil, nil) when AVLP_NEO4J_URI is unset.
func NewFromEnv(logf knowledge.Logf) (*Graph, error) {
	cfg := ConfigFromEnv()
	cfg.Logf = logf
	if cfg.URI == "" {
		return nil, nil
	}
	return New(cfg)
}

// Close releases the driver.
func (g *Graph) Close(ctx context.Context) error {
	if g == nil || g.driver == nil {
		return nil
	}
	return g.driver.Close(ctx)
}

func (g *Graph) executeRead(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)
	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		var rows []map[string]any
		for res.Next(ctx) {
			rec := res.Record()
			row := make(map[string]any, len(rec.Keys))
			for _, k := range rec.Keys {
				v, _ := rec.Get(k)
				row[k] = v
			}
			rows = append(rows, row)
		}
		return rows, res.Err()
	})
	if err != nil {
		return nil, err
	}
	rows, _ := result.([]map[string]any)
	return rows, nil
}

func (g *Graph) guarded(ctx context.Context, cacheKey string, cypher string, params map[string]any, decode func([]map[string]any) (any, error)) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil {
		return nil, fmt.Errorf("neo4jgraph: graph is nil")
	}
	if cacheKey != "" {
		if v, ok := g.cache.get(cacheKey); ok {
			return v, nil
		}
	}
	if !g.br.allow() {
		return nil, fmt.Errorf("neo4jgraph: breaker open")
	}
	rows, err := g.run(ctx, cypher, params)
	if err != nil {
		g.br.failure(err)
		return nil, err
	}
	g.br.success()
	out, err := decode(rows)
	if err != nil {
		return nil, err
	}
	if cacheKey != "" {
		g.cache.set(cacheKey, out)
	}
	return out, nil
}

// Concept implements knowledge.KnowledgeGraph.
func (g *Graph) Concept(ctx context.Context, id knowledge.ConceptID) (knowledge.Concept, error) {
	key := "concept:" + string(id)
	v, err := g.guarded(ctx, key, cypherConcept, map[string]any{"id": string(id)}, func(rows []map[string]any) (any, error) {
		if len(rows) == 0 {
			return knowledge.Concept{}, knowledge.ErrConceptNotFound
		}
		c, err := conceptFromRow(rows[0], "id", "title", "summary", "track", "tags", "source")
		return c, err
	})
	if err != nil {
		return knowledge.Concept{}, err
	}
	return v.(knowledge.Concept), nil
}

// Prerequisites implements knowledge.KnowledgeGraph.
func (g *Graph) Prerequisites(ctx context.Context, id knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return g.relations(ctx, "prereq", cypherPrerequisites, id, opts, []knowledge.EdgeKind{knowledge.EdgeRequires})
}

// Dependents implements knowledge.KnowledgeGraph.
func (g *Graph) Dependents(ctx context.Context, id knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return g.relations(ctx, "dep", cypherDependents, id, opts, []knowledge.EdgeKind{knowledge.EdgeRequires})
}

// Neighbors implements knowledge.KnowledgeGraph.
func (g *Graph) Neighbors(ctx context.Context, id knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	kinds := opts.Kinds
	if len(kinds) == 0 {
		kinds = []knowledge.EdgeKind{
			knowledge.EdgeRequires, knowledge.EdgeDeepens, knowledge.EdgeContinues, knowledge.EdgeAlternative,
		}
	}
	params := traverseParams(id, opts)
	params["kinds"] = kindStrings(kinds)
	key := fmt.Sprintf("neighbors:%s:%v:%v:%d", id, kinds, opts.MinStrength, opts.Limit)
	v, err := g.guarded(ctx, key, cypherNeighbors, params, func(rows []map[string]any) (any, error) {
		return relationsFromRows(rows)
	})
	if err != nil {
		return nil, err
	}
	return v.([]knowledge.Relation), nil
}

func (g *Graph) relations(
	ctx context.Context,
	prefix string,
	cypher string,
	id knowledge.ConceptID,
	opts knowledge.TraverseOptions,
	_ []knowledge.EdgeKind,
) ([]knowledge.Relation, error) {
	params := traverseParams(id, opts)
	key := fmt.Sprintf("%s:%s:%v:%d", prefix, id, opts.MinStrength, opts.Limit)
	v, err := g.guarded(ctx, key, cypher, params, func(rows []map[string]any) (any, error) {
		return relationsFromRows(rows)
	})
	if err != nil {
		return nil, err
	}
	return v.([]knowledge.Relation), nil
}

// Path implements knowledge.KnowledgeGraph.
func (g *Graph) Path(ctx context.Context, from, to knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.ConceptID, error) {
	depth := opts.MaxDepth
	if depth <= 0 {
		depth = knowledge.MaxTraversalDepth
	}
	params := map[string]any{"from": string(from), "to": string(to), "depth": depth}
	key := fmt.Sprintf("path:%s:%s:%d", from, to, depth)
	v, err := g.guarded(ctx, key, cypherPath, params, func(rows []map[string]any) (any, error) {
		if len(rows) == 0 {
			return nil, knowledge.ErrNoPath
		}
		raw, _ := rows[0]["ids"].([]any)
		if raw == nil {
			// driver may return []string
			if ss, ok := rows[0]["ids"].([]string); ok {
				out := make([]knowledge.ConceptID, 0, len(ss))
				for _, s := range ss {
					out = append(out, knowledge.ConceptID(s))
				}
				return out, nil
			}
			return nil, knowledge.ErrNoPath
		}
		out := make([]knowledge.ConceptID, 0, len(raw))
		for _, x := range raw {
			out = append(out, knowledge.ConceptID(fmt.Sprint(x)))
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]knowledge.ConceptID), nil
}

// Health implements knowledge.KnowledgeGraph.
func (g *Graph) Health(ctx context.Context) error {
	_, err := g.guarded(ctx, "", cypherHealth, nil, func(rows []map[string]any) (any, error) {
		return true, nil
	})
	return err
}

func traverseParams(id knowledge.ConceptID, opts knowledge.TraverseOptions) map[string]any {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	min := opts.MinStrength
	return map[string]any{
		"id":          string(id),
		"minStrength": min,
		"limit":       limit,
	}
}

func kindStrings(kinds []knowledge.EdgeKind) []string {
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, strings.ToUpper(string(k)))
	}
	return out
}

func conceptFromRow(row map[string]any, idK, titleK, summaryK, trackK, tagsK, sourceK string) (knowledge.Concept, error) {
	idRaw := fmt.Sprint(row[idK])
	id, err := knowledge.ParseConceptID(idRaw)
	if err != nil {
		return knowledge.Concept{}, err
	}
	var tags []string
	switch t := row[tagsK].(type) {
	case []any:
		for _, x := range t {
			tags = append(tags, fmt.Sprint(x))
		}
	case []string:
		tags = append(tags, t...)
	}
	return knowledge.Concept{
		ID:      id,
		Title:   fmt.Sprint(nullStr(row[titleK])),
		Summary: fmt.Sprint(nullStr(row[summaryK])),
		Track:   knowledge.Track(fmt.Sprint(nullStr(row[trackK]))),
		Tags:    tags,
		Source:  fmt.Sprint(nullStr(row[sourceK])),
	}, nil
}

func relationsFromRows(rows []map[string]any) ([]knowledge.Relation, error) {
	out := make([]knowledge.Relation, 0, len(rows))
	for _, row := range rows {
		peer, err := conceptFromRow(row, "peer_id", "peer_title", "peer_summary", "peer_track", "peer_tags", "peer_source")
		if err != nil {
			return nil, err
		}
		kind := knowledge.EdgeKind(strings.ToLower(fmt.Sprint(row["kind"])))
		if kind == "" {
			kind = knowledge.EdgeRequires
		}
		strength, _ := asFloat(row["strength"])
		depth, _ := asInt(row["depth"])
		if depth <= 0 {
			depth = 1
		}
		out = append(out, knowledge.Relation{
			Kind:        kind,
			Strength:    strength,
			RationaleES: fmt.Sprint(nullStr(row["rationale_es"])),
			Source:      fmt.Sprint(nullStr(row["source"])),
			Peer:        peer,
			Depth:       depth,
		})
	}
	return out, nil
}

func nullStr(v any) any {
	if v == nil {
		return ""
	}
	return v
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	default:
		return 0, false
	}
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int64:
		return int(x), true
	case int:
		return x, true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

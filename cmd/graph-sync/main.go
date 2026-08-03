// Package main is the curriculum → Neo4j synchronizer (write path).
// The router only reads via pkg/knowledge/neo4jgraph.
package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/knowledge/neo4jgraph"
)

func main() {
	curriculum := flag.String("curriculum", "data/knowledge/curriculum.json", "curriculum JSON (same LoadFile as router)")
	seedsDir := flag.String("seeds", "data/nodes/interactive", "interactive seeds dir for -validate-seeds")
	dryRun := flag.Bool("dry-run", false, "print plan without writing to Neo4j")
	prune := flag.Bool("prune", false, "delete concepts/edges absent from the file (default is additive)")
	validateSeeds := flag.Bool("validate-seeds", false, "report unknown concepts declared on interactive seeds")
	flag.Parse()

	log.SetFlags(0)

	g, report, err := knowledge.LoadFile(*curriculum, knowledge.LoadOptions{
		Strict: knowledge.StrictFromEnv(),
		Logf:   log.Printf,
	})
	if err != nil {
		log.Fatalf("curriculum inválido — abortando antes de tocar Neo4j: %v", err)
	}
	for _, w := range report.Warnings {
		log.Printf("aviso: %s", w)
	}

	if *validateSeeds {
		unknown, err := unknownSeedConcepts(*seedsDir, g)
		if err != nil {
			log.Fatalf("validate-seeds: %v", err)
		}
		for _, u := range unknown {
			log.Printf("validate-seeds: concepto desconocido %s (nodo %s)", u.Concept, u.NodeID)
		}
		if len(unknown) == 0 {
			log.Printf("validate-seeds: ok (%s)", *seedsDir)
		}
	}

	plan := buildPlan(g, *prune)
	printPlan(plan)

	if *dryRun {
		log.Printf("dry-run: no se escribió nada en Neo4j")
		return
	}

	cfg := neo4jgraph.ConfigFromEnv()
	if cfg.URI == "" {
		log.Fatalf("AVLP_NEO4J_URI requerido para sincronizar (o usá -dry-run)")
	}
	if cfg.MaxTransactionRetryTime <= 0 {
		cfg.MaxTransactionRetryTime = 2 * time.Second
	}

	auth := neo4j.NoAuth()
	if cfg.User != "" || cfg.Password != "" {
		auth = neo4j.BasicAuth(cfg.User, cfg.Password, "")
	}
	driver, err := neo4j.NewDriverWithContext(cfg.URI, auth, func(c *neo4j.Config) {
		c.MaxTransactionRetryTime = cfg.MaxTransactionRetryTime
	})
	if err != nil {
		log.Fatalf("driver: %v", err)
	}
	defer driver.Close(context.Background())

	ctx := context.Background()
	syncedAt := time.Now().UTC()
	if err := applyPlan(ctx, driver, plan, syncedAt); err != nil {
		log.Fatalf("sync: %v", err)
	}
	log.Printf("sync ok: conceptos=%d aristas=%d prune=%v synced_at=%s",
		len(plan.Concepts), len(plan.Edges), plan.Prune, syncedAt.Format(time.RFC3339))
}

type seedMiss struct {
	NodeID  string
	Concept string
}

func unknownSeedConcepts(dir string, g *knowledge.MemoryGraph) ([]seedMiss, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(abs); err != nil {
		return nil, err
	}
	known := map[knowledge.ConceptID]struct{}{}
	for _, id := range g.ConceptIDs() {
		known[id] = struct{}{}
	}
	var misses []seedMiss
	reg.ForEach(func(n *dua.InteractiveVideoNode) {
		for _, raw := range n.Concepts {
			id, err := knowledge.NormalizeConceptRef(raw)
			if err != nil {
				misses = append(misses, seedMiss{NodeID: n.NodeID, Concept: raw + " (" + err.Error() + ")"})
				continue
			}
			if _, ok := known[id]; !ok {
				misses = append(misses, seedMiss{NodeID: n.NodeID, Concept: string(id)})
			}
		}
	})
	return misses, nil
}

type syncPlan struct {
	Concepts []knowledge.Concept
	Edges    []knowledge.Edge
	Prune    bool
}

func buildPlan(g *knowledge.MemoryGraph, prune bool) syncPlan {
	ids := g.ConceptIDs()
	concepts := make([]knowledge.Concept, 0, len(ids))
	for _, id := range ids {
		c, err := g.Concept(context.Background(), id)
		if err != nil {
			continue
		}
		concepts = append(concepts, c)
	}
	return syncPlan{
		Concepts: concepts,
		Edges:    append([]knowledge.Edge(nil), g.Edges()...),
		Prune:    prune,
	}
}

func printPlan(p syncPlan) {
	log.Printf("plan: %d conceptos, %d aristas, prune=%v", len(p.Concepts), len(p.Edges), p.Prune)
	byKind := map[knowledge.EdgeKind]int{}
	for _, e := range p.Edges {
		byKind[e.Kind]++
	}
	for _, k := range []knowledge.EdgeKind{
		knowledge.EdgeRequires, knowledge.EdgeDeepens, knowledge.EdgeContinues, knowledge.EdgeAlternative,
	} {
		if n := byKind[k]; n > 0 {
			log.Printf("  aristas %s: %d", k, n)
		}
	}
}

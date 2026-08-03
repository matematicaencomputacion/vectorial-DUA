package evals

import (
	"fmt"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// UpsertInteractiveSeeds loads *.json from dir (via dua.Registry.LoadDir) into idx.
// Used by golden evals that target interactive ULIDs (e.g. variables-escopes-typo-static).
// Calibrate/simmatrix keep SeedDemoNodes-only so el umbral hash no se mezcla con
// el corpus interactivo.
func UpsertInteractiveSeeds(idx *vector.Index, dir string) (int, error) {
	if idx == nil {
		return 0, fmt.Errorf("index is nil")
	}
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(dir); err != nil {
		return 0, err
	}
	var upsertErr error
	count := 0
	reg.ForEach(func(node *dua.InteractiveVideoNode) {
		if upsertErr != nil || node == nil || len(node.Embedding) == 0 {
			return
		}
		fitted, err := vector.FitIndexEmbedding(node.Embedding, idx.Dims())
		if err != nil {
			upsertErr = fmt.Errorf("%s: %w", node.NodeID, err)
			return
		}
		format := "visual"
		if parts, err := vector.ParseNodeID(node.NodeID); err == nil && parts.Format != "" {
			format = parts.Format
		}
		if err := idx.Upsert(vector.Node{
			ID:           node.NodeID,
			DimensionDUA: node.DimensionDUA,
			Difficulty:   "basico",
			Format:       format,
			ResourceURL:  "interactive://" + node.NodeID,
			Embedding:    fitted,
			Concepts:     append([]string(nil), node.Concepts...),
		}); err != nil {
			upsertErr = err
			return
		}
		count++
	})
	if upsertErr != nil {
		return count, upsertErr
	}
	return count, nil
}

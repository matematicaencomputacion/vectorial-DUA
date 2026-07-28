package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// SimCell is one query×node cosine score.
type SimCell struct {
	NodeID     string  `json:"node_id"`
	Dimension  string  `json:"dimension_dua"`
	Format     string  `json:"format"`
	Similarity float32 `json:"similarity"`
	IsNearest  bool    `json:"is_nearest"`
}

// SimRow is the full similarity row for one golden query.
type SimRow struct {
	CaseID    string    `json:"case_id"`
	QueryText string    `json:"query_text"`
	Expected  string    `json:"expected_outcome"`
	Cells     []SimCell `json:"cells"`
}

// SimMatrixReport is the calibration artifact for descriptor/threshold tuning.
type SimMatrixReport struct {
	EmbedderMode string   `json:"embedder_mode"`
	Threshold    float32  `json:"threshold"`
	NodeIDs      []string `json:"node_ids"`
	Rows         []SimRow `json:"rows"`
}

// BuildSimMatrix embeds each golden query and scores it against every index node.
func BuildSimMatrix(ctx context.Context, cases []Case, idx *vector.Index, emb rag.Embedder, mode string) (SimMatrixReport, error) {
	if idx == nil {
		return SimMatrixReport{}, fmt.Errorf("index is nil")
	}
	if emb == nil {
		emb = rag.NewHashEmbedder(rag.DefaultEmbedDims)
	}
	if mode == "" {
		mode = "hash"
	}

	nodes := idx.Nodes()
	nodeIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		nodeIDs = append(nodeIDs, n.ID)
	}

	report := SimMatrixReport{
		EmbedderMode: mode,
		Threshold:    vector.EffectiveDefaultThreshold(),
		NodeIDs:      nodeIDs,
	}

	for _, c := range cases {
		queryText := strings.TrimSpace(c.QueryText)
		if queryText == "" && len(c.QueryEmbedding) == 0 {
			continue
		}
		query := append([]float32(nil), c.QueryEmbedding...)
		if len(query) == 0 {
			var err error
			query, err = emb.Embed(ctx, queryText)
			if err != nil {
				return SimMatrixReport{}, fmt.Errorf("%s: embed: %w", c.CaseID, err)
			}
		}
		fitted, err := vector.FitIndexEmbedding(query, idx.Dims())
		if err != nil {
			return SimMatrixReport{}, fmt.Errorf("%s: fit: %w", c.CaseID, err)
		}

		cells := make([]SimCell, 0, len(nodes))
		bestIdx := -1
		bestSim := float32(-1)
		for i, n := range nodes {
			sim := vector.CosineSimilarity(fitted, n.Embedding)
			cells = append(cells, SimCell{
				NodeID:     n.ID,
				Dimension:  n.DimensionDUA,
				Format:     n.Format,
				Similarity: sim,
			})
			if sim > bestSim {
				bestSim = sim
				bestIdx = i
			}
		}
		if bestIdx >= 0 {
			cells[bestIdx].IsNearest = true
		}
		// Rank high→low for readability in JSON.
		sort.SliceStable(cells, func(i, j int) bool {
			return cells[i].Similarity > cells[j].Similarity
		})

		report.Rows = append(report.Rows, SimRow{
			CaseID:    c.CaseID,
			QueryText: queryText,
			Expected:  c.EffectiveExpectedOutcome(mode),
			Cells:     cells,
		})
	}
	return report, nil
}

// WriteSimMatrixJSON writes the matrix report as indented JSON.
func WriteSimMatrixJSON(path string, report SimMatrixReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// FormatSimMatrixTable renders a compact human-readable table to a string.
func FormatSimMatrixTable(report SimMatrixReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "simmatrix embedder=%s threshold=%.2f nodes=%d queries=%d\n\n",
		report.EmbedderMode, report.Threshold, len(report.NodeIDs), len(report.Rows))

	// Short labels for columns: Dimension::format (last ULID segment trimmed).
	short := make([]string, len(report.NodeIDs))
	for i, id := range report.NodeIDs {
		short[i] = shortNodeLabel(id)
	}

	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprint(w, "case")
	for _, s := range short {
		fmt.Fprintf(w, "\t%s", s)
	}
	fmt.Fprint(w, "\tnearest\n")

	for _, row := range report.Rows {
		byID := make(map[string]SimCell, len(row.Cells))
		var nearest string
		for _, c := range row.Cells {
			byID[c.NodeID] = c
			if c.IsNearest {
				nearest = shortNodeLabel(c.NodeID)
			}
		}
		fmt.Fprint(w, row.CaseID)
		for _, id := range report.NodeIDs {
			c := byID[id]
			mark := ""
			if c.IsNearest {
				mark = "*"
			}
			fmt.Fprintf(w, "\t%.3f%s", c.Similarity, mark)
		}
		fmt.Fprintf(w, "\t%s\n", nearest)
	}
	_ = w.Flush()

	b.WriteString("\n* = nearest. Labels: Dimension::format (prefix of node_id).\n")
	return b.String()
}

func shortNodeLabel(id string) string {
	parts := strings.Split(id, "::")
	if len(parts) >= 4 {
		return parts[1] + "::" + parts[3]
	}
	if len(id) > 28 {
		return id[:28]
	}
	return id
}

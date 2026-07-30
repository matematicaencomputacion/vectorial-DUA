package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// ChunkSimCase is one query used to calibrate the RAG retrieval floor.
type ChunkSimCase struct {
	CaseID               string `json:"case_id"`
	QueryText            string `json:"query_text"`
	Role                 string `json:"role"` // on_topic | off_topic
	ExpectedSourceSubstr string `json:"expected_source_substr,omitempty"`
}

// ChunkSimCell is one query×chunk cosine score.
type ChunkSimCell struct {
	ChunkID          string  `json:"chunk_id"`
	Source           string  `json:"source"`
	Title            string  `json:"title"`
	Similarity       float32 `json:"similarity"`
	IsNearest        bool    `json:"is_nearest"`
	IsExpectedSource bool    `json:"is_expected_source"`
}

// ChunkSimRow contains all chunk scores for one calibration query.
type ChunkSimRow struct {
	CaseID               string         `json:"case_id"`
	QueryText            string         `json:"query_text"`
	Role                 string         `json:"role"`
	ExpectedSourceSubstr string         `json:"expected_source_substr,omitempty"`
	Cells                []ChunkSimCell `json:"cells"`
}

// ChunkSimMatrixReport calibrates AVLP_RAG_MIN_SIMILARITY.
type ChunkSimMatrixReport struct {
	EmbedderMode           string        `json:"embedder_mode"`
	CurrentMinSimilarity   float32       `json:"min_similarity_current"`
	SuggestedMinSimilarity float32       `json:"suggested_min_similarity"`
	WorstOnTopic           float32       `json:"worst_on_topic"`
	BestOffTopic           float32       `json:"best_off_topic"`
	Margin                 float32       `json:"margin"`
	Warning                string        `json:"warning,omitempty"`
	ChunkIDs               []string      `json:"chunk_ids"`
	Rows                   []ChunkSimRow `json:"rows"`
}

// LoadChunkSimCases reads the query set for RAG-floor calibration.
func LoadChunkSimCases(path string) ([]ChunkSimCase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []ChunkSimCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

// BuildChunkSimMatrix scores each query against every indexed RAG chunk.
func BuildChunkSimMatrix(
	ctx context.Context,
	cases []ChunkSimCase,
	store *rag.Store,
	emb rag.Embedder,
	mode string,
) (ChunkSimMatrixReport, error) {
	if store == nil || store.Len() == 0 {
		return ChunkSimMatrixReport{}, fmt.Errorf("RAG store is empty")
	}
	if emb == nil {
		emb = rag.NewHashEmbedder(rag.DefaultEmbedDims)
	}
	if mode == "" {
		mode = "hash"
	}
	chunks := store.Chunks()
	report := ChunkSimMatrixReport{
		EmbedderMode:         mode,
		CurrentMinSimilarity: rag.MinSimilarityFromEnv(),
		WorstOnTopic:         1,
		ChunkIDs:             make([]string, 0, len(chunks)),
	}
	for _, chunk := range chunks {
		report.ChunkIDs = append(report.ChunkIDs, chunk.ID)
	}

	haveOnTopic := false
	haveOffTopic := false
	for _, c := range cases {
		query, err := emb.Embed(ctx, c.QueryText)
		if err != nil {
			return ChunkSimMatrixReport{}, fmt.Errorf("%s: embed: %w", c.CaseID, err)
		}
		cells := make([]ChunkSimCell, 0, len(chunks))
		bestIndex := -1
		bestSimilarity := float32(-1)
		bestExpected := float32(-1)
		for i, chunk := range chunks {
			similarity := vector.CosineSimilarity(query, chunk.Embedding)
			expected := c.ExpectedSourceSubstr != "" &&
				strings.Contains(strings.ToLower(chunk.Source), strings.ToLower(c.ExpectedSourceSubstr))
			cells = append(cells, ChunkSimCell{
				ChunkID:          chunk.ID,
				Source:           chunk.Source,
				Title:            chunk.Title,
				Similarity:       similarity,
				IsExpectedSource: expected,
			})
			if similarity > bestSimilarity {
				bestSimilarity = similarity
				bestIndex = i
			}
			if expected && similarity > bestExpected {
				bestExpected = similarity
			}
		}
		if bestIndex >= 0 {
			cells[bestIndex].IsNearest = true
		}
		switch strings.ToLower(strings.TrimSpace(c.Role)) {
		case "on_topic":
			if bestExpected >= 0 {
				haveOnTopic = true
				if bestExpected < report.WorstOnTopic {
					report.WorstOnTopic = bestExpected
				}
			}
		case "off_topic":
			haveOffTopic = true
			if bestSimilarity > report.BestOffTopic {
				report.BestOffTopic = bestSimilarity
			}
		}
		sort.SliceStable(cells, func(i, j int) bool {
			return cells[i].Similarity > cells[j].Similarity
		})
		report.Rows = append(report.Rows, ChunkSimRow{
			CaseID:               c.CaseID,
			QueryText:            c.QueryText,
			Role:                 c.Role,
			ExpectedSourceSubstr: c.ExpectedSourceSubstr,
			Cells:                cells,
		})
	}

	if !haveOnTopic || !haveOffTopic {
		report.Warning = "insufficient on-topic/off-topic evidence to calibrate the RAG floor"
		report.SuggestedMinSimilarity = report.CurrentMinSimilarity
		if !haveOnTopic {
			report.WorstOnTopic = 0
		}
		return report, nil
	}
	report.Margin = report.WorstOnTopic - report.BestOffTopic
	report.SuggestedMinSimilarity = (report.WorstOnTopic + report.BestOffTopic) / 2
	if report.Margin <= 0 {
		report.Warning = fmt.Sprintf(
			"overlap: worst_on_topic=%.3f <= best_off_topic=%.3f",
			report.WorstOnTopic,
			report.BestOffTopic,
		)
	} else if report.Margin < 0.05 {
		report.Warning = fmt.Sprintf("thin RAG margin %.3f < 0.05", report.Margin)
	}
	return report, nil
}

// WriteChunkSimMatrixJSON writes the RAG matrix as indented JSON.
func WriteChunkSimMatrixJSON(path string, report ChunkSimMatrixReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// FormatChunkSimMatrixTable renders a compact query×chunk summary.
func FormatChunkSimMatrixTable(report ChunkSimMatrixReport) string {
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"simmatrix-rag embedder=%s floor=%.3f chunks=%d queries=%d\n\n",
		report.EmbedderMode,
		report.CurrentMinSimilarity,
		len(report.ChunkIDs),
		len(report.Rows),
	)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "case\trole\tnearest\tsim\texpected")
	for _, row := range report.Rows {
		var nearest ChunkSimCell
		for _, cell := range row.Cells {
			if cell.IsNearest {
				nearest = cell
				break
			}
		}
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%.3f\t%t\n",
			row.CaseID,
			row.Role,
			filepath.Base(nearest.ChunkID),
			nearest.Similarity,
			nearest.IsExpectedSource,
		)
	}
	_ = w.Flush()
	fmt.Fprintf(
		&b,
		"\ncurrent=%.3f suggested=%.3f worst_on=%.3f best_off=%.3f margin=%.3f\n",
		report.CurrentMinSimilarity,
		report.SuggestedMinSimilarity,
		report.WorstOnTopic,
		report.BestOffTopic,
		report.Margin,
	)
	if report.Warning != "" {
		fmt.Fprintf(&b, "WARNING: %s\n", report.Warning)
	}
	return b.String()
}

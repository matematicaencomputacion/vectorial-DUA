package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vectorial-dua/avlp/harness/telemetry"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

const (
	weightCosine    = 0.35
	weightDimension = 0.30
	weightFormat    = 0.20
	weightRogers    = 0.15
	passThreshold   = 0.80
)

// Case is a golden pedagogical routing scenario.
type Case struct {
	CaseID                 string    `json:"case_id"`
	Description            string    `json:"description"`
	StudentID              string    `json:"student_id"`
	QueryEmbedding         []float32 `json:"query_embedding"`
	QueryText              string    `json:"query_text,omitempty"`
	MinSimilarityThreshold float32   `json:"min_similarity_threshold"`
	ExpectedDimensionDUA   string    `json:"expected_dimension_dua"`
	ExpectedFormat         string    `json:"expected_format"`
	ExpectedOutcome        string    `json:"expected_outcome"` // static | live
	ExpectedNodeIDPrefix   string    `json:"expected_node_id_prefix"`
}

// SignalScores holds weighted quality signals.
type SignalScores struct {
	CosineOK          float64 `json:"cosine_ok"`
	DUADimensionMatch float64 `json:"dua_dimension_match"`
	FormatMatch       float64 `json:"format_match"`
	RogersSafety      float64 `json:"rogers_safety"`
	Aggregate         float64 `json:"aggregate"`
}

// CaseResult is the scored outcome of one case.
type CaseResult struct {
	CaseID             string       `json:"case_id"`
	Passed             bool         `json:"passed"`
	AggregateScore     float64      `json:"aggregate_score"`
	Signals            SignalScores `json:"signals"`
	ActualOutcome      string       `json:"actual_outcome"`
	ActualNodeID       string       `json:"actual_node_id,omitempty"`
	ActualDimensionDUA string       `json:"actual_dimension_dua,omitempty"`
	ActualFormat       string       `json:"actual_format,omitempty"`
	SimilarityScore    float32      `json:"similarity_score"`
	Message            string       `json:"message,omitempty"`
}

// Report aggregates suite results.
type Report struct {
	RunID             string       `json:"run_id"`
	StartedAtUnixMS   int64        `json:"started_at_unix_ms"`
	FinishedAtUnixMS  int64        `json:"finished_at_unix_ms"`
	TotalCases        int          `json:"total_cases"`
	PassedCases       int          `json:"passed_cases"`
	FailedCases       int          `json:"failed_cases"`
	PassRate          float64      `json:"pass_rate"`
	Results           []CaseResult `json:"results"`
}

// LoadCases reads a golden JSON array from path.
func LoadCases(path string) ([]Case, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := json.Unmarshal(b, &cases); err != nil {
		return nil, fmt.Errorf("parse golden cases: %w", err)
	}
	return cases, nil
}

// Score computes pedagogical quality signals for a routing outcome.
func Score(c Case, out vector.RouteOutcome) CaseResult {
	actual := "pending"
	if out.Matched && out.IsLiveGenerated {
		actual = "live"
	} else if out.Matched {
		actual = "static"
	} else if out.TrackingULID != "" {
		actual = "live" // pending live station
	}

	signals := SignalScores{}

	// cosine_ok / outcome correctness
	switch strings.ToLower(c.ExpectedOutcome) {
	case "static":
		if out.Matched && !out.IsLiveGenerated && out.Similarity >= vector.ResolveThreshold(c.MinSimilarityThreshold) {
			signals.CosineOK = 1
		}
	case "live":
		if out.IsLiveGenerated && out.Matched {
			signals.CosineOK = 1
		} else if !out.Matched && out.TrackingULID != "" {
			signals.CosineOK = 1 // pending fallback without RAG
		}
	default:
		signals.CosineOK = 0
	}

	// dua dimension
	if out.Matched && c.ExpectedDimensionDUA != "" {
		if strings.EqualFold(out.Node.DimensionDUA, c.ExpectedDimensionDUA) {
			signals.DUADimensionMatch = 1
		}
	} else if strings.EqualFold(c.ExpectedOutcome, "live") && (out.IsLiveGenerated || !out.Matched) {
		signals.DUADimensionMatch = 0.7
	}

	// format
	if out.Matched && c.ExpectedFormat != "" {
		if strings.EqualFold(out.Node.Format, c.ExpectedFormat) {
			signals.FormatMatch = 1
		}
	} else if strings.EqualFold(c.ExpectedOutcome, "live") && (out.IsLiveGenerated || !out.Matched) {
		signals.FormatMatch = 0.7
	}

	// rogers_safety: live generated or pending is safe; forcing static when live expected is not
	signals.RogersSafety = 1
	if strings.EqualFold(c.ExpectedOutcome, "live") && out.Matched && !out.IsLiveGenerated {
		signals.RogersSafety = 0
	}
	if strings.EqualFold(c.ExpectedOutcome, "static") && !out.Matched {
		signals.RogersSafety = 0.4
	}

	if c.ExpectedNodeIDPrefix != "" && out.Matched {
		if !strings.HasPrefix(out.Node.ID, c.ExpectedNodeIDPrefix) {
			signals.DUADimensionMatch *= 0.5
		}
	}

	signals.Aggregate = weightCosine*signals.CosineOK +
		weightDimension*signals.DUADimensionMatch +
		weightFormat*signals.FormatMatch +
		weightRogers*signals.RogersSafety

	res := CaseResult{
		CaseID:             c.CaseID,
		Passed:             signals.Aggregate >= passThreshold,
		AggregateScore:     signals.Aggregate,
		Signals:            signals,
		ActualOutcome:      actual,
		SimilarityScore:    out.Similarity,
		ActualNodeID:       out.Node.ID,
		ActualDimensionDUA: out.Node.DimensionDUA,
		ActualFormat:       out.Node.Format,
	}
	if !res.Passed {
		res.Message = fmt.Sprintf("score %.3f < %.2f (expected=%s actual=%s)", signals.Aggregate, passThreshold, c.ExpectedOutcome, actual)
	}
	return res
}

// Runner executes golden cases against an in-process router.
type Runner struct {
	Router *vector.Router
	Tel    *telemetry.Collector
}

// Run executes all cases and returns a report.
func (r *Runner) Run(cases []Case) Report {
	start := time.Now().UTC()
	runID := start.Format("20060102T150405Z")
	report := Report{
		RunID:           runID,
		StartedAtUnixMS: start.UnixMilli(),
		TotalCases:      len(cases),
	}

	for _, c := range cases {
		t0 := time.Now()
		query := append([]float32(nil), c.QueryEmbedding...)
		if len(query) == 0 && c.QueryText != "" {
			var err error
			query, err = rag.DefaultEmbedder().Embed(context.Background(), c.QueryText)
			if err != nil {
				result := CaseResult{CaseID: c.CaseID, Passed: false, Message: fmt.Sprintf("embed query_text: %v", err)}
				report.Results = append(report.Results, result)
				report.FailedCases++
				continue
			}
		}
		dim := c.ExpectedDimensionDUA
		if dim == "" {
			dim = "Representacion"
		}
		format := c.ExpectedFormat
		if format == "" {
			format = "conceptual"
		}
		out, err := r.Router.QueryNearestWithOptions(context.Background(), c.StudentID, query, c.MinSimilarityThreshold, vector.QueryOptions{
			DoubtText:   c.QueryText,
			Frustration: 0.6,
			Dimension:   dim,
			Format:      format,
		})
		if r.Tel != nil {
			r.Tel.ObserveRouting(time.Since(t0))
		}
		var result CaseResult
		if err != nil {
			result = CaseResult{CaseID: c.CaseID, Passed: false, Message: err.Error()}
		} else {
			result = Score(c, out)
		}
		report.Results = append(report.Results, result)
		if result.Passed {
			report.PassedCases++
			if r.Tel != nil {
				r.Tel.Inc("eval_pass_total", 1)
			}
		} else {
			report.FailedCases++
			if r.Tel != nil {
				r.Tel.Inc("eval_fail_total", 1)
			}
		}
		if out.Matched {
			if r.Tel != nil {
				r.Tel.Inc("routing_match_total", 1)
			}
		} else if err == nil {
			if r.Tel != nil {
				r.Tel.Inc("routing_live_total", 1)
			}
		}
	}

	report.FinishedAtUnixMS = time.Now().UTC().UnixMilli()
	if report.TotalCases > 0 {
		report.PassRate = float64(report.PassedCases) / float64(report.TotalCases)
	}
	return report
}

// WriteReport writes the eval report as indented JSON.
func WriteReport(path string, report Report) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

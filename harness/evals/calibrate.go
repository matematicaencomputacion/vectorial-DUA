package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CalibrationCase is per-golden threshold evidence.
type CalibrationCase struct {
	CaseID           string  `json:"case_id"`
	ExpectedOutcome  string  `json:"expected_outcome"`
	CorrectNodeID    string  `json:"correct_node_id,omitempty"`
	CorrectSim       float32 `json:"correct_sim,omitempty"`
	BestIncorrectSim float32 `json:"best_incorrect_sim"`
	BestIncorrectID  string  `json:"best_incorrect_id,omitempty"`
	HasCorrect       bool    `json:"has_correct"`
}

// CalibrationReport suggests a similarity threshold from golden×index scores.
type CalibrationReport struct {
	EmbedderMode       string            `json:"embedder_mode"`
	CurrentThreshold   float32           `json:"current_threshold"`
	SuggestedThreshold float32           `json:"suggested_threshold"`
	WorstCorrect       float32           `json:"worst_correct"`
	BestIncorrect      float32           `json:"best_incorrect"`
	Margin             float32           `json:"margin"`
	Warning            string            `json:"warning,omitempty"`
	Cases              []CalibrationCase `json:"cases"`
}

// BuildCalibration derives a suggested threshold from a simmatrix using
// semantic expected_outcome (not expected_outcome_hash).
//
// For static cases: correct = node matching expected_node_id_prefix (or dim+format);
// incorrect = max sim among other nodes.
// For live cases: no correct static node; best_incorrect = nearest static sim
// (out-of-manifold ceiling).
func BuildCalibration(matrix SimMatrixReport, cases []Case) CalibrationReport {
	byID := make(map[string]Case, len(cases))
	for _, c := range cases {
		byID[c.CaseID] = c
	}

	rep := CalibrationReport{
		EmbedderMode:     matrix.EmbedderMode,
		CurrentThreshold: matrix.Threshold,
		WorstCorrect:     1,
		BestIncorrect:    0,
	}

	var haveCorrect, haveIncorrect bool

	for _, row := range matrix.Rows {
		c, ok := byID[row.CaseID]
		if !ok {
			continue
		}
		expected := strings.ToLower(strings.TrimSpace(c.ExpectedOutcome))
		cc := CalibrationCase{
			CaseID:          c.CaseID,
			ExpectedOutcome: expected,
		}

		switch expected {
		case "static":
			correctID, correctSim, okCorrect := pickCorrect(row.Cells, c)
			bestWrong, bestWrongID := pickBestIncorrect(row.Cells, correctID)
			cc.BestIncorrectSim = bestWrong
			cc.BestIncorrectID = bestWrongID
			haveIncorrect = true
			if bestWrong > rep.BestIncorrect {
				rep.BestIncorrect = bestWrong
			}
			if okCorrect {
				cc.HasCorrect = true
				cc.CorrectNodeID = correctID
				cc.CorrectSim = correctSim
				haveCorrect = true
				if correctSim < rep.WorstCorrect {
					rep.WorstCorrect = correctSim
				}
			}
		case "live":
			best, bestID := pickBestAny(row.Cells)
			cc.BestIncorrectSim = best
			cc.BestIncorrectID = bestID
			haveIncorrect = true
			if best > rep.BestIncorrect {
				rep.BestIncorrect = best
			}
		}
		rep.Cases = append(rep.Cases, cc)
	}

	if !haveCorrect || !haveIncorrect {
		rep.Warning = "insufficient static/live evidence to suggest a threshold"
		rep.SuggestedThreshold = matrix.Threshold
		if !haveCorrect {
			rep.WorstCorrect = 0
		}
		return rep
	}

	margin := rep.WorstCorrect - rep.BestIncorrect
	rep.Margin = margin
	rep.SuggestedThreshold = (rep.WorstCorrect + rep.BestIncorrect) / 2
	if margin <= 0 {
		rep.Warning = fmt.Sprintf("overlap: worst_correct=%.3f <= best_incorrect=%.3f — descriptors/threshold need work",
			rep.WorstCorrect, rep.BestIncorrect)
	} else if margin < 0.05 {
		rep.Warning = fmt.Sprintf("thin margin %.3f < 0.05 — re-check descriptors before deploying", margin)
	}
	return rep
}

func pickCorrect(cells []SimCell, c Case) (id string, sim float32, ok bool) {
	prefix := strings.TrimSpace(c.ExpectedNodeIDPrefix)
	for _, cell := range cells {
		if prefix != "" && strings.HasPrefix(cell.NodeID, prefix) {
			return cell.NodeID, cell.Similarity, true
		}
	}
	if prefix == "" && c.ExpectedDimensionDUA != "" {
		for _, cell := range cells {
			if !strings.EqualFold(cell.Dimension, c.ExpectedDimensionDUA) {
				continue
			}
			if c.ExpectedFormat != "" && !strings.EqualFold(cell.Format, c.ExpectedFormat) {
				continue
			}
			if !ok || cell.Similarity > sim {
				id, sim, ok = cell.NodeID, cell.Similarity, true
			}
		}
	}
	return id, sim, ok
}

func pickBestIncorrect(cells []SimCell, correctID string) (float32, string) {
	best := float32(-1)
	var id string
	for _, cell := range cells {
		if correctID != "" && cell.NodeID == correctID {
			continue
		}
		if cell.Similarity > best {
			best = cell.Similarity
			id = cell.NodeID
		}
	}
	if best < 0 {
		return 0, ""
	}
	return best, id
}

func pickBestAny(cells []SimCell) (float32, string) {
	best := float32(-1)
	var id string
	for _, cell := range cells {
		if cell.Similarity > best {
			best = cell.Similarity
			id = cell.NodeID
		}
	}
	if best < 0 {
		return 0, ""
	}
	return best, id
}

// WriteCalibrationJSON writes the calibration report as indented JSON.
func WriteCalibrationJSON(path string, report CalibrationReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// FormatCalibrationReport renders a human-readable calibration summary.
func FormatCalibrationReport(rep CalibrationReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "calibrate embedder=%s current_threshold=%.3f\n", rep.EmbedderMode, rep.CurrentThreshold)
	fmt.Fprintf(&b, "  worst_correct=%.3f  best_incorrect=%.3f  margin=%.3f\n",
		rep.WorstCorrect, rep.BestIncorrect, rep.Margin)
	fmt.Fprintf(&b, "  suggested_threshold=%.3f\n", rep.SuggestedThreshold)
	if rep.Warning != "" {
		fmt.Fprintf(&b, "  WARNING: %s\n", rep.Warning)
	}
	b.WriteString("\n")
	for _, c := range rep.Cases {
		if c.HasCorrect {
			fmt.Fprintf(&b, "  %-36s static correct=%.3f (%s)  best_wrong=%.3f\n",
				c.CaseID, c.CorrectSim, shortNodeLabel(c.CorrectNodeID), c.BestIncorrectSim)
		} else {
			fmt.Fprintf(&b, "  %-36s %-6s best_sim=%.3f (%s)\n",
				c.CaseID, c.ExpectedOutcome, c.BestIncorrectSim, shortNodeLabel(c.BestIncorrectID))
		}
	}
	return b.String()
}

package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Advice is student-facing guidance about unmet foundational concepts.
type Advice struct {
	MessageES string
	Gaps      []Relation
}

// Advisor combines curriculum structure with visit evidence.
type Advisor struct {
	Graph  KnowledgeGraph
	Visits ConceptVisitStore
}

// Advise returns rogerian Spanish copy when focus has unmet prerequisites.
// Empty MessageES means no gap (or nothing to say).
func (a *Advisor) Advise(ctx context.Context, studentID string, focus ConceptID) (Advice, error) {
	if a == nil || a.Graph == nil {
		return Advice{}, fmt.Errorf("knowledge: advisor not configured")
	}
	focusConcept, err := a.Graph.Concept(ctx, focus)
	if err != nil {
		return Advice{}, err
	}
	prereqs, err := a.Graph.Prerequisites(ctx, focus, TraverseOptions{})
	if err != nil {
		return Advice{}, err
	}
	var gaps []Relation
	for _, rel := range prereqs {
		visited := false
		if a.Visits != nil {
			visited, err = a.Visits.HasVisited(ctx, studentID, rel.Peer.ID)
			if err != nil {
				return Advice{}, err
			}
		}
		if !visited {
			gaps = append(gaps, rel)
		}
	}
	if len(gaps) == 0 {
		return Advice{}, nil
	}
	return Advice{
		MessageES: formatAdvice(focusConcept, gaps),
		Gaps:      gaps,
	}, nil
}

// AdviseForConcepts unions gaps across several focus concepts (node binding).
func (a *Advisor) AdviseForConcepts(ctx context.Context, studentID string, focuses []ConceptID) (Advice, error) {
	seen := map[ConceptID]struct{}{}
	var gaps []Relation
	var primary Concept
	for i, focus := range focuses {
		adv, err := a.Advise(ctx, studentID, focus)
		if err != nil {
			if errors.Is(err, ErrConceptNotFound) {
				continue
			}
			return Advice{}, err
		}
		if i == 0 || primary.ID == "" {
			if c, cerr := a.Graph.Concept(ctx, focus); cerr == nil {
				primary = c
			}
		}
		for _, g := range adv.Gaps {
			if _, ok := seen[g.Peer.ID]; ok {
				continue
			}
			seen[g.Peer.ID] = struct{}{}
			gaps = append(gaps, g)
		}
	}
	if len(gaps) == 0 {
		return Advice{}, nil
	}
	gaps = sortRelations(gaps)
	if primary.ID == "" {
		primary.Title = "este tema"
	}
	return Advice{
		MessageES: formatAdvice(primary, gaps),
		Gaps:      gaps,
	}, nil
}

func formatAdvice(focus Concept, gaps []Relation) string {
	focusTitle := strings.TrimSpace(focus.Title)
	if focusTitle == "" {
		focusTitle = "este tema"
	}
	parts := make([]string, 0, len(gaps))
	for _, g := range gaps {
		title := strings.TrimSpace(g.Peer.Title)
		if title == "" {
			title = "un tema previo"
		}
		rationale := studentFacingRationale(g.RationaleES)
		if rationale != "" {
			parts = append(parts, fmt.Sprintf("«%s» (%s)", title, rationale))
		} else {
			parts = append(parts, fmt.Sprintf("«%s»", title))
		}
	}
	switch len(parts) {
	case 1:
		return fmt.Sprintf(
			"Antes de seguir con «%s», te conviene mirar %s. Cuando quieras, volvemos a tu duda.",
			focusTitle, parts[0],
		)
	default:
		return fmt.Sprintf(
			"Antes de seguir con «%s», conviene pasar por %s. Vamos a tu ritmo.",
			focusTitle, joinES(parts),
		)
	}
}

func studentFacingRationale(raw string) string {
	s := strings.TrimSpace(raw)
	// Strip draft authorship markers so the student never sees editorial meta-copy.
	for _, prefix := range []string{
		"[BORRADOR — curaduría pendiente]",
		"[BORRADOR - curaduría pendiente]",
		"[BORRADOR]",
	} {
		s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
	}
	return s
}

func joinES(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " y " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " y " + parts[len(parts)-1]
	}
}

// RecordConcepts marks a list of concept refs (id or slug) as visited for student.
func RecordConcepts(ctx context.Context, store ConceptVisitStore, studentID string, refs []string) error {
	if store == nil || strings.TrimSpace(studentID) == "" {
		return nil
	}
	for _, raw := range refs {
		id, err := NormalizeConceptRef(raw)
		if err != nil {
			continue
		}
		if err := store.RecordVisit(ctx, studentID, id); err != nil {
			return err
		}
	}
	return nil
}

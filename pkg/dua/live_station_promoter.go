package dua

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

var (
	ErrInvalidTrackingULID  = errors.New("invalid tracking ULID")
	ErrStationNotFound      = errors.New("live station not found")
	ErrStationNotReady      = errors.New("live station is not ready")
	ErrPromotionUnavailable = errors.New("live station promotion unavailable")
)

// PromotionResult is the durable result of curating one ready live station.
type PromotionResult struct {
	TrackingULID string
	Node         *InteractiveVideoNode
	SeedPath     string
	Created      bool
}

// LiveStationPromoter persists reviewed stations and replaces their live index
// entry with a curated interactive node. Calls are serialized for idempotency.
type LiveStationPromoter struct {
	Ledger   *vector.StationLedger
	Index    *vector.Index
	Registry *Registry
	SeedsDir string

	mu sync.Mutex
}

// Promote converts a ready station into a persistent curated seed.
func (p *LiveStationPromoter) Promote(trackingULID string) (PromotionResult, error) {
	if !vector.ValidateTrackingULID(trackingULID) {
		return PromotionResult{}, ErrInvalidTrackingULID
	}
	if p == nil || p.Index == nil || strings.TrimSpace(p.SeedsDir) == "" {
		return PromotionResult{}, ErrPromotionUnavailable
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	seedPath, err := promotedSeedPath(p.SeedsDir, trackingULID)
	if err != nil {
		return PromotionResult{}, fmt.Errorf("%w: %v", ErrPromotionUnavailable, err)
	}
	if node, found, err := readPromotedSeed(seedPath, trackingULID); err != nil {
		return PromotionResult{}, err
	} else if found {
		if err := p.installCurated(node); err != nil {
			return PromotionResult{}, err
		}
		return PromotionResult{
			TrackingULID: trackingULID,
			Node:         node.Clone(),
			SeedPath:     seedPath,
			Created:      false,
		}, nil
	}

	if p.Ledger == nil {
		return PromotionResult{}, ErrPromotionUnavailable
	}
	rec := p.Ledger.Get(trackingULID)
	if rec == nil {
		return PromotionResult{}, ErrStationNotFound
	}
	if rec.Status != vector.StationReady || rec.Result == nil {
		return PromotionResult{}, ErrStationNotReady
	}

	node, err := buildPromotedNode(rec)
	if err != nil {
		return PromotionResult{}, err
	}
	if err := writePromotedSeed(seedPath, node); err != nil {
		return PromotionResult{}, fmt.Errorf("write promoted seed: %w", err)
	}
	if err := p.installCurated(node); err != nil {
		return PromotionResult{}, err
	}
	return PromotionResult{
		TrackingULID: trackingULID,
		Node:         node.Clone(),
		SeedPath:     seedPath,
		Created:      true,
	}, nil
}

func buildPromotedNode(rec *vector.StationRecord) (*InteractiveVideoNode, error) {
	if rec == nil || rec.Result == nil {
		return nil, ErrStationNotReady
	}
	live := rec.Result.Node
	parts, err := vector.ParseNodeID(live.ID)
	if err != nil {
		return nil, fmt.Errorf("promoted node id: %w", err)
	}
	if len(live.Embedding) == 0 {
		return nil, fmt.Errorf("promoted node embedding is empty")
	}
	title := truncateRunes(strings.TrimSpace(rec.Request.DoubtText), 80)
	if title == "" {
		title = "Estación promovida"
	}
	descriptor := strings.TrimSpace(rec.Request.DoubtText)
	if descriptor == "" {
		descriptor = title
	}
	node := &InteractiveVideoNode{
		NodeID:              live.ID,
		DimensionDUA:        live.DimensionDUA,
		Titulo:              title,
		LayoutType:          LayoutInteractiveDashboard,
		StageMediaDefault:   "master://promoted/" + parts.ULID,
		Embedding:           append([]float32(nil), live.Embedding...),
		EmbeddingDescriptor: descriptor,
		Botonera: []InteractiveButton{{
			IDBtn:      "ask_different",
			Label:      "+ Tengo una duda diferente",
			ActionType: ActionAskAgent,
		}},
		StageMarkdownDefault:     rec.Result.Content,
		RetrievedSources:         append([]string(nil), rec.Result.Sources...),
		PromotedFromTrackingULID: rec.TrackingULID,
	}
	if node.DimensionDUA == "" {
		node.DimensionDUA = parts.Dimension
	}
	if err := node.Validate(); err != nil {
		return nil, fmt.Errorf("promoted seed: %w", err)
	}
	return node, nil
}

func (p *LiveStationPromoter) installCurated(node *InteractiveVideoNode) error {
	parts, err := vector.ParseNodeID(node.NodeID)
	if err != nil {
		return err
	}
	if len(node.Embedding) != p.Index.Dims() {
		return fmt.Errorf("promoted embedding dims mismatch: got %d want %d", len(node.Embedding), p.Index.Dims())
	}
	if err := p.Index.Upsert(vector.Node{
		ID:           node.NodeID,
		DimensionDUA: node.DimensionDUA,
		Difficulty:   parts.Difficulty,
		Format:       parts.Format,
		ResourceURL:  "interactive://" + node.NodeID,
		Embedding:    append([]float32(nil), node.Embedding...),
	}); err != nil {
		return fmt.Errorf("index promoted node: %w", err)
	}
	if p.Registry != nil {
		if err := p.Registry.Put(node); err != nil {
			return fmt.Errorf("register promoted node: %w", err)
		}
	}
	return nil
}

func promotedSeedPath(dir, trackingULID string) (string, error) {
	if !vector.ValidateTrackingULID(trackingULID) {
		return "", ErrInvalidTrackingULID
	}
	base, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "promoted-"+trackingULID+".json")
	rel, err := filepath.Rel(base, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("seed path escapes configured directory")
	}
	return path, nil
}

func readPromotedSeed(path, trackingULID string) (*InteractiveVideoNode, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read promoted seed: %w", err)
	}
	var node InteractiveVideoNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, false, fmt.Errorf("decode promoted seed: %w", err)
	}
	if node.PromotedFromTrackingULID != trackingULID {
		return nil, false, fmt.Errorf("promoted seed tracking ULID mismatch")
	}
	if err := node.Validate(); err != nil {
		return nil, false, fmt.Errorf("invalid promoted seed: %w", err)
	}
	return &node, true, nil
}

func writePromotedSeed(path string, node *InteractiveVideoNode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".promoted-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
}

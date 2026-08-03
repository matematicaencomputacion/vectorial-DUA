package dua

import (
	"fmt"
	"strings"
)

// ButtonActionType is the action fired from the lateral botonera.
type ButtonActionType string

const (
	ActionPlayClip    ButtonActionType = "play_clip"
	ActionOpenIDECell ButtonActionType = "open_ide_cell"
	ActionAskAgent    ButtonActionType = "ask_agent"
)

// InteractiveButton is one atomized DUA control.
type InteractiveButton struct {
	IDBtn           string           `json:"id_btn"`
	Label           string           `json:"label"`
	ActionType      ButtonActionType `json:"action_type"`
	MediaURL        string           `json:"media_url,omitempty"`
	TimestampStart  float32          `json:"timestamp_start,omitempty"`
	TimestampEnd    float32          `json:"timestamp_end,omitempty"`
	CellCode        string           `json:"cell_code,omitempty"`
	VectorDelta     []float32        `json:"vector_delta,omitempty"`
	IsLiveGenerated bool             `json:"is_live_generated,omitempty"`
}

// InteractiveVideoNode is a Stage + botonera DUA resource.
type InteractiveVideoNode struct {
	NodeID                   string               `json:"node_id"`
	DimensionDUA             string               `json:"dimension_dua"`
	Titulo                   string               `json:"titulo"`
	LayoutType               LayoutType           `json:"layout_type"`
	StageMediaDefault        string               `json:"stage_media_default"`
	Botonera                 []InteractiveButton  `json:"botonera,omitempty"`
	Embedding                []float32            `json:"embedding,omitempty"`
	EmbeddingDescriptor      string               `json:"embedding_descriptor,omitempty"`
	BotoneraSchema           *DUANodeBotonera     `json:"botonera_schema,omitempty"`
	Hierarchy                *DUAHierarchicalTree `json:"hierarchy,omitempty"`
	StageMarkdownDefault     string               `json:"stage_markdown_default,omitempty"`
	RetrievedSources         []string             `json:"retrieved_sources,omitempty"`
	PromotedFromTrackingULID string               `json:"promoted_from_tracking_ulid,omitempty"`
	CaptionsURL              string               `json:"captions_url,omitempty"`
	Transcript               string               `json:"transcript,omitempty"`
	AltText                  string               `json:"alt_text,omitempty"`
	AudioDescriptionURL      string               `json:"audio_description_url,omitempty"`
	Concepts                 []string             `json:"concepts,omitempty"` // knowledge concept ids (Ola 7)
}

// Validate checks structural integrity of an interactive node.
func (n *InteractiveVideoNode) Validate() error {
	if n == nil {
		return fmt.Errorf("node is nil")
	}
	if strings.TrimSpace(n.NodeID) == "" {
		return fmt.Errorf("node_id is required")
	}
	if !ValidDimension(n.DimensionDUA) {
		return fmt.Errorf("invalid dimension_dua: %s", n.DimensionDUA)
	}
	if n.LayoutType != LayoutInteractiveDashboard {
		return fmt.Errorf("layout_type must be %s", LayoutInteractiveDashboard)
	}
	if strings.TrimSpace(n.Titulo) == "" {
		return fmt.Errorf("titulo is required")
	}
	if strings.TrimSpace(n.StageMediaDefault) == "" {
		return fmt.Errorf("stage_media_default is required")
	}
	if n.BotoneraSchema != nil {
		if err := n.BotoneraSchema.Validate(); err != nil {
			return fmt.Errorf("botonera_schema: %w", err)
		}
	}
	if n.Hierarchy != nil {
		if err := n.Hierarchy.Validate(); err != nil {
			return fmt.Errorf("hierarchy: %w", err)
		}
	}
	if len(n.Botonera) == 0 && n.BotoneraSchema == nil && n.Hierarchy == nil {
		return fmt.Errorf("botonera, botonera_schema, or hierarchy is required")
	}
	seen := map[string]struct{}{}
	for i, b := range n.Botonera {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("botonera[%d]: %w", i, err)
		}
		if _, ok := seen[b.IDBtn]; ok {
			return fmt.Errorf("duplicate id_btn: %s", b.IDBtn)
		}
		seen[b.IDBtn] = struct{}{}
	}
	return nil
}

// Validate checks a single button.
func (b InteractiveButton) Validate() error {
	if strings.TrimSpace(b.IDBtn) == "" {
		return fmt.Errorf("id_btn is required")
	}
	if strings.TrimSpace(b.Label) == "" {
		return fmt.Errorf("label is required")
	}
	switch b.ActionType {
	case ActionPlayClip:
		if strings.TrimSpace(b.MediaURL) == "" && b.TimestampEnd <= b.TimestampStart {
			return fmt.Errorf("play_clip requires media_url or valid timestamps")
		}
	case ActionOpenIDECell:
		if strings.TrimSpace(b.CellCode) == "" {
			return fmt.Errorf("open_ide_cell requires cell_code")
		}
	case ActionAskAgent:
		// reserved entry point for "duda diferente"
	default:
		return fmt.Errorf("unknown action_type: %s", b.ActionType)
	}
	return nil
}

// Clone returns a deep copy of the node (botonera slice copied).
func (n *InteractiveVideoNode) Clone() *InteractiveVideoNode {
	if n == nil {
		return nil
	}
	out := *n
	out.Botonera = append([]InteractiveButton(nil), n.Botonera...)
	out.Embedding = append([]float32(nil), n.Embedding...)
	out.RetrievedSources = append([]string(nil), n.RetrievedSources...)
	out.Concepts = append([]string(nil), n.Concepts...)
	for i := range out.Botonera {
		out.Botonera[i].VectorDelta = append([]float32(nil), n.Botonera[i].VectorDelta...)
	}
	if n.BotoneraSchema != nil {
		schema := *n.BotoneraSchema
		schema.FlatButtons = append([]InteractiveButton(nil), n.BotoneraSchema.FlatButtons...)
		schema.DepthOptions = append([]DepthVariant(nil), n.BotoneraSchema.DepthOptions...)
		schema.CognitiveOptions = append([]CognitiveVariant(nil), n.BotoneraSchema.CognitiveOptions...)
		schema.EmergencyOptions = append([]EmergencyVariant(nil), n.BotoneraSchema.EmergencyOptions...)
		schema.DepthAxis = append([]string(nil), n.BotoneraSchema.DepthAxis...)
		schema.FormatAxis = append([]string(nil), n.BotoneraSchema.FormatAxis...)
		schema.MatrixCells = append([]CombinedCell(nil), n.BotoneraSchema.MatrixCells...)
		out.BotoneraSchema = &schema
	}
	out.Hierarchy = n.Hierarchy.Clone()
	return &out
}

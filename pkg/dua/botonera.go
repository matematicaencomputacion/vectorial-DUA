package dua

import "fmt"

// BotoneraSchemaKind selects a reusable structural botonera pattern.
type BotoneraSchemaKind string

const (
	SchemaFlat      BotoneraSchemaKind = "flat"
	SchemaDepth     BotoneraSchemaKind = "depth"
	SchemaCognitive BotoneraSchemaKind = "cognitive"
	SchemaEmergency BotoneraSchemaKind = "emergency"
	SchemaCombined  BotoneraSchemaKind = "combined"
)

// MediaFormatType is how content is rendered on the Stage.
type MediaFormatType string

const (
	MediaVideo             MediaFormatType = "video"
	MediaLiveCode          MediaFormatType = "live_code"
	MediaDiagram           MediaFormatType = "diagram"
	MediaAudioDebate       MediaFormatType = "audio_debate"
	MediaInteractiveCanvas MediaFormatType = "interactive_canvas"
	MediaTextHint          MediaFormatType = "text_hint"
)

// DepthVariant is a zoom level in the thematic depth botonera.
type DepthVariant struct {
	VariantID            string          `json:"variant_id"`
	Label                string          `json:"label"`
	MediaURL             string          `json:"media_url"`
	DurationSeconds      int32           `json:"duration_seconds"`
	FormatType           MediaFormatType `json:"format_type"`
	PreferenceDelta      []float32       `json:"preference_delta,omitempty"`
	CaptionsURL          string          `json:"captions_url,omitempty"`
	Transcript           string          `json:"transcript,omitempty"`
	AltText              string          `json:"alt_text,omitempty"`
	AudioDescriptionURL  string          `json:"audio_description_url,omitempty"`
}

// CognitiveVariant presents the same concept in a different learning modality.
type CognitiveVariant struct {
	VariantID           string          `json:"variant_id"`
	Label               string          `json:"label"`
	MediaURL            string          `json:"media_url,omitempty"`
	CellCode            string          `json:"cell_code,omitempty"`
	FormatType          MediaFormatType `json:"format_type"`
	PreferenceDelta     []float32       `json:"preference_delta,omitempty"`
	CaptionsURL         string          `json:"captions_url,omitempty"`
	Transcript          string          `json:"transcript,omitempty"`
	AltText             string          `json:"alt_text,omitempty"`
	AudioDescriptionURL string          `json:"audio_description_url,omitempty"`
}

// EmergencyVariant supports blockage diagnosis (hints never include full solution).
type EmergencyVariant struct {
	VariantID           string          `json:"variant_id"`
	Label               string          `json:"label"`
	MediaURL            string          `json:"media_url,omitempty"`
	HintText            string          `json:"hint_text,omitempty"`
	WalkthroughURL      string          `json:"walkthrough_url,omitempty"`
	FormatType          MediaFormatType `json:"format_type"`
	PreferenceDelta     []float32       `json:"preference_delta,omitempty"`
	CaptionsURL         string          `json:"captions_url,omitempty"`
	Transcript          string          `json:"transcript,omitempty"`
	AltText             string          `json:"alt_text,omitempty"`
	AudioDescriptionURL string          `json:"audio_description_url,omitempty"`
}

// CombinedCell is one cell in the depth × format matrix.
type CombinedCell struct {
	DepthID             string          `json:"depth_id"`
	FormatID            string          `json:"format_id"`
	MediaURL            string          `json:"media_url,omitempty"`
	CellCode            string          `json:"cell_code,omitempty"`
	DurationSeconds     int32           `json:"duration_seconds,omitempty"`
	FormatType          MediaFormatType `json:"format_type"`
	CaptionsURL         string          `json:"captions_url,omitempty"`
	Transcript          string          `json:"transcript,omitempty"`
	AltText             string          `json:"alt_text,omitempty"`
	AudioDescriptionURL string          `json:"audio_description_url,omitempty"`
}

// DUANodeBotonera is a reusable structural botonera attached to a node.
type DUANodeBotonera struct {
	Kind             BotoneraSchemaKind  `json:"kind"`
	TopicTitle       string              `json:"topic_title,omitempty"`
	FlatButtons      []InteractiveButton `json:"flat_buttons,omitempty"`
	DepthOptions     []DepthVariant      `json:"depth_options,omitempty"`
	CognitiveOptions []CognitiveVariant  `json:"cognitive_options,omitempty"`
	EmergencyOptions []EmergencyVariant  `json:"emergency_options,omitempty"`
	DepthAxis        []string            `json:"depth_axis,omitempty"`
	FormatAxis       []string            `json:"format_axis,omitempty"`
	MatrixCells      []CombinedCell      `json:"matrix_cells,omitempty"`
}

// Validate checks the active schema kind.
func (b *DUANodeBotonera) Validate() error {
	if b == nil {
		return fmt.Errorf("botonera_schema is nil")
	}
	switch b.Kind {
	case SchemaFlat:
		if len(b.FlatButtons) == 0 {
			return fmt.Errorf("flat schema requires flat_buttons")
		}
		for i, btn := range b.FlatButtons {
			if err := btn.Validate(); err != nil {
				return fmt.Errorf("flat_buttons[%d]: %w", i, err)
			}
		}
	case SchemaDepth:
		if len(b.DepthOptions) < 2 {
			return fmt.Errorf("depth schema requires at least 2 depth_options")
		}
		hasExpress, hasCore := false, false
		for i, d := range b.DepthOptions {
			if err := d.Validate(); err != nil {
				return fmt.Errorf("depth_options[%d]: %w", i, err)
			}
			switch d.VariantID {
			case "express_30s", "express":
				hasExpress = true
			case "core", "standard":
				hasCore = true
			}
		}
		if !hasExpress || !hasCore {
			return fmt.Errorf("depth schema must include express and core variants")
		}
	case SchemaCognitive:
		if len(b.CognitiveOptions) == 0 {
			return fmt.Errorf("cognitive schema requires cognitive_options")
		}
		for i, c := range b.CognitiveOptions {
			if err := c.Validate(); err != nil {
				return fmt.Errorf("cognitive_options[%d]: %w", i, err)
			}
		}
	case SchemaEmergency:
		if len(b.EmergencyOptions) == 0 {
			return fmt.Errorf("emergency schema requires emergency_options")
		}
		for i, e := range b.EmergencyOptions {
			if err := e.Validate(); err != nil {
				return fmt.Errorf("emergency_options[%d]: %w", i, err)
			}
		}
	case SchemaCombined:
		if len(b.DepthAxis) == 0 || len(b.FormatAxis) == 0 {
			return fmt.Errorf("combined schema requires depth_axis and format_axis")
		}
		if len(b.MatrixCells) == 0 {
			return fmt.Errorf("combined schema requires matrix_cells")
		}
		for i, cell := range b.MatrixCells {
			if err := cell.Validate(b.DepthAxis, b.FormatAxis); err != nil {
				return fmt.Errorf("matrix_cells[%d]: %w", i, err)
			}
		}
	default:
		return fmt.Errorf("unknown botonera kind: %s", b.Kind)
	}
	return nil
}

// Validate a depth variant.
func (d DepthVariant) Validate() error {
	if d.VariantID == "" || d.Label == "" {
		return fmt.Errorf("variant_id and label required")
	}
	if d.MediaURL == "" {
		return fmt.Errorf("media_url required")
	}
	if d.DurationSeconds <= 0 {
		return fmt.Errorf("duration_seconds must be > 0")
	}
	return nil
}

// Validate a cognitive variant.
func (c CognitiveVariant) Validate() error {
	if c.VariantID == "" || c.Label == "" {
		return fmt.Errorf("variant_id and label required")
	}
	if c.VariantID == "live_coding" {
		if c.MediaURL == "" && c.CellCode == "" {
			return fmt.Errorf("live_coding requires media_url or cell_code")
		}
		return nil
	}
	if c.MediaURL == "" && c.CellCode == "" {
		return fmt.Errorf("media_url or cell_code required")
	}
	return nil
}

// Validate an emergency variant.
func (e EmergencyVariant) Validate() error {
	if e.VariantID == "" || e.Label == "" {
		return fmt.Errorf("variant_id and label required")
	}
	switch e.VariantID {
	case "hint":
		if e.HintText == "" {
			return fmt.Errorf("hint requires hint_text")
		}
	case "common_errors":
		if e.MediaURL == "" {
			return fmt.Errorf("common_errors requires media_url")
		}
	case "test_cases":
		if e.MediaURL == "" && e.WalkthroughURL == "" {
			return fmt.Errorf("test_cases requires media_url or walkthrough_url")
		}
	}
	return nil
}

// Validate a combined matrix cell against declared axes.
func (c CombinedCell) Validate(depthAxis, formatAxis []string) error {
	if c.DepthID == "" || c.FormatID == "" {
		return fmt.Errorf("depth_id and format_id required")
	}
	if !contains(depthAxis, c.DepthID) {
		return fmt.Errorf("depth_id %q not in depth_axis", c.DepthID)
	}
	if !contains(formatAxis, c.FormatID) {
		return fmt.Errorf("format_id %q not in format_axis", c.FormatID)
	}
	if c.MediaURL == "" && c.CellCode == "" {
		return fmt.Errorf("media_url or cell_code required")
	}
	return nil
}

// ResolveCombined returns the cell for a depth×format selection.
func (b *DUANodeBotonera) ResolveCombined(depthID, formatID string) (CombinedCell, bool) {
	if b == nil {
		return CombinedCell{}, false
	}
	for _, c := range b.MatrixCells {
		if c.DepthID == depthID && c.FormatID == formatID {
			return c, true
		}
	}
	return CombinedCell{}, false
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

package dua

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AccessibilityGap locates one media surface missing a11y alternatives.
type AccessibilityGap struct {
	Location string   // e.g. stage_media_default, depth_options[express_30s]
	MediaURL string
	Missing  []string // transcript, captions_url, alt_text
}

// MediaAccessibilityReport summarizes media a11y gaps for one interactive node.
type MediaAccessibilityReport struct {
	NodeID string
	Gaps   []AccessibilityGap
}

// HasGaps reports whether any accessibility gaps were found.
func (r MediaAccessibilityReport) HasGaps() bool {
	return len(r.Gaps) > 0
}

// Summary is a compact log-friendly description.
func (r MediaAccessibilityReport) Summary() string {
	if !r.HasGaps() {
		return "ok"
	}
	parts := make([]string, 0, len(r.Gaps))
	for _, g := range r.Gaps {
		parts = append(parts, fmt.Sprintf("%s missing [%s]", g.Location, strings.Join(g.Missing, ", ")))
	}
	return strings.Join(parts, "; ")
}

// RequireMediaA11yFromEnv reads AVLP_REQUIRE_MEDIA_A11Y (default false).
func RequireMediaA11yFromEnv() bool {
	v := strings.TrimSpace(os.Getenv("AVLP_REQUIRE_MEDIA_A11Y"))
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

// AccessibilityReport walks stage media, botonera variants and hierarchy leaves
// that present video/audio and lists missing textual alternatives.
func AccessibilityReport(n *InteractiveVideoNode) MediaAccessibilityReport {
	out := MediaAccessibilityReport{}
	if n == nil {
		return out
	}
	out.NodeID = n.NodeID
	collectMediaGap(&out, "stage_media_default", n.StageMediaDefault, "", n.CaptionsURL, n.Transcript, n.AltText)
	if n.BotoneraSchema != nil {
		b := n.BotoneraSchema
		for _, d := range b.DepthOptions {
			collectMediaGap(&out, "depth_options["+d.VariantID+"]", d.MediaURL, d.FormatType, d.CaptionsURL, d.Transcript, d.AltText)
		}
		for _, c := range b.CognitiveOptions {
			collectMediaGap(&out, "cognitive_options["+c.VariantID+"]", c.MediaURL, c.FormatType, c.CaptionsURL, c.Transcript, c.AltText)
		}
		for _, e := range b.EmergencyOptions {
			url := e.MediaURL
			if url == "" {
				url = e.WalkthroughURL
			}
			collectMediaGap(&out, "emergency_options["+e.VariantID+"]", url, e.FormatType, e.CaptionsURL, e.Transcript, e.AltText)
		}
		for _, cell := range b.MatrixCells {
			loc := fmt.Sprintf("matrix_cells[%s|%s]", cell.DepthID, cell.FormatID)
			collectMediaGap(&out, loc, cell.MediaURL, cell.FormatType, cell.CaptionsURL, cell.Transcript, cell.AltText)
		}
	}
	if n.Hierarchy != nil {
		walkSubtopicGaps(&out, n.Hierarchy.Subtopics, "hierarchy")
	}
	return out
}

func walkSubtopicGaps(out *MediaAccessibilityReport, nodes []SubtopicNode, prefix string) {
	for i := range nodes {
		s := &nodes[i]
		loc := fmt.Sprintf("%s.subtopics[%s]", prefix, s.SubtopicID)
		collectMediaGap(out, loc, s.MediaURL, MediaVideo, s.CaptionsURL, s.Transcript, s.AltText)
		if len(s.ChildSubtopics) > 0 {
			walkSubtopicGaps(out, s.ChildSubtopics, loc)
		}
	}
}

func collectMediaGap(out *MediaAccessibilityReport, location, mediaURL string, format MediaFormatType, captions, transcript, alt string) {
	if !needsAVAccessibility(mediaURL, format) {
		return
	}
	var missing []string
	if format == MediaDiagram {
		if strings.TrimSpace(alt) == "" {
			missing = append(missing, "alt_text")
		}
	} else {
		if strings.TrimSpace(transcript) == "" && strings.TrimSpace(captions) == "" {
			missing = append(missing, "transcript|captions_url")
		}
		if strings.TrimSpace(alt) == "" {
			missing = append(missing, "alt_text")
		}
	}
	if len(missing) == 0 {
		return
	}
	out.Gaps = append(out.Gaps, AccessibilityGap{
		Location: location,
		MediaURL: mediaURL,
		Missing:  missing,
	})
}

func needsAVAccessibility(mediaURL string, format MediaFormatType) bool {
	switch format {
	case MediaVideo, MediaAudioDebate:
		return strings.TrimSpace(mediaURL) != ""
	case MediaTextHint, MediaLiveCode, MediaInteractiveCanvas:
		return false
	case MediaDiagram:
		// Diagrams need alt_text when they are the primary media surface.
		return strings.TrimSpace(mediaURL) != ""
	}
	u := strings.ToLower(strings.TrimSpace(mediaURL))
	if u == "" {
		return false
	}
	if strings.Contains(u, "/videos/") || strings.Contains(u, "/audio/") {
		return true
	}
	for _, ext := range []string{".mp4", ".webm", ".ogg", ".mp3", ".m4a", ".wav"} {
		if strings.Contains(u, ext) {
			return true
		}
	}
	return false
}

// EnforceAccessibility returns an error when the env flag requires a11y and gaps exist.
func EnforceAccessibility(n *InteractiveVideoNode) error {
	if !RequireMediaA11yFromEnv() {
		return nil
	}
	rep := AccessibilityReport(n)
	if !rep.HasGaps() {
		return nil
	}
	return fmt.Errorf("media accessibility required: %s", rep.Summary())
}

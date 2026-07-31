package dua_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestValidateAllowsMissingA11yFields(t *testing.T) {
	n := &dua.InteractiveVideoNode{
		NodeID:            "dua::Representacion::basico::visual::01TESTA11Y000000000000000",
		DimensionDUA:      "Representacion",
		Titulo:            "Sin a11y",
		LayoutType:        dua.LayoutInteractiveDashboard,
		StageMediaDefault: "https://cdn.example.com/videos/x.mp4",
		Botonera: []dua.InteractiveButton{{
			IDBtn:      "ask_different",
			Label:      "+ duda",
			ActionType: dua.ActionAskAgent,
		}},
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("Validate must not require a11y fields: %v", err)
	}
	if !dua.AccessibilityReport(n).HasGaps() {
		t.Fatal("expected gaps for video without transcript/alt")
	}
}

func TestAccessibilityReportFindsVideoGaps(t *testing.T) {
	n := &dua.InteractiveVideoNode{
		NodeID:            "n1",
		StageMediaDefault: "https://cdn.example.com/videos/root.mp4",
		BotoneraSchema: &dua.DUANodeBotonera{
			Kind: dua.SchemaDepth,
			DepthOptions: []dua.DepthVariant{{
				VariantID:       "express_30s",
				Label:           "Express",
				MediaURL:        "https://cdn.example.com/videos/express.mp4",
				DurationSeconds: 30,
				FormatType:      dua.MediaVideo,
			}},
		},
	}
	rep := dua.AccessibilityReport(n)
	if len(rep.Gaps) < 2 {
		t.Fatalf("expected stage + depth gaps, got %+v", rep.Gaps)
	}
	joined := rep.Summary()
	if !strings.Contains(joined, "stage_media_default") || !strings.Contains(joined, "depth_options[express_30s]") {
		t.Fatalf("summary missing locations: %s", joined)
	}
}

func TestAccessibilityReportClearWhenComplete(t *testing.T) {
	n := &dua.InteractiveVideoNode{
		NodeID:            "n2",
		StageMediaDefault: "https://cdn.example.com/videos/root.mp4",
		AltText:           "Root clip",
		Transcript:        "Texto del root",
		BotoneraSchema: &dua.DUANodeBotonera{
			Kind: dua.SchemaCognitive,
			CognitiveOptions: []dua.CognitiveVariant{{
				VariantID:  "animation",
				Label:      "Anim",
				MediaURL:   "https://cdn.example.com/videos/a.mp4",
				FormatType: dua.MediaVideo,
				AltText:    "Animación",
				Transcript: "Habla la animación",
			}},
		},
	}
	if rep := dua.AccessibilityReport(n); rep.HasGaps() {
		t.Fatalf("unexpected gaps: %s", rep.Summary())
	}
}

func TestToProtoIncludesA11yFields(t *testing.T) {
	n := &dua.InteractiveVideoNode{
		NodeID:            "n3",
		StageMediaDefault: "https://cdn.example.com/videos/root.mp4",
		AltText:           "alt root",
		Transcript:        "transcript root",
		CaptionsURL:       "https://cdn.example.com/videos/root.vtt",
	}
	pb := dua.ToProto(n)
	if pb.GetAltText() != "alt root" || pb.GetTranscript() != "transcript root" || pb.GetCaptionsUrl() == "" {
		t.Fatalf("proto missing a11y: %+v", pb)
	}
}

func TestRequireMediaA11yRejectsOnPut(t *testing.T) {
	t.Setenv("AVLP_REQUIRE_MEDIA_A11Y", "true")
	reg := dua.NewRegistry()
	n := &dua.InteractiveVideoNode{
		NodeID:            "dua::Representacion::basico::visual::01TESTA11YREQ00000000000",
		DimensionDUA:      "Representacion",
		Titulo:            "Estricto",
		LayoutType:        dua.LayoutInteractiveDashboard,
		StageMediaDefault: "https://cdn.example.com/videos/x.mp4",
		Botonera: []dua.InteractiveButton{{
			IDBtn:      "ask_different",
			Label:      "+ duda",
			ActionType: dua.ActionAskAgent,
		}},
	}
	err := reg.Put(n)
	if err == nil || !strings.Contains(err.Error(), "media accessibility required") {
		t.Fatalf("expected a11y error, got %v", err)
	}
}

func TestLoadDirLogsA11yGaps(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "node_id": "dua::Representacion::basico::visual::01TESTA11YLOG00000000000",
  "dimension_dua": "Representacion",
  "titulo": "Log gaps",
  "layout_type": "interactive_dashboard",
  "stage_media_default": "https://cdn.example.com/videos/log.mp4",
  "embedding_descriptor": "nodo de prueba a11y",
  "botonera": [{"id_btn":"ask_different","label":"+","action_type":"ask_agent"}]
}`
	if err := os.WriteFile(filepath.Join(dir, "gap.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	var logs []string
	reg := dua.NewRegistry()
	reg.Logf = func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	if _, err := reg.LoadDir(dir); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range logs {
		if strings.Contains(line, "media a11y gaps") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a11y log, got %#v", logs)
	}
}

func TestCuratedSeedsHaveNoA11yGaps(t *testing.T) {
	dir := filepath.Join("..", "..", "data", "nodes", "interactive")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "promoted-") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var n dua.InteractiveVideoNode
		if err := json.Unmarshal(raw, &n); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if rep := dua.AccessibilityReport(&n); rep.HasGaps() {
			t.Fatalf("%s still has a11y gaps: %s", e.Name(), rep.Summary())
		}
	}
}

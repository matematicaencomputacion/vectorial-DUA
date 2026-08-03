package knowledge_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

func writeTempCurriculum(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "curriculum.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write temp curriculum: %v", err)
	}
	return path
}

func TestLoadFileValidation(t *testing.T) {
	validHeader := `{"version":1,"concepts":[
		{"id":"concept:a","title":"A","summary":"","track":"python"},
		{"id":"concept:b","title":"B","summary":"","track":"ingles"}
	],`

	cases := []struct {
		name    string
		body    string
		wantSub string
	}{
		{
			name:    "bad version",
			body:    `{"version":2,"concepts":[],"edges":[]}`,
			wantSub: "unsupported version",
		},
		{
			name: "invalid slug",
			body: `{"version":1,"concepts":[{"id":"concept:Bad_Slug","title":"X","track":"python"}],"edges":[]}`,
			wantSub: "invalid concept slug",
		},
		{
			name: "duplicate concept",
			body: `{"version":1,"concepts":[
				{"id":"concept:a","title":"A","track":"python"},
				{"id":"concept:a","title":"A2","track":"python"}
			],"edges":[]}`,
			wantSub: "duplicate concept",
		},
		{
			name: "unknown endpoint",
			body: validHeader + `"edges":[{"from":"concept:a","to":"concept:missing","kind":"requires","strength":0.5,"rationale_es":"x"}]}`,
			wantSub: "unknown to",
		},
		{
			name: "self edge",
			body: validHeader + `"edges":[{"from":"concept:a","to":"concept:a","kind":"requires","strength":0.5,"rationale_es":"x"}]}`,
			wantSub: "self-edge",
		},
		{
			name: "duplicate edge",
			body: validHeader + `"edges":[
				{"from":"concept:b","to":"concept:a","kind":"requires","strength":0.5,"rationale_es":"uno"},
				{"from":"concept:b","to":"concept:a","kind":"requires","strength":0.6,"rationale_es":"dos"}
			]}`,
			wantSub: "duplicate edge",
		},
		{
			name: "requires cycle with printed path",
			body: validHeader + `"edges":[
				{"from":"concept:a","to":"concept:b","kind":"requires","strength":0.5,"rationale_es":"ab"},
				{"from":"concept:b","to":"concept:a","kind":"requires","strength":0.5,"rationale_es":"ba"}
			]}`,
			wantSub: "cycle in requires",
		},
		{
			name: "strength out of range",
			body: validHeader + `"edges":[{"from":"concept:b","to":"concept:a","kind":"requires","strength":0,"rationale_es":"x"}]}`,
			wantSub: "strength",
		},
		{
			name: "empty rationale on requires",
			body: validHeader + `"edges":[{"from":"concept:b","to":"concept:a","kind":"requires","strength":0.5,"rationale_es":""}]}`,
			wantSub: "rationale_es",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempCurriculum(t, tc.body)
			_, _, err := knowledge.LoadFile(path, knowledge.LoadOptions{})
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
			if tc.name == "requires cycle with printed path" {
				if !strings.Contains(err.Error(), "concept:") {
					t.Fatalf("cycle error should print concept ids: %v", err)
				}
			}
		})
	}
}

func TestLoadFileAllowsAlternativeCycleLikePair(t *testing.T) {
	body := `{"version":1,"concepts":[
		{"id":"concept:a","title":"A","track":"python"},
		{"id":"concept:b","title":"B","track":"ingles"}
	],"edges":[
		{"from":"concept:a","to":"concept:b","kind":"alternative","strength":0.4,"rationale_es":"alt"}
	]}`
	path := writeTempCurriculum(t, body)
	g, rep, err := knowledge.LoadFile(path, knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(g.Edges()) != 1 {
		t.Fatalf("edges=%d", len(g.Edges()))
	}
	_ = rep
}

func TestLoadFileWarnsOrphanConcept(t *testing.T) {
	body := `{"version":1,"concepts":[
		{"id":"concept:a","title":"A","track":"python"},
		{"id":"concept:lonely","title":"Lonely","track":"plataforma"}
	],"edges":[
		{"from":"concept:a","to":"concept:lonely","kind":"requires","strength":0.5,"rationale_es":"x"}
	]}`
	// Wait - that gives lonely an edge. Need truly orphan:
	body = `{"version":1,"concepts":[
		{"id":"concept:a","title":"A","track":"python"},
		{"id":"concept:lonely","title":"Lonely","track":"plataforma"}
	],"edges":[]}`
	path := writeTempCurriculum(t, body)
	var logs []string
	_, rep, err := knowledge.LoadFile(path, knowledge.LoadOptions{
		Logf: func(format string, args ...any) {
			logs = append(logs, strings.TrimSpace(format))
		},
	})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	found := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "concept without edges: concept:lonely") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected orphan warning, got %#v logs=%#v", rep.Warnings, logs)
	}
}

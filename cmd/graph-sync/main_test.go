package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDryRunValidatesCurriculumWithoutNeo4j(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	curriculum := filepath.Join(repo, "data", "knowledge", "curriculum.json")
	cmd := exec.Command("go", "run", "./cmd/graph-sync",
		"-curriculum", curriculum,
		"-dry-run",
		"-validate-seeds",
		"-seeds", filepath.Join(repo, "data", "nodes", "interactive"),
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "AVLP_NEO4J_URI=") // ensure no accidental write
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("graph-sync dry-run: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "dry-run: no se escribió nada") {
		t.Fatalf("expected dry-run message, got:\n%s", s)
	}
	if !strings.Contains(s, "plan:") {
		t.Fatalf("expected plan summary, got:\n%s", s)
	}
}

func TestInvalidCurriculumNeverTouchesNeo4j(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"version":1,"concepts":[{"id":"concept:a","title":"A","track":"python"}],"edges":[{"from":"concept:a","to":"concept:a","kind":"requires","strength":1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/graph-sync", "-curriculum", bad, "-dry-run")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure on cycle, got:\n%s", out)
	}
	if !strings.Contains(string(out), "abortando antes de tocar Neo4j") && !strings.Contains(string(out), "cycle") && !strings.Contains(string(out), "ciclo") {
		// LoadFile reports cycle in Spanish/English — accept any abort
		if !strings.Contains(string(out), "curriculum inválido") {
			t.Fatalf("expected curriculum abort, got:\n%s", out)
		}
	}
}

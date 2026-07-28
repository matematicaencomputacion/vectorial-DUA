package dua_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestFileProfileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")

	store, err := dua.NewFileProfileStoreWithDebounce(path, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	delta := []float32{0.1, 0.0, -0.1, 0.2, 0.05}
	want, err := store.Apply("stu-1", delta)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := dua.NewFileProfileStoreWithDebounce(path, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	got := reloaded.Get("stu-1")
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%v want=%v full=%v", i, got[i], want[i], got)
		}
	}
}

func TestFileProfileStoreCorruptDoesNotBreakStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := dua.NewFileProfileStoreWithDebounce(path, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got := store.Get("stu-x")
	def := dua.DefaultVe()
	for i := range def {
		if got[i] != def[i] {
			t.Fatalf("expected default after corrupt load, got %v", got)
		}
	}
}

func TestFileProfileStoreVeDimsMismatchDiscards(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	doc := map[string]any{
		"version":  1,
		"ve_dims":  3,
		"profiles": map[string][]float32{"stu-1": {0.9, 0.9, 0.9}},
	}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := dua.NewFileProfileStoreWithDebounce(path, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got := store.Get("stu-1")
	if got[0] == 0.9 {
		t.Fatal("expected ve_dims mismatch to discard snapshot")
	}
}

func TestFileProfileStoreDiscardsBadProfileDims(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	doc := map[string]any{
		"version": 1,
		"ve_dims": dua.VeDims,
		"profiles": map[string][]float32{
			"good": {0.7, 0.6, 0.5, 0.4, 0.3},
			"bad":  {0.1, 0.2},
		},
	}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := dua.NewFileProfileStoreWithDebounce(path, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if store.Get("good")[0] != 0.7 {
		t.Fatal("expected good profile loaded")
	}
	if store.Get("bad")[0] != dua.DefaultVe()[0] {
		t.Fatal("expected bad profile discarded")
	}
}

func TestFileProfileStoreAtomicNoOrphanTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	store, err := dua.NewFileProfileStoreWithDebounce(path, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply("stu-1", []float32{0.1, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || len(e.Name()) > len("profiles.json") && filepath.Ext(e.Name()) != ".json" {
			// CreateTemp uses name.*.tmp — ensure none remain
			name := e.Name()
			if len(name) >= 4 && name[len(name)-4:] == ".tmp" {
				t.Fatalf("orphan temp left behind: %s", name)
			}
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
}

func TestFileProfileStoreDebounceFewerWritesThanApplies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	store, err := dua.NewFileProfileStoreWithDebounce(path, 80*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	const n = 20
	for i := 0; i < n; i++ {
		if _, err := store.Apply("stu-1", []float32{0.01, 0, 0, 0, 0}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(200 * time.Millisecond)
	writes := store.WriteCount()
	if writes == 0 {
		t.Fatal("expected at least one flush")
	}
	if writes >= int64(n) {
		t.Fatalf("debounce failed: writes=%d applies=%d", writes, n)
	}
}

func TestFileProfileStoreFlushFailureKeepsDirtyAndRetries(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "profiles.json")
	// Parent path component is a file → MkdirAll fails for nested snapshot.
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(blocked, "profiles.json")

	store, err := dua.NewFileProfileStoreWithDebounce(goodPath, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	want, err := store.Apply("stu-1", []float32{0.15, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}

	store.Relocate(badPath)
	if err := store.Flush(); err == nil {
		t.Fatal("expected flush failure on invalid path")
	}
	if store.WriteCount() != 0 {
		t.Fatalf("expected no durable write after failure, got %d", store.WriteCount())
	}

	store.Relocate(goodPath)
	if err := store.Flush(); err != nil {
		t.Fatalf("retry flush: %v", err)
	}
	if store.WriteCount() != 1 {
		t.Fatalf("writes=%d want 1", store.WriteCount())
	}

	reloaded, err := dua.NewFileProfileStoreWithDebounce(goodPath, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	got := reloaded.Get("stu-1")
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("persisted mismatch at %d: got=%v want=%v", i, got[i], want[i])
		}
	}
}

func TestFileProfileStoreConcurrentAppliesRace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	store, err := dua.NewFileProfileStoreWithDebounce(path, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("stu-%d", i%8)
			_, _ = store.Apply(id, []float32{0.01, 0, 0, 0, 0})
			_ = store.Get(id)
		}(i)
	}
	wg.Wait()
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
}

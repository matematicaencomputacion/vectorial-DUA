package dua_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestApplyDeltaRejectsInvalidDeltaDims(t *testing.T) {
	_, err := dua.ApplyDelta(dua.DefaultVe(), []float32{0.1, 0.2})
	if err == nil {
		t.Fatal("expected error for invalid delta dims")
	}
}

func TestApplyDeltaClampsRange(t *testing.T) {
	base := []float32{0.95, 0.05, 0.4, 0.5, 0.5}
	delta := []float32{0.2, -0.2, 0.8, -0.9, 0.1}

	got, err := dua.ApplyDelta(base, delta)
	if err != nil {
		t.Fatal(err)
	}

	want := []float32{1.0, 0.0, 1.0, 0.0, 0.6}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%v want=%v (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestApplyDeltaUsesDefaultWhenBaseTooShort(t *testing.T) {
	got, err := dua.ApplyDelta([]float32{0.1, 0.2}, []float32{0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	want := dua.DefaultVe()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%v want=%v", i, got[i], want[i])
		}
	}
}

func TestProfileStoreGetDefaultAndDefensiveCopy(t *testing.T) {
	store := dua.NewProfileStore()
	got := store.Get("stu-1")
	if len(got) != dua.VeDims {
		t.Fatalf("dims=%d want=%d", len(got), dua.VeDims)
	}
	got[0] = 0

	again := store.Get("stu-1")
	if again[0] != dua.DefaultVe()[0] {
		t.Fatal("expected defensive copy on Get")
	}
}

func TestProfileStoreApplyPersistsAndRejectsInvalidDelta(t *testing.T) {
	store := dua.NewProfileStore()
	if _, err := store.Apply("stu-1", []float32{0.1}); err == nil {
		t.Fatal("expected invalid delta error")
	}

	next, err := store.Apply("stu-1", []float32{0.1, 0.0, -0.1, 0.2, 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if next[0] <= dua.DefaultVe()[0] {
		t.Fatalf("expected dominio increase, got %v", next[0])
	}

	cur := store.Get("stu-1")
	for i := range next {
		if cur[i] != next[i] {
			t.Fatalf("store mismatch at %d: got=%v want=%v", i, cur[i], next[i])
		}
	}
}

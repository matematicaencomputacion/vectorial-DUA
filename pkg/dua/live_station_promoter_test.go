package dua_test

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestLiveStationPromoterCreatesAndReplaysCuratedSeed(t *testing.T) {
	promoter, tracking, liveNode := readyPromoter(t)

	first, err := promoter.Promote(tracking)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.Node.NodeID != liveNode.ID {
		t.Fatalf("first promotion=%+v", first)
	}
	if _, err := os.Stat(first.SeedPath); err != nil {
		t.Fatalf("seed missing: %v", err)
	}
	stored, ok := promoter.Registry.Get(liveNode.ID)
	if !ok || stored.StageMarkdownDefault == "" ||
		stored.PromotedFromTrackingULID != tracking ||
		len(stored.RetrievedSources) != 1 {
		t.Fatalf("stored node=%+v ok=%v", stored, ok)
	}
	indexed := indexedNodeByID(t, promoter.Index, liveNode.ID)
	if indexed.IsLiveGenerated || indexed.ResourceURL != "interactive://"+liveNode.ID {
		t.Fatalf("indexed node not curated: %+v", indexed)
	}
	reloaded := dua.NewRegistry()
	if _, err := reloaded.LoadDir(promoter.SeedsDir); err != nil {
		t.Fatalf("reload promoted seed: %v", err)
	}
	reloadedNode, ok := reloaded.Get(liveNode.ID)
	if !ok || reloadedNode.StageMarkdownDefault != stored.StageMarkdownDefault {
		t.Fatalf("reloaded node=%+v ok=%v", reloadedNode, ok)
	}

	promoter.Ledger = nil
	replay, err := promoter.Promote(tracking)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Created || replay.Node.NodeID != first.Node.NodeID || replay.SeedPath != first.SeedPath {
		t.Fatalf("replay=%+v first=%+v", replay, first)
	}
}

func TestLiveStationPromoterRejectsMissingAndNotReady(t *testing.T) {
	idx := vector.NewIndex()
	promoter := &dua.LiveStationPromoter{
		Ledger:   vector.NewStationLedger(0),
		Index:    idx,
		Registry: dua.NewRegistry(),
		SeedsDir: t.TempDir(),
	}
	missing, err := vector.NewTrackingULID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := promoter.Promote(missing); err != dua.ErrStationNotFound {
		t.Fatalf("missing err=%v", err)
	}

	pending, err := vector.NewTrackingULID()
	if err != nil {
		t.Fatal(err)
	}
	promoter.Ledger.RegisterInProgress(pending, "stu", vector.LiveRequest{DoubtText: "pendiente"})
	if _, err := promoter.Promote(pending); err != dua.ErrStationNotReady {
		t.Fatalf("pending err=%v", err)
	}
	if _, err := promoter.Promote("../escape"); err != dua.ErrInvalidTrackingULID {
		t.Fatalf("invalid err=%v", err)
	}
}

func TestLiveStationPromoterConcurrentReplayWritesOnce(t *testing.T) {
	promoter, tracking, _ := readyPromoter(t)
	const workers = 12
	var created atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			res, err := promoter.Promote(tracking)
			if err != nil {
				errs <- err
				return
			}
			if res.Created {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := created.Load(); got != 1 {
		t.Fatalf("created count=%d want=1", got)
	}
}

func readyPromoter(t *testing.T) (*dua.LiveStationPromoter, string, vector.Node) {
	t.Helper()
	idx := vector.NewIndex()
	embedding := make([]float32, idx.Dims())
	embedding[0] = 1
	tracking, err := vector.NewTrackingULID()
	if err != nil {
		t.Fatal(err)
	}
	liveNode, err := idx.RegisterLiveNode(
		"Representacion",
		"adaptativo",
		"visual",
		"live://stations/"+tracking,
		embedding,
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger := vector.NewStationLedger(0)
	ledger.RegisterInProgress(tracking, "stu", vector.LiveRequest{
		DoubtText:      "¿Cómo entiendo el scope de una variable?",
		QueryEmbedding: embedding,
		Dimension:      "Representacion",
		Format:         "visual",
	})
	ledger.MarkReady(tracking, vector.LiveResult{
		Node:         liveNode,
		Content:      "# Scope\n\nContenido revisado.",
		Sources:      []string{"knowledge/scope.md"},
		TrackingULID: tracking,
	})
	return &dua.LiveStationPromoter{
		Ledger:   ledger,
		Index:    idx,
		Registry: dua.NewRegistry(),
		SeedsDir: t.TempDir(),
	}, tracking, liveNode
}

func indexedNodeByID(t *testing.T, idx *vector.Index, id string) vector.Node {
	t.Helper()
	for _, node := range idx.Nodes() {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("indexed node %q not found", id)
	return vector.Node{}
}

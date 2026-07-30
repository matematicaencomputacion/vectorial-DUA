package vector

import (
	"sync"
	"testing"
	"time"
)

func TestLiveNodeTTLExpiresOnNearestAndReleasesULID(t *testing.T) {
	t.Setenv("AVLP_LIVE_NODE_TTL", "1h")
	idx := NewIndex()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	idx.now = func() time.Time { return now }
	idx.ttl = time.Hour

	embedding := unitEmbedding(idx.Dims())
	curated, err := idx.RegisterNode("Representacion", "basico", "visual", "master://curated", embedding)
	if err != nil {
		t.Fatal(err)
	}
	live, err := idx.RegisterLiveNode("Representacion", "adaptativo", "visual", "live://station", embedding)
	if err != nil {
		t.Fatal(err)
	}
	liveParts, err := ParseNodeID(live.ID)
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Hour + time.Nanosecond)
	got := idx.Nearest(embedding)
	if !got.Found || got.Node.ID != curated.ID {
		t.Fatalf("nearest after purge=%+v want curated %s", got, curated.ID)
	}
	if idx.Len() != 1 {
		t.Fatalf("Len=%d want=1", idx.Len())
	}
	if idx.HasULID(liveParts.ULID) {
		t.Fatalf("expired live ULID %s remains in ring", liveParts.ULID)
	}
}

func TestCuratedNodeSurvivesLiveTTL(t *testing.T) {
	idx := NewIndex()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	idx.now = func() time.Time { return now }
	idx.ttl = time.Nanosecond

	embedding := unitEmbedding(idx.Dims())
	node, err := idx.RegisterNode("Accion", "basico", "practica", "ide://curated", embedding)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour)

	got := idx.Nearest(embedding)
	if !got.Found || got.Node.ID != node.ID || idx.Len() != 1 {
		t.Fatalf("curated node expired: match=%+v len=%d", got, idx.Len())
	}
}

func TestLiveNodeTTLFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "default", value: "", want: 24 * time.Hour},
		{name: "valid", value: "90m", want: 90 * time.Minute},
		{name: "invalid", value: "tomorrow", want: 24 * time.Hour},
		{name: "non-positive", value: "0s", want: 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AVLP_LIVE_NODE_TTL", tt.value)
			if got := LiveNodeTTLFromEnv(); got != tt.want {
				t.Fatalf("LiveNodeTTLFromEnv()=%s want=%s", got, tt.want)
			}
		})
	}
}

func TestLiveNodeTTLPurgeConcurrentWithRegistration(t *testing.T) {
	idx := NewIndex()
	idx.ttl = time.Nanosecond
	embedding := unitEmbedding(idx.Dims())

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, _ = idx.RegisterLiveNode("Representacion", "adaptativo", "visual", "live://station", embedding)
			_ = idx.Nearest(embedding)
			_ = idx.Nodes()
		}()
	}
	wg.Wait()
}

func unitEmbedding(dims int) []float32 {
	out := make([]float32, dims)
	out[0] = 1
	return out
}

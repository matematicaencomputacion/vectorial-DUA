package neo4jgraph

import (
	"testing"
	"time"
)

func TestTTLCacheExpiry(t *testing.T) {
	c := newTTLCache(50 * time.Millisecond)
	now := time.Unix(100, 0)
	c.now = func() time.Time { return now }
	c.set("k", "v")
	if v, ok := c.get("k"); !ok || v != "v" {
		t.Fatalf("get=%v %v", v, ok)
	}
	now = now.Add(100 * time.Millisecond)
	if _, ok := c.get("k"); ok {
		t.Fatal("expected expiry")
	}
}

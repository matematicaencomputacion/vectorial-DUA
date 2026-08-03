package neo4jgraph

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestBreakerTripsOncePerWindow(t *testing.T) {
	var logs atomic.Int32
	b := newBreaker(3, 100*time.Millisecond, func(string, ...any) { logs.Add(1) })
	now := time.Unix(0, 0)
	b.now = func() time.Time { return now }

	if !b.allow() {
		t.Fatal("should allow initially")
	}
	b.failure(errTest)
	b.failure(errTest)
	if logs.Load() != 0 {
		t.Fatal("should not log before limit")
	}
	b.failure(errTest)
	if logs.Load() != 1 {
		t.Fatalf("log=%d", logs.Load())
	}
	if b.allow() {
		t.Fatal("should be open")
	}
	// more failures while open must not spam logs
	b.failure(errTest)
	b.failure(errTest)
	if logs.Load() != 1 {
		t.Fatalf("expected single log, got %d", logs.Load())
	}
	now = now.Add(200 * time.Millisecond)
	if !b.allow() {
		t.Fatal("should allow after cooldown")
	}
	b.success()
	if !b.allow() {
		t.Fatal("closed after success")
	}
}

var errTest = errString("bolt down")

type errString string

func (e errString) Error() string { return string(e) }

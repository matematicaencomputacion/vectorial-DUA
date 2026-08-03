package neo4jgraph

import (
	"sync"
	"time"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

// breaker trips open after consecutive Failures until Cooldown elapses.
type breaker struct {
	limit    int
	cooldown time.Duration
	logf     knowledge.Logf
	now      func() time.Time

	mu            sync.Mutex
	failures      int
	openUntil     time.Time
	loggedOpenFor time.Time // openUntil value we already logged for
}

func newBreaker(limit int, cooldown time.Duration, logf knowledge.Logf) *breaker {
	if limit <= 0 {
		limit = breakerLimit
	}
	if cooldown <= 0 {
		cooldown = defaultCooldown
	}
	return &breaker{
		limit:    limit,
		cooldown: cooldown,
		logf:     logf,
		now:      time.Now,
	}
}

func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if now.Before(b.openUntil) {
		return false
	}
	return true
}

func (b *breaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openUntil = time.Time{}
	b.loggedOpenFor = time.Time{}
}

func (b *breaker) failure(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures < b.limit {
		return
	}
	until := b.now().Add(b.cooldown)
	if !until.Equal(b.openUntil) {
		b.openUntil = until
	}
	// Log once per open window (not per request while open).
	if b.loggedOpenFor.Equal(b.openUntil) {
		return
	}
	b.loggedOpenFor = b.openUntil
	if b.logf != nil {
		b.logf("neo4jgraph: breaker open for %s after %d failures: %v", b.cooldown, b.failures, err)
	}
}

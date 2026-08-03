// Package neo4jgraph is a read-only Neo4j backend for knowledge.KnowledgeGraph.
//
// When AVLP_NEO4J_URI is empty, NewFromEnv returns (nil, nil) — same optional
// pattern as the HTTP LLM / STT / embedder backends. Routing never depends on
// this package; orientation may fall back to the file MemoryGraph.
package neo4jgraph

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

const (
	defaultCooldown = 30 * time.Second
	defaultCacheTTL = 5 * time.Minute
	maxTxRetry      = 2 * time.Second
	breakerLimit    = 3
)

// Config configures a read-only Neo4j curriculum client.
type Config struct {
	URI      string
	User     string
	Password string
	// Cooldown is how long the breaker stays open after breakerLimit failures.
	Cooldown time.Duration
	// CacheTTL caches successful graph lookups (Concept / traversal). Zero → default 5m.
	CacheTTL time.Duration
	// MaxTransactionRetryTime overrides the driver default (30s) — keep short.
	MaxTransactionRetryTime time.Duration
	Logf                    knowledge.Logf
}

// ConfigFromEnv reads AVLP_NEO4J_* and AVLP_KNOWLEDGE_CACHE_TTL.
func ConfigFromEnv() Config {
	return Config{
		URI:                     strings.TrimSpace(os.Getenv("AVLP_NEO4J_URI")),
		User:                    strings.TrimSpace(os.Getenv("AVLP_NEO4J_USER")),
		Password:                os.Getenv("AVLP_NEO4J_PASSWORD"),
		Cooldown:               durationFromEnv("AVLP_NEO4J_COOLDOWN", defaultCooldown),
		CacheTTL:                durationFromEnv("AVLP_KNOWLEDGE_CACHE_TTL", defaultCacheTTL),
		MaxTransactionRetryTime: maxTxRetry,
	}
}

func durationFromEnv(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return def
}

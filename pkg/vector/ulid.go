package vector

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var nodeIDPattern = regexp.MustCompile(`^dua::[^:]+::[^:]+::[^:]+::[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

// NodeIDParts are the hierarchical components of a DUA node key.
type NodeIDParts struct {
	Dimension  string
	Difficulty string
	Format     string
	ULID       string
}

// NewNodeID builds dua::<dimension>::<dificultad>::<formato>::<ulid>.
func NewNodeID(dimension, difficulty, format string) (string, error) {
	parts := NodeIDParts{
		Dimension:  strings.TrimSpace(dimension),
		Difficulty: strings.TrimSpace(difficulty),
		Format:     strings.TrimSpace(format),
	}
	if parts.Dimension == "" || parts.Difficulty == "" || parts.Format == "" {
		return "", fmt.Errorf("dimension, difficulty and format are required")
	}
	if strings.Contains(parts.Dimension, ":") || strings.Contains(parts.Difficulty, ":") || strings.Contains(parts.Format, ":") {
		return "", fmt.Errorf("node id parts must not contain ':'")
	}

	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), entropy)
	if err != nil {
		return "", fmt.Errorf("generate ulid: %w", err)
	}
	parts.ULID = id.String()
	return FormatNodeID(parts), nil
}

// FormatNodeID joins hierarchical parts into the canonical key.
func FormatNodeID(p NodeIDParts) string {
	return fmt.Sprintf("dua::%s::%s::%s::%s", p.Dimension, p.Difficulty, p.Format, p.ULID)
}

// ParseNodeID validates and splits a DUA node identifier.
func ParseNodeID(nodeID string) (NodeIDParts, error) {
	if !nodeIDPattern.MatchString(nodeID) {
		return NodeIDParts{}, fmt.Errorf("invalid node id format: %q", nodeID)
	}
	parts := strings.Split(nodeID, "::")
	if len(parts) != 5 || parts[0] != "dua" {
		return NodeIDParts{}, fmt.Errorf("invalid node id structure: %q", nodeID)
	}
	if _, err := ulid.Parse(parts[4]); err != nil {
		return NodeIDParts{}, fmt.Errorf("invalid ulid segment: %w", err)
	}
	return NodeIDParts{
		Dimension:  parts[1],
		Difficulty: parts[2],
		Format:     parts[3],
		ULID:       parts[4],
	}, nil
}

// ValidateNodeID reports whether nodeID matches the hierarchical ULID schema.
func ValidateNodeID(nodeID string) bool {
	_, err := ParseNodeID(nodeID)
	return err == nil
}

// ULIDTime extracts the embedded timestamp from a node id ULID segment.
func ULIDTime(nodeID string) (time.Time, error) {
	parts, err := ParseNodeID(nodeID)
	if err != nil {
		return time.Time{}, err
	}
	id, err := ulid.Parse(parts.ULID)
	if err != nil {
		return time.Time{}, err
	}
	return ulid.Time(id.Time()), nil
}

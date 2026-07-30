package vector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// DefaultConfigPath is the runtime config written by harness calibrate --apply.
	DefaultConfigPath = "data/avlp.json"

	thresholdConfigVersion = 1
)

// ThresholdSource identifies where the effective routing threshold came from.
type ThresholdSource string

const (
	ThresholdSourceEnv     ThresholdSource = "env"
	ThresholdSourceFile    ThresholdSource = "file"
	ThresholdSourceDefault ThresholdSource = "default"
)

// ThresholdResolution carries the effective value and its operational source.
type ThresholdResolution struct {
	Value      float32
	Source     ThresholdSource
	ConfigPath string
}

type thresholdConfig struct {
	Version             int     `json:"version"`
	SimilarityThreshold float32 `json:"similarity_threshold"`
}

// ResolveEffectiveThreshold applies env > file > built-in default precedence.
// An empty configPath disables file lookup, which keeps library tests hermetic.
func ResolveEffectiveThreshold(configPath string) ThresholdResolution {
	if value, ok := validThresholdString(os.Getenv("AVLP_SIMILARITY_THRESHOLD")); ok {
		return ThresholdResolution{Value: value, Source: ThresholdSourceEnv}
	}
	configPath = strings.TrimSpace(configPath)
	if configPath != "" {
		if value, err := ReadThresholdConfig(configPath); err == nil {
			return ThresholdResolution{
				Value:      value,
				Source:     ThresholdSourceFile,
				ConfigPath: configPath,
			}
		}
	}
	return ThresholdResolution{
		Value:  DefaultSimilarityThreshold,
		Source: ThresholdSourceDefault,
	}
}

// ReadThresholdConfig reads and validates a versioned routing config.
func ReadThresholdConfig(path string) (float32, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var cfg thresholdConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return 0, fmt.Errorf("decode threshold config: %w", err)
	}
	if cfg.Version != thresholdConfigVersion {
		return 0, fmt.Errorf("unsupported threshold config version %d", cfg.Version)
	}
	if !validThreshold(cfg.SimilarityThreshold) {
		return 0, fmt.Errorf("similarity_threshold must be in (0, 1]")
	}
	return cfg.SimilarityThreshold, nil
}

// WriteThresholdConfig atomically persists a calibrated routing threshold.
func WriteThresholdConfig(path string, threshold float32) error {
	if !validThreshold(threshold) {
		return fmt.Errorf("similarity threshold must be in (0, 1]")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("config path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(thresholdConfig{
		Version:             thresholdConfigVersion,
		SimilarityThreshold: threshold,
	}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".avlp-config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func validThresholdString(raw string) (float32, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 32)
	if err != nil {
		return 0, false
	}
	threshold := float32(value)
	return threshold, validThreshold(threshold)
}

func validThreshold(value float32) bool {
	return value > 0 && value <= 1
}

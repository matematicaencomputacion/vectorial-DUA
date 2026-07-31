// Package stt provides speech-to-text clients for the Master web prototype.
package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultSTTModel    = "whisper-1"
	defaultSTTTimeout  = 30 * time.Second
	defaultSTTLanguage = "es"
	// MaxAudioBytes is the gateway/client size guard for a single upload (~60s).
	MaxAudioBytes = 10 << 20
)

// HTTPTranscriberConfig configures an OpenAI-compatible transcriptions client.
type HTTPTranscriberConfig struct {
	URL        string
	Model      string
	APIKey     string
	Language   string
	Timeout    time.Duration
	MaxRetries int
	Client     *http.Client
}

// HTTPTranscriber calls POST …/audio/transcriptions.
type HTTPTranscriber struct {
	endpoint   string
	model      string
	apiKey     string
	language   string
	timeout    time.Duration
	maxRetries int
	client     *http.Client
}

type transcriptionResponse struct {
	Text  string `json:"text"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// NewHTTPTranscriber builds a transcriber from explicit configuration.
func NewHTTPTranscriber(cfg HTTPTranscriberConfig) (*HTTPTranscriber, error) {
	endpoint, err := normalizeTranscriptionsURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultSTTModel
	}
	lang := strings.TrimSpace(cfg.Language)
	if lang == "" {
		lang = defaultSTTLanguage
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultSTTTimeout
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPTranscriber{
		endpoint:   endpoint,
		model:      model,
		apiKey:     cfg.APIKey,
		language:   lang,
		timeout:    timeout,
		maxRetries: maxRetries,
		client:     client,
	}, nil
}

// NewHTTPTranscriberFromEnv returns nil when AVLP_STT_URL is unset.
func NewHTTPTranscriberFromEnv() (*HTTPTranscriber, error) {
	rawURL := strings.TrimSpace(os.Getenv("AVLP_STT_URL"))
	if rawURL == "" {
		return nil, nil
	}
	return NewHTTPTranscriber(HTTPTranscriberConfig{
		URL:      rawURL,
		Model:    strings.TrimSpace(os.Getenv("AVLP_STT_MODEL")),
		APIKey:   os.Getenv("AVLP_STT_API_KEY"),
		Language: strings.TrimSpace(os.Getenv("AVLP_STT_LANGUAGE")),
		Timeout:  sttTimeoutFromEnv(),
	})
}

// Enabled reports whether a transcriber is configured.
func (h *HTTPTranscriber) Enabled() bool {
	return h != nil
}

// ModelName returns the configured model for telemetry.
func (h *HTTPTranscriber) ModelName() string {
	if h == nil {
		return ""
	}
	return h.model
}

// Transcribe sends audio bytes as multipart file and returns recognized text.
func (h *HTTPTranscriber) Transcribe(ctx context.Context, audio []byte, filename, contentType string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("HTTPTranscriber is nil")
	}
	if len(audio) == 0 {
		return "", fmt.Errorf("audio is empty")
	}
	if len(audio) > MaxAudioBytes {
		return "", fmt.Errorf("audio exceeds %d bytes", MaxAudioBytes)
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "audio.webm"
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var lastErr error
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		text, status, err := h.doTranscribe(ctx, audio, filename, contentType)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if status < 500 || status > 599 || attempt == h.maxRetries {
			break
		}
	}
	return "", fmt.Errorf("STT %s: %w", h.endpoint, lastErr)
}

func (h *HTTPTranscriber) doTranscribe(ctx context.Context, audio []byte, filename, contentType string) (string, int, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", 0, err
	}
	if _, err := part.Write(audio); err != nil {
		return "", 0, err
	}
	if err := w.WriteField("model", h.model); err != nil {
		return "", 0, err
	}
	if h.language != "" {
		if err := w.WriteField("language", h.language); err != nil {
			return "", 0, err
		}
	}
	if err := w.Close(); err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, &body)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	// Some servers ignore the part Content-Type; keep filename extension authoritative.
	_ = contentType
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request failed (timeout=%s): %w", h.timeout, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(raw))
		var parsed transcriptionResponse
		if json.Unmarshal(raw, &parsed) == nil && parsed.Error != nil && parsed.Error.Message != "" {
			message = parsed.Error.Message
		}
		return "", resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	var parsed transcriptionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		return "", resp.StatusCode, fmt.Errorf("empty transcription in response")
	}
	return text, resp.StatusCode, nil
}

func normalizeTranscriptionsURL(raw string) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(raw), "/")
	if endpoint == "" {
		return "", fmt.Errorf("HTTPTranscriber: URL is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("HTTPTranscriber: invalid URL %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("HTTPTranscriber: URL scheme must be http or https")
	}
	if strings.HasSuffix(endpoint, "/audio/transcriptions") {
		return endpoint, nil
	}
	return endpoint + "/audio/transcriptions", nil
}

func sttTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AVLP_STT_TIMEOUT"))
	if raw == "" {
		return defaultSTTTimeout
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultSTTTimeout
	}
	return timeout
}

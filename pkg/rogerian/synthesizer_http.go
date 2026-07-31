package rogerian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultLLMModel   = "qwen3:4b-instruct"
	defaultLLMTimeout = 30 * time.Second
)

// HTTPSynthesizerConfig configures an OpenAI Chat Completions client.
type HTTPSynthesizerConfig struct {
	URL        string
	Model      string
	APIKey     string
	Timeout    time.Duration
	MaxRetries int
	Client     *http.Client
}

// HTTPSynthesizer calls an OpenAI-compatible chat/completions endpoint.
type HTTPSynthesizer struct {
	endpoint   string
	model      string
	apiKey     string
	timeout    time.Duration
	maxRetries int
	client     *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// NewHTTPSynthesizer builds a synthesizer from explicit configuration.
func NewHTTPSynthesizer(cfg HTTPSynthesizerConfig) (*HTTPSynthesizer, error) {
	endpoint, err := normalizeChatURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultLLMModel
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultLLMTimeout
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPSynthesizer{
		endpoint:   endpoint,
		model:      model,
		apiKey:     cfg.APIKey,
		timeout:    timeout,
		maxRetries: maxRetries,
		client:     client,
	}, nil
}

// NewHTTPSynthesizerFromEnv returns nil when AVLP_LLM_URL is unset.
func NewHTTPSynthesizerFromEnv() (*HTTPSynthesizer, error) {
	rawURL := strings.TrimSpace(os.Getenv("AVLP_LLM_URL"))
	if rawURL == "" {
		return nil, nil
	}
	return NewHTTPSynthesizer(HTTPSynthesizerConfig{
		URL:     rawURL,
		Model:   strings.TrimSpace(os.Getenv("AVLP_LLM_MODEL")),
		APIKey:  os.Getenv("AVLP_LLM_API_KEY"),
		Timeout: llmTimeoutFromEnv(),
	})
}

// Synthesize requests grounded Markdown using the bundle's complete prompt.
func (h *HTTPSynthesizer) Synthesize(ctx context.Context, bundle PromptBundle) (string, error) {
	if h == nil {
		return "", fmt.Errorf("HTTPSynthesizer is nil")
	}
	if strings.TrimSpace(bundle.FullPrompt) == "" {
		return "", fmt.Errorf("PromptBundle.FullPrompt is required")
	}
	system := strings.TrimSpace(bundle.SystemStyle)
	if !bundle.EmptyContext {
		system = strings.TrimSpace(system + " " + strings.Join([]string{
			"Usa EXCLUSIVAMENTE el contexto verificado incluido en el mensaje.",
			"No completes vacíos con conocimiento externo ni inventes hechos.",
			"Si el contexto no alcanza, dilo explícitamente.",
			"Cita afirmaciones usando [#].",
			"No generes una sección Fuentes: la aplicación la agrega.",
		}, " "))
	}
	body, err := json.Marshal(chatRequest{
		Model: h.model,
		Messages: []chatMessage{
			{Role: "system", Content: strings.TrimSpace(system)},
			{Role: "user", Content: bundle.FullPrompt},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", fmt.Errorf("LLM marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		content, status, err := h.doSynthesize(ctx, body)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if status < 500 || status > 599 || attempt == h.maxRetries {
			break
		}
	}
	return "", fmt.Errorf("LLM %s: %w", h.endpoint, lastErr)
}

// ModelName returns the configured model for operational telemetry.
func (h *HTTPSynthesizer) ModelName() string {
	if h == nil {
		return ""
	}
	return h.model
}

func (h *HTTPSynthesizer) doSynthesize(ctx context.Context, body []byte) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request failed (timeout=%s): %w", h.timeout, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(raw))
		var parsed chatResponse
		if json.Unmarshal(raw, &parsed) == nil && parsed.Error != nil && parsed.Error.Message != "" {
			message = parsed.Error.Message
		}
		return "", resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", resp.StatusCode, fmt.Errorf("empty choices in response")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", resp.StatusCode, fmt.Errorf("empty completion in response")
	}
	return content, resp.StatusCode, nil
}

func normalizeChatURL(raw string) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(raw), "/")
	if endpoint == "" {
		return "", fmt.Errorf("HTTPSynthesizer: URL is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("HTTPSynthesizer: invalid URL %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("HTTPSynthesizer: URL scheme must be http or https")
	}
	if strings.HasSuffix(endpoint, "/chat/completions") {
		return endpoint, nil
	}
	return endpoint + "/chat/completions", nil
}

func llmTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AVLP_LLM_TIMEOUT"))
	if raw == "" {
		return defaultLLMTimeout
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultLLMTimeout
	}
	return timeout
}

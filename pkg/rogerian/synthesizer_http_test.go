package rogerian_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/internal/testenv"
	"github.com/vectorial-dua/avlp/pkg/rogerian"
)

func testBundle() rogerian.PromptBundle {
	return rogerian.PromptBundle{
		SystemStyle: "Facilitador DUA.",
		FullPrompt:  "## Contexto verificado\n[1] Un .env separa secretos.",
	}
}

func chatCompletion(content string) []byte {
	body, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{
			"message": map[string]any{"role": "assistant", "content": content},
		}},
	})
	return body
}

func TestHTTPSynthesizerSuccess(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path=%s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Model != "test-model" || len(request.Messages) != 2 {
			t.Errorf("request=%+v", request)
		}
		if request.Messages[1].Content != testBundle().FullPrompt {
			t.Errorf("user prompt=%q", request.Messages[1].Content)
		}
		if !strings.Contains(request.Messages[0].Content, "EXCLUSIVAMENTE") ||
			!strings.Contains(request.Messages[0].Content, "No generes una sección Fuentes") {
			t.Errorf("system guardrail=%q", request.Messages[0].Content)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatCompletion("# Explicación\nContenido grounded [1]."))
	}))
	defer srv.Close()

	synth, err := rogerian.NewHTTPSynthesizer(rogerian.HTTPSynthesizerConfig{
		URL:    srv.URL + "/v1",
		Model:  "test-model",
		APIKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := synth.Synthesize(context.Background(), testBundle())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Contenido grounded") {
		t.Fatalf("content=%q", content)
	}
	if auth != "Bearer secret" {
		t.Fatalf("auth=%q", auth)
	}
	if synth.ModelName() != "test-model" {
		t.Fatalf("model=%q", synth.ModelName())
	}
}

func TestHTTPSynthesizerRetries5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(chatCompletion("recuperado"))
	}))
	defer srv.Close()

	synth, err := rogerian.NewHTTPSynthesizer(rogerian.HTTPSynthesizerConfig{
		URL:   srv.URL,
		Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := synth.Synthesize(context.Background(), testBundle())
	if err != nil {
		t.Fatal(err)
	}
	if content != "recuperado" || attempts != 2 {
		t.Fatalf("content=%q attempts=%d", content, attempts)
	}
}

func TestHTTPSynthesizerDoesNotRetry4xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()
	synth, err := rogerian.NewHTTPSynthesizer(rogerian.HTTPSynthesizerConfig{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = synth.Synthesize(context.Background(), testBundle())
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("error=%v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts=%d", attempts)
	}
}

func TestHTTPSynthesizerTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write(chatCompletion("late"))
	}))
	defer srv.Close()
	synth, err := rogerian.NewHTTPSynthesizer(rogerian.HTTPSynthesizerConfig{
		URL:     srv.URL,
		Timeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = synth.Synthesize(context.Background(), testBundle())
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error=%v", err)
	}
}

func TestHTTPSynthesizerRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "invalid JSON", body: `{`, want: "decode response"},
		{name: "empty choices", body: `{"choices":[]}`, want: "empty choices"},
		{name: "empty content", body: `{"choices":[{"message":{"content":" "}}]}`, want: "empty completion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			synth, err := rogerian.NewHTTPSynthesizer(rogerian.HTTPSynthesizerConfig{URL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			_, err = synth.Synthesize(context.Background(), testBundle())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
		})
	}
}

func TestHTTPSynthesizerFromEnv(t *testing.T) {
	testenv.Isolate(t)
	none, err := rogerian.NewHTTPSynthesizerFromEnv()
	if err != nil || none != nil {
		t.Fatalf("none=%v err=%v", none, err)
	}
	t.Setenv("AVLP_LLM_URL", "http://localhost:11434/v1")
	t.Setenv("AVLP_LLM_MODEL", "qwen-test")
	t.Setenv("AVLP_LLM_TIMEOUT", "2s")
	synth, err := rogerian.NewHTTPSynthesizerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if synth == nil || synth.ModelName() != "qwen-test" {
		t.Fatalf("synth=%v", synth)
	}
}

func TestHTTPSynthesizerValidatesURL(t *testing.T) {
	for _, raw := range []string{"", "localhost:11434/v1", "ftp://example.com/v1"} {
		if _, err := rogerian.NewHTTPSynthesizer(rogerian.HTTPSynthesizerConfig{URL: raw}); err == nil {
			t.Fatalf("URL %q should fail", raw)
		}
	}
}

package stt_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/stt"
)

func TestHTTPTranscriberSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := r.FormValue("model"); got != "whisper-1" {
			t.Fatalf("model=%q", got)
		}
		if got := r.FormValue("language"); got != "es" {
			t.Fatalf("language=%q", got)
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("file: %v", err)
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		if string(b) != "fake-audio" {
			t.Fatalf("audio=%q", b)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hola mundo"}`))
	}))
	t.Cleanup(srv.Close)

	tr, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{URL: srv.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	text, err := tr.Transcribe(context.Background(), []byte("fake-audio"), "clip.webm", "audio/webm")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hola mundo" {
		t.Fatalf("text=%q", text)
	}
}

func TestHTTPTranscriberRetryOn5xx(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`busy`))
			return
		}
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	tr, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	text, err := tr.Transcribe(context.Background(), []byte("a"), "a.webm", "audio/webm")
	if err != nil {
		t.Fatal(err)
	}
	if text != "ok" {
		t.Fatalf("text=%q", text)
	}
	if n.Load() != 2 {
		t.Fatalf("attempts=%d", n.Load())
	}
}

func TestHTTPTranscriberDoesNotRetry4xx(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad audio"}}`))
	}))
	t.Cleanup(srv.Close)

	tr, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.Transcribe(context.Background(), []byte("a"), "a.webm", "audio/webm")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("err=%v", err)
	}
	if n.Load() != 1 {
		t.Fatalf("attempts=%d", n.Load())
	}
}

func TestHTTPTranscriberTimeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"text":"late"}`))
	}))
	t.Cleanup(srv.Close)

	tr, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{
		URL:     srv.URL,
		Timeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.Transcribe(context.Background(), []byte("a"), "a.webm", "audio/webm")
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestHTTPTranscriberFromEnv(t *testing.T) {
	t.Setenv("AVLP_STT_URL", "")
	none, err := stt.NewHTTPTranscriberFromEnv()
	if err != nil || none != nil {
		t.Fatalf("none=%v err=%v", none, err)
	}
	t.Setenv("AVLP_STT_URL", "http://127.0.0.1:8081/v1")
	t.Setenv("AVLP_STT_MODEL", "small")
	t.Setenv("AVLP_STT_LANGUAGE", "es")
	tr, err := stt.NewHTTPTranscriberFromEnv()
	if err != nil || tr == nil {
		t.Fatalf("tr=%v err=%v", tr, err)
	}
	if tr.ModelName() != "small" {
		t.Fatalf("model=%s", tr.ModelName())
	}
}

func TestHTTPTranscriberRejectsEmptyAndOversize(t *testing.T) {
	t.Parallel()
	tr, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{URL: "http://127.0.0.1:9/v1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Transcribe(context.Background(), nil, "a.webm", "audio/webm"); err == nil {
		t.Fatal("expected empty error")
	}
	big := make([]byte, stt.MaxAudioBytes+1)
	if _, err := tr.Transcribe(context.Background(), big, "a.webm", "audio/webm"); err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestHTTPTranscriberValidatesURL(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "notaurl", "ftp://x"} {
		if _, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{URL: raw}); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

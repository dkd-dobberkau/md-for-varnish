package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handleHealthz(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
	if body["service"] != "markdown-sidecar" {
		t.Errorf("expected service=markdown-sidecar, got %q", body["service"])
	}
}

func TestConvert_SuccessfulConversion(t *testing.T) {
	sampleHTML := loadFixture(t)

	// Mock origin server returning HTML
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, sampleHTML)
	}))
	defer origin.Close()

	cfg := Config{
		OriginBaseURL: origin.URL,
		OriginTimeout: 10_000_000_000, // 10s
		ContentSignal: "ai-train=yes, search=yes, ai-input=yes",
		TokenModel:    "cl100k_base",
	}
	client := origin.Client()

	handler := convertHandler(cfg, client)
	req := httptest.NewRequest(http.MethodGet, "/test-page", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Errorf("expected text/markdown content-type, got %q", ct)
	}
	if resp.Header.Get("X-Markdown-Tokens") == "" {
		t.Error("expected X-Markdown-Tokens header")
	}
	if resp.Header.Get("X-Conversion-Time-Ms") == "" {
		t.Error("expected X-Conversion-Time-Ms header")
	}
	if resp.Header.Get("Content-Signal") != "ai-train=yes, search=yes, ai-input=yes" {
		t.Errorf("unexpected Content-Signal: %q", resp.Header.Get("Content-Signal"))
	}

	body := w.Body.String()
	if !strings.Contains(body, "# Willkommen") {
		t.Error("expected markdown heading in response body")
	}
}

func TestConvert_NonHTMLPassthrough(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"key": "value"}`)
	}))
	defer origin.Close()

	cfg := Config{
		OriginBaseURL: origin.URL,
		OriginTimeout: 10_000_000_000,
		TokenModel:    "cl100k_base",
	}
	client := origin.Client()

	handler := convertHandler(cfg, client)
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content-type passthrough, got %q", ct)
	}
}

func TestConvert_OriginErrorForwarded(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer origin.Close()

	cfg := Config{
		OriginBaseURL: origin.URL,
		OriginTimeout: 10_000_000_000,
		TokenModel:    "cl100k_base",
	}
	client := origin.Client()

	handler := convertHandler(cfg, client)
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestConvert_OriginUnreachable(t *testing.T) {
	cfg := Config{
		OriginBaseURL: "http://127.0.0.1:19999",
		OriginTimeout: 1_000_000_000, // 1s
		TokenModel:    "cl100k_base",
	}
	client := &http.Client{Timeout: cfg.OriginTimeout}

	handler := convertHandler(cfg, client)
	req := httptest.NewRequest(http.MethodGet, "/some-page", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", resp.StatusCode)
	}
}

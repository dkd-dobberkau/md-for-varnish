package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the sidecar configuration from environment variables.
type Config struct {
	OriginBaseURL   string
	OriginTimeout   time.Duration
	ContentSignal   string
	TokenModel      string
	LogLevel        slog.Level
	ExtraSelectors  []string
	ListenAddr      string
}

func loadConfig() Config {
	cfg := Config{
		OriginBaseURL: getenv("ORIGIN_BASE_URL", "http://localhost:8080"),
		ContentSignal: getenv("CONTENT_SIGNAL", "ai-train=yes, search=yes, ai-input=yes"),
		TokenModel:    getenv("TOKEN_MODEL", "cl100k_base"),
		ListenAddr:    ":8000",
	}

	timeoutSec, err := strconv.Atoi(getenv("ORIGIN_TIMEOUT", "10"))
	if err != nil {
		timeoutSec = 10
	}
	cfg.OriginTimeout = time.Duration(timeoutSec) * time.Second

	switch strings.ToUpper(getenv("LOG_LEVEL", "INFO")) {
	case "DEBUG":
		cfg.LogLevel = slog.LevelDebug
	case "WARN", "WARNING":
		cfg.LogLevel = slog.LevelWarn
	case "ERROR":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}

	if extra := os.Getenv("STRIP_SELECTORS"); extra != "" {
		for _, s := range strings.Split(extra, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				cfg.ExtraSelectors = append(cfg.ExtraSelectors, trimmed)
			}
		}
	}

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Built-in healthcheck mode for distroless containers
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		resp, err := http.Get("http://localhost:8000/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := loadConfig()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})))

	client := &http.Client{
		Timeout: cfg.OriginTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /{path...}", convertHandler(cfg, client))

	slog.Info("Starting markdown-sidecar", "addr", cfg.ListenAddr, "origin", cfg.OriginBaseURL)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "markdown-sidecar",
	})
}

func convertHandler(cfg Config, client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build origin URL
		originURL := buildOriginURL(cfg.OriginBaseURL, r.URL)

		slog.Info("Converting", "url", originURL)
		start := time.Now()

		// Fetch from origin
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, originURL, nil)
		if err != nil {
			slog.Error("Failed to create request", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Accept", "text/html")
		req.Header.Set("User-Agent", "markdown-sidecar/0.1")
		if forwarded := r.RemoteAddr; forwarded != "" {
			// Extract just the IP (strip port)
			host := forwarded
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				host = host[:idx]
			}
			req.Header.Set("X-Forwarded-For", host)
		}

		resp, err := client.Do(req)
		if err != nil {
			slog.Error("Origin request failed", "error", err)
			http.Error(w, "Origin unreachable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Forward origin errors
		if resp.StatusCode >= 400 {
			w.WriteHeader(resp.StatusCode)
			fmt.Fprint(w, "Origin returned an error")
			return
		}

		// Non-HTML passthrough
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			for k, vals := range resp.Header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}

		// Read HTML body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Failed to read origin response", "error", err)
			http.Error(w, "Failed to read origin", http.StatusBadGateway)
			return
		}

		// Convert
		mdText, _ := htmlToMarkdown(string(body), cfg.ExtraSelectors)
		tokenCount := countTokens(mdText, cfg.TokenModel)
		durationMs := time.Since(start).Milliseconds()

		slog.Info("Converted", "url", originURL, "tokens", tokenCount, "duration_ms", durationMs)

		// Set response headers
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("X-Markdown-Tokens", strconv.Itoa(tokenCount))
		w.Header().Set("X-Conversion-Time-Ms", strconv.FormatInt(durationMs, 10))
		w.Header().Set("Content-Signal", cfg.ContentSignal)
		w.Header().Set("Vary", "Accept")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, mdText)
	}
}

// buildOriginURL constructs the full origin URL from the base URL and request path.
func buildOriginURL(baseURL string, reqURL *url.URL) string {
	base := strings.TrimRight(baseURL, "/")
	path := reqURL.Path
	if path == "" {
		path = "/"
	}
	origin := base + path
	if reqURL.RawQuery != "" {
		origin += "?" + reqURL.RawQuery
	}
	return origin
}

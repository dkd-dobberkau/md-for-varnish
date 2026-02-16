# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Markdown for Agents – a Varnish sidecar that converts HTML pages to Markdown for AI agents and crawlers, following the [Cloudflare "Markdown for Agents"](https://blog.cloudflare.com/markdown-for-agents/) convention. Requests with `Accept: text/markdown` get a token-efficient Markdown response; all other requests pass through to the origin (TYPO3).

## Architecture

Three-service Docker stack:

1. **Varnish** (`default.vcl`) – Content router. Inspects `Accept` header: `text/markdown` → sidecar, everything else → TYPO3 origin. Caches both variants separately via `hash_data("markdown")`.
2. **Markdown Sidecar** (`go-sidecar/`) – Go HTTP service (~9 MB distroless image). Fetches HTML from origin, strips chrome (nav, footer, scripts, cookie banners), isolates content (`<main>`, `<article>`, `#content`), converts to Markdown with YAML front matter, counts tokens via tiktoken.
3. **TYPO3 Origin** – Placeholder nginx in dev; real TYPO3 in production. Configured via `ORIGIN_BASE_URL`.

## Commands

```bash
# Start the full stack
docker compose up -d

# Rebuild sidecar after code changes
docker compose up -d --build markdown-sidecar

# Test HTML passthrough
curl http://localhost/

# Test Markdown conversion
curl -H "Accept: text/markdown" http://localhost/

# View sidecar logs
docker compose logs -f markdown-sidecar

# Run Go tests
cd go-sidecar && go test -v ./...
```

## File Layout

```
go-sidecar/
  main.go              ← HTTP server, routes, config
  convert.go           ← HTML→Markdown conversion pipeline
  convert_test.go      ← Unit tests: metadata, stripping, conversion, token counting
  handler_test.go      ← HTTP handler tests: healthz, conversion, passthrough, errors
  go.mod / go.sum      ← Go module dependencies
varnish/default.vcl    ← Varnish content routing
test/html/             ← Nginx test HTML fixtures
tests/fixtures/page.html ← Full TYPO3-like test page (used by Go tests)
Dockerfile             ← Go multi-stage build (distroless)
docker-compose.yml     ← Stack: Varnish + Sidecar + Origin
```

## Key Environment Variables

Configured in `docker-compose.yml` on the `markdown-sidecar` service:

| Variable | Purpose |
|----------|---------|
| `ORIGIN_BASE_URL` | Origin server URL (default: `http://localhost:8080`) |
| `ORIGIN_TIMEOUT` | Origin request timeout in seconds |
| `CONTENT_SIGNAL` | Content-Signal header value per contentsignals.org |
| `STRIP_SELECTORS` | Additional CSS selectors to strip (comma-separated) |
| `TOKEN_MODEL` | tiktoken encoding model (default: `cl100k_base`) |
| `LOG_LEVEL` | Log level (DEBUG, INFO, WARN, ERROR) |

## Conversion Pipeline (convert.go)

1. `extractMetadata()` – Pulls title, description, author, keywords, og:image from `<head>` for YAML front matter
2. `findContentRoot()` – Finds `<main>` / `<article>` / `#content` / `.content` / `<body>` as content root
3. `stripNonContent()` – Removes elements matching strip selectors list
4. `removeImages()` – All `<img>` tags stripped (useless for agents)
5. HTML-to-Markdown conversion – ATX headings, dash bullets, pipe tables (via html-to-markdown v2)
6. `cleanBlankLines()` – Max 2 consecutive blank lines
7. `buildFrontMatter()` – YAML front matter from metadata
8. `countTokens()` – tiktoken estimation, fallback to len/4

## Varnish VCL (default.vcl)

- `vcl_recv`: Routes based on `Accept` header, normalizes it for cache consistency
- `vcl_hash`: Adds `"markdown"` to cache key for markdown requests
- `vcl_backend_response`: Sets TTL (300s) and grace periods, ensures `Vary: Accept`
- `vcl_deliver`: Adds `X-Cache` / `X-Cache-Hits` debug headers
- Health probe on sidecar `/healthz` endpoint (10s interval)

## Go Dependencies

- `github.com/JohannesKaufmann/html-to-markdown/v2` – HTML→Markdown conversion
- `github.com/PuerkitoBio/goquery` – HTML parsing and CSS selectors
- `github.com/pkoukk/tiktoken-go` – Token counting
- `net/http` (stdlib) – HTTP server with Go 1.22+ routing
- `log/slog` (stdlib) – Structured logging

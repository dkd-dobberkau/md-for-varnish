# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Markdown for Agents – a Varnish sidecar that converts HTML pages to Markdown for AI agents and crawlers, following the [Cloudflare "Markdown for Agents"](https://blog.cloudflare.com/markdown-for-agents/) convention. Requests with `Accept: text/markdown` get a token-efficient Markdown response; all other requests pass through to the origin (TYPO3).

## Architecture

Three-service Docker stack:

1. **Varnish** (`default.vcl`) – Content router. Inspects `Accept` header: `text/markdown` → sidecar, everything else → TYPO3 origin. Caches both variants separately via `hash_data("markdown")`.
2. **Markdown Sidecar** (`main.py`) – FastAPI service. Fetches HTML from origin, strips chrome (nav, footer, scripts, cookie banners), isolates content (`<main>`, `<article>`, `#content`), converts to Markdown with YAML front matter, counts tokens via tiktoken.
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
curl http://localhost/ -H "Accept: text/markdown"

# View sidecar logs
docker compose logs -f markdown-sidecar
```

## File Layout

```
app/main.py            ← FastAPI sidecar service
varnish/default.vcl    ← Varnish content routing
test/html/             ← Nginx test HTML fixtures
tests/                 ← pytest test suite
  conftest.py          ← Shared fixtures (sample_html, minimal_html, html_no_main)
  fixtures/page.html   ← Full TYPO3-like test page
  test_convert.py      ← Unit tests: metadata, stripping, conversion, token counting
  test_api.py          ← API tests: healthz, convert endpoint, error handling
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
| `LOG_LEVEL` | Python log level |

## Conversion Pipeline (main.py)

1. `extract_metadata()` – Pulls title, description, author, og:image from `<head>` for YAML front matter
2. Content isolation – Finds `<main>` / `<article>` / `#content` / `body` as content root
3. `strip_non_content()` – Removes elements matching `STRIP_SELECTORS` list
4. Image removal – All `<img>` tags stripped (useless for agents)
5. `markdownify()` conversion – ATX headings, dash bullets
6. Blank line cleanup – Max 2 consecutive blank lines
7. `count_tokens()` – tiktoken estimation, fallback to len/4

## Varnish VCL (default.vcl)

- `vcl_recv`: Routes based on `Accept` header, normalizes it for cache consistency
- `vcl_hash`: Adds `"markdown"` to cache key for markdown requests
- `vcl_backend_response`: Sets TTL (300s) and grace periods, ensures `Vary: Accept`
- `vcl_deliver`: Adds `X-Cache` / `X-Cache-Hits` debug headers
- Health probe on sidecar `/healthz` endpoint (10s interval)

## Python Tooling

**Immer `uv` verwenden** – kein pip, kein pip-compile. Gilt für lokale Entwicklung und Docker.

```bash
# Dependency hinzufügen
uv add <package>

# Dev-Dependencies installieren
uv sync --dev

# Lokal ausführen
uv run uvicorn app.main:app --reload

# Sync nach pyproject.toml-Änderung
uv sync

# Alle Tests
uv run pytest

# Einzelnen Test
uv run pytest tests/test_convert.py::TestHtmlToMarkdown::test_contains_heading -v

# Nur API-Tests
uv run pytest tests/test_api.py -v
```

Dependencies: `httpx`, `tiktoken`, `beautifulsoup4`, `markdownify`, `fastapi`, `uvicorn`

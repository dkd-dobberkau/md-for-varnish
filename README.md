# Markdown for Agents – Varnish Sidecar

Ein leichtgewichtiger Go-Sidecar-Service, der hinter Varnish HTML-Seiten in Markdown konvertiert – kompatibel mit der [Cloudflare "Markdown for Agents"](https://blog.cloudflare.com/markdown-for-agents/) Konvention.

AI-Agents und Crawler, die per `Accept: text/markdown` anfragen, erhalten automatisch eine tokeneffiziente Markdown-Version der Seite. Alle anderen Requests werden unverändert an den Origin (z.B. TYPO3) durchgereicht.

## Architektur

```
                         ┌──────────────────────┐
  Browser ──────────────▶│                      │──────▶ TYPO3 Origin
  (Accept: text/html)    │       Varnish        │        (HTML)
                         │    Content Routing    │
  AI Agent ─────────────▶│                      │──────▶ Markdown Sidecar
  (Accept: text/markdown)│   Vary: Accept Cache  │        (HTML → MD)
                         └──────────────────────┘
```

Varnish prüft den `Accept`-Header und routet:

- `text/html` (Standard) → direkt zum TYPO3 Origin
- `text/markdown` → zum Sidecar, der das HTML vom Origin holt, konvertiert und zurückliefert

Beide Varianten werden separat gecacht (via `hash_data("markdown")`).

| Service | Image | Beschreibung |
|---------|-------|--------------|
| Varnish | `varnish:7.6` | Content Router mit separaten Cache-Keys für HTML/Markdown |
| Markdown Sidecar | Go auf distroless | ~9 MB Image, <50ms Startup |
| Origin (Dev) | `nginx:alpine` | Platzhalter; in Produktion durch echtes CMS ersetzen |

## Response-Headers

Konvertierte Responses enthalten diese Headers, analog zu Cloudflare:

| Header | Beispiel | Beschreibung |
|--------|---------|-------------|
| `Content-Type` | `text/markdown; charset=utf-8` | MIME-Type |
| `X-Markdown-Tokens` | `129` | Geschätzte Token-Anzahl (cl100k_base) |
| `X-Conversion-Time-Ms` | `44` | Konvertierungsdauer |
| `Content-Signal` | `ai-train=yes, search=yes, ai-input=yes` | Nutzungserlaubnis gemäß [contentsignals.org](https://contentsignals.org/) |
| `Vary` | `Accept` | Cache-Differenzierung |
| `Cache-Control` | `public, max-age=300` | Caching |

## Konvertierungspipeline

1. **Metadata-Extraktion** – Titel, Description, Author, Keywords, OG-Image werden als YAML Front Matter vorangestellt
2. **Content-Isolation** – Der Service sucht `<main>`, `<article>`, `#content` oder `.content` als Inhaltsbereich
3. **Chrome-Stripping** – Navigation, Footer, Cookie-Banner, Sidebar, Scripts, Styles, Formulare werden entfernt
4. **Bilder-Entfernung** – Alle `<img>` Tags werden entfernt (für Agents nutzlos)
5. **Markdown-Konvertierung** – Headings, Links, Listen, Tabellen, Blockquotes, Code-Blöcke bleiben erhalten
6. **Cleanup** – Überflüssige Leerzeilen werden auf max. 2 reduziert
7. **Token-Counting** – Schätzung via tiktoken (cl100k_base) mit len/4 Fallback

## Beispiel-Output

```markdown
---
title: "Meine Seite"
description: "Seitenbeschreibung"
author: "Autor"
---

# Hauptüberschrift

Dies ist ein **Absatz** mit [einem Link](https://example.com).

## Features

- Erster Punkt
- Zweiter Punkt

| Name  | Wert |
|-------|------|
| Alpha | 1    |
| Beta  | 2    |
```

## Quickstart

```bash
# Repository klonen und starten
docker compose up -d

# Test: HTML (normal)
curl http://localhost/

# Test: Markdown (für Agents)
curl -H "Accept: text/markdown" http://localhost/
```

## Konfiguration

Alle Einstellungen über Umgebungsvariablen in `docker-compose.yml`:

| Variable | Default | Beschreibung |
|----------|---------|-------------|
| `ORIGIN_BASE_URL` | `http://localhost:8080` | URL des TYPO3 Origin |
| `ORIGIN_TIMEOUT` | `10` | Timeout für Origin-Requests in Sekunden |
| `CONTENT_SIGNAL` | `ai-train=yes, search=yes, ai-input=yes` | Content Signals Header |
| `STRIP_SELECTORS` | *(leer)* | Zusätzliche CSS-Selektoren zum Entfernen (kommasepariert) |
| `TOKEN_MODEL` | `cl100k_base` | tiktoken Encoding-Modell |
| `LOG_LEVEL` | `INFO` | Log-Level (DEBUG, INFO, WARN, ERROR) |

### Eigene Elemente ausschließen

Wenn das TYPO3-Template zusätzliche Elemente enthält, die nicht im Markdown auftauchen sollen (Werbebanner, Tracking-Pixel, spezifische Widgets), können diese per `STRIP_SELECTORS` konfiguriert werden:

```yaml
environment:
  STRIP_SELECTORS: ".ad-banner,.tracking-pixel,#my-widget"
```

## Entwicklung

```bash
# Go-Tests ausführen
cd go-sidecar && go test -v ./...

# Sidecar nach Änderungen neu bauen
docker compose up -d --build markdown-sidecar

# Sidecar-Logs anzeigen
docker compose logs -f markdown-sidecar
```

## Projektstruktur

```
md-for-varnish/
├── go-sidecar/
│   ├── main.go              # HTTP-Server, Routes, Config
│   ├── convert.go           # HTML→Markdown Pipeline
│   ├── convert_test.go      # Unit-Tests für Konvertierung
│   ├── handler_test.go      # HTTP-Handler-Tests
│   ├── go.mod
│   └── go.sum
├── varnish/
│   └── default.vcl          # Varnish Content-Routing
├── test/
│   └── html/                # Test-HTML für nginx Placeholder
├── tests/
│   └── fixtures/page.html   # Test-Fixture für Go-Tests
├── docker-compose.yml       # Stack: Varnish + Sidecar + Origin
├── Dockerfile               # Go Multi-Stage Build (distroless)
└── README.md
```

## Produktionsbetrieb

- `ORIGIN_BASE_URL` auf die tatsächliche TYPO3-Instanz setzen (internes Netzwerk)
- Den `typo3`-Service in `docker-compose.yml` durch den echten Origin ersetzen oder entfernen
- Varnish-Healthcheck für den Sidecar ist bereits konfiguriert (Probe auf `/healthz`)
- Cache-TTL in der VCL an die eigene Aktualisierungsfrequenz anpassen
- Content Signals gemäß eigener Policy konfigurieren
- Monitoring auf `X-Conversion-Time-Ms` für Performance-Überwachung

## Lizenz

[MIT](LICENSE)

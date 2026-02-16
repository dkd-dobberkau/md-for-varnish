# Markdown for Agents – Varnish Sidecar

Ein leichtgewichtiger Sidecar-Service, der hinter Varnish HTML-Seiten in Markdown konvertiert – kompatibel mit der [Cloudflare "Markdown for Agents"](https://blog.cloudflare.com/markdown-for-agents/) Konvention.

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

## Response-Headers

Konvertierte Responses enthalten diese Headers, analog zu Cloudflare:

| Header | Beispiel | Beschreibung |
|--------|---------|-------------|
| `Content-Type` | `text/markdown; charset=utf-8` | MIME-Type |
| `X-Markdown-Tokens` | `417` | Geschätzte Token-Anzahl (cl100k_base) |
| `X-Conversion-Time-Ms` | `42` | Konvertierungsdauer |
| `Content-Signal` | `ai-train=yes, search=yes, ai-input=yes` | Nutzungserlaubnis gemäß contentsignals.org |
| `Vary` | `Accept` | Cache-Differenzierung |

## Konvertierungspipeline

1. **Metadata-Extraktion** – Titel, Description, Author, OG-Image werden als YAML Front Matter vorangestellt
2. **Content-Isolation** – Der Service sucht `<main>`, `<article>` oder `#content` als Inhaltsbereich
3. **Chrome-Stripping** – Navigation, Footer, Cookie-Banner, Sidebar, Scripts, Styles werden entfernt
4. **Markdown-Konvertierung** – Headings, Links, Tabellen, Blockquotes, Code-Blöcke bleiben erhalten
5. **Cleanup** – Überflüssige Leerzeilen werden reduziert, Bilder entfernt (für Agents nutzlos)
6. **Token-Counting** – Schätzung via tiktoken (cl100k_base, kompatibel mit GPT-4 / Claude)

## Quickstart

```bash
# Repository klonen und starten
docker compose up -d

# Test: HTML (normal)
curl http://localhost/

# Test: Markdown (für Agents)
curl http://localhost/ -H "Accept: text/markdown"
```

## Konfiguration

Alle Einstellungen über Umgebungsvariablen in `docker-compose.yml`:

| Variable | Default | Beschreibung |
|----------|---------|-------------|
| `ORIGIN_BASE_URL` | `http://localhost:8080` | URL des TYPO3 Origin |
| `ORIGIN_TIMEOUT` | `10` | Timeout für Origin-Requests in Sekunden |
| `CONTENT_SIGNAL` | `ai-train=yes, search=yes, ai-input=yes` | Content Signals Header |
| `STRIP_SELECTORS` | (leer) | Zusätzliche CSS-Selektoren zum Entfernen (kommasepariert) |
| `LOG_LEVEL` | `INFO` | Log-Level |
| `TOKEN_MODEL` | `cl100k_base` | tiktoken Encoding-Modell |

### Eigene Elemente ausschließen

Wenn das TYPO3-Template zusätzliche Elemente enthält, die nicht im Markdown auftauchen sollen (Werbebanner, Tracking-Pixel, spezifische Widgets), können diese per `STRIP_SELECTORS` konfiguriert werden:

```yaml
environment:
  STRIP_SELECTORS: ".ad-banner,.tracking-pixel,#my-widget"
```

## Lokaler Test ohne Docker

```bash
pip install -r requirements.txt
python test/test_convert.py
```

## Projektstruktur

```
markdown-sidecar/
├── app/
│   └── main.py              # FastAPI Sidecar-Service
├── varnish/
│   └── default.vcl          # Varnish Content-Routing
├── test/
│   ├── html/
│   │   └── index.html       # Test-HTML
│   └── test_convert.py      # Lokaler Konvertierungstest
├── docker-compose.yml        # Stack: Varnish + Sidecar + Origin
├── Dockerfile                # Sidecar Container
├── requirements.txt
└── README.md
```

## Produktionsbetrieb

Für den Einsatz in Produktion:

- `ORIGIN_BASE_URL` auf die tatsächliche TYPO3-Instanz setzen (internes Netzwerk)
- Den `typo3`-Service in `docker-compose.yml` durch den echten Origin ersetzen oder entfernen
- Varnish-Healthcheck für den Sidecar ist bereits konfiguriert (Probe auf `/healthz`)
- Cache-TTL in der VCL an die eigene Aktualisierungsfrequenz anpassen
- Content Signals gemäß eigener Policy konfigurieren
- Monitoring auf `X-Conversion-Time-Ms` für Performance-Überwachung

## Nächste Schritte

- TYPO3-Extension, die Content Signals direkt als Meta-Tag ausgibt
- Konfigurierbare Bildbehandlung (Base64-Inline vs. URL-Referenz vs. Entfernung)
- robots.txt / ai.txt Integration für granulare Agent-Steuerung
- Metriken-Endpoint für Prometheus/Grafana

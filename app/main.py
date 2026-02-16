"""
Markdown for Agents – Sidecar Service
======================================

A lightweight HTTP service that sits behind Varnish as a backend.
When Varnish detects an Accept: text/markdown request, it routes
to this sidecar, which fetches the original HTML from the TYPO3
origin, strips non-content elements, converts to clean Markdown,
and returns the result with appropriate headers.

Designed to work with the Cloudflare "Markdown for Agents" convention:
  - Content-Type: text/markdown; charset=utf-8
  - X-Markdown-Tokens: <estimated token count>
  - Content-Signal: ai-train=yes, search=yes, ai-input=yes
"""

import os
import logging
import time
from urllib.parse import urlparse, urljoin

import httpx
import tiktoken
from bs4 import BeautifulSoup
from markdownify import markdownify as md
from fastapi import FastAPI, Request, Response, HTTPException
from fastapi.responses import PlainTextResponse

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

ORIGIN_BASE_URL = os.getenv("ORIGIN_BASE_URL", "http://localhost:8080")
ORIGIN_TIMEOUT = int(os.getenv("ORIGIN_TIMEOUT", "10"))
CONTENT_SIGNAL = os.getenv(
    "CONTENT_SIGNAL", "ai-train=yes, search=yes, ai-input=yes"
)
LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
TOKEN_MODEL = os.getenv("TOKEN_MODEL", "cl100k_base")

# Elements to strip before conversion – everything that is chrome, not content
STRIP_SELECTORS = [
    "nav", "header", "footer",
    "script", "style", "noscript",
    "iframe", "object", "embed",
    ".cookie-banner", ".cookie-consent",
    "#cookie-banner", "#cookie-consent",
    "[role='navigation']", "[role='banner']", "[role='contentinfo']",
    ".breadcrumb", ".pagination",
    ".sidebar", "aside",
    "form",
]

# Additional selectors can be provided via env (comma-separated)
EXTRA_STRIP = os.getenv("STRIP_SELECTORS", "")
if EXTRA_STRIP:
    STRIP_SELECTORS.extend([s.strip() for s in EXTRA_STRIP.split(",") if s.strip()])

# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------

logging.basicConfig(level=getattr(logging, LOG_LEVEL.upper(), logging.INFO))
logger = logging.getLogger("markdown-sidecar")

app = FastAPI(
    title="Markdown for Agents – Sidecar",
    version="0.1.0",
    docs_url="/healthz/docs",
)

# Lazy-loaded tiktoken encoder
_encoder = None


def get_encoder():
    global _encoder
    if _encoder is None:
        _encoder = tiktoken.get_encoding(TOKEN_MODEL)
    return _encoder


# ---------------------------------------------------------------------------
# HTML → Markdown pipeline
# ---------------------------------------------------------------------------

def extract_metadata(soup: BeautifulSoup) -> dict:
    """Pull useful metadata from <head> for the YAML front matter."""
    meta = {}

    title_tag = soup.find("title")
    if title_tag and title_tag.string:
        meta["title"] = title_tag.string.strip()

    for name in ("description", "author", "keywords"):
        tag = soup.find("meta", attrs={"name": name})
        if tag and tag.get("content"):
            meta[name] = tag["content"].strip()

    og_image = soup.find("meta", attrs={"property": "og:image"})
    if og_image and og_image.get("content"):
        meta["image"] = og_image["content"].strip()

    return meta


def strip_non_content(soup: BeautifulSoup) -> BeautifulSoup:
    """Remove navigation, chrome, scripts and other non-content elements."""
    for selector in STRIP_SELECTORS:
        for element in soup.select(selector):
            element.decompose()
    return soup


def html_to_markdown(html: str, page_url: str = "") -> tuple[str, dict]:
    """
    Convert an HTML document to clean Markdown.

    Returns (markdown_text, metadata_dict).
    """
    soup = BeautifulSoup(html, "html.parser")

    # 1. Extract metadata before stripping
    metadata = extract_metadata(soup)

    # 2. Try to isolate <main> or <article> content
    content_root = (
        soup.find("main")
        or soup.find("article")
        or soup.find("div", {"id": "content"})
        or soup.find("div", {"class": "content"})
        or soup.body
        or soup
    )

    # 3. Strip non-content elements
    strip_non_content(content_root)

    # 4. Remove images before conversion (useless for agents)
    for img in content_root.find_all("img"):
        img.decompose()

    # 5. Convert to Markdown
    markdown_body = md(
        str(content_root),
        heading_style="ATX",
        bullets="-",
    )

    # 6. Clean up excessive blank lines
    lines = markdown_body.splitlines()
    cleaned = []
    blank_count = 0
    for line in lines:
        if line.strip() == "":
            blank_count += 1
            if blank_count <= 2:
                cleaned.append("")
        else:
            blank_count = 0
            cleaned.append(line)
    markdown_body = "\n".join(cleaned).strip()

    # 7. Build front matter
    if metadata:
        front_matter_lines = ["---"]
        for key, value in metadata.items():
            safe_value = value.replace('"', '\\"')
            front_matter_lines.append(f'{key}: "{safe_value}"')
        front_matter_lines.append("---")
        front_matter = "\n".join(front_matter_lines)
        markdown_body = front_matter + "\n\n" + markdown_body

    return markdown_body, metadata


def count_tokens(text: str) -> int:
    """Estimate token count using tiktoken."""
    try:
        return len(get_encoder().encode(text))
    except Exception:
        # Rough fallback: ~4 chars per token
        return len(text) // 4


# ---------------------------------------------------------------------------
# HTTP client
# ---------------------------------------------------------------------------

def get_http_client() -> httpx.AsyncClient:
    return httpx.AsyncClient(
        timeout=ORIGIN_TIMEOUT,
        follow_redirects=True,
        headers={"User-Agent": "markdown-sidecar/0.1"},
    )


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------

@app.get("/healthz")
async def healthz():
    return {"status": "ok", "service": "markdown-sidecar"}


@app.api_route("/{path:path}", methods=["GET", "HEAD"])
async def convert(request: Request, path: str):
    """
    Fetch the original page from the TYPO3 origin and return Markdown.

    Varnish should route here when Accept contains text/markdown.
    The request path is forwarded to the origin as-is.
    """
    origin_url = urljoin(ORIGIN_BASE_URL.rstrip("/") + "/", path)

    # Forward query string
    if request.url.query:
        origin_url += f"?{request.url.query}"

    logger.info("Converting %s", origin_url)
    start = time.monotonic()

    async with get_http_client() as client:
        try:
            origin_response = await client.get(
                origin_url,
                headers={
                    "Accept": "text/html",
                    "X-Forwarded-For": request.client.host or "127.0.0.1",
                },
            )
        except httpx.RequestError as exc:
            logger.error("Origin request failed: %s", exc)
            raise HTTPException(status_code=502, detail="Origin unreachable")

    if origin_response.status_code >= 400:
        raise HTTPException(
            status_code=origin_response.status_code,
            detail="Origin returned an error",
        )

    content_type = origin_response.headers.get("content-type", "")
    if "text/html" not in content_type:
        # Not HTML – pass through as-is
        return Response(
            content=origin_response.content,
            status_code=origin_response.status_code,
            headers=dict(origin_response.headers),
        )

    html = origin_response.text
    markdown_text, metadata = html_to_markdown(html, page_url=origin_url)
    token_count = count_tokens(markdown_text)
    duration_ms = int((time.monotonic() - start) * 1000)

    logger.info(
        "Converted %s → %d tokens in %d ms",
        origin_url, token_count, duration_ms,
    )

    return PlainTextResponse(
        content=markdown_text,
        status_code=200,
        headers={
            "Content-Type": "text/markdown; charset=utf-8",
            "X-Markdown-Tokens": str(token_count),
            "X-Conversion-Time-Ms": str(duration_ms),
            "Content-Signal": CONTENT_SIGNAL,
            "Vary": "Accept",
            "Cache-Control": "public, max-age=300",
        },
    )

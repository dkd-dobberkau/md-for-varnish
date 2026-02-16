"""Tests for the FastAPI endpoints (healthz + convert)."""

import httpx
import pytest
from fastapi.testclient import TestClient

from app.main import app


@pytest.fixture
def client():
    return TestClient(app)


# ---------------------------------------------------------------------------
# Health check
# ---------------------------------------------------------------------------

class TestHealthz:
    def test_returns_ok(self, client):
        resp = client.get("/healthz")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "ok"
        assert data["service"] == "markdown-sidecar"


# ---------------------------------------------------------------------------
# Convert endpoint
# ---------------------------------------------------------------------------

class TestConvertEndpoint:
    def test_origin_unreachable_returns_502(self, client, monkeypatch):
        """When the origin is down, the sidecar should return 502."""
        monkeypatch.setattr("app.main.ORIGIN_BASE_URL", "http://127.0.0.1:19999")
        resp = client.get("/some-page")
        assert resp.status_code == 502

    def test_successful_conversion(self, client, monkeypatch, sample_html):
        """Mock the origin response and verify markdown conversion."""
        async def mock_get(self, url, **kwargs):
            return httpx.Response(
                status_code=200,
                headers={"content-type": "text/html; charset=utf-8"},
                text=sample_html,
            )

        monkeypatch.setattr(httpx.AsyncClient, "get", mock_get)

        resp = client.get("/test-page")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "text/markdown; charset=utf-8"
        assert "X-Markdown-Tokens" in resp.headers
        assert "X-Conversion-Time-Ms" in resp.headers
        assert resp.headers["Content-Signal"] == "ai-train=yes, search=yes, ai-input=yes"
        assert "# Willkommen" in resp.text

    def test_non_html_passthrough(self, client, monkeypatch):
        """Non-HTML responses should be passed through unchanged."""
        async def mock_get(self, url, **kwargs):
            return httpx.Response(
                status_code=200,
                headers={"content-type": "application/json"},
                content=b'{"key": "value"}',
            )

        monkeypatch.setattr(httpx.AsyncClient, "get", mock_get)

        resp = client.get("/api/data")
        assert resp.status_code == 200
        assert "application/json" in resp.headers["content-type"]

    def test_origin_error_forwarded(self, client, monkeypatch):
        """Origin 4xx/5xx should be forwarded as HTTPException."""
        async def mock_get(self, url, **kwargs):
            return httpx.Response(status_code=404)

        monkeypatch.setattr(httpx.AsyncClient, "get", mock_get)

        resp = client.get("/not-found")
        assert resp.status_code == 404

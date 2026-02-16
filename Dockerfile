FROM python:3.12-slim AS builder

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev

COPY app/ ./app/

FROM python:3.12-slim

RUN useradd -m -u 1000 appuser

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
COPY --chown=appuser:appuser --from=builder /app /app

WORKDIR /app

USER appuser

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/healthz')" || exit 1

CMD ["uv", "run", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "4"]

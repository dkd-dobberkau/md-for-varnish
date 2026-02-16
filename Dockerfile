# ── Build stage ──────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go-sidecar/go.mod go-sidecar/go.sum ./
RUN go mod download

COPY go-sidecar/*.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /markdown-sidecar .

# ── Runtime stage ────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /markdown-sidecar /markdown-sidecar

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s \
    CMD ["/markdown-sidecar", "-healthcheck"]

USER nonroot:nonroot

ENTRYPOINT ["/markdown-sidecar"]

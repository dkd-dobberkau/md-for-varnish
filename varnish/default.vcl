# Markdown for Agents – Varnish VCL Configuration
# =================================================
#
# This VCL implements content negotiation at the caching layer:
#   - Requests with Accept: text/markdown → markdown-sidecar backend
#   - All other requests → TYPO3 origin (default)
#
# Markdown responses are cached separately via Vary: Accept.

vcl 4.1;

# ── Backends ────────────────────────────────────────────────────

backend typo3 {
    .host = "typo3";
    .port = "80";
    .connect_timeout = 5s;
    .first_byte_timeout = 30s;
    .between_bytes_timeout = 10s;
}

backend markdown_sidecar {
    .host = "markdown-sidecar";
    .port = "8000";
    .connect_timeout = 5s;
    .first_byte_timeout = 15s;
    .between_bytes_timeout = 10s;
    .probe = {
        .url = "/healthz";
        .interval = 10s;
        .timeout = 3s;
        .window = 5;
        .threshold = 3;
    }
}


# ── Request handling ────────────────────────────────────────────

sub vcl_recv {
    # Route markdown requests to the sidecar
    if (req.http.Accept ~ "text/markdown") {
        set req.backend_hint = markdown_sidecar;

        # Normalize the Accept header for consistent cache keys
        set req.http.X-Original-Accept = req.http.Accept;
        set req.http.Accept = "text/markdown";

        # Only cache GET and HEAD
        if (req.method != "GET" && req.method != "HEAD") {
            return (pass);
        }

        return (hash);
    }

    # Default: route to TYPO3 origin
    set req.backend_hint = typo3;
}


# ── Cache key: include content type variant ─────────────────────

sub vcl_hash {
    hash_data(req.url);

    if (req.http.host) {
        hash_data(req.http.host);
    } else {
        hash_data(server.ip);
    }

    # Separate cache entries for markdown vs. HTML
    if (req.http.Accept == "text/markdown") {
        hash_data("markdown");
    }

    return (lookup);
}


# ── Response handling ───────────────────────────────────────────

sub vcl_backend_response {
    # Respect Vary: Accept from sidecar
    if (beresp.http.Content-Type ~ "text/markdown") {
        # Cache markdown for 5 minutes (the sidecar sets Cache-Control too)
        set beresp.ttl = 300s;
        set beresp.grace = 60s;

        # Ensure Vary is set so Varnish differentiates HTML vs. Markdown
        set beresp.http.Vary = "Accept";
    }

    # Standard TYPO3 HTML caching
    if (beresp.http.Content-Type ~ "text/html") {
        set beresp.grace = 120s;
    }
}


sub vcl_deliver {
    # Expose cache hit/miss for debugging
    if (obj.hits > 0) {
        set resp.http.X-Cache = "HIT";
        set resp.http.X-Cache-Hits = obj.hits;
    } else {
        set resp.http.X-Cache = "MISS";
    }
}

## Context
- Kvass currently uses Prometheus config packages from 2021 and a custom config representation in `pkg/scrape/promcfg.go`.
- Prometheus 2.55 adds `http_headers` to `HTTPClientConfig`, with support for `values`, `secrets`, and `files`.
- Prometheus rejects attempts to override headers it sets internally.
- Sidecar config injection marshals `config.Config` to YAML and rewrites `<secret>` placeholders for bearer_token and basic_auth passwords.

## Goals / Non-Goals
- Goals: support Prometheus 2.55 `http_headers` across all `HTTPClientConfig` locations; preserve headers through parsing and injection; honor headers during Kvass internal scrapes; keep injected configs functional with real header values; align validation with Prometheus 2.55.
- Non-Goals: redesign the scraping pipeline or add new auth mechanisms beyond `http_headers`.

## Decisions
- Upgrade Prometheus config dependencies (and staging vendored code) to a version that includes `http_headers`.
- Extend `pkg/scrape/promcfg.go` to mirror the Prometheus 2.55 schema for `http_headers`.
- Enforce Prometheus 2.55 reserved header validation during config load with case-insensitive matching.
- Normalize `http_headers.*.files` to absolute paths based on the config file directory; if the config has no file path (API), use a configurable base directory (default `/etc/prometheus/headers`).
- Validate header file readability at config load; read file-based header values per request and trim surrounding whitespace.
- Preserve `http_headers` in injected shard configs for all `HTTPClientConfig` sections, including `secrets` and `files`.
- Sidecar proxy forwards `http_headers` from Prometheus shard scrape requests to targets.
- When legacy `headers` and `http_headers` both exist (remote_write/remote_read), legacy headers take precedence (case-insensitive).
- Ensure secret header values remain redacted in logs and configuration display output.

## Risks / Trade-offs
- Upgrading Prometheus dependencies may require follow-up fixes across config-related code.
- Secret values will be written into injected shard configs; avoid logging generated configs and limit access to injected files.
- Per-request file reads may increase file I/O and require the files to be present and readable in the sidecar environment.

## Migration Plan
- Backwards compatible: configs without `http_headers` remain unchanged.
- Provide tests to ensure existing scrape behavior is unaffected.

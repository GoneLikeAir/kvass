# Change: Support Prometheus 2.55 http_headers

## Why
Prometheus 2.55 introduces `http_headers` on `HTTPClientConfig`, enabling custom headers for scrape jobs and other HTTP clients. Kvass currently uses older Prometheus config packages and custom config structs that do not recognize this field, which can drop headers during parsing or injection and break scrapes that rely on custom headers.

## What Changes
- Support `http_headers` in all `HTTPClientConfig` occurrences (e.g., scrape_configs, http_sd_configs, kubernetes_sd_configs, alerting.alertmanagers, remote_write, remote_read).
- Align with Prometheus 2.55 behavior for header semantics and validation, including case-insensitive reserved header rejection.
- Preserve `http_headers` in injected shard configs, including `values`, `secrets`, and `files` (with absolute paths), so generated configs remain functional.
- Ensure file-based header values are read per request; resolve relative paths against the config file directory, or a configurable base directory (default `/etc/prometheus/headers`) when the config is API-supplied. Configuration load fails if referenced header files are unreadable.
- Require the sidecar proxy to forward `http_headers` from Prometheus shard scrape requests to targets.
- When legacy `headers` and `http_headers` are both set (remote_write/remote_read), legacy `headers` take precedence for overlapping names (case-insensitive).
- Ensure `http_headers` secrets are not exposed via logs or configuration display output.
- Upgrade Prometheus config dependencies (and the vendored Prometheus code) to include `http_headers`.

## Impact
- Affected specs: `configure-scrape-http-headers`
- Affected code: `pkg/scrape/promcfg.go`, `pkg/scrape/scrape.go`, `pkg/sidecar/injector.go`, `pkg/prom/config.go`, `go.mod`, `staging/src/github.com/promethues/prometheus`

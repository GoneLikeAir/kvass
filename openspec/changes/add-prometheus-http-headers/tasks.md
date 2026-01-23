## 1. Implementation
- [x] 1.1 Upgrade Prometheus config dependencies and staging vendored code to include `http_headers` from 2.55.
- [x] 1.2 Add `http_headers` schema support to `HTTPClientConfig` in `pkg/scrape/promcfg.go` (values, secrets, files).
- [x] 1.3 Enforce Prometheus 2.55 reserved header validation with case-insensitive matching and the upstream denylist.
- [x] 1.4 Normalize `http_headers.*.files` to absolute paths during config load; if the config has no file path (API), use a configurable base directory (default `/etc/prometheus/headers`); validate readability at load; read file values per request (trim whitespace).
- [x] 1.5 Apply legacy `headers` precedence for remote_write/remote_read where both legacy headers and `http_headers` are set.
- [x] 1.6 Ensure multi-value header order is `values` then `secrets` then `files`.
- [x] 1.7 Preserve `http_headers` during config injection in `pkg/sidecar/injector.go` across all `HTTPClientConfig` sections, including secret values and absolute file paths.
- [x] 1.8 Ensure sidecar proxy forwards `http_headers` from Prometheus shard scrape requests to targets.
- [x] 1.9 Ensure secret header values are not exposed in logs or configuration display output.
- [x] 1.10 Add tests for config load (values/secrets/files), reserved header rejection, base dir fallback, file readability failure, file refresh per request, legacy headers precedence, multi-value composition order, injection output (secrets and absolute file paths), redaction, and sidecar forwarding.
- [x] 1.11 Update example configs or docs if they show scrape configs.

## 2. Validation
- [x] 2.1 Run `go test ./pkg/scrape/...`
- [x] 2.2 Run `go test ./pkg/sidecar/...`
- [x] 2.3 Run `go test ./pkg/prom/...`

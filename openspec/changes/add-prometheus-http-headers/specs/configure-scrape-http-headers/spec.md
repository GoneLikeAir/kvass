## ADDED Requirements
### Requirement: HTTP headers in HTTPClientConfig
The system SHALL support `http_headers` for every configuration section that embeds `HTTPClientConfig` (for example: scrape_configs, http_sd_configs, kubernetes_sd_configs, alerting.alertmanagers, remote_write, remote_read), consistent with Prometheus 2.55 semantics for `values`, `secrets`, and `files`.

#### Scenario: Load config with http_headers across HTTPClientConfig sections
- **WHEN** a configuration defines `http_headers` in a scrape job and in an alertmanager config
- **THEN** configuration loading succeeds and the header definitions are retained.

#### Scenario: Multi-value header composition
- **WHEN** a header defines `values`, `secrets`, and `files`
- **THEN** all values are added as multiple header values in the outgoing request in this order: `values`, then `secrets`, then `files`.

#### Scenario: Internal scraping uses http_headers
- **WHEN** Kvass performs an internal scrape using a job that defines `http_headers`
- **THEN** the HTTP request includes those header values in addition to existing headers.

### Requirement: Reserved header enforcement
The system SHALL reject configurations that attempt to set headers reserved by Prometheus, using the Prometheus 2.55 reserved header denylist with case-insensitive matching.

#### Scenario: Reject reserved headers (case-insensitive)
- **WHEN** a configuration sets `http_headers` with a reserved header name (for example, `Authorization` or `authorization`)
- **THEN** configuration loading fails with a validation error.

### Requirement: File-based header values
The system SHALL resolve `http_headers.*.files` relative to the configuration file directory; if the config has no file path (API-supplied), the base directory SHALL be configurable (default `/etc/prometheus/headers`). The system SHALL read the files on each request and trim surrounding whitespace from each value.

#### Scenario: Resolve relative file paths
- **WHEN** `http_headers` specifies a relative file path
- **THEN** the value is read from a path relative to the config file directory.

#### Scenario: Resolve relative file paths without config file path
- **WHEN** a configuration is loaded from API without a file path and `http_headers` specifies a relative file path
- **THEN** the value is read from a path under the configured base directory.

#### Scenario: Refresh file-based values per request
- **WHEN** a header file content changes between two scrape requests
- **THEN** the later request uses the updated file content.

#### Scenario: Fail on unreadable header files
- **WHEN** any header file in `http_headers.*.files` is unreadable during config load
- **THEN** configuration loading fails with a validation error.

### Requirement: Legacy headers precedence
The system SHALL apply legacy `headers` (where supported, such as remote_write/remote_read) before `http_headers`, with legacy headers taking precedence on name conflicts using case-insensitive matching.

#### Scenario: Legacy headers override http_headers
- **WHEN** legacy `headers` and `http_headers` both define the same header name
- **THEN** the legacy header value is used.

### Requirement: Sidecar proxy forwards http_headers
The system SHALL forward `http_headers` from Prometheus shard scrape requests through the sidecar proxy to the target.

#### Scenario: Forward headers through sidecar proxy
- **WHEN** a Prometheus shard scrapes via the sidecar proxy with `http_headers` configured
- **THEN** the target receives the same headers.

### Requirement: Injected configs preserve http_headers
The system SHALL preserve functional `http_headers` in injected shard configs, including `values`, `secrets`, and `files` with absolute paths.

#### Scenario: Preserve secret header values in injected config
- **WHEN** a scrape job defines `http_headers` with `secrets`
- **THEN** the injected shard config contains the usable secret values (not `<secret>` placeholders).

#### Scenario: Preserve file-based headers with absolute paths
- **WHEN** a scrape job defines `http_headers` with `files`
- **THEN** the injected shard config contains absolute file paths.

### Requirement: Secret header redaction
The system SHALL NOT expose `http_headers` secret values in logs or configuration display output.

#### Scenario: Redact secret headers from logs and config output
- **WHEN** configuration content is logged or rendered for display
- **THEN** secret header values are redacted.

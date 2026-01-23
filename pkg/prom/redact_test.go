package prom

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactHTTPHeadersSecrets(t *testing.T) {
	r := require.New(t)
	raw := []byte(`global: {}
scrape_configs:
- job_name: test
  http_headers:
    X-Test:
      secrets: ["s1", "s2"]
remote_write:
- url: http://127.0.0.1
  http_headers:
    X-Write:
      secrets:
      - write-secret
alerting:
  alertmanagers:
  - scheme: http
    http_headers:
      X-Alert:
        secrets: ["alert-secret"]
`)

	redacted, err := RedactHTTPHeadersSecrets(raw)
	r.NoError(err)

	output := string(redacted)
	r.NotContains(output, "s1")
	r.NotContains(output, "write-secret")
	r.NotContains(output, "alert-secret")
	r.Contains(output, redactedSecretValue)
	r.GreaterOrEqual(strings.Count(output, redactedSecretValue), 3)
}

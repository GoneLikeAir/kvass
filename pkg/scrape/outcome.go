package scrape

import (
	"mime"
	"strings"
)

const (
	FailOpenReasonUnexpectedResponse = "unexpected_response"
	FailOpenReasonUnsupportedFormat  = "unsupported_format"
	FailOpenReasonParseError         = "parse_error"
	FailOpenReasonInternalPanic      = "internal_panic"
)

// FailOpenReasons is the fixed set of fail-open classifications.
var FailOpenReasons = []string{
	FailOpenReasonUnexpectedResponse,
	FailOpenReasonUnsupportedFormat,
	FailOpenReasonParseError,
	FailOpenReasonInternalPanic,
}

const maxContentTypeLen = 128

// FilterOutcome is the detailed result of one filter/parse attempt.
// FilterAndStat remains the compatibility wrapper over this type.
type FilterOutcome struct {
	Out      []byte
	Series   int64
	BodySize int64
	FailOpen bool
	Reason   string
	Err      error
}

func classifyParseFailure(contentType string) string {
	ct := CanonicalContentType(contentType)
	if strings.Contains(ct, "json") || strings.Contains(ct, "html") {
		return FailOpenReasonUnexpectedResponse
	}
	return FailOpenReasonParseError
}

func isUnsupportedFormat(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "protobuf") || strings.Contains(ct, "delimited")
}

// CanonicalContentType returns a bounded, lowercased media type without parameters.
func CanonicalContentType(contentType string) string {
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		return ""
	}
	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		if i := strings.IndexByte(ct, ';'); i >= 0 {
			ct = strings.TrimSpace(ct[:i])
		}
		media, _, err = mime.ParseMediaType(ct)
		if err != nil || !strings.Contains(media, "/") {
			return "invalid"
		}
		return boundString(strings.ToLower(media), maxContentTypeLen)
	}
	if !strings.Contains(media, "/") {
		return "invalid"
	}
	return boundString(strings.ToLower(media), maxContentTypeLen)
}

func boundString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}

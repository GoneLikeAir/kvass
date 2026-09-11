package scrape

import (
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/relabel"
	"tkestack.io/kvass/pkg/metricdrop"
)

func TestFilterAndStatOutcome_ClassifiesReasons(t *testing.T) {
	drop := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	tests := []struct {
		name        string
		raw         string
		ct          string
		wantReason  string
		wantFail    bool
		wantErrNil  bool
		wantForward bool
	}{
		{
			name:        "protobuf",
			raw:         "not-proto",
			ct:          "application/vnd.google.protobuf",
			wantReason:  FailOpenReasonUnsupportedFormat,
			wantFail:    true,
			wantErrNil:  true,
			wantForward: true,
		},
		{
			name:        "json",
			raw:         `{"error":"login"}`,
			ct:          "application/json; charset=utf-8",
			wantReason:  FailOpenReasonUnexpectedResponse,
			wantFail:    true,
			wantErrNil:  false,
			wantForward: true,
		},
		{
			name:        "html",
			raw:         "<html><body>login</body></html>",
			ct:          "text/html",
			wantReason:  FailOpenReasonUnexpectedResponse,
			wantFail:    true,
			wantErrNil:  false,
			wantForward: true,
		},
		{
			name:        "broken text",
			raw:         "not a prometheus metric line {{{\n",
			ct:          "text/plain",
			wantReason:  FailOpenReasonParseError,
			wantFail:    true,
			wantErrNil:  false,
			wantForward: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.raw)
			o := FilterAndStatOutcome("job", nil, raw, tt.ct, nil, drop)
			if o.FailOpen != tt.wantFail || o.Reason != tt.wantReason {
				t.Fatalf("failOpen=%v reason=%q want failOpen=%v reason=%q err=%v", o.FailOpen, o.Reason, tt.wantFail, tt.wantReason, o.Err)
			}
			if tt.wantErrNil && o.Err != nil {
				t.Fatalf("err=%v", o.Err)
			}
			if !tt.wantErrNil && o.Err == nil {
				t.Fatal("expected parse err for compatibility wrapper")
			}
			if tt.wantForward && string(o.Out) != tt.raw {
				t.Fatalf("must forward original, got %q", o.Out)
			}
			out, _, _, failOpen, err := FilterAndStat("job", nil, raw, tt.ct, nil, drop)
			if failOpen != o.FailOpen || string(out) != string(o.Out) {
				t.Fatalf("wrapper mismatch failOpen=%v out=%q", failOpen, out)
			}
			if (err == nil) != (o.Err == nil) {
				t.Fatalf("wrapper err=%v outcome err=%v", err, o.Err)
			}
		})
	}
}

func TestFilterAndStatOutcome_JSONContentTypeValidTextFilters(t *testing.T) {
	raw := []byte("idle_metric 1\nkeep_metric 2\n")
	drop := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	o := FilterAndStatOutcome("job", nil, raw, "application/json", nil, drop)
	if o.FailOpen || o.Reason != "" || o.Err != nil {
		t.Fatalf("valid text must not fail-open: %+v err=%v", o, o.Err)
	}
	if strings.Contains(string(o.Out), "idle_metric 1") || !strings.Contains(string(o.Out), "keep_metric 2") {
		t.Fatalf("filter contract: %s", o.Out)
	}
}

func TestFilterAndStatOutcome_UnknownContentTypeValidTextFilters(t *testing.T) {
	raw := []byte("idle_metric 1\nkeep_metric 2\n")
	drop := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	o := FilterAndStatOutcome("job", nil, raw, "not-a-real-type/foo", nil, drop)
	if o.FailOpen || o.Err != nil {
		t.Fatalf("unknown content-type with valid text must filter: failOpen=%v err=%v", o.FailOpen, o.Err)
	}
	if strings.Contains(string(o.Out), "idle_metric 1") {
		t.Fatalf("dropped sample still present: %s", o.Out)
	}
}

func TestFilterAndStatOutcome_InnerPanicSafe(t *testing.T) {
	// Invalid relabel input reaches the actual filter recovery boundary.
	rc := []*relabel.Config{nil}
	raw := []byte("keep_metric 1\n")
	drop := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	o := FilterAndStatOutcome("job", nil, raw, "text/plain", rc, drop)
	if !o.FailOpen || o.Reason != FailOpenReasonInternalPanic || o.Err != nil {
		t.Fatalf("inner panic: failOpen=%v reason=%q err=%v", o.FailOpen, o.Reason, o.Err)
	}
	if string(o.Out) != string(raw) {
		t.Fatalf("must forward original on panic")
	}
	out, _, _, failOpen, err := FilterAndStat("job", nil, raw, "text/plain", rc, drop)
	if !failOpen || err != nil || string(out) != string(raw) {
		t.Fatalf("wrapper panic semantics failOpen=%v err=%v", failOpen, err)
	}
}

func TestCanonicalContentType(t *testing.T) {
	got := CanonicalContentType("Application/JSON; charset=utf-8")
	if got != "application/json" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("a", 200) + "/b"
	if len(CanonicalContentType(long)) > maxContentTypeLen {
		t.Fatal("must bound length")
	}
}

func TestCanonicalContentTypeMalformedDoesNotExposePayload(t *testing.T) {
	for _, ct := range []string{"secret-token", "Bearer very-secret-value", "text/plain\r\nAuthorization: secret", "<html>secret</html>"} {
		if got := CanonicalContentType(ct); got != "invalid" {
			t.Fatalf("malformed media type must be replaced, got %q", got)
		}
	}
	if got := CanonicalContentType("application/json; token=\"unterminated-secret"); got != "application/json" {
		t.Fatalf("valid media type with malformed parameters: %q", got)
	}
}

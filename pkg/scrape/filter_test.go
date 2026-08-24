package scrape

import (
	"strings"
	"testing"

	"tkestack.io/kvass/pkg/metricdrop"
)

func TestFilterAndStat_OmitsDroppedSeries(t *testing.T) {
	raw := []byte("# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\nkeep_metric 2\n")
	set := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	out, series, _, failOpen, err := FilterAndStat("job", nil, raw, "text/plain", nil, set)
	if err != nil || failOpen {
		t.Fatalf("err=%v failOpen=%v", err, failOpen)
	}
	body := string(out)
	if strings.Contains(body, "idle_metric 1") {
		t.Fatalf("dropped sample still present: %s", body)
	}
	if !strings.Contains(body, "keep_metric 2") {
		t.Fatalf("kept sample missing: %s", body)
	}
	if series != 1 {
		t.Fatalf("series=%d", series)
	}
}

func TestFilterAndStat_OmitsHelpForFullyDropped(t *testing.T) {
	raw := []byte("# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\n")
	set := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	out, _, _, _, err := FilterAndStat("job", nil, raw, "text/plain", nil, set)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "idle_metric") {
		t.Fatalf("help/type for fully dropped name must be omitted: %s", out)
	}
}

func TestFilterAndStat_PreservesRawSampleBytes(t *testing.T) {
	raw := []byte("keep_metric 2 1234567890\n")
	out, _, _, _, err := FilterAndStat("job", nil, raw, "text/plain", nil, &metricdrop.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "keep_metric 2 1234567890") {
		t.Fatalf("timestamp bytes lost: %s", out)
	}
}

func TestFilterAndStat_ProtobufFailOpen(t *testing.T) {
	raw := []byte("not-really-proto")
	out, _, _, failOpen, err := FilterAndStat("job", nil, raw, "application/vnd.google.protobuf", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !failOpen {
		t.Fatal("expected fail-open")
	}
	if string(out) != string(raw) {
		t.Fatal("must forward original")
	}
}

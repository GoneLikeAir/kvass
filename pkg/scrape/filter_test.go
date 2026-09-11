package scrape

import (
	"fmt"
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
	set := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"other_idle": {}}}
	out, _, _, _, err := FilterAndStat("job", nil, raw, "text/plain", nil, set)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "keep_metric 2 1234567890") {
		t.Fatalf("timestamp bytes lost: %s", out)
	}
}

func TestFilterAndStat_OpenMetricsKeepsEOFAndOriginalValue(t *testing.T) {
	raw := []byte("# TYPE keep_metric gauge\nkeep_metric 2.0\nidle_metric 1\n# EOF\n")
	set := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	out, _, _, failOpen, err := FilterAndStat("job", nil, raw, "application/openmetrics-text; version=0.0.1", nil, set)
	if err != nil || failOpen {
		t.Fatalf("err=%v failOpen=%v", err, failOpen)
	}
	body := string(out)
	if !strings.Contains(body, "# EOF") {
		t.Fatalf("openmetrics rewrite must keep # EOF: %s", body)
	}
	if !strings.Contains(body, "keep_metric 2.0") {
		t.Fatalf("original sample bytes lost: %s", body)
	}
	if strings.Contains(body, "idle_metric 1") {
		t.Fatalf("dropped sample still present: %s", body)
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

func TestFilterAndStat_PreservesRepeatedSeriesSamples(t *testing.T) {
	raw := []byte("keep_metric{instance=\"a\"} 1.0 1000\nkeep_metric{instance=\"a\"} 2.0 2000\n")
	set := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	out, series, _, failOpen, err := FilterAndStat("job", nil, raw, "text/plain", nil, set)
	if err != nil || failOpen || series != 2 || string(out) != string(raw) {
		t.Fatalf("sample values/timestamps changed: out=%q series=%d failOpen=%v err=%v", out, series, failOpen, err)
	}
}

func BenchmarkFilterAndStat_LargeScrape(b *testing.B) {
	for _, count := range []int{1000, 10000, 50000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var input strings.Builder
			for i := 0; i < count; i++ {
				fmt.Fprintf(&input, "keep_metric{instance=\"target-%d\",description=\"abcdefghijklmnopqrstuvwxyz0123456789\"} 1.0\n", i)
			}
			raw := []byte(input.String())
			set := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				_, _, _, _, err := FilterAndStat("job", nil, raw, "text/plain", nil, set)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

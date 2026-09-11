package sidecar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestFailOpenLogsRateIncludesSummaries(t *testing.T) {
	clk := &testClock{t: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)}
	var buf bytes.Buffer
	lg := logrus.New()
	lg.SetOutput(&buf)
	lg.SetFormatter(&logrus.JSONFormatter{})
	p := newObservabilityProxy(t, enabledDropSnap(), lg, nil)
	p.now = clk.Now
	p.logLimiter.maxEntries = 2
	p.logLimiter.rateMax = 1
	emit := func(id string) {
		p.emitFailOpenLog("parse_error", "job", id, "http://example.test", "text/plain", true, "gen", clk.Now())
	}
	emit("a")
	emit("a")
	// Each new target evicts a pending, suppressed event. Eviction must not
	// create an unbounded stream of summary logs outside the rate budget.
	for i := 0; i < 20; i++ {
		emit(fmt.Sprint(i))
	}
	if got := bytes.Count(buf.Bytes(), []byte{'\n'}); got != 1 {
		t.Fatalf("rate permits 1 log, got %d: %s", got, buf.String())
	}
	clk.Advance(time.Minute)
	buf.Reset()
	emit("19")
	if got := bytes.Count(buf.Bytes(), []byte{'\n'}); got != 1 {
		t.Fatalf("summary and current must share one log, got %d", got)
	}
	var event map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &event); err != nil {
		t.Fatal(err)
	}
	if event["targetId"] != "19" || event["suppressed"] != float64(1) {
		t.Fatalf("current target suppressed attribution: %v", event)
	}
	if event["evictedSuppressed"] != float64(19) {
		t.Fatalf("evicted count must be explicitly process-level, got %v", event)
	}
}

func TestFailOpenLimiterPreservesPendingAcrossRateRejection(t *testing.T) {
	clk := &testClock{t: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)}
	l := newFailOpenLogLimiter(clk.Now)
	l.rateMax = 1
	if !l.Observe("a").LogCurrent {
		t.Fatal("first log")
	}
	l.Observe("a")
	l.Observe("a")
	clk.Advance(time.Minute)
	l.Observe("b") // Consume the new second's budget.
	if l.Observe("a").LogCurrent {
		t.Fatal("rate cap")
	}
	clk.Advance(time.Second)
	got := l.Observe("a")
	if !got.LogCurrent || got.SummarySuppressed != 3 {
		t.Fatalf("must retain 2 old + 1 rate-suppressed and retry promptly: %+v", got)
	}
}

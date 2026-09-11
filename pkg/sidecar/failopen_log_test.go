package sidecar

import (
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestFailOpenLogLimiter_WindowAndEvictAndRate(t *testing.T) {
	clk := &testClock{t: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)}
	l := newFailOpenLogLimiter(clk.Now)
	l.maxEntries = 2
	l.window = time.Minute
	l.rateMax = 100
	l.ratePeriod = time.Hour

	first := l.Observe("job\x00t1\x00parse_error")
	if !first.LogCurrent {
		t.Fatal("first event must log")
	}
	second := l.Observe("job\x00t1\x00parse_error")
	if second.LogCurrent {
		t.Fatal("duplicate inside window must suppress")
	}
	clk.Advance(time.Minute + time.Second)
	after := l.Observe("job\x00t1\x00parse_error")
	if !after.LogCurrent || after.SummarySuppressed != 1 {
		t.Fatalf("window expiry should log summary+event: %+v", after)
	}

	_ = l.Observe("job\x00t2\x00parse_error")
	evict := l.Observe("job\x00t3\x00parse_error")
	if !evict.LogCurrent {
		t.Fatalf("new key after eviction must log: %+v", evict)
	}
	if l.len() > 2 {
		t.Fatalf("cache len=%d want <=2", l.len())
	}

	l.rateMax = 1
	l.rateCount = 0
	l.rateWindowStart = time.Time{}
	clk.Advance(time.Hour)
	a := l.Observe("job\x00t4\x00parse_error")
	if !a.LogCurrent {
		t.Fatalf("rate first should log: %+v", a)
	}
	b := l.Observe("job\x00t5\x00parse_error")
	if b.LogCurrent {
		t.Fatalf("rate cap must skip: %+v", b)
	}
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestSanitizeScrapeAddr(t *testing.T) {
	u := mustParseURL("http://user:s3cret@example.com:9090/hidden-path/metrics?token=leak#frag")
	got := sanitizeScrapeAddr(u)
	if strings.Contains(got, "s3cret") || strings.Contains(got, "hidden-path") || strings.Contains(got, "leak") || strings.Contains(got, "user") {
		t.Fatalf("addr not sanitized: %q", got)
	}
	if !strings.HasPrefix(got, "http://example.com:9090") {
		t.Fatalf("got %q", got)
	}
}

func TestBoundLogField(t *testing.T) {
	s := strings.Repeat("x", failOpenLogMaxField+20)
	if len(boundLogField(s)) != failOpenLogMaxField {
		t.Fatal("field must be bounded")
	}
}

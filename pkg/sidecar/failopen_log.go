package sidecar

import (
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	failOpenLogCacheSize  = 256
	failOpenLogWindow     = time.Minute
	failOpenLogRateMax    = 10
	failOpenLogRatePeriod = time.Second
	failOpenLogMaxField   = 128
)

type failOpenLogEntry struct {
	lastLog    time.Time
	suppressed int
}

type failOpenLogLimiter struct {
	mu                sync.Mutex
	now               func() time.Time
	cache             map[string]*failOpenLogEntry
	order             []string
	window            time.Duration
	maxEntries        int
	rateMax           int
	ratePeriod        time.Duration
	rateCount         int
	rateWindowStart   time.Time
	evictedSuppressed int
}

type failOpenLogDecision struct {
	LogCurrent        bool
	SummarySuppressed int
	EvictedSuppressed int
}

func newFailOpenLogLimiter(now func() time.Time) *failOpenLogLimiter {
	if now == nil {
		now = time.Now
	}
	return &failOpenLogLimiter{
		now:        now,
		cache:      make(map[string]*failOpenLogEntry, failOpenLogCacheSize),
		window:     failOpenLogWindow,
		maxEntries: failOpenLogCacheSize,
		rateMax:    failOpenLogRateMax,
		ratePeriod: failOpenLogRatePeriod,
	}
}

func failOpenLogKey(job, targetID, reason string) string {
	return job + "\x00" + targetID + "\x00" + reason
}

func (l *failOpenLogLimiter) Observe(key string) failOpenLogDecision {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.cache[key]
	if !ok {
		// FIFO eviction bounds memory. Preserve pending counts as a process-wide
		// aggregate, never attach an evicted target's counts to the new target.
		if len(l.cache) >= l.maxEntries && len(l.order) > 0 {
			evict := l.order[0]
			l.evictedSuppressed += l.cache[evict].suppressed
			delete(l.cache, evict)
			l.order = l.order[1:]
		}
		e = &failOpenLogEntry{}
		l.cache[key] = e
		l.order = append(l.order, key)
	}
	if !e.lastLog.IsZero() && now.Sub(e.lastLog) < l.window {
		e.suppressed++
		return failOpenLogDecision{}
	}
	// Do not advance the per-key window or discard pending counts when the
	// process budget is exhausted: retry on a later event after it refills.
	if !l.allowRateLocked(now) {
		e.suppressed++
		return failOpenLogDecision{}
	}
	dec := failOpenLogDecision{LogCurrent: true, SummarySuppressed: e.suppressed, EvictedSuppressed: l.evictedSuppressed}
	e.lastLog = now
	e.suppressed = 0
	l.evictedSuppressed = 0
	return dec
}

func (l *failOpenLogLimiter) allowRateLocked(now time.Time) bool {
	if l.rateWindowStart.IsZero() || now.Sub(l.rateWindowStart) >= l.ratePeriod {
		l.rateWindowStart = now
		l.rateCount = 0
	}
	if l.rateCount >= l.rateMax {
		return false
	}
	l.rateCount++
	return true
}

func (l *failOpenLogLimiter) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.cache)
}

func sanitizeScrapeAddr(u *url.URL) string {
	if u == nil {
		return ""
	}
	out := url.URL{Scheme: u.Scheme, Host: u.Host}
	return boundLogField(out.String())
}

func boundLogField(s string) string {
	if len(s) <= failOpenLogMaxField {
		return s
	}
	return s[:failOpenLogMaxField]
}

func (p *Proxy) emitFailOpenLog(reason, job, targetID, addr, contentType string, filtering bool, gen string, now time.Time) {
	key := failOpenLogKey(job, targetID, reason)
	dec := p.logLimiter.Observe(key)
	fields := logrus.Fields{
		"job":             job,
		"targetId":        targetID,
		"address":         addr,
		"contentType":     contentType,
		"reason":          reason,
		"filteringActive": filtering,
		"generation":      gen,
		"eventTime":       now.UTC().Format(time.RFC3339Nano),
	}
	if dec.LogCurrent {
		fields["suppressed"] = dec.SummarySuppressed
		fields["evictedSuppressed"] = dec.EvictedSuppressed
		p.log.WithFields(fields).Warn("metric drop fail-open")
	}
}

package sidecar

import (
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tkestack.io/kvass/pkg/metricdrop"
	"tkestack.io/kvass/pkg/scrape"
	"tkestack.io/kvass/pkg/shard"
)

type failOpenSeriesKey struct {
	reason    string
	filtering bool
}

type failOpenState struct {
	mu                sync.RWMutex
	total             uint64
	byReason          map[string]uint64
	byReasonFiltering map[failOpenSeriesKey]uint64
	lastError         string
	lastFailure       shard.DropSetLastFailure
}

var failOpenMetricDesc = prometheus.NewDesc(
	"kvass_metric_drop_fail_open_total",
	"Metric-drop fail-open events by reason and whether filtering was active.",
	[]string{"reason", "filtering_active"},
	nil,
)

func newFailOpenState() *failOpenState {
	return &failOpenState{
		byReason:          make(map[string]uint64, len(scrape.FailOpenReasons)),
		byReasonFiltering: make(map[failOpenSeriesKey]uint64, len(scrape.FailOpenReasons)*2),
	}
}

func dropSetFilteringActive(dropSet *metricdrop.Snapshot) bool {
	return dropSet != nil && dropSet.Enabled && len(dropSet.Names) > 0
}

func (p *Proxy) recordFailOpen(reason, job, hashStr, contentType string, realURL url.URL, dropSet *metricdrop.Snapshot) {
	if reason == "" {
		reason = scrape.FailOpenReasonParseError
	}
	filtering := dropSetFilteringActive(dropSet)
	gen := ""
	if dropSet != nil {
		gen = dropSet.Generation
	}
	now := p.now()
	addr := sanitizeScrapeAddr(&realURL)
	ct := scrape.CanonicalContentType(contentType)
	job = boundLogField(job)
	hashStr = boundLogField(hashStr)
	gen = boundLogField(gen)

	p.failOpen.mu.Lock()
	p.failOpen.total++
	p.failOpen.byReason[reason]++
	p.failOpen.byReasonFiltering[failOpenSeriesKey{reason: reason, filtering: filtering}]++
	p.failOpen.lastError = "fail-open: " + reason
	p.failOpen.lastFailure = shard.DropSetLastFailure{
		Reason:          reason,
		Time:            now,
		Job:             job,
		TargetID:        hashStr,
		Address:         addr,
		ContentType:     ct,
		FilteringActive: filtering,
		Generation:      gen,
	}
	p.failOpen.mu.Unlock()

	p.emitFailOpenLog(reason, job, hashStr, addr, ct, filtering, gen, now)
}

func (p *Proxy) FailOpenSnapshot() (total uint64, byReason map[string]uint64, last shard.DropSetLastFailure, lastError string) {
	p.failOpen.mu.RLock()
	defer p.failOpen.mu.RUnlock()
	byReason = make(map[string]uint64, len(scrape.FailOpenReasons))
	for _, reason := range scrape.FailOpenReasons {
		byReason[reason] = p.failOpen.byReason[reason]
	}
	return p.failOpen.total, byReason, p.failOpen.lastFailure, p.failOpen.lastError
}

func (p *Proxy) failOpenSeries(reason string, filtering bool) uint64 {
	p.failOpen.mu.RLock()
	defer p.failOpen.mu.RUnlock()
	return p.failOpen.byReasonFiltering[failOpenSeriesKey{reason: reason, filtering: filtering}]
}

func (p *Proxy) MetricsHandler() http.Handler {
	return p.metricsHandler
}

type failOpenCollector struct {
	p *Proxy
}

func (c *failOpenCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- failOpenMetricDesc
}

func (c *failOpenCollector) Collect(ch chan<- prometheus.Metric) {
	for _, reason := range scrape.FailOpenReasons {
		for _, active := range []bool{true, false} {
			v := c.p.failOpenSeries(reason, active)
			ch <- prometheus.MustNewConstMetric(failOpenMetricDesc, prometheus.CounterValue, float64(v), reason, strconv.FormatBool(active))
		}
	}
}

func newFailOpenMetrics(p *Proxy) (prometheus.Gatherer, http.Handler) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(&failOpenCollector{p: p})
	return reg, promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

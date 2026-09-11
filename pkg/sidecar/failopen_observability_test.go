package sidecar

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/sirupsen/logrus"
	"tkestack.io/kvass/pkg/metricdrop"
	"tkestack.io/kvass/pkg/prom"
	"tkestack.io/kvass/pkg/scrape"
	"tkestack.io/kvass/pkg/target"
)

const (
	secretUser     = "user"
	secretPassword = "s3cret-token"
	secretQuery    = "leak-query"
	secretPath     = "/hidden-path/metrics"
	panicSecret    = "panic-password=supersecret"
)

func enabledDropSnap() *metricdrop.Snapshot {
	return &metricdrop.Snapshot{
		Enabled:    true,
		Generation: "gen-9",
		Hash:       "hash-9",
		Names:      map[string]struct{}{"idle_metric": {}},
	}
}

func proxyJob() *config.ScrapeConfig {
	return &config.ScrapeConfig{JobName: "job1", ScrapeTimeout: model.Duration(time.Second)}
}

func newObservabilityProxy(t *testing.T, snap *metricdrop.Snapshot, log logrus.FieldLogger, getDrop func() *metricdrop.Snapshot) *Proxy {
	t.Helper()
	if log == nil {
		lg := logrus.New()
		lg.SetOutput(io.Discard)
		log = lg
	}
	if getDrop == nil {
		getDrop = func() *metricdrop.Snapshot { return snap }
	}
	return NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: proxyJob(), Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		getDrop,
		log,
	)
}

func newTargetServer(t *testing.T, contentType, body string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func doProxy(t *testing.T, p *Proxy, rawURL string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, rawURL, nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	return w.Result()
}

func proxyURL(ts *httptest.Server, extra map[string]string) string {
	u, _ := url.Parse(ts.URL + "/metrics")
	q := u.Query()
	q.Set("_jobName", "job1")
	q.Set("_scheme", "http")
	q.Set("_hash", "1")
	for k, v := range extra {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func runtimeInfoMap(t *testing.T, svc *Service) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shard/runtimeinfo/", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("runtimeinfo status=%d body=%s", w.Code, w.Body.String())
	}
	var wrapped struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("runtimeinfo json: %v body=%s", err, w.Body.String())
	}
	if wrapped.Data == nil {
		t.Fatalf("runtimeinfo missing data: %s", w.Body.String())
	}
	return wrapped.Data
}

func newServiceWithProxy(t *testing.T, promURL string, dropSet *metricdrop.Set, p *Proxy) *Service {
	t.Helper()
	svc := NewService("", promURL, func() (int64, error) { return 0, nil },
		prom.NewConfigManager(), NewTargetsManager(t.TempDir(), logrus.New()), logrus.New())
	svc.SetDropRuntime(dropSet, p)
	return svc
}

func TestProxy_JSONUnexpectedResponse_ClassifiedOnce(t *testing.T) {
	body := `{"error":"login page","Authorization":"Bearer ` + secretPassword + `"}`
	ts := newTargetServer(t, "application/json; charset=utf-8", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("fail-open must forward original bytes, got %q", got)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(strings.ToLower(ct), "json") {
		t.Fatalf("content-type changed: %q", ct)
	}
	if p.FailOpenCount() != 1 {
		t.Fatalf("fail-open count=%d want 1", p.FailOpenCount())
	}

	svc := newServiceWithProxy(t, ts.URL, nil, p)
	info := runtimeInfoMap(t, svc)
	byReason, _ := info["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason == nil {
		t.Fatalf("missing dropSetFailOpenByReason: %v", info)
	}
	if byReason["unexpected_response"] != float64(1) {
		t.Fatalf("unexpected_response=%v info=%v", byReason["unexpected_response"], info)
	}
	last, _ := info["dropSetLastFailure"].(map[string]interface{})
	if last == nil {
		t.Fatalf("missing dropSetLastFailure: %v", info)
	}
	if last["reason"] != "unexpected_response" {
		t.Fatalf("last reason=%v", last["reason"])
	}
	if last["job"] != "job1" {
		t.Fatalf("last job=%v", last["job"])
	}
	if last["targetId"] != "1" && last["targetId"] != float64(1) {
		t.Fatalf("last targetId=%v", last["targetId"])
	}
	if last["time"] == nil || last["time"] == "" {
		t.Fatal("lastFailure time must be set")
	}
	if last["filteringActive"] != true {
		t.Fatalf("filteringActive=%v", last["filteringActive"])
	}
	if last["generation"] != "gen-9" {
		t.Fatalf("generation=%v", last["generation"])
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/shard/metrics/", nil)
	metricsW := httptest.NewRecorder()
	svc.ServeHTTP(metricsW, metricsReq)
	metricsBody := metricsW.Body.String()
	if !strings.Contains(metricsBody, "kvass_metric_drop_fail_open_total") {
		t.Fatalf("new metrics route missing counter: status=%d body=%s", metricsW.Code, metricsBody)
	}
	if !strings.Contains(metricsBody, `reason="unexpected_response"`) || !strings.Contains(metricsBody, `filtering_active="true"`) {
		t.Fatalf("counter labels missing: %s", metricsBody)
	}
}

func TestProxy_ProtobufFailOpenWhenFilteringOff(t *testing.T) {
	body := "not-really-proto"
	ts := newTargetServer(t, "application/vnd.google.protobuf", body)
	p := newObservabilityProxy(t, nil, nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("must forward original, got %q", got)
	}
	if p.FailOpenCount() != 1 {
		t.Fatalf("filtering-off protobuf must still count, got %d", p.FailOpenCount())
	}
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	info := runtimeInfoMap(t, svc)
	byReason, _ := info["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason["unsupported_format"] != float64(1) {
		t.Fatalf("unsupported_format=%v info=%v", byReason["unsupported_format"], info)
	}
	last, _ := info["dropSetLastFailure"].(map[string]interface{})
	if last["filteringActive"] != false {
		t.Fatalf("filteringActive=%v", last["filteringActive"])
	}
	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/shard/metrics/", nil)
	metricsW := httptest.NewRecorder()
	svc.ServeHTTP(metricsW, metricsReq)
	if !strings.Contains(metricsW.Body.String(), `filtering_active="false"`) {
		t.Fatalf("expected filtering_active=false counter: %s", metricsW.Body.String())
	}
}

func TestProxy_ProtobufUnsupportedFormat(t *testing.T) {
	body := "not-really-proto"
	ts := newTargetServer(t, "application/vnd.google.protobuf", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("protobuf fail-open must forward original, got %q", got)
	}
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	info := runtimeInfoMap(t, svc)
	byReason, _ := info["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason["unsupported_format"] != float64(1) {
		t.Fatalf("unsupported_format=%v info=%v", byReason["unsupported_format"], info)
	}
}

func TestProxy_BrokenTextParseError(t *testing.T) {
	body := "not a prometheus metric line {{{ secret=" + secretPassword + "\n"
	ts := newTargetServer(t, "text/plain; version=0.0.4", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("parse fail-open must forward original, got %q", got)
	}
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	info := runtimeInfoMap(t, svc)
	byReason, _ := info["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason["parse_error"] != float64(1) {
		t.Fatalf("parse_error=%v info=%v", byReason["parse_error"], info)
	}
}

func TestProxy_JSONContentTypeValidTextStillFilters(t *testing.T) {
	body := "# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\nkeep_metric 2\n"
	ts := newTargetServer(t, "application/json", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(got), "idle_metric 1") {
		t.Fatalf("valid text with json content-type must still filter: %s", got)
	}
	if !strings.Contains(string(got), "keep_metric 2") {
		t.Fatalf("kept sample missing: %s", got)
	}
	if p.FailOpenCount() != 0 {
		t.Fatalf("must not fail-open, count=%d", p.FailOpenCount())
	}
}

func TestProxy_HTMLUnexpectedResponse(t *testing.T) {
	body := "<html><body>login</body></html>"
	ts := newTargetServer(t, "text/html", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("html fail-open must forward original, got %q", got)
	}
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	info := runtimeInfoMap(t, svc)
	byReason, _ := info["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason["unexpected_response"] != float64(1) {
		t.Fatalf("html unexpected_response=%v info=%v", byReason["unexpected_response"], info)
	}
}

func TestService_OldMetricsStillReverseProxied(t *testing.T) {
	promCalled := false
	tProm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" || r.URL.Path == "/metrics/" {
			promCalled = true
			_, _ = w.Write([]byte("prometheus_build_info 1\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tProm.Close()
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	svc := newServiceWithProxy(t, tProm.URL, nil, p)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)
	if !promCalled {
		t.Fatal("existing /metrics must still reverse-proxy Prometheus")
	}
	if !strings.Contains(w.Body.String(), "prometheus_build_info") {
		t.Fatalf("proxied /metrics body=%s", w.Body.String())
	}
}

func TestService_DropSetLoadErrorNotCoveredByProxy(t *testing.T) {
	body := `{"error":"not metrics"}`
	ts := newTargetServer(t, "application/json", body)
	dir := t.TempDir()
	dropDir := dir + "/drop"
	if err := os.MkdirAll(dropDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(dropDir+"/"+metricdrop.FileEnabled, []byte("true\n"), 0644)
	_ = os.WriteFile(dropDir+"/"+metricdrop.FileGeneration, []byte("g1\n"), 0644)
	gz, err := metricdrop.GzipNames([]string{"idle_metric"})
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(dropDir+"/"+metricdrop.FileNamesGZ, gz, 0644)
	set := metricdrop.NewSet(dropDir, dir+"/store.json")
	set.Reload()
	_ = os.WriteFile(dropDir+"/"+metricdrop.FileNamesGZ, []byte("not-gzip"), 0644)
	set.Reload()
	if set.Load().LastError == "" {
		t.Fatal("setup: expected load error")
	}
	loadErr := set.Load().LastError
	p := NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: proxyJob(), Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		func() *metricdrop.Snapshot { return set.Load() },
		logrus.New(),
	)
	_ = doProxy(t, p, proxyURL(ts, nil))
	svc := newServiceWithProxy(t, ts.URL, set, p)
	info := runtimeInfoMap(t, svc)
	if info["dropSetLoadError"] != loadErr {
		t.Fatalf("dropSetLoadError=%v want %q info=%v", info["dropSetLoadError"], loadErr, info)
	}
}

func TestProxy_SecretURLAndPanicNotLogged(t *testing.T) {
	body := `{"token":"` + secretPassword + `"}`
	ts := newTargetServer(t, "application/json", body)
	var logBuf bytes.Buffer
	lg := logrus.New()
	lg.SetOutput(&logBuf)
	lg.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableColors: true})

	p := newObservabilityProxy(t, enabledDropSnap(), lg, nil)
	u, _ := url.Parse(ts.URL)
	u.User = url.UserPassword(secretUser, secretPassword)
	u.Path = secretPath
	q := u.Query()
	q.Set("_jobName", "job1")
	q.Set("_scheme", "http")
	q.Set("_hash", "1")
	q.Set("token", secretQuery)
	u.RawQuery = q.Encode()
	resp := doProxy(t, p, u.String())
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("body changed: %q", got)
	}

	logs := logBuf.String()
	for _, secret := range []string{secretPassword, secretQuery, secretUser + ":", secretPath, "Authorization", "Bearer"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log leaked %q: %s", secret, logs)
		}
	}
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	info := runtimeInfoMap(t, svc)
	raw, _ := json.Marshal(info)
	if strings.Contains(string(raw), secretPassword) {
		t.Fatalf("runtimeinfo leaked secret: %s", raw)
	}
	last, _ := info["dropSetLastFailure"].(map[string]interface{})
	if last == nil {
		t.Fatal("missing lastFailure")
	}
	addr, _ := last["address"].(string)
	if strings.Contains(addr, secretPassword) || strings.Contains(addr, secretPath) || strings.Contains(addr, secretQuery) {
		t.Fatalf("address not sanitized: %q", addr)
	}

	logBuf.Reset()
	panicProxy := NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: proxyJob(), Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		func() *metricdrop.Snapshot { panic(panicSecret) },
		lg,
	)
	_ = doProxy(t, panicProxy, proxyURL(ts, nil))
	if panicProxy.FailOpenCount() != 1 {
		t.Fatalf("outer panic count=%d", panicProxy.FailOpenCount())
	}
	if strings.Contains(logBuf.String(), panicSecret) {
		t.Fatalf("panic payload leaked: %s", logBuf.String())
	}
	if strings.Contains(panicProxy.LastError(), panicSecret) {
		t.Fatalf("LastError leaked panic: %q", panicProxy.LastError())
	}
	svc2 := newServiceWithProxy(t, ts.URL, nil, panicProxy)
	info2 := runtimeInfoMap(t, svc2)
	byReason, _ := info2["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason["internal_panic"] != float64(1) {
		t.Fatalf("internal_panic=%v info=%v", byReason["internal_panic"], info2)
	}
}

func TestProxy_NormalFilterDoesNotCountFailOpen(t *testing.T) {
	body := "# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\nkeep_metric 2\n"
	ts := newTargetServer(t, "text/plain", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	resp := doProxy(t, p, proxyURL(ts, nil))
	got, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(got), "idle_metric 1") || !strings.Contains(string(got), "keep_metric 2") {
		t.Fatalf("filter contract broken: %s", got)
	}
	if p.FailOpenCount() != 0 {
		t.Fatalf("count=%d", p.FailOpenCount())
	}
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/shard/metrics/", nil)
	metricsW := httptest.NewRecorder()
	svc.ServeHTTP(metricsW, metricsReq)
	if strings.Contains(metricsW.Body.String(), `kvass_metric_drop_fail_open_total{`) &&
		strings.Contains(metricsW.Body.String(), "} 1") {
		t.Fatalf("normal filter must not increment counter: %s", metricsW.Body.String())
	}
}

func TestProxy_ConcurrentFailOpenAndReadonly(t *testing.T) {
	body := `{"err":"x"}`
	ts := newTargetServer(t, "application/json", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	svc := newServiceWithProxy(t, ts.URL, nil, p)
	var wg sync.WaitGroup
	const n = 40
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = doProxy(t, p, proxyURL(ts, nil))
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.FailOpenCount()
			_ = p.LastError()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/shard/runtimeinfo/", nil)
			w := httptest.NewRecorder()
			svc.ServeHTTP(w, req)
			mreq := httptest.NewRequest(http.MethodGet, "/api/v1/shard/metrics/", nil)
			mw := httptest.NewRecorder()
			svc.ServeHTTP(mw, mreq)
		}()
	}
	wg.Wait()
	if p.FailOpenCount() != n {
		t.Fatalf("count=%d want %d", p.FailOpenCount(), n)
	}
	info := runtimeInfoMap(t, svc)
	byReason, _ := info["dropSetFailOpenByReason"].(map[string]interface{})
	if byReason["unexpected_response"] != float64(n) {
		t.Fatalf("byReason=%v want %d", byReason, n)
	}
}

func TestProxy_InnerPanicFallbackRecordedOnce(t *testing.T) {
	const body = "keep_metric 1\n"
	ts := newTargetServer(t, "text/plain", body)
	p := newObservabilityProxy(t, enabledDropSnap(), nil, nil)
	p.getJob = func(string) *scrape.JobInfo {
		cfg := proxyJob()
		cfg.MetricRelabelConfigs = []*relabel.Config{nil}
		return &scrape.JobInfo{Config: cfg, Cli: http.DefaultClient}
	}
	resp := doProxy(t, p, proxyURL(ts, nil))
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK || string(got) != body || resp.Header.Get("Content-Type") != "text/plain" {
		t.Fatalf("panic fallback: status=%d body=%q err=%v", resp.StatusCode, got, err)
	}
	total, reasons, last, _ := p.FailOpenSnapshot()
	if total != 1 || reasons["internal_panic"] != 1 || last.Reason != "internal_panic" || !last.FilteringActive {
		t.Fatalf("inner panic recorded once: total=%d reasons=%v last=%+v", total, reasons, last)
	}
}

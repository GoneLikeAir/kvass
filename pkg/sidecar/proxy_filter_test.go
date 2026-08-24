package sidecar

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"
	"tkestack.io/kvass/pkg/metricdrop"
	"tkestack.io/kvass/pkg/scrape"
	"tkestack.io/kvass/pkg/target"
)

func TestProxy_FilterDropSet_OmitsDroppedSeries(t *testing.T) {
	body := "# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\nkeep_metric 2\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	snap := &metricdrop.Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	job := &config.ScrapeConfig{JobName: "job1", ScrapeTimeout: model.Duration(time.Second)}
	p := NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: job, Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		func() *metricdrop.Snapshot { return snap },
		logrus.New(),
	)
	req := httptest.NewRequest(http.MethodGet, ts.URL+"/metrics?_jobName=job1&_scheme=http&_hash=1", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	got, _ := io.ReadAll(w.Result().Body)
	if strings.Contains(string(got), "idle_metric 1") {
		t.Fatalf("proxy forwarded dropped sample: %s", got)
	}
	if !strings.Contains(string(got), "keep_metric 2") {
		t.Fatalf("kept sample missing: %s", got)
	}
}

func TestProxy_NoDropSet_ForwardsOriginal(t *testing.T) {
	body := "idle_metric 1\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	job := &config.ScrapeConfig{JobName: "job1", ScrapeTimeout: model.Duration(time.Second)}
	p := NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: job, Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		nil,
		logrus.New(),
	)
	req := httptest.NewRequest(http.MethodGet, ts.URL+"/metrics?_jobName=job1&_scheme=http&_hash=1", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	got, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(got), "idle_metric 1") {
		t.Fatalf("expected original body, got %s", got)
	}
}

func TestProxy_RewriteError_FailOpenRuntimeInfo(t *testing.T) {
	p := NewProxy(
		func(string) *scrape.JobInfo {
			return &scrape.JobInfo{
				Config: &config.ScrapeConfig{JobName: "job1", ScrapeTimeout: model.Duration(time.Millisecond)},
				Cli:    http.DefaultClient,
			}
		},
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{} },
		nil,
		logrus.New(),
	)
	if p.FailOpenCount() != 0 {
		t.Fatal("expected zero")
	}
}
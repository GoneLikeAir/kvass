package sidecar

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"
	"tkestack.io/kvass/pkg/metricdrop"
	"tkestack.io/kvass/pkg/prom"
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
	dumpEvidence(t, "g1-proxy-body.txt", string(got))
}

func dumpEvidence(t *testing.T, name, body string) {
	t.Helper()
	dir := os.Getenv("LAND_EVIDENCE")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/"+name, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestProxy_NoDropSet_RuntimeInfoDisabled(t *testing.T) {
	body := "idle_metric 1\nkeep_metric 2\n"
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
	if string(got) != body && !strings.Contains(string(got), "idle_metric 1") {
		t.Fatalf("unfiltered body mismatch: %s", got)
	}
	svc := NewService("", ts.URL, func() (int64, error) { return 0, nil },
		prom.NewConfigManager(), NewTargetsManager(t.TempDir(), logrus.New()), logrus.New())
	svc.SetDropRuntime(metricdrop.NewSet("", ""), p)
	rtReq := httptest.NewRequest(http.MethodGet, "/api/v1/shard/runtimeinfo/", nil)
	rtW := httptest.NewRecorder()
	svc.ginEngine.ServeHTTP(rtW, rtReq)
	if !strings.Contains(rtW.Body.String(), `"dropSetEnabled":false`) {
		t.Fatalf("expected dropSetEnabled=false, got %s", rtW.Body.String())
	}
	dumpEvidence(t, "g5-runtimeinfo.json", rtW.Body.String())
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
	body := "idle_metric 1\nkeep_metric 2\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.google.protobuf")
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
	if string(got) != body {
		t.Fatalf("fail-open must forward original body, got %s", got)
	}
	if p.FailOpenCount() == 0 {
		t.Fatal("expected fail-open count")
	}
	if p.LastError() == "" {
		t.Fatal("expected last error")
	}

	svc := NewService("", ts.URL, func() (int64, error) { return 0, nil },
		prom.NewConfigManager(), NewTargetsManager(t.TempDir(), logrus.New()), logrus.New())
	svc.SetDropRuntime(nil, p)
	rtReq := httptest.NewRequest(http.MethodGet, "/api/v1/shard/runtimeinfo/", nil)
	rtW := httptest.NewRecorder()
	svc.ginEngine.ServeHTTP(rtW, rtReq)
	if !strings.Contains(rtW.Body.String(), `"dropSetFailOpen"`) {
		t.Fatalf("runtimeinfo missing fail-open: %s", rtW.Body.String())
	}
	if !strings.Contains(rtW.Body.String(), `"dropSetFailOpen":1`) && !strings.Contains(rtW.Body.String(), `"dropSetFailOpen": 1`) {
		t.Fatalf("expected fail-open count in runtimeinfo: %s", rtW.Body.String())
	}
	dumpEvidence(t, "g5-failopen-runtimeinfo.json", rtW.Body.String())
}

func TestProxy_RestoreAfterDropSetPublish(t *testing.T) {
	body := "# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\nkeep_metric 2\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	dir := t.TempDir()
	dropDir := dir + "/drop"
	if err := os.MkdirAll(dropDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeDropDir := func(enabled bool, gen string, names []string) {
		t.Helper()
		_ = os.WriteFile(dropDir+"/"+metricdrop.FileEnabled, []byte(fmt.Sprintf("%v\n", enabled)), 0644)
		_ = os.WriteFile(dropDir+"/"+metricdrop.FileGeneration, []byte(gen+"\n"), 0644)
		gz, err := metricdrop.GzipNames(names)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(dropDir+"/"+metricdrop.FileNamesGZ, gz, 0644)
		_ = os.WriteFile(dropDir+"/"+metricdrop.FileContentHash, []byte(metricdrop.HashUncompressed(names)+"\n"), 0644)
	}
	writeDropDir(true, "1", []string{"idle_metric"})
	set := metricdrop.NewSet(dropDir, dir+"/store.json")
	set.Reload()
	if !set.Load().Contains("idle_metric") {
		t.Fatal("setup drop-set")
	}
	job := &config.ScrapeConfig{JobName: "job1", ScrapeTimeout: model.Duration(time.Second)}
	p := NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: job, Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		func() *metricdrop.Snapshot { return set.Load() },
		logrus.New(),
	)
	req := httptest.NewRequest(http.MethodGet, ts.URL+"/metrics?_jobName=job1&_scheme=http&_hash=1", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	got, _ := io.ReadAll(w.Result().Body)
	if strings.Contains(string(got), "idle_metric 1") {
		t.Fatalf("dropped sample still present: %s", got)
	}
	writeDropDir(true, "2", nil)
	set.Reload()
	if set.Load().Contains("idle_metric") {
		t.Fatal("published restore must drop the name from set")
	}
	if set.Load().Generation != "2" {
		t.Fatalf("generation=%s", set.Load().Generation)
	}
	req2 := httptest.NewRequest(http.MethodGet, ts.URL+"/metrics?_jobName=job1&_scheme=http&_hash=1", nil)
	w2 := httptest.NewRecorder()
	p.ServeHTTP(w2, req2)
	got2, _ := io.ReadAll(w2.Result().Body)
	if !strings.Contains(string(got2), "idle_metric 1") {
		t.Fatalf("restored sample missing: %s", got2)
	}
	dumpEvidence(t, "g3-proxy-body-restored.txt", string(got2))
}

func TestProxy_GetDropSetDoesNotRebuild(t *testing.T) {
	dir := t.TempDir()
	dropDir := dir + "/drop"
	if err := os.MkdirAll(dropDir, 0755); err != nil {
		t.Fatal(err)
	}
	set := metricdrop.NewSet(dropDir, dir+"/store.json")
	gz, _ := metricdrop.GzipNames([]string{"idle_metric"})
	_ = os.WriteFile(dropDir+"/"+metricdrop.FileEnabled, []byte("true\n"), 0644)
	_ = os.WriteFile(dropDir+"/"+metricdrop.FileNamesGZ, gz, 0644)
	set.Reload()
	ptr := set.Load()
	body := "idle_metric 1\nkeep_metric 2\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	job := &config.ScrapeConfig{JobName: "job1", ScrapeTimeout: model.Duration(time.Second)}
	p := NewProxy(
		func(string) *scrape.JobInfo { return &scrape.JobInfo{Config: job, Cli: http.DefaultClient} },
		func() map[uint64]*target.ScrapeStatus { return map[uint64]*target.ScrapeStatus{1: {}} },
		func() *metricdrop.Snapshot { return set.Load() },
		logrus.New(),
	)
	req := httptest.NewRequest(http.MethodGet, ts.URL+"/metrics?_jobName=job1&_scheme=http&_hash=1", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	if set.Load() != ptr {
		t.Fatal("ServeHTTP must not Reload/rebuild drop-set")
	}
}

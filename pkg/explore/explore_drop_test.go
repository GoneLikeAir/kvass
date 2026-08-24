package explore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"
	"tkestack.io/kvass/pkg/discovery"
	"tkestack.io/kvass/pkg/metricdrop"
	"tkestack.io/kvass/pkg/prom"
	"tkestack.io/kvass/pkg/scrape"
	"tkestack.io/kvass/pkg/target"
)

func TestExplore_UsesFilteredSeries(t *testing.T) {
	body := "# HELP idle_metric x\n# TYPE idle_metric gauge\nidle_metric 1\nkeep_metric 2\n"
	hts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer hts.Close()

	sm := scrape.New(logrus.New())
	if err := sm.ApplyConfig(&prom.ConfigInfo{
		Config: &config.Config{ScrapeConfigs: []*config.ScrapeConfig{{
			JobName: "job1", ScrapeTimeout: model.Duration(time.Second),
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	e := New(sm, logrus.New())
	cur := &metricdrop.Snapshot{}
	e.SetDropSet(func() *metricdrop.Snapshot { return cur })
	info := sm.GetJob("job1")
	if info == nil {
		t.Fatal("job")
	}
	parsed, _ := url.Parse(hts.URL)
	series, bodySize, err := e.explore(info, parsed, hts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if series != 2 {
		t.Fatalf("unfiltered series=%d", series)
	}
	cur = &metricdrop.Snapshot{Enabled: true, Hash: "h2", Generation: "2", Names: map[string]struct{}{"idle_metric": {}}}
	series2, bodySize2, err := e.explore(info, parsed, hts.URL)
	if err != nil {
		t.Fatal(err)
	}
	g1out, _, g1size, _, err := scrape.FilterAndStat("job1", parsed, []byte(body), "text/plain", nil, cur)
	if err != nil {
		t.Fatal(err)
	}
	if series2 != 1 {
		t.Fatalf("filtered series=%d", series2)
	}
	if bodySize2 != g1size {
		t.Fatalf("bodySize=%d want G1 size %d out=%q", bodySize2, g1size, g1out)
	}
	e.UpdateTargets(map[string][]*discovery.SDTargets{
		"job1": {{ShardTarget: &target.Target{Hash: 1}}},
	})
	e.targets[1].exploring = true
	e.lastDropHash = "old"
	st := e.Get(1)
	if e.lastDropHash == "old" {
		t.Fatal("drop-set hash change must invalidate explore cache")
	}
	if e.targets[1].exploring == false {
		t.Fatal("Get after hash change must re-queue explore")
	}
	_ = st
	if dir := os.Getenv("LAND_EVIDENCE"); dir != "" {
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(dir+"/g4-explore.txt", []byte(fmt.Sprintf("series=%d bodySize=%d g1BodySize=%d firstBodySize=%d\n", series2, bodySize2, g1size, bodySize)), 0644)
	}
}

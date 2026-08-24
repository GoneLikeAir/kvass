package explore

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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
	body := "idle_metric 1\nkeep_metric 2\n"
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
	if series2 != 1 {
		t.Fatalf("filtered series=%d", series2)
	}
	if bodySize2 >= bodySize && series2 == series {
		t.Fatalf("expected smaller filtered body")
	}
	e.UpdateTargets(map[string][]*discovery.SDTargets{
		"job1": {{ShardTarget: &target.Target{Hash: 1}}},
	})
	e.targets[1].exploring = true
	e.lastDropHash = "old"
	_ = e.Get(1)
	if e.lastDropHash == "old" {
		t.Fatal("drop-set hash change must invalidate explore cache")
	}
}

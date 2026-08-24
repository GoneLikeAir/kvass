/*
 * Tencent is pleased to support the open source community by making TKEStack available.
 *
 * Copyright (C) 2012-2019 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package explore

import (
	"context"
	"fmt"
	"net/url"
	"tkestack.io/kvass/pkg/discovery"
	"tkestack.io/kvass/pkg/metricdrop"
	"tkestack.io/kvass/pkg/prom"
	"tkestack.io/kvass/pkg/scrape"
	"tkestack.io/kvass/pkg/utils/types"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
	"tkestack.io/kvass/pkg/target"
)

type exploringTarget struct {
	exploring bool
	job       string
	target    *target.Target
	rt        *target.ScrapeStatus
}

// Explore will explore Target before it assigned to Shard
type Explore struct {
	logger        logrus.FieldLogger
	scrapeManager *scrape.Manager

	targets     map[uint64]*exploringTarget
	targetsLock sync.Mutex

	retryInterval time.Duration
	needExplore   chan *exploringTarget
	explore       func(scrapeInfo *scrape.JobInfo, URL *url.URL, url string) (int64, int64, error)
	getDropSet    func() *metricdrop.Snapshot
	lastDropHash  string
}

// New create a new Explore
func New(scrapeManager *scrape.Manager, log logrus.FieldLogger) *Explore {
	e := &Explore{
		logger:        log,
		scrapeManager: scrapeManager,
		needExplore:   make(chan *exploringTarget, 10000),
		retryInterval: time.Second * 5,
		targets:       map[uint64]*exploringTarget{},
	}
	e.explore = func(scrapeInfo *scrape.JobInfo, URL *url.URL, url string) (int64, int64, error) {
		return exploreWithDrop(scrapeInfo, URL, url, e.getDropSet)
	}
	return e
}

func (e *Explore) SetDropSet(get func() *metricdrop.Snapshot) {
	e.getDropSet = get
}

// Get return the target scrape status of the target by hash
// if target is never explored, it will be send to explore
func (e *Explore) invalidateIfDropChanged() {
	hash := ""
	if e.getDropSet != nil {
		if snap := e.getDropSet(); snap != nil {
			hash = snap.Hash + ":" + snap.Generation
			if !snap.Enabled {
				hash = "disabled:" + snap.Hash
			}
		}
	}
	if hash == e.lastDropHash {
		return
	}
	e.lastDropHash = hash
	for _, t := range e.targets {
		t.exploring = false
	}
}

func (e *Explore) Get(hash uint64) *target.ScrapeStatus {
	e.targetsLock.Lock()
	defer e.targetsLock.Unlock()
	e.invalidateIfDropChanged()

	r := e.targets[hash]
	if r == nil {
		return nil
	}

	if !r.exploring {
		r.exploring = true
		e.needExplore <- r
	}

	return r.rt
}

// ApplyConfig delete invalid job's targets according to config
// the new targets will be add by UpdateTargets
func (e *Explore) ApplyConfig(cfg *prom.ConfigInfo) error {
	jobs := make([]string, 0, len(cfg.Config.ScrapeConfigs))
	for _, j := range cfg.Config.ScrapeConfigs {
		jobs = append(jobs, j.JobName)
	}

	e.targetsLock.Lock()
	defer e.targetsLock.Unlock()

	newTargets := map[uint64]*exploringTarget{}
	for hash, v := range e.targets {
		if types.FindString(v.job, jobs...) {
			newTargets[hash] = v
		}
	}
	e.targets = newTargets
	return nil
}

// UpdateTargets update global target info, if target is new , it will send for exploring
func (e *Explore) UpdateTargets(targets map[string][]*discovery.SDTargets) {
	e.targetsLock.Lock()
	defer e.targetsLock.Unlock()

	all := map[uint64]*exploringTarget{}
	for job, ts := range targets {
		for _, t := range ts {
			hash := t.ShardTarget.Hash
			if e.targets[hash] != nil {
				all[hash] = e.targets[hash]
			} else {
				all[hash] = &exploringTarget{
					job:    job,
					rt:     target.NewScrapeStatus(0),
					target: t.ShardTarget,
				}
			}
		}
	}
	e.targets = all
}

// Run start Explore exploring engine
// every Target will be explore MaxExploreTime times
// "con" is the max worker goroutines
func (e *Explore) Run(ctx context.Context, con int) error {
	var g errgroup.Group
	for i := 0; i < con; i++ {
		g.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case temp := <-e.needExplore:
					if temp == nil {
						continue
					}
					tar := temp
					hash := tar.target.Hash
					err := e.exploreOnce(ctx, tar)
					if err != nil {
						go func() {
							time.Sleep(e.retryInterval)
							e.targetsLock.Lock()
							defer e.targetsLock.Unlock()
							if e.targets[hash] != nil {
								e.needExplore <- tar
							}
						}()
					}
				}
			}
		})
	}

	return g.Wait()
}

func (e *Explore) exploreOnce(ctx context.Context, t *exploringTarget) (err error) {
	defer t.rt.SetScrapeErr(time.Now(), err)
	info := e.scrapeManager.GetJob(t.job)
	if info == nil {
		return fmt.Errorf("can not found %s  scrape info", t.job)
	}

	url := t.target.URL(info.Config).String()
	series, bodySize, err := e.explore(info, t.target.URL(info.Config), url)
	if err != nil {
		return errors.Wrapf(err, "explore failed : %s/%s", t.job, url)
	}

	t.rt.Series = series
	t.rt.BodySize = bodySize
	t.target.Series = series
	return nil
}

func explore(scrapeInfo *scrape.JobInfo, URL *url.URL, url string) (int64, int64, error) {
	return exploreWithDrop(scrapeInfo, URL, url, nil)
}

func exploreWithDrop(scrapeInfo *scrape.JobInfo, URL *url.URL, url string, get func() *metricdrop.Snapshot) (int64, int64, error) {
	data, typ, err := scrapeInfo.Scrape(url, nil)
	if err != nil {
		return 0, 0, err
	}
	var snap *metricdrop.Snapshot
	if get != nil {
		snap = get()
	}
	out, series, bodySize, _, ferr := scrape.FilterAndStat(scrapeInfo.Config.JobName, URL, data, typ, scrapeInfo.Config.MetricRelabelConfigs, snap)
	_ = out
	if ferr != nil {
		series2, body2, serr := scrape.StatisticSeries(scrapeInfo.Config.JobName, URL, data, typ, scrapeInfo.Config.MetricRelabelConfigs)
		return series2, body2, serr
	}
	return series, bodySize, nil
}

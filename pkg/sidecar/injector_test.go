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
package sidecar

import (
	"github.com/prometheus/common/model"
	"os"
	"path"
	"path/filepath"
	"tkestack.io/kvass/pkg/prom"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"testing"
	"tkestack.io/kvass/pkg/target"
)

func TestInjector_UpdateConfig(t *testing.T) {
	cfg := `global: {}
scrape_configs:
- job_name: job
  honor_timestamps: false
  bearer_token: job
remote_write:
- url: http://127.0.0.1
  bearer_token: write
remote_read:
- url: http://127.0.0.1
  bearer_token: read
`
	tar := &target.Target{
		Hash: 1,
		Labels: labels.Labels{
			{
				Name:  model.AddressLabel,
				Value: "127.0.0.1:80",
			},
			{
				Name:  model.SchemeLabel,
				Value: "https",
			},
			{
				Name:  model.MetricsPathLabel,
				Value: "/metrics",
			},
		},
	}

	r := require.New(t)
	outFile := path.Join(t.TempDir(), "out")
	_ = os.Setenv("POD_NAME", "rep-0")

	in := NewInjector(outFile,
		InjectConfigOptions{
			ProxyURL:      "http://127.0.0.1:8008",
			PrometheusURL: "http://127.0.0.1:9090",
		}, logrus.New())

	r.NoError(in.ApplyConfig(&prom.ConfigInfo{
		RawContent: []byte(cfg),
	}))
	r.NoError(in.UpdateTargets(map[string][]*target.Target{
		"job": {tar},
	}))
	out, err := config.LoadFile(outFile, false, false, log.NewNopLogger())
	r.NoError(err)

	outJob := out.ScrapeConfigs[0]
	outSD := outJob.ServiceDiscoveryConfigs[0].(discovery.StaticConfig)[0]
	r.Equal("http://127.0.0.1:8008", outJob.HTTPClientConfig.ProxyURL.String())
	r.Equal(model.LabelValue("127.0.0.1:80"), outSD.Targets[0][model.AddressLabel])
	r.Equal(model.LabelValue("http"), outSD.Labels[model.SchemeLabel])
	r.Equal(model.LabelValue("/metrics"), outSD.Labels[model.MetricsPathLabel])
	r.Equal(model.LabelValue("https"), outSD.Labels[model.ParamLabelPrefix+paramScheme])
	r.Equal(model.LabelValue("job"), outSD.Labels[model.ParamLabelPrefix+paramJobName])
	r.Equal(model.LabelValue("1"), outSD.Labels[model.ParamLabelPrefix+paramHash])
	writeAuth := out.RemoteWriteConfigs[0].HTTPClientConfig.Authorization
	r.NotNil(writeAuth)
	r.Equal("Bearer", writeAuth.Type)
	r.Equal("write", string(writeAuth.Credentials))
	readAuth := out.RemoteReadConfigs[0].HTTPClientConfig.Authorization
	r.NotNil(readAuth)
	r.Equal("Bearer", readAuth.Type)
	r.Equal("read", string(readAuth.Credentials))

	outSelf := out.ScrapeConfigs[1]
	outSelfSD := outSelf.ServiceDiscoveryConfigs[0].(discovery.StaticConfig)[0]
	r.Nil(outSelf.HTTPClientConfig.ProxyURL.URL)
	r.Equal(model.LabelValue("127.0.0.1:9090"), outSelfSD.Targets[0][model.AddressLabel])
	r.Equal(model.LabelValue("0"), outSelfSD.Labels["shard"])
	r.Equal(model.LabelValue("rep-0"), outSelfSD.Labels["replicate"])
}

func TestInjector_PreservesHTTPHeaders(t *testing.T) {
	r := require.New(t)
	baseDir := t.TempDir()
	headerFile := filepath.Join(baseDir, "header.txt")
	r.NoError(os.WriteFile(headerFile, []byte("file-secret"), 0644))

	cfg := `global: {}
scrape_configs:
- job_name: job
  http_headers:
    X-Test:
      values: ["v1"]
      secrets: ["s1"]
      files: ["header.txt"]
remote_write:
- url: http://127.0.0.1
  http_headers:
    X-Write:
      secrets: ["write-secret"]
remote_read:
- url: http://127.0.0.1
  http_headers:
    X-Read:
      secrets: ["read-secret"]
`

	tar := &target.Target{
		Hash: 1,
		Labels: labels.Labels{
			{
				Name:  model.AddressLabel,
				Value: "127.0.0.1:80",
			},
		},
	}

	outFile := path.Join(t.TempDir(), "out")
	_ = os.Setenv("POD_NAME", "rep-0")

	in := NewInjector(outFile,
		InjectConfigOptions{
			ProxyURL:           "http://127.0.0.1:8008",
			PrometheusURL:      "http://127.0.0.1:9090",
			HTTPHeadersBaseDir: baseDir,
		}, logrus.New())

	r.NoError(in.ApplyConfig(&prom.ConfigInfo{
		RawContent: []byte(cfg),
	}))
	r.NoError(in.UpdateTargets(map[string][]*target.Target{
		"job": {tar},
	}))

	out, err := config.LoadFile(outFile, false, false, log.NewNopLogger())
	r.NoError(err)

	jobHeaders := out.ScrapeConfigs[0].HTTPClientConfig.HTTPHeaders.Headers["X-Test"]
	r.Equal("s1", string(jobHeaders.Secrets[0]))
	r.Equal(headerFile, jobHeaders.Files[0])

	writeHeaders := out.RemoteWriteConfigs[0].HTTPClientConfig.HTTPHeaders.Headers["X-Write"]
	r.Equal("write-secret", string(writeHeaders.Secrets[0]))

	readHeaders := out.RemoteReadConfigs[0].HTTPClientConfig.HTTPHeaders.Headers["X-Read"]
	r.Equal("read-secret", string(readHeaders.Secrets[0]))
}

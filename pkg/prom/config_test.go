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

package prom

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"
)

func TestConfigManager_ReloadFromFile(t *testing.T) {
	type caseInfo struct {
		fileExist    bool
		content      string
		wantErr      bool
		wantCallBack bool
	}

	successCase := func() *caseInfo {
		return &caseInfo{
			fileExist: true,
			content: `
global:
  evaluation_interval: 10s
  scrape_interval: 15s
  external_labels:
    replica: pod1
scrape_configs:
- job_name: "test"
  static_configs:
  - targets:
    - 127.0.0.1:9091
`,
			wantErr:      false,
			wantCallBack: true,
		}
	}

	var cases = []struct {
		desc       string
		updateCase func(c *caseInfo)
	}{
		{
			desc:       "success",
			updateCase: func(c *caseInfo) {},
		},
		{
			desc: "file not exit, want err ",
			updateCase: func(c *caseInfo) {
				c.fileExist = false
				c.wantErr = true
			},
		},
		{
			desc: "empty content, want err",
			updateCase: func(c *caseInfo) {
				c.content = ""
				c.wantErr = true
			},
		},
		{
			desc: "wrong content format, want err",
			updateCase: func(c *caseInfo) {
				c.content = "a : a : a"
				c.wantErr = true
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.desc, func(t *testing.T) {
			r := require.New(t)
			c := successCase()
			cs.updateCase(c)
			file := path.Join(t.TempDir(), "a")
			if c.fileExist {
				_ = ioutil.WriteFile(file, []byte(c.content), 0777)
			}

			m := NewConfigManager()
			updated := false
			m.AddReloadCallbacks(func(cfg *ConfigInfo) error {
				updated = true
				r.Equal(string(cfg.RawContent), c.content)
				r.Equal("5051925342772717236", cfg.ConfigHash)
				r.Equal(1, len(cfg.Config.ScrapeConfigs))
				return nil
			})

			err := m.ReloadFromFile(file)
			if c.wantErr {
				r.Error(err)
				return
			}
			r.Equal(c.wantCallBack, updated)
		})
	}
}

func TestConfigManager_HTTPHeaders_BaseDirAndFiles(t *testing.T) {
	r := require.New(t)
	dir := t.TempDir()
	headerFile := filepath.Join(dir, "h1")
	r.NoError(ioutil.WriteFile(headerFile, []byte("value1\n"), 0644))

	cfg := `scrape_configs:
- job_name: test
  http_headers:
    X-Test:
      files: ["h1"]
`
	m := NewConfigManager()
	m.SetHTTPHeadersBaseDir(dir)
	r.NoError(m.ReloadFromRaw([]byte(cfg)))

	g := m.ConfigInfo().Config.ScrapeConfigs[0].HTTPClientConfig.HTTPHeaders.Headers["X-Test"].Files[0]
	r.Equal(headerFile, g)
}

func TestConfigManager_HTTPHeaders_LegacyPrecedence(t *testing.T) {
	r := require.New(t)
	cfg := `remote_write:
- url: http://example
  headers:
    X-Test: legacy
  http_headers:
    X-Test:
      values: ["new"]
`
	m := NewConfigManager()
	r.NoError(m.ReloadFromRaw([]byte(cfg)))

	clientCfg := m.ConfigInfo().Config.RemoteWriteConfigs[0].HTTPClientConfig
	if clientCfg.HTTPHeaders == nil {
		return
	}
	_, exists := clientCfg.HTTPHeaders.Headers["X-Test"]
	r.False(exists)
}

func TestConfigManager_HTTPHeaders_UnreadableFile(t *testing.T) {
	r := require.New(t)
	cfg := `scrape_configs:
- job_name: test
  http_headers:
    X-Test:
      files: ["/no/such/file"]
`
	m := NewConfigManager()
	r.Error(m.ReloadFromRaw([]byte(cfg)))
}

func TestConfigManager_HTTPHeaders_ReservedReject(t *testing.T) {
	r := require.New(t)
	cfg := `scrape_configs:
- job_name: test
  http_headers:
    Authorization:
      values: ["bad"]
`
	m := NewConfigManager()
	r.Error(m.ReloadFromRaw([]byte(cfg)))
}

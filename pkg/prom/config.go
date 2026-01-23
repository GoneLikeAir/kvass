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
	"fmt"
	logkit "github.com/go-kit/log"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/pkg/errors"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

const (
	defaultConfig = `
global:
  external_labels:
    status: default
`
	defaultHTTPHeadersBaseDir = "/etc/prometheus/headers"
)

// ConfigInfo include all information of current config
type ConfigInfo struct {
	// RawContent is the content of config file
	RawContent []byte
	// ConfigHash is a md5 of config file content
	ConfigHash string
	// Config is the marshaled prometheus config
	Config *config.Config
}

// DefaultConfig init a ConfigInfo with default prometheus config
var DefaultConfig = &ConfigInfo{
	RawContent: []byte(defaultConfig),
	ConfigHash: "",
	Config:     &config.DefaultConfig,
}

// ConfigManager do config manager
type ConfigManager struct {
	callbacks          []func(cfg *ConfigInfo) error
	currentConfig      *ConfigInfo
	httpHeadersBaseDir string
}

// NewConfigManager return an config manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		currentConfig: &ConfigInfo{
			Config: &config.DefaultConfig,
		},
	}
}

// SetHTTPHeadersBaseDir sets the base directory for http_headers file paths.
func (c *ConfigManager) SetHTTPHeadersBaseDir(dir string) {
	if dir != "" {
		c.httpHeadersBaseDir = dir
	}
}

// ReloadFromFile reload config from file and do all callbacks
func (c *ConfigManager) ReloadFromFile(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return c.reloadFromRawWithPath(data, file)
}

// ReloadFromRaw reload config from raw data
func (c *ConfigManager) ReloadFromRaw(data []byte) (err error) {
	return c.reloadFromRawWithPath(data, "")
}

func (c *ConfigManager) reloadFromRawWithPath(data []byte, filePath string) (err error) {
	info := &ConfigInfo{}
	info.RawContent = data
	if len(info.RawContent) == 0 {
		return errors.New("config content is empty")
	}

	info.Config, err = config.Load(string(data), true, logkit.NewNopLogger())
	if err != nil {
		return errors.Wrapf(err, "marshal config")
	}

	if err := applyConfigDirectory(info.Config, filePath, c.httpHeadersBaseDir); err != nil {
		return err
	}
	applyLegacyHeadersPrecedence(info.Config)
	if err := validateHTTPHeaderFiles(info.Config); err != nil {
		return err
	}

	// config hash don't include external labels
	eLb := info.Config.GlobalConfig.ExternalLabels
	info.Config.GlobalConfig.ExternalLabels = []labels.Label{}
	hash, err := hashstructure.Hash(info.Config, hashstructure.FormatV2, nil)
	if err != nil {
		return errors.Wrapf(err, "get config hash")
	}

	info.ConfigHash = fmt.Sprint(hash)
	info.Config.GlobalConfig.ExternalLabels = eLb
	c.currentConfig = info

	for _, f := range c.callbacks {
		if err := f(c.currentConfig); err != nil {
			return err
		}
	}

	return nil
}

// ConfigInfo return current config info
func (c *ConfigManager) ConfigInfo() *ConfigInfo {
	return c.currentConfig
}

// AddReloadCallbacks add callbacks of config reload event
func (c *ConfigManager) AddReloadCallbacks(f ...func(c *ConfigInfo) error) {
	c.callbacks = append(c.callbacks, f...)
}

func applyConfigDirectory(cfg *config.Config, filePath, baseDir string) error {
	if cfg == nil {
		return nil
	}
	if filePath != "" {
		cfg.SetDirectory(filepath.Dir(filePath))
		return nil
	}
	if baseDir == "" {
		baseDir = defaultHTTPHeadersBaseDir
	}
	cfg.SetDirectory(baseDir)
	return nil
}

func applyLegacyHeadersPrecedence(cfg *config.Config) {
	if cfg == nil {
		return
	}
	for i := range cfg.RemoteWriteConfigs {
		removeOverlappingHTTPHeaders(cfg.RemoteWriteConfigs[i].Headers, &cfg.RemoteWriteConfigs[i].HTTPClientConfig)
	}
	for i := range cfg.RemoteReadConfigs {
		removeOverlappingHTTPHeaders(cfg.RemoteReadConfigs[i].Headers, &cfg.RemoteReadConfigs[i].HTTPClientConfig)
	}
}

func removeOverlappingHTTPHeaders(legacy map[string]string, httpCfg *config_util.HTTPClientConfig) {
	if len(legacy) == 0 || httpCfg == nil || httpCfg.HTTPHeaders == nil {
		return
	}
	for legacyKey := range legacy {
		canonical := http.CanonicalHeaderKey(legacyKey)
		for headerKey := range httpCfg.HTTPHeaders.Headers {
			if http.CanonicalHeaderKey(headerKey) == canonical {
				delete(httpCfg.HTTPHeaders.Headers, headerKey)
			}
		}
	}
	if len(httpCfg.HTTPHeaders.Headers) == 0 {
		httpCfg.HTTPHeaders = nil
	}
}

func validateHTTPHeaderFiles(cfg *config.Config) error {
	return visitHTTPClientConfigs(cfg, func(httpCfg *config_util.HTTPClientConfig) error {
		if httpCfg == nil || httpCfg.HTTPHeaders == nil {
			return nil
		}
		for name, header := range httpCfg.HTTPHeaders.Headers {
			for _, file := range header.Files {
				if file == "" {
					continue
				}
				if _, err := os.ReadFile(file); err != nil {
					return errors.Wrapf(err, "read http_header file %q for %q", file, name)
				}
			}
		}
		return nil
	})
}

func visitHTTPClientConfigs(cfg *config.Config, fn func(*config_util.HTTPClientConfig) error) error {
	if cfg == nil {
		return nil
	}
	for _, sc := range cfg.ScrapeConfigs {
		if err := fn(&sc.HTTPClientConfig); err != nil {
			return err
		}
	}
	for i := range cfg.RemoteWriteConfigs {
		if err := fn(&cfg.RemoteWriteConfigs[i].HTTPClientConfig); err != nil {
			return err
		}
	}
	for i := range cfg.RemoteReadConfigs {
		if err := fn(&cfg.RemoteReadConfigs[i].HTTPClientConfig); err != nil {
			return err
		}
	}
	for i := range cfg.AlertingConfig.AlertmanagerConfigs {
		if err := fn(&cfg.AlertingConfig.AlertmanagerConfigs[i].HTTPClientConfig); err != nil {
			return err
		}
	}
	return nil
}

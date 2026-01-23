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
	"fmt"
	"github.com/prometheus/prometheus/discovery"
	"net/url"
	"os"
	"strings"
	"sync"
	"tkestack.io/kvass/pkg/prom"
	"tkestack.io/kvass/pkg/target"

	"github.com/prometheus/prometheus/model/relabel"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"
	config_util "github.com/prometheus/common/config"
	"gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"
)

const (
	paramJobName = "_jobName"
	paramHash    = "_hash"
	paramScheme  = "_scheme"
)

// InjectConfigOptions indicate what to inject to config file
type InjectConfigOptions struct {
	// ProxyURL will be injected to all job if it is not empty
	ProxyURL string
	// PrometheusURL will be injected
	PrometheusURL string
	// HTTPHeadersBaseDir sets the base directory for http_headers files.
	HTTPHeadersBaseDir string
}

// Injector gen injected config file
type Injector struct {
	sync.Mutex
	outFile    string
	option     InjectConfigOptions
	curTargets map[string][]*target.Target
	curCfg     *prom.ConfigInfo
	writeFile  func(filename string, data []byte, perm os.FileMode) error
	log        logrus.FieldLogger
}

// NewInjector create new injector with InjectConfigOptions
func NewInjector(outFile string, option InjectConfigOptions, log logrus.FieldLogger) *Injector {
	return &Injector{
		outFile:    outFile,
		option:     option,
		curTargets: map[string][]*target.Target{},
		writeFile:  os.WriteFile,
		log:        log,
		curCfg:     prom.DefaultConfig,
	}
}

// UpdateTargets set new targets
func (i *Injector) UpdateTargets(ts map[string][]*target.Target) error {
	i.curTargets = ts
	return i.inject()
}

// ApplyConfig gen new config
func (i *Injector) ApplyConfig(cfg *prom.ConfigInfo) error {
	i.curCfg = cfg
	return i.inject()
}

func (i *Injector) injectJobs(cfg *config.Config) error {
	for _, job := range cfg.ScrapeConfigs {
		if i.option.ProxyURL != "" {
			u, err := url.Parse(i.option.ProxyURL)
			if err != nil {
				return err
			}

			job.HTTPClientConfig.ProxyURL = config_util.URL{
				URL: u,
			}
		}

		job.ServiceDiscoveryConfigs = []discovery.Config{
			discovery.StaticConfig(target2targetGroup(job.JobName, i.curTargets[job.JobName])),
		}

		job.Scheme = "http"
		job.HTTPClientConfig.BearerToken = ""
		job.HTTPClientConfig.BasicAuth = nil
		job.HTTPClientConfig.TLSConfig = config_util.TLSConfig{}

		// fix invalid label
		job.RelabelConfigs = []*relabel.Config{
			{
				Separator:   ";",
				Regex:       relabel.MustNewRegexp(target.PrefixForInvalidLabelName + "(.+)"),
				Replacement: "$1",
				Action:      relabel.LabelMap,
			},
		}
	}

	return nil
}

func (i *Injector) injectSelfMonitor(cfg *config.Config) {
	if i.option.PrometheusURL != "" {
		u, _ := url.Parse(i.option.PrometheusURL)
		podName := os.Getenv("POD_NAME")
		ss := strings.Split(podName, "-")
		shard := "0"
		if len(ss) > 0 {
			shard = ss[len(ss)-1]
		}

		cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, &config.ScrapeConfig{
			JobName: "prometheus_shards",
			ServiceDiscoveryConfigs: []discovery.Config{
				discovery.StaticConfig([]*targetgroup.Group{
					{
						Targets: []model.LabelSet{
							{
								model.AddressLabel: model.LabelValue(u.Host),
							},
						},
						Labels: map[model.LabelName]model.LabelValue{
							"replicate": model.LabelValue(podName),
							"shard":     model.LabelValue(shard),
						},
					},
				}),
			}})
	}
}

func (i *Injector) marshal(cfg *config.Config) ([]byte, error) {
	bTokens := make([]string, 0)
	password := make([]string, 0)

	for _, w := range cfg.RemoteWriteConfigs {
		if w.HTTPClientConfig.BearerToken != "" {
			bTokens = append(bTokens, string(w.HTTPClientConfig.BearerToken))
		}

		if w.HTTPClientConfig.BasicAuth != nil && w.HTTPClientConfig.BasicAuth.Password != "" {
			password = append(password, string(w.HTTPClientConfig.BasicAuth.Password))
		}

	}

	for _, w := range cfg.RemoteReadConfigs {
		if w.HTTPClientConfig.BearerToken != "" {
			bTokens = append(bTokens, string(w.HTTPClientConfig.BearerToken))
		}

		if w.HTTPClientConfig.BasicAuth != nil && w.HTTPClientConfig.BasicAuth.Password != "" {
			password = append(password, string(w.HTTPClientConfig.BasicAuth.Password))
		}

	}

	gen, err := yaml.Marshal(&cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal config failed")
	}

	gen, err = replaceAuthorizationCredentials(gen, cfg)
	if err != nil {
		return nil, err
	}

	gen, err = replaceHTTPHeaderSecrets(gen, cfg)
	if err != nil {
		return nil, err
	}

	data := string(gen)
	for _, token := range bTokens {
		data = strings.Replace(data, "bearer_token: <secret>", fmt.Sprintf("bearer_token: %s", token), 1)
	}

	for _, pd := range password {
		data = strings.Replace(data, "password: <secret>", fmt.Sprintf("password: %s", pd), 1)
	}

	return []byte(data), nil
}

func replaceAuthorizationCredentials(raw []byte, cfg *config.Config) ([]byte, error) {
	if cfg == nil {
		return raw, nil
	}

	writeCreds := make([]string, len(cfg.RemoteWriteConfigs))
	for i, w := range cfg.RemoteWriteConfigs {
		if w.HTTPClientConfig.Authorization != nil && w.HTTPClientConfig.Authorization.Credentials != "" {
			writeCreds[i] = string(w.HTTPClientConfig.Authorization.Credentials)
		}
	}
	readCreds := make([]string, len(cfg.RemoteReadConfigs))
	for i, r := range cfg.RemoteReadConfigs {
		if r.HTTPClientConfig.Authorization != nil && r.HTTPClientConfig.Authorization.Credentials != "" {
			readCreds[i] = string(r.HTTPClientConfig.Authorization.Credentials)
		}
	}

	var node yamlv3.Node
	if err := yamlv3.Unmarshal(raw, &node); err != nil {
		return nil, errors.Wrapf(err, "unmarshal config for authorization")
	}
	replaceAuthorizationSection(&node, "remote_write", writeCreds)
	replaceAuthorizationSection(&node, "remote_read", readCreds)
	gen, err := yamlv3.Marshal(&node)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal config for authorization")
	}
	return gen, nil
}

func replaceHTTPHeaderSecrets(raw []byte, cfg *config.Config) ([]byte, error) {
	if cfg == nil {
		return raw, nil
	}

	scrapeHeaders := collectHTTPHeaderSecretsFromScrape(cfg)
	remoteWriteHeaders := collectHTTPHeaderSecretsFromRemoteWrite(cfg)
	remoteReadHeaders := collectHTTPHeaderSecretsFromRemoteRead(cfg)
	alertHeaders := collectHTTPHeaderSecretsFromAlertmanagers(cfg)

	var node yamlv3.Node
	if err := yamlv3.Unmarshal(raw, &node); err != nil {
		return nil, errors.Wrapf(err, "unmarshal config for http_headers")
	}
	replaceHTTPHeaderSection(&node, "scrape_configs", scrapeHeaders)
	replaceHTTPHeaderSection(&node, "remote_write", remoteWriteHeaders)
	replaceHTTPHeaderSection(&node, "remote_read", remoteReadHeaders)
	replaceHTTPHeaderAlertmanagers(&node, alertHeaders)
	gen, err := yamlv3.Marshal(&node)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal config for http_headers")
	}
	return gen, nil
}

func collectHTTPHeaderSecrets(httpCfg *config_util.HTTPClientConfig) map[string][]string {
	if httpCfg == nil || httpCfg.HTTPHeaders == nil {
		return nil
	}
	secrets := make(map[string][]string)
	for name, header := range httpCfg.HTTPHeaders.Headers {
		if len(header.Secrets) == 0 {
			continue
		}
		vals := make([]string, 0, len(header.Secrets))
		for _, secret := range header.Secrets {
			if secret == "" {
				continue
			}
			vals = append(vals, string(secret))
		}
		if len(vals) > 0 {
			secrets[name] = vals
		}
	}
	if len(secrets) == 0 {
		return nil
	}
	return secrets
}

func collectHTTPHeaderSecretsFromScrape(cfg *config.Config) []map[string][]string {
	res := make([]map[string][]string, len(cfg.ScrapeConfigs))
	for i, sc := range cfg.ScrapeConfigs {
		res[i] = collectHTTPHeaderSecrets(&sc.HTTPClientConfig)
	}
	return res
}

func collectHTTPHeaderSecretsFromRemoteWrite(cfg *config.Config) []map[string][]string {
	res := make([]map[string][]string, len(cfg.RemoteWriteConfigs))
	for i := range cfg.RemoteWriteConfigs {
		res[i] = collectHTTPHeaderSecrets(&cfg.RemoteWriteConfigs[i].HTTPClientConfig)
	}
	return res
}

func collectHTTPHeaderSecretsFromRemoteRead(cfg *config.Config) []map[string][]string {
	res := make([]map[string][]string, len(cfg.RemoteReadConfigs))
	for i := range cfg.RemoteReadConfigs {
		res[i] = collectHTTPHeaderSecrets(&cfg.RemoteReadConfigs[i].HTTPClientConfig)
	}
	return res
}

func collectHTTPHeaderSecretsFromAlertmanagers(cfg *config.Config) []map[string][]string {
	res := make([]map[string][]string, len(cfg.AlertingConfig.AlertmanagerConfigs))
	for i := range cfg.AlertingConfig.AlertmanagerConfigs {
		res[i] = collectHTTPHeaderSecrets(&cfg.AlertingConfig.AlertmanagerConfigs[i].HTTPClientConfig)
	}
	return res
}

func replaceHTTPHeaderSection(root *yamlv3.Node, key string, secrets []map[string][]string) {
	if root == nil || len(secrets) == 0 {
		return
	}

	mapping := root
	if root.Kind == yamlv3.DocumentNode && len(root.Content) > 0 {
		mapping = root.Content[0]
	}
	if mapping.Kind != yamlv3.MappingNode {
		return
	}

	seq := findMapValue(mapping, key)
	if seq == nil || seq.Kind != yamlv3.SequenceNode {
		return
	}
	for idx, item := range seq.Content {
		if idx >= len(secrets) {
			break
		}
		if len(secrets[idx]) == 0 {
			continue
		}
		replaceHTTPHeaderSecretsInItem(item, secrets[idx])
	}
}

func replaceHTTPHeaderAlertmanagers(root *yamlv3.Node, secrets []map[string][]string) {
	if root == nil || len(secrets) == 0 {
		return
	}

	mapping := root
	if root.Kind == yamlv3.DocumentNode && len(root.Content) > 0 {
		mapping = root.Content[0]
	}
	if mapping.Kind != yamlv3.MappingNode {
		return
	}
	alerting := findMapValue(mapping, "alerting")
	if alerting == nil || alerting.Kind != yamlv3.MappingNode {
		return
	}
	seq := findMapValue(alerting, "alertmanagers")
	if seq == nil || seq.Kind != yamlv3.SequenceNode {
		return
	}
	for idx, item := range seq.Content {
		if idx >= len(secrets) {
			break
		}
		if len(secrets[idx]) == 0 {
			continue
		}
		replaceHTTPHeaderSecretsInItem(item, secrets[idx])
	}
}

func replaceHTTPHeaderSecretsInItem(item *yamlv3.Node, secrets map[string][]string) {
	if item == nil || item.Kind != yamlv3.MappingNode || len(secrets) == 0 {
		return
	}
	headersNode := findMapValue(item, "http_headers")
	if headersNode == nil {
		return
	}
	replaceHTTPHeaderSecretsInMap(headersNode, secrets)
}

func replaceHTTPHeaderSecretsInMap(headersNode *yamlv3.Node, secrets map[string][]string) {
	if headersNode == nil || headersNode.Kind != yamlv3.MappingNode {
		return
	}
	for i := 0; i < len(headersNode.Content)-1; i += 2 {
		headerName := headersNode.Content[i].Value
		headerNode := headersNode.Content[i+1]
		if headerNode.Kind != yamlv3.MappingNode {
			continue
		}
		values, ok := secrets[headerName]
		if !ok || len(values) == 0 {
			continue
		}
		secretsNode := findMapValue(headerNode, "secrets")
		if secretsNode == nil {
			continue
		}
		switch secretsNode.Kind {
		case yamlv3.SequenceNode:
			for idx, secretNode := range secretsNode.Content {
				if idx >= len(values) {
					break
				}
				secretNode.Value = values[idx]
			}
		case yamlv3.ScalarNode:
			secretsNode.Value = values[0]
		}
	}
}

func replaceAuthorizationSection(root *yamlv3.Node, key string, creds []string) {
	if root == nil || len(creds) == 0 {
		return
	}

	mapping := root
	if root.Kind == yamlv3.DocumentNode && len(root.Content) > 0 {
		mapping = root.Content[0]
	}
	if mapping.Kind != yamlv3.MappingNode {
		return
	}

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		seq := mapping.Content[i+1]
		if seq.Kind != yamlv3.SequenceNode {
			return
		}
		for idx, item := range seq.Content {
			if idx >= len(creds) {
				break
			}
			if creds[idx] == "" {
				continue
			}
			authNode := findMapValue(item, "authorization")
			if authNode == nil {
				continue
			}
			credNode := findMapValue(authNode, "credentials")
			if credNode == nil {
				continue
			}
			credNode.Value = creds[idx]
		}
		return
	}
}

func findMapValue(node *yamlv3.Node, key string) *yamlv3.Node {
	if node == nil || node.Kind != yamlv3.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func (i *Injector) inject() error {
	i.Lock()
	defer i.Unlock()
	// create a default empty config for prometheus and thanos sidecar
	if i.curCfg == prom.DefaultConfig {
		return i.writeFile(i.outFile, i.curCfg.RawContent, 0755)
	}

	cfg := &config.Config{}
	if err := yaml.Unmarshal(i.curCfg.RawContent, &cfg); err != nil {
		return errors.Wrapf(err, "unmarshal config")
	}
	if i.option.HTTPHeadersBaseDir != "" {
		cfg.SetDirectory(i.option.HTTPHeadersBaseDir)
	}

	if err := i.injectJobs(cfg); err != nil {
		return errors.Wrapf(err, "inject jobs")
	}
	i.injectSelfMonitor(cfg)

	data, err := i.marshal(cfg)
	if err != nil {
		return errors.Wrapf(err, "marshal injected config")
	}

	if err := i.writeFile(i.outFile, data, 0755); err != nil {
		return errors.Wrapf(err, "write file failed")
	}

	i.log.Infof("config inject completed")
	return nil
}

func target2targetGroup(job string, ts []*target.Target) []*targetgroup.Group {
	ret := make([]*targetgroup.Group, 0)

	for _, t := range ts {
		ls := model.LabelSet{}
		scheme := "http"
		address := ""
		for _, v := range t.Labels {
			if v.Name == model.SchemeLabel {
				scheme = v.Value
			}
			if v.Name == model.AddressLabel {
				address = v.Value
			}

			ls[model.LabelName(v.Name)] = model.LabelValue(v.Value)
		}

		ls[model.LabelName(model.SchemeLabel)] = "http"
		ls[model.LabelName(fmt.Sprintf("%s%s", model.ParamLabelPrefix, paramScheme))] = model.LabelValue(scheme)
		ls[model.LabelName(fmt.Sprintf("%s%s", model.ParamLabelPrefix, paramJobName))] = model.LabelValue(job)
		ls[model.LabelName(fmt.Sprintf("%s%s", model.ParamLabelPrefix, paramHash))] = model.LabelValue(fmt.Sprint(t.Hash))

		ret = append(ret, &targetgroup.Group{
			Targets: []model.LabelSet{
				{
					model.AddressLabel: model.LabelValue(address),
				},
			},
			Labels: ls,
		})
	}

	return ret
}

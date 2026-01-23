package scrape

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultPromConfig(t *testing.T) {
	r := require.New(t)

	// 测试创建默认配置
	config := NewDefaultPromConfig()
	r.NotNil(config)

	// 验证全局配置
	r.Equal(model.Duration(1*time.Minute), config.GlobalConfig.ScrapeInterval)
	r.Equal(model.Duration(10*time.Second), config.GlobalConfig.ScrapeTimeout)
	r.Equal(model.Duration(1*time.Minute), config.GlobalConfig.EvaluationInterval)

	// 验证抓取配置
	r.Len(config.ScrapeConfigs, 1)
	r.Equal("/metrics", config.ScrapeConfigs[0].MetricsPath)
	r.Equal("http", config.ScrapeConfigs[0].Scheme)
	r.False(config.ScrapeConfigs[0].HonorLabels)
	r.True(config.ScrapeConfigs[0].HonorTimestamps)

	// 验证告警管理器配置
	r.Len(config.AlertingConfig.AlertmanagerConfigs, 1)
	r.Equal("http", config.AlertingConfig.AlertmanagerConfigs[0].Scheme)
	r.Equal(model.Duration(10*time.Second), config.AlertingConfig.AlertmanagerConfigs[0].Timeout)
	r.Equal(AlertmanagerAPIVersionV1, config.AlertingConfig.AlertmanagerConfigs[0].APIVersion)

	// 验证远程写入配置
	r.Len(config.RemoteWriteConfigs, 0)

	// 验证远程读取配置
	r.Len(config.RemoteReadConfigs, 0)
}

func TestLoad(t *testing.T) {
	r := require.New(t)

	// 测试加载有效配置
	configStr := `
global:
  scrape_interval: 30s
  scrape_timeout: 10s
  evaluation_interval: 30s

scrape_configs:
  - job_name: 'test_job'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/custom_metrics'
    scheme: 'https'
    honor_labels: true
    honor_timestamps: false

alerting:
  alertmanagers:
    - scheme: 'https'
      timeout: 30s
      api_version: 'v2'
`

	config, err := Load(configStr)
	r.NoError(err)
	r.NotNil(config)

	// 验证全局配置
	r.Equal(model.Duration(30*time.Second), config.GlobalConfig.ScrapeInterval)
	r.Equal(model.Duration(10*time.Second), config.GlobalConfig.ScrapeTimeout)
	r.Equal(model.Duration(30*time.Second), config.GlobalConfig.EvaluationInterval)

	// 验证抓取配置
	r.Len(config.ScrapeConfigs, 1)
	r.Equal("test_job", config.ScrapeConfigs[0].JobName)
	r.Equal("/custom_metrics", config.ScrapeConfigs[0].MetricsPath)
	r.Equal("https", config.ScrapeConfigs[0].Scheme)
	r.True(config.ScrapeConfigs[0].HonorLabels)
	r.False(config.ScrapeConfigs[0].HonorTimestamps)

	// 验证告警管理器配置
	r.Len(config.AlertingConfig.AlertmanagerConfigs, 1)
	r.Equal("https", config.AlertingConfig.AlertmanagerConfigs[0].Scheme)
	r.Equal(model.Duration(30*time.Second), config.AlertingConfig.AlertmanagerConfigs[0].Timeout)
	r.Equal(AlertmanagerAPIVersionV2, config.AlertingConfig.AlertmanagerConfigs[0].APIVersion)
}

func TestPromConfig_HTTPHeaders_LoadAndValidate(t *testing.T) {
	r := require.New(t)
	dir := t.TempDir()
	headerFile := filepath.Join(dir, "h1")
	r.NoError(ioutil.WriteFile(headerFile, []byte("value1\n"), 0644))

	cfg := `scrape_configs:
- job_name: test
  http_headers:
    X-Test:
      values: ["v1"]
      secrets: ["s1"]
      files: ["h1"]
`
	loaded, err := LoadWithBaseDir(cfg, dir)
	r.NoError(err)

	hdr := loaded.ScrapeConfigs[0].HTTPClientConfig.HTTPHeaders.Headers["X-Test"]
	r.Equal("v1", hdr.Values[0])
	r.Equal("s1", hdr.Secrets[0])
	r.Equal(headerFile, hdr.Files[0])
}

func TestPromConfig_HTTPHeaders_ReservedReject(t *testing.T) {
	r := require.New(t)
	cfg := `scrape_configs:
- job_name: test
  http_headers:
    Authorization:
      values: ["bad"]
`
	_, err := Load(cfg)
	r.Error(err)
}

func TestLoad_InvalidConfig(t *testing.T) {
	r := require.New(t)

	// 测试加载无效配置
	invalidConfigStr := `
invalid_yaml: [
  this is not valid yaml
`

	config, err := Load(invalidConfigStr)
	r.Error(err)
	r.Nil(config)
}

func TestLoad_EmptyConfig(t *testing.T) {
	r := require.New(t)

	// 测试加载空配置
	config, err := Load("")
	r.NoError(err)
	r.NotNil(config)

	// 验证默认值已应用
	r.Equal(model.Duration(1*time.Minute), config.GlobalConfig.ScrapeInterval)
	r.Equal(model.Duration(10*time.Second), config.GlobalConfig.ScrapeTimeout)
	r.Len(config.ScrapeConfigs, 1)
}

func TestPromConfig_String(t *testing.T) {
	r := require.New(t)

	// 测试配置转字符串
	config := &PromConfig{
		GlobalConfig: GlobalCfg{
			ScrapeInterval: model.Duration(30 * time.Second),
			ScrapeTimeout:  model.Duration(10 * time.Second),
		},
		ScrapeConfigs: []*ScrapeConfig{
			{
				JobName:     "test_job",
				MetricsPath: "/metrics",
				Scheme:      "http",
			},
		},
	}

	str := config.String()
	r.NotEmpty(str)
	r.Contains(str, "scrape_interval: 30s")
	r.Contains(str, "scrape_timeout: 10s")
	r.Contains(str, "job_name: test_job")
	r.Contains(str, "metrics_path: /metrics")
	r.Contains(str, "scheme: http")
}

func TestK8sSDConfig_Name(t *testing.T) {
	r := require.New(t)

	config := &K8sSDConfig{}
	r.Equal("kubernetes", config.Name())
}

func TestFileSDConfig_Name(t *testing.T) {
	r := require.New(t)

	config := &FileSDConfig{}
	r.Equal("file", config.Name())
}

func TestStaticConfig_Name(t *testing.T) {
	r := require.New(t)

	config := StaticConfig{}
	r.Equal("static", config.Name())
}

func TestHTTPSDConfig_Name(t *testing.T) {
	r := require.New(t)

	config := &HTTPSDConfig{}
	r.Equal("http", config.Name())
}

func TestDefaultConfigValues(t *testing.T) {
	r := require.New(t)

	// 验证默认全局配置
	r.Equal(model.Duration(1*time.Minute), DefaultGlobalConfig.ScrapeInterval)
	r.Equal(model.Duration(10*time.Second), DefaultGlobalConfig.ScrapeTimeout)
	r.Equal(model.Duration(1*time.Minute), DefaultGlobalConfig.EvaluationInterval)

	// 验证默认抓取配置
	r.Equal("/metrics", DefaultScrapeConfig.MetricsPath)
	r.Equal("http", DefaultScrapeConfig.Scheme)
	r.False(DefaultScrapeConfig.HonorLabels)
	r.True(DefaultScrapeConfig.HonorTimestamps)

	// 验证默认告警管理器配置
	r.Equal("http", DefaultAlertmanagerConfig.Scheme)
	r.Equal(model.Duration(10*time.Second), DefaultAlertmanagerConfig.Timeout)
	r.Equal(AlertmanagerAPIVersionV1, DefaultAlertmanagerConfig.APIVersion)

	// 验证默认远程写入配置
	r.Equal(model.Duration(30*time.Second), DefaultRemoteWriteConfig.RemoteTimeout)

	// 验证默认队列配置
	r.Equal(200, DefaultQueueConfig.MaxShards)
	r.Equal(1, DefaultQueueConfig.MinShards)
	r.Equal(500, DefaultQueueConfig.MaxSamplesPerSend)
	r.Equal(2500, DefaultQueueConfig.Capacity)
	r.Equal(model.Duration(5*time.Second), DefaultQueueConfig.BatchSendDeadline)
	r.Equal(model.Duration(30*time.Millisecond), DefaultQueueConfig.MinBackoff)
	r.Equal(model.Duration(100*time.Millisecond), DefaultQueueConfig.MaxBackoff)

	// 验证默认远程读取配置
	r.Equal(model.Duration(1*time.Minute), DefaultRemoteReadConfig.RemoteTimeout)
}

func TestAlertmanagerAPIVersionConstants(t *testing.T) {
	r := require.New(t)

	r.Equal("v1", string(AlertmanagerAPIVersionV1))
	r.Equal("v2", string(AlertmanagerAPIVersionV2))
}

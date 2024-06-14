package scrape

import (
	"fmt"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
	"time"
)

// all the structures in this file are reference from prometheus config, but change some data type in order to print a raw-string config

// refer from prometheus/config
type BasicAuth struct {
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty"`
	PasswordFile string `json:"password_file,omitempty" yaml:"password_file,omitempty"`
}

type Authorization struct {
	Type            string `json:"type,omitempty" yaml:"type,omitempty"`
	Credentials     string `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	CredentialsFile string `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`
}

type TLSConfig struct {
	// The CA cert to use for the targets.
	CAFile string `json:"ca_file,omitempty" yaml:"ca_file,omitempty"`
	// The client cert file for the targets.
	CertFile string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	// The client key file for the targets.
	KeyFile string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
	// Used to verify the hostname for the targets.
	ServerName string `json:"server_name,omitempty" yaml:"server_name,omitempty"`
	// Disable target certificate validation.
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
}

type HTTPClientConfig struct {
	// The HTTP basic authentication credentials for the targets.
	BasicAuth *BasicAuth `json:"basic_auth,omitempty" yaml:"basic_auth,omitempty"`
	// The HTTP authorization credentials for the targets.
	Authorization *Authorization `json:"authorization,omitempty" yaml:"authorization,omitempty"`
	// The bearer token for the targets. Deprecated in favour of
	// Authorization.Credentials.
	BearerToken string `json:"bearer_token,omitempty" yaml:"bearer_token,omitempty"`
	// The bearer token file for the targets. Deprecated in favour of
	// Authorization.CredentialsFile.
	BearerTokenFile string `json:"bearer_token_file,omitempty" yaml:"bearer_token_file,omitempty"`
	// HTTP proxy server to use to connect to the targets.
	ProxyURL string `json:"proxy_url,omitempty" yaml:"proxy_url,omitempty"`
	// TLSConfig to use to connect to the targets.
	TLSConfig TLSConfig `json:"tls_config,omitempty" yaml:"tls_config,omitempty"`
	// FollowRedirects specifies whether the client should follow HTTP 3xx redirects.
	// The omitempty flag is not set, because it would be hidden from the
	// marshalled configuration when set to false.
	FollowRedirects bool `json:"follow_redirects,omitempty" yaml:"follow_redirects,omitempty"`
	EnableHttp2     bool `yaml:"enable_http2,omitempty"`
}

//func (c *HTTPClientConfig) Cfg2Dto(cfg *commoncfg.HTTPClientConfig) {
//	c.FollowRedirects = cfg.FollowRedirects
//	c.TLSConfig = TLSConfig{
//		CAFile:             cfg.TLSConfig.CAFile,
//		CertFile:           cfg.TLSConfig.CertFile,
//		KeyFile:            cfg.TLSConfig.KeyFile,
//		ServerName:         cfg.TLSConfig.ServerName,
//		InsecureSkipVerify: cfg.TLSConfig.InsecureSkipVerify,
//	}
//	c.ProxyURL = cfg.ProxyURL.String()
//	c.BearerTokenFile = cfg.BearerTokenFile
//	c.BearerToken = string(cfg.BearerToken)
//
//	if cfg.Authorization != nil {
//		c.Authorization = &Authorization{
//			Type:            cfg.Authorization.Type,
//			Credentials:     string(cfg.Authorization.Credentials),
//			CredentialsFile: cfg.Authorization.CredentialsFile,
//		}
//	}
//
//	if cfg.BasicAuth != nil {
//		c.BasicAuth = &BasicAuth{
//			Username:     cfg.BasicAuth.Username,
//			Password:     string(cfg.BasicAuth.Password),
//			PasswordFile: cfg.BasicAuth.PasswordFile,
//		}
//	}
//}

//func (dto *HTTPClientConfig) Dto2Cfg() (*commoncfg.HTTPClientConfig, error) {
//	url2, err := url.Parse(dto.ProxyURL)
//	if err != nil {
//		return nil, err
//	}
//
//	cfg := &commoncfg.HTTPClientConfig{
//		BearerToken:     commoncfg.Secret(dto.BearerToken),
//		BearerTokenFile: dto.BearerTokenFile,
//		ProxyURL:        commoncfg.URL{url2},
//		TLSConfig:       commoncfg.TLSConfig{
//			CAFile:             dto.TLSConfig.CAFile,
//			CertFile:           dto.TLSConfig.CertFile,
//			KeyFile:            dto.TLSConfig.KeyFile,
//			ServerName:         dto.TLSConfig.ServerName,
//			InsecureSkipVerify: dto.TLSConfig.InsecureSkipVerify,
//		},
//		FollowRedirects: dto.FollowRedirects,
//	}
//
//	if dto.BasicAuth != nil {
//		ba := &commoncfg.BasicAuth{
//			Username:     dto.BasicAuth.Username,
//			Password:     commoncfg.Secret(dto.BasicAuth.Password),
//			PasswordFile: dto.BasicAuth.PasswordFile,
//		}
//		cfg.BasicAuth = ba
//	}
//
//	if dto.Authorization != nil {
//		auth := &commoncfg.Authorization{
//			ResourceType:            dto.Authorization.ResourceType,
//			Credentials:     commoncfg.Secret(dto.Authorization.Credentials),
//			CredentialsFile: dto.Authorization.CredentialsFile,
//		}
//		cfg.Authorization = auth
//	}
//
//	return cfg, nil
//}

type QueueConfig struct {
	// Number of samples to buffer per shard before we block. Defaults to
	// MaxSamplesPerSend.
	Capacity int `json:"capacity,omitempty" yaml:"capacity,omitempty"`

	// Max number of shards, i.e. amount of concurrency.
	MaxShards int `json:"max_shards,omitempty" yaml:"max_shards,omitempty"`

	// Min number of shards, i.e. amount of concurrency.
	MinShards int `json:"min_shards,omitempty" yaml:"min_shards,omitempty"`

	// Maximum number of samples per send.
	MaxSamplesPerSend int `json:"max_samples_per_send,omitempty" yaml:"max_samples_per_send,omitempty"`

	// Maximum time sample will wait in buffer.
	BatchSendDeadline model.Duration `json:"batch_send_deadline,omitempty" yaml:"batch_send_deadline,omitempty"`

	// On recoverable errors, backoff exponentially.
	MinBackoff model.Duration `json:"min_backoff,omitempty" yaml:"min_backoff,omitempty"`
	MaxBackoff model.Duration `json:"max_backoff,omitempty" yaml:"max_backoff,omitempty"`
}

type MetadataConfig struct {
	// Send controls whether we send metric metadata to remote storage.
	Send bool `json:"send" yaml:"send"`
	// SendInterval controls how frequently we send metric metadata.
	SendIntervalSec int64 `json:"send_interval_sec" yaml:"send_interval_sec"`
}

type RemoteWriteConfig struct {
	URL           string         `json:"url" yaml:"url"`
	RemoteTimeout model.Duration `json:"remote_timeout,omitempty" yaml:"remote_timeout,omitempty"`
	//Headers             map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	WriteRelabelConfigs []*RelabelConfig `json:"write_relabel_configs,omitempty" yaml:"write_relabel_configs,omitempty"`
	Name                string           `json:"name,omitempty" yaml:"name,omitempty"`

	HTTPClientConfig `json:",inline" yaml:",inline"`
	QueueConfig      QueueConfig `json:"queue_config,omitempty" yaml:"queue_config,omitempty"`
	//MetadataConfig   MetadataConfig   `json:"metadata_config,omitempty" yaml:"metadata_config,omitempty"`
}

//func (dto *RemoteWriteConfig) Cfg2Dto(cfg *config.RemoteWriteConfig) {
//	dto.HTTPClientConfig.Cfg2Dto(&cfg.HTTPClientConfig)
//	dto.Name = cfg.Name
//	dto.RemoteTimeoutSec = int64(time.Duration(cfg.RemoteTimeout) / time.Second)
//	dto.URL = cfg.URL.String()
//	//dto.MetadataConfig = MetadataConfig{
//	//	Send:            cfg.MetadataConfig.Send,
//	//	SendIntervalSec: int64(time.Duration(cfg.MetadataConfig.SendInterval) / time.Second),
//	//}
//
//	dto.QueueConfig = QueueConfig{
//		Capacity:             cfg.QueueConfig.Capacity,
//		MaxShards:            cfg.QueueConfig.MaxShards,
//		MinShards:            cfg.QueueConfig.MinShards,
//		MaxSamplesPerSend:    cfg.QueueConfig.MaxSamplesPerSend,
//		BatchSendDeadlineSec: int64(time.Duration(cfg.QueueConfig.BatchSendDeadline) / time.Second),
//		MinBackoffSec:        int64(time.Duration(cfg.QueueConfig.MinBackoff) / time.Second),
//		MaxBackoffSec:        int64(time.Duration(cfg.QueueConfig.MaxBackoff) / time.Second),
//	}
//
//	//dto.Headers = cfg.Headers
//	if cfg.WriteRelabelConfigs != nil && len(cfg.WriteRelabelConfigs) > 0 {
//		relabelCfgs := make([]*RelabelConfig, 0)
//		for _, c := range cfg.WriteRelabelConfigs {
//			rcfg := &RelabelConfig{}
//			rcfg.Cfg2Dto(c)
//			relabelCfgs = append(relabelCfgs, rcfg)
//		}
//		dto.WriteRelabelConfigs = relabelCfgs
//	}
//}

//func (dto *RemoteWriteConfig) Dto2Cfg() (*config.RemoteWriteConfig, error) {
//	url2, err := url.Parse(dto.URL)
//	if err != nil {
//		return nil, err
//	}
//	httpClientCfg, err := (&dto.HTTPClientConfig).Dto2Cfg()
//	if err != nil {
//		return nil, err
//	}
//
//	cfg := &config.RemoteWriteConfig{
//		URL:                 &commoncfg.URL{url2},
//		RemoteTimeout:       commonmodel.Duration(time.Second * time.Duration(dto.RemoteTimeoutSec)),
//		//Headers:             dto.Headers,
//		Name:                dto.Name,
//		HTTPClientConfig:    *httpClientCfg,
//		QueueConfig:         config.QueueConfig{
//			Capacity:          dto.QueueConfig.Capacity,
//			MaxShards:         dto.QueueConfig.MaxShards,
//			MinShards:         dto.QueueConfig.MinShards,
//			MaxSamplesPerSend: dto.QueueConfig.MaxSamplesPerSend,
//			BatchSendDeadline: commonmodel.Duration(time.Second * time.Duration(dto.QueueConfig.BatchSendDeadlineSec)),
//			MinBackoff:        commonmodel.Duration(time.Second * time.Duration(dto.QueueConfig.MinBackoffSec)),
//			MaxBackoff:        commonmodel.Duration(time.Second * time.Duration(dto.QueueConfig.MaxBackoffSec)),
//		},
//		//MetadataConfig:      config.MetadataConfig{
//		//	Send:         dto.MetadataConfig.Send,
//		//	SendInterval: commonmodel.Duration(time.Second * time.Duration(dto.MetadataConfig.SendIntervalSec)),
//		//},
//	}
//	if dto.WriteRelabelConfigs != nil && len(dto.WriteRelabelConfigs) > 0 {
//		relabelConfigs := make([]*relabel.AlertmanagerCfg, len(dto.WriteRelabelConfigs))
//		for _, c := range dto.WriteRelabelConfigs {
//			p, err := ParseRelabelConfig(c)
//			if err != nil {
//				return nil, err
//			}
//			relabelConfigs = append(relabelConfigs, p)
//		}
//		cfg.WriteRelabelConfigs = relabelConfigs
//	}
//
//	return cfg, nil
//}

type RemoteReadConfig struct {
	URL           string         `json:"url" yaml:"url"`
	RemoteTimeout model.Duration `json:"remote_timeout,omitempty" yaml:"remote_timeout,omitempty"`
	ReadRecent    bool           `json:"read_recent,omitempty" yaml:"read_recent,omitempty"`
	Name          string         `json:"name,omitempty" yaml:"name,omitempty"`

	HTTPClientConfig `json:",inline" yaml:",inline"`

	// RequiredMatchers is an optional list of equality matchers which have to
	// be present in a selector to query the remote read endpoint.
	RequiredMatchers map[string]string `json:"required_matchers,omitempty" yaml:"required_matchers,omitempty"`
}

//func (dto *RemoteReadConfig) Dto2Cfg() (*config.RemoteReadConfig, error) {
//	url2, err := url.Parse(dto.URL)
//	if err != nil {
//		return nil, err
//	}
//	httpClientCfg, err := (&dto.HTTPClientConfig).Dto2Cfg()
//	if err != nil {
//		return nil, err
//	}
//
//	cfg := &config.RemoteReadConfig{
//		URL:              &commoncfg.URL{url2},
//		RemoteTimeout:    commonmodel.Duration(time.Second * time.Duration(dto.RemoteTimeoutSec)),
//		ReadRecent:       dto.ReadRecent,
//		Name:             dto.Name,
//		HTTPClientConfig: *httpClientCfg,
//	}
//	if dto.RequiredMatchers != nil && len(dto.RequiredMatchers) > 0 {
//		labelSet := make(map[commonmodel.LabelName]commonmodel.LabelValue)
//		for k, v := range dto.RequiredMatchers {
//			labelSet[commonmodel.LabelName(k)] = commonmodel.LabelValue(v)
//		}
//
//		cfg.RequiredMatchers = labelSet
//	}
//	return cfg, nil
//}

//func (dto *RemoteReadConfig) Cfg2Dto(cfg *config.RemoteReadConfig) {
//	dto.URL = cfg.URL.String()
//	dto.Name = cfg.Name
//	dto.ReadRecent = cfg.ReadRecent
//	dto.RemoteTimeoutSec = int64(time.Duration(cfg.RemoteTimeout) / time.Second)
//
//	dto.HTTPClientConfig.Cfg2Dto(&cfg.HTTPClientConfig)
//
//	if cfg.RequiredMatchers != nil && len(cfg.RequiredMatchers) > 0 {
//		labelSet := make(map[string]string)
//		for k, v := range cfg.RequiredMatchers {
//			labelSet[string(k)] = string(v)
//		}
//		dto.RequiredMatchers = labelSet
//
//	}
//}

type RelabelConfig struct {
	SourceLabels []string `yaml:"source_labels,flow,omitempty" json:"source_labels,flow,omitempty"`
	// Separator is the string between concatenated values from the source labels.
	Separator string `yaml:"separator,omitempty" json:"separator,omitempty"`
	// Regex against which the concatenation is matched.
	Regex string `yaml:"regex,omitempty" json:"regex,omitempty"`
	// Modulus to take of the hash of concatenated values from the source labels.
	Modulus uint64 `yaml:"modulus,omitempty" json:"modulus,omitempty"`
	// TargetLabel is the label to which the resulting string is written in a replacement.
	// Regexp interpolation is allowed for the replace action.
	TargetLabel string `yaml:"target_label,omitempty" json:"target_label,omitempty"`
	// Replacement is the regex replacement pattern to be used.
	Replacement string `yaml:"replacement,omitempty" json:"replacement,omitempty"`
	// Action is the action to be performed for the relabeling.
	Action string `yaml:"action,omitempty" json:"action,omitempty"`
}

//func (dto *RelabelConfig) Cfg2Dto(cfg *relabel.AlertmanagerCfg) {
//
//	dto.Separator 	= cfg.Separator
//	dto.Regex 		= cfg.Regex.String()
//	dto.Modulus 	= cfg.Modulus
//	dto.TargetLabel = cfg.TargetLabel
//	dto.Replacement = cfg.Replacement
//	dto.Action 		= string(cfg.Action)
//
//	if cfg.SourceLabels != nil && len(cfg.SourceLabels) > 0 {
//		sourceLabels := make([]string, 0)
//		for _, l := range cfg.SourceLabels {
//			sourceLabels = append(sourceLabels, string(l))
//		}
//		dto.SourceLabels = sourceLabels
//	}
//
//}

//func ParseRelabelConfig(dto *RelabelConfig) (*relabel.AlertmanagerCfg, error) {
//
//	regex, err := relabel.NewRegexp(dto.Regex)
//	if err != nil {
//		return nil, err
//	}
//
//	cfg := &relabel.AlertmanagerCfg{
//		Separator:    dto.Separator,
//		Regex:        regex,
//		Modulus:      dto.Modulus,
//		TargetLabel:  dto.TargetLabel,
//		Replacement:  dto.Replacement,
//		Action:       relabel.Action(dto.Action),
//	}
//
//	if dto.SourceLabels != nil {
//		labelNames := make([]commonmodel.LabelName, len(dto.SourceLabels))
//		for _, l := range dto.SourceLabels {
//			labelNames = append(labelNames, commonmodel.LabelName(l))
//		}
//		cfg.SourceLabels = labelNames
//	}
//
//	return cfg, nil
//}

type ScrapeConfig struct {
	// The job name to which the job label is set by default.
	JobName string `yaml:"job_name" json:"job_name"`
	// Indicator whether the scraped metrics should remain unmodified.
	HonorLabels bool `yaml:"honor_labels,omitempty" json:"honor_labels,omitempty"`
	// Indicator whether the scraped timestamps should be respected.
	HonorTimestamps bool `yaml:"honor_timestamps" json:"honor_timestamps"`
	// A set of query parameters with which the target is scraped.
	Params map[string][]string `yaml:"params,omitempty" json:"params,omitempty"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval model.Duration `yaml:"scrape_interval,omitempty" json:"scrape_interval_sec,omitempty"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout model.Duration `yaml:"scrape_timeout,omitempty" json:"scrape_timeout_sec,omitempty"`
	// The HTTP resource path on which to fetch metrics from targets.
	MetricsPath string `yaml:"metrics_path,omitempty" json:"metrics_path,omitempty"`
	// The URL scheme with which to fetch metrics from targets.
	Scheme string `yaml:"scheme,omitempty" json:"scheme,omitempty"`
	// More than this many samples post metric-relabeling will cause the scrape to fail.
	SampleLimit uint `yaml:"sample_limit,omitempty" json:"sample_limit,omitempty"`
	// More than this many targets after the target relabeling will cause the
	// scrapes to fail.
	TargetLimit uint `yaml:"target_limit,omitempty" json:"target_limit,omitempty"`

	// We cannot do proper Go type embedding below as the parser will then parse
	// values arbitrarily into the overflow maps of further-down types.

	ServiceDiscoveryConfigs []interface{}   `yaml:"-" json:"-"`
	FileSDConfig            []*FileSDConfig `yaml:"file_sd_configs,omitempty" json:"file_sd_configs,omitempty"`
	K8sSDConfig             []*K8sSDConfig  `yaml:"kubernetes_sd_configs,omitempty" json:"kubernetes_sd_configs,omitempty"`
	HTTPSDConfig            []*HTTPSDConfig `yaml:"http_sd_configs,omitempty" json:"http_sd_configs,omitempty"`
	StaticConfig            *StaticConfig   `yaml:"static_configs,omitempty" json:"static_configs,omitempty"`
	HTTPClientConfig        `yaml:",inline" json:",inline"`

	// List of target relabel configurations.
	RelabelConfigs []*RelabelConfig `yaml:"relabel_configs,omitempty" json:"relabel_configs,omitempty"`
	// List of metric relabel configurations.
	MetricRelabelConfigs []*RelabelConfig    `yaml:"metric_relabel_configs,omitempty" json:"metric_relabel_configs,omitempty"`
	Subsystem            string              `yaml:"-" json:"subsystem,omitempty"`
	Keep                 map[string][]string `yaml:"-" json:"keep,omitempty"`
	Drop                 map[string][]string `yaml:"-" json:"drop,omitempty"`
}

type AlertmanagerConfigs []*AlertmanagerConfig
type AlertmanagerAPIVersion string

type AlertingConfig struct {
	AlertRelabelConfigs []*RelabelConfig    `yaml:"alert_relabel_configs,omitempty"`
	AlertmanagerConfigs AlertmanagerConfigs `yaml:"alertmanagers,omitempty"`
}

type AlertmanagerConfig struct {
	// We cannot do proper Go type embedding below as the parser will then parse
	// values arbitrarily into the overflow maps of further-down types.
	FollowRedirects         bool           `yaml:"follow_redirects,omitempty"`
	EnableHttp2             bool           `yaml:"enable_http2,omitempty"`
	ServiceDiscoveryConfigs interface{}    `yaml:"-"`
	K8sSDConfig             []*K8sSDConfig `yaml:"kubernetes_sd_configs,omitempty" json:"kubernetes_sd_configs,omitempty"`
	StaticConfig            *StaticConfig  `yaml:"static_configs,omitempty" json:"static_configs,omitempty"`

	//HTTPClientConfig `yaml:",inline"`

	// The URL scheme to use when talking to Alertmanagers.
	Scheme string `yaml:"scheme,omitempty"`
	// Path prefix to add in front of the push endpoint path.
	PathPrefix string `yaml:"path_prefix,omitempty"`
	// The timeout used when sending alerts.
	Timeout model.Duration `yaml:"timeout,omitempty"`

	// The api version of Alertmanager.
	APIVersion AlertmanagerAPIVersion `yaml:"api_version"`

	// List of Alertmanager relabel configurations.
	RelabelConfigs []*RelabelConfig `yaml:"relabel_configs,omitempty"`
}

type PromConfig struct {
	IsDefault          bool                 `yaml:"-" json:"-"`
	GlobalConfig       GlobalCfg            `yaml:"global" json:"global"`
	RuleFiles          []string             `yaml:"rule_files,omitempty" json:"rule_files,omitempty"`
	ScrapeConfigs      []*ScrapeConfig      `yaml:"scrape_configs,omitempty" json:"scrape_configs,omitempty"`
	RemoteWriteConfigs []*RemoteWriteConfig `yaml:"remote_write,omitempty" json:"remote_write,omitempty"`
	RemoteReadConfigs  []*RemoteReadConfig  `yaml:"remote_read,omitempty" json:"remote_read,omitempty"`
	AlertingConfig     AlertingConfig       `yaml:"alerting,omitempty" json:"alerting,omitempty"`
}

type GlobalCfg struct {
	ScrapeInterval     model.Duration    `yaml:"scrape_interval,omitempty" json:"scrape_interval,omitempty"`
	ScrapeTimeout      model.Duration    `yaml:"scrape_timeout,omitempty" json:"scrape_timeout,omitempty"`
	EvaluationInterval model.Duration    `yaml:"evaluation_interval,omitempty" json:"evaluation_interval,omitempty"`
	ExternalLabels     map[string]string `yaml:"external_labels,omitempty" json:"external_labels,omitempty"`
	QueryLogFile       string            `yaml:"query_log_file,omitempty" json:"query_log_file,omitempty"`
}

//func (c *GlobalCfg) Cfg2Dto(cfg *config.AlertGlobalConfig) {
//	ScrapeTimeoutSec := int64(time.Duration(cfg.ScrapeTimeout) / time.Second)
//	c.ScrapeTimeoutSec = &ScrapeTimeoutSec
//
//	ScrapeIntervalSec := int64(time.Duration(cfg.ScrapeInterval) / time.Second)
//	c.ScrapeIntervalSec = &ScrapeIntervalSec
//
//	EvaluationIntervalSec := int64(time.Duration(cfg.EvaluationInterval) / time.Second)
//	c.EvaluationIntervalSec = &EvaluationIntervalSec
//
//	if cfg.ExternalLabels != nil && len(cfg.ExternalLabels) > 0 {
//		labels := make(map[string]string)
//		for _, l := range cfg.ExternalLabels {
//			labels[l.Name] = l.Value
//		}
//	}
//}

const (
	// AlertmanagerAPIVersionV1 represents
	// github.com/prometheus/alertmanager/api/v1.
	AlertmanagerAPIVersionV1 AlertmanagerAPIVersion = "v1"
	// AlertmanagerAPIVersionV2 represents
	// github.com/prometheus/alertmanager/api/v2.
	AlertmanagerAPIVersionV2 AlertmanagerAPIVersion = "v2"
)

func NewDefaultPromConfig() *PromConfig {
	sc := DefaultScrapeConfig
	ac := DefaultAlertmanagerConfig
	//wc := DefaultRemoteWriteConfig
	//rc := DefaultRemoteReadConfig
	gc := DefaultGlobalConfig
	return &PromConfig{
		GlobalConfig:  gc,
		ScrapeConfigs: []*ScrapeConfig{&sc},
		AlertingConfig: AlertingConfig{
			AlertmanagerConfigs: []*AlertmanagerConfig{&ac},
		},
		RemoteWriteConfigs: []*RemoteWriteConfig{},
		RemoteReadConfigs:  []*RemoteReadConfig{},
	}
}

var (
	// DefaultConfig is the default top-level configuration.
	DefaultConfig = PromConfig{
		GlobalConfig:  DefaultGlobalConfig,
		ScrapeConfigs: []*ScrapeConfig{&DefaultScrapeConfig},
		AlertingConfig: AlertingConfig{
			AlertmanagerConfigs: []*AlertmanagerConfig{&DefaultAlertmanagerConfig},
		},
		RemoteWriteConfigs: []*RemoteWriteConfig{&DefaultRemoteWriteConfig},
		RemoteReadConfigs:  []*RemoteReadConfig{&DefaultRemoteReadConfig},
	}

	// DefaultGlobalConfig is the default global configuration.
	DefaultGlobalConfig = GlobalCfg{
		ScrapeInterval:     model.Duration(1 * time.Minute),
		ScrapeTimeout:      model.Duration(10 * time.Second),
		EvaluationInterval: model.Duration(1 * time.Minute),
	}

	// DefaultScrapeConfig is the default scrape configuration.
	DefaultScrapeConfig = ScrapeConfig{
		// ScrapeTimeout and ScrapeInterval default to the
		// configured globals.
		MetricsPath:     "/metrics",
		Scheme:          "http",
		HonorLabels:     false,
		HonorTimestamps: true,
	}

	// DefaultAlertmanagerConfig is the default alertmanager configuration.
	DefaultAlertmanagerConfig = AlertmanagerConfig{
		Scheme:     "http",
		Timeout:    model.Duration(10 * time.Second),
		APIVersion: AlertmanagerAPIVersionV1,
	}

	// DefaultRemoteWriteConfig is the default remote write configuration.
	DefaultRemoteWriteConfig = RemoteWriteConfig{
		RemoteTimeout: model.Duration(30 * time.Second),
		QueueConfig:   DefaultQueueConfig,
	}

	// DefaultQueueConfig is the default remote queue configuration.
	DefaultQueueConfig = QueueConfig{
		// With a maximum of 200 shards, assuming an average of 100ms remote write
		// time and 500 samples per batch, we will be able to push 1M samples/s.
		MaxShards:         200,
		MinShards:         1,
		MaxSamplesPerSend: 500,

		// Each shard will have a max of 2500 samples pending in its channel, plus the pending
		// samples that have been enqueued. Theoretically we should only ever have about 3000 samples
		// per shard pending. At 200 shards that's 600k.
		Capacity:          2500,
		BatchSendDeadline: model.Duration(5 * time.Second),

		// Backoff times for retrying a batch of samples on recoverable errors.
		MinBackoff: model.Duration(30 * time.Millisecond),
		MaxBackoff: model.Duration(100 * time.Millisecond),
	}

	// DefaultRemoteReadConfig is the default remote read configuration.
	DefaultRemoteReadConfig = RemoteReadConfig{
		RemoteTimeout: model.Duration(1 * time.Minute),
	}
)

func Load(s string) (*PromConfig, error) {
	cfg := &PromConfig{}
	// If the entire config body is empty the UnmarshalYAML method is
	// never called. We thus have to set the DefaultConfig at the entry
	// point as well.
	*cfg = DefaultConfig

	err := yaml.UnmarshalStrict([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c PromConfig) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}

type SelectorConfig struct {
	Role  string `yaml:"role,omitempty" json:"role,omitempty"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	Field string `yaml:"field,omitempty" json:"field,omitempty"`
}

type NamespaceDiscovery struct {
	Names []string `yaml:"names,omitempty"`
}

type K8sSDConfig struct {
	APIServer          string `yaml:"api_server,omitempty" json:"api_server,omitempty"`
	Role               string `yaml:"role" json:"role"`
	KubeconfigFile     string `yaml:"kubeconfig_file,omitempty"`
	HTTPClientConfig   `json:",inline" yaml:",inline"`
	NamespaceDiscovery NamespaceDiscovery `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
	Selectors          []SelectorConfig   `yaml:"selectors,omitempty" json:"selectors,omitempty"`
}

func (*K8sSDConfig) Name() string { return "kubernetes" }

type FileSDConfig struct {
	Files           []string       `yaml:"files" json:"files"`
	RefreshInterval model.Duration `yaml:"refresh_interval,omitempty" json:"refresh_interval,omitempty"`
}

func (*FileSDConfig) Name() string { return "file" }

type Group struct {
	// in prometheus, the type of Targets is like map[string]string, while parse to Prometheus.Targets, the key is model.AddressLabel(__address__)
	Targets []string `json:"targets" yaml:"targets"`
	// Labels is a set of labels that is common across all targets in the group.
	Labels map[string]string `json:"labels" yaml:"labels"`

	// Source is an identifier that describes a group of targets.
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
}

type StaticConfig []*Group

func (StaticConfig) Name() string { return "static" }

type HTTPSDConfig struct {
	HTTPClientConfig HTTPClientConfig `yaml:",inline"`
	RefreshInterval  model.Duration   `yaml:"refresh_interval,omitempty"`
	URL              string           `yaml:"url"`
}

func (*HTTPSDConfig) Name() string { return "http" }

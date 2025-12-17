package scrape

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"go.uber.org/atomic"
)

var MetricCollector *MetricInfoCollector

type CollectTask struct {
	JobName        string         `json:"jobName"`
	ScrapeInterval model.Duration `json:"scrapeInterval"`
	StartTime      time.Time      `json:"startTime,omitempty"`
}

type MetricInfoCollector struct {
	data       sync.Map
	waiting    sync.Map
	collecting sync.Map
	targetInfo sync.Map
	currency   *atomic.Int32
	configPath string
	//lg         logrus.FieldLogger
}

// MetricInfo 存储指标的元数据信息，包括名称、类型、帮助信息、单位、子系统ID和标签等。
// 该结构体是线程安全的，使用 sync.RWMutex 保护并发访问。
// 读操作可以并发执行，写操作需要独占访问。
// 每个 MetricInfo 实例都有自己的锁，采用细粒度锁策略，减少锁竞争。
type MetricInfo struct {
	Name         string
	Type         string
	Help         string
	Unit         string
	SubsystemId  string
	ExistsLabels map[string]bool
	Labels       []string
	// mu 用于保护 MetricInfo 结构体的并发访问
	// 使用读写锁允许多个读操作并发执行，写操作需要独占访问
	mu sync.RWMutex
}

func NewMetricInfo(name string) *MetricInfo {
	return &MetricInfo{
		Name:         name,
		ExistsLabels: make(map[string]bool),
		Labels:       make([]string, 0),
	}
}

func InitMetricCollector(configPath string, runTask ...bool) *MetricInfoCollector {
	mc := &MetricInfoCollector{
		data:       sync.Map{},
		waiting:    sync.Map{},
		collecting: sync.Map{},
		targetInfo: sync.Map{},
		currency:   atomic.NewInt32(int32(0)),
		configPath: configPath,
		//lg:         lg,
	}
	if len(runTask) == 0 || runTask[0] {
		go mc.taskDiscovering()
		go mc.checkTask()
		go mc.syncTargetInfo()
	}
	MetricCollector = mc
	return mc
}

func (c *MetricInfoCollector) syncTargetInfo() {
	for {
		c.syncTargetInfoOnce()
		<-time.NewTimer(time.Minute * 3).C
	}
}

type TargetInfo struct {
	SubsystemId   string
	SubsystemName string
	Labels        labels.Labels
	UpdateTime    time.Time
}

func (c *MetricInfoCollector) syncTargetInfoOnce() {
	file, err := os.Open(c.configPath)
	if err != nil {
		Log("Failed to open file: %v", err)
		return
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		Log("Failed to read prometheus file content: %v", err)
		return
	}
	cfg, err := Load(string(content))
	if err != nil {
		Log("load prometheus config failed, err=%s", err.Error())
		return
	}
	for _, job := range cfg.ScrapeConfigs {
		if job.StaticConfig == nil {
			continue
		}
		for _, group := range *job.StaticConfig {
			if len(group.Targets) > 0 && len(group.Labels) > 0 {
				host := group.Labels["__address__"]
				path := group.Labels["__metrics_path__"]
				if path == "" {
					path = "/metrics"
				}
				subsystem := c.getSubsystem(group.Labels)
				for k, _ := range group.Labels {
					if strings.HasPrefix(k, "__") || k == "job" || k == "job_name" || strings.HasPrefix(k, "weps_subsystem") {
						delete(group.Labels, k)
					}
				}
				lset := labels.FromMap(group.Labels)
				if host != "" && subsystem != "" {
					// cache target info
					info := &TargetInfo{
						SubsystemId: subsystem,
						Labels:      lset,
						UpdateTime:  time.Now(),
					}
					Log("job: %s, target info: %s%s, subsystem: %s, labels: %s", job.JobName, host, path, subsystem, lset.String())
					c.targetInfo.Store(fmt.Sprintf("%s%s", host, path), info)
				}
			}
		}
	}
}

func (c *MetricInfoCollector) getSubsystem(lset map[string]string) string {
	subsystem := lset["subsystem"]
	if subsystem != "" {
		return subsystem
	}
	subsystem = lset["subsystem_id"]
	if subsystem != "" {
		return subsystem
	}
	subsystem = lset["subsystem_name"]
	if subsystem != "" {
		return subsystem
	}
	subsystem = lset["subsystemId"]
	if subsystem != "" {
		return subsystem
	}
	subsystem = lset["subsystemName"]
	if subsystem != "" {
		return subsystem
	}
	return subsystem
}

func (c *MetricInfoCollector) GetTargetInfo(host, path string) *TargetInfo {
	v, ok := c.targetInfo.Load(fmt.Sprintf("%s%s", host, path))
	if ok {
		return v.(*TargetInfo)
	}
	return nil
}

func (c *MetricInfoCollector) taskDiscovering() {
	ticker := time.NewTicker(time.Second * 30)
	for {
		select {
		case <-ticker.C:
			c.discoverOnce()
		}
	}

}

func (c *MetricInfoCollector) discoverOnce() {
	filePath := "/tmp/task.json"
	// 判断文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		//Log("File '%s' does not exist. Not task discovered", filePath)
		return
	} else if err != nil {
		Log("Failed to check file existence: %v", err)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		Log("Failed to open file: %v", err)
		return
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		Log("Failed to read file content: %v", err)
		return
	}
	var tasks []*CollectTask
	err = json.Unmarshal(content, &tasks)
	if err != nil {
		Log("unmarshal task info failed, err: %v", err)
		return
	}
	for _, t := range tasks {
		if _, ok := c.collecting.Load(t.JobName); ok {
			continue
		}
		Log("add task to queue, task info: %v", t)
		c.waiting.Store(t.JobName, t)
	}
	os.Remove(filePath)
}

func (c *MetricInfoCollector) NeedCollect(jobName string) bool {
	if _, ok := c.collecting.Load(jobName); ok {
		return true
	}
	return false
}

func (c *MetricInfoCollector) checkTask() {
	ticker := time.NewTicker(time.Second * 20)
	for {
		select {
		case <-ticker.C:
			c.checkTaskOnce()
		}
	}
}

func (c *MetricInfoCollector) checkTaskOnce() {
	hasTask := false
	c.collecting.Range(func(key, value interface{}) bool {
		jobName := key.(string)
		task := value.(*CollectTask)
		if task.StartTime.Add(time.Duration(task.ScrapeInterval) * 3).Before(time.Now()) {
			Log("collect task finish, jobName: %s, startTime: %s", task.JobName, task.StartTime.String())
			c.collecting.Delete(jobName)
			c.currency.Add(int32(-1))
			if err := c.dumpJobMetrics(jobName); err != nil {
				Log("dump job metrics info to file failed, err=%s", err.Error())
			}
		} else {
			hasTask = true
		}
		return true
	})

	c.waiting.Range(func(key, value interface{}) bool {
		hasTask = true
		if c.currency.Load() < 5 {
			jobName := key.(string)
			task := value.(*CollectTask)
			task.StartTime = time.Now()
			c.collecting.Store(jobName, task)
			c.waiting.Delete(jobName)
			c.currency.Add(int32(1))
			Log("collect task for %s started", task.JobName)
			return true
		} else {
			return false
		}
	})
	filename := "/tmp/hasTaskRunning"
	ioutil.WriteFile(filename, []byte(fmt.Sprintf("%t", hasTask)), 0644)
}

// AddHelp 为指定作业和指标添加帮助信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Help 字段的并发访问
func (c *MetricInfoCollector) AddHelp(jobName, metricName, helpInfo string) {
	mi := c.loadOrInitMetricInfo(jobName, metricName)
	mi.mu.Lock()
	mi.Help = helpInfo
	mi.mu.Unlock()
}

// AddUnit 为指定作业和指标添加单位信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Unit 字段的并发访问
func (c *MetricInfoCollector) AddUnit(jobName, metricName, unit string) {
	mi := c.loadOrInitMetricInfo(jobName, metricName)
	mi.mu.Lock()
	mi.Unit = unit
	mi.mu.Unlock()
}

// AddLabels 为指定作业和指标添加标签信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Labels 和 MetricInfo.ExistsLabels 字段的并发访问
// 会过滤掉已存在的标签和 __name__ 标签
func (c *MetricInfoCollector) AddLabels(jobName, metricName string, lset labels.Labels) {
	mi := c.loadOrInitMetricInfo(jobName, metricName)
	mi.mu.Lock()
	defer mi.mu.Unlock()

	for _, l := range lset {
		if l.Name == "__name__" {
			continue
		}
		if _, ok := mi.ExistsLabels[l.Name]; ok {
			continue
		}
		mi.Labels = append(mi.Labels, l.Name)
		mi.ExistsLabels[l.Name] = true
	}
}

// AddSubsystemInfo 为指定作业和指标添加子系统信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.SubsystemId 字段的并发访问
func (c *MetricInfoCollector) AddSubsystemInfo(jobName, metricName string, subsystemId string) {
	mi := c.loadOrInitMetricInfo(jobName, metricName)
	mi.mu.Lock()
	mi.SubsystemId = subsystemId
	mi.mu.Unlock()
}

// AddType 为指定作业和指标添加类型信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Type 字段的并发访问
func (c *MetricInfoCollector) AddType(jobName, metricName, mType string) {
	mi := c.loadOrInitMetricInfo(jobName, metricName)
	mi.mu.Lock()
	mi.Type = mType
	mi.mu.Unlock()
}

func (c *MetricInfoCollector) loadOrInitJobData(jobName string) *sync.Map {
	jobData, ok := c.data.Load(jobName)
	if ok {
		return jobData.(*sync.Map)
	}
	d := &sync.Map{}
	c.data.Store(jobName, d)
	return d
}

func (c *MetricInfoCollector) loadOrInitMetricInfo(jobName, metricName string) *MetricInfo {
	jobData := c.loadOrInitJobData(jobName)
	metricInfo, ok := jobData.Load(metricName)
	if !ok {
		mi := NewMetricInfo(metricName)
		jobData.Store(metricName, mi)
		return mi
	} else {
		return metricInfo.(*MetricInfo)
	}
}

// dumpJobMetrics 将指定作业的指标信息导出到文件
// 该方法是线程安全的，使用读锁保护对 MetricInfo 各字段的并发访问
// 导出完成后会从内存中删除该作业的数据
// 导出格式为：子系统ID|指标名称|类型|单位|帮助信息|标签列表
func (c *MetricInfoCollector) dumpJobMetrics(jobName string) error {
	var rows []string
	data := c.loadOrInitJobData(jobName)
	defer c.data.Delete(jobName)
	data.Range(func(key, value interface{}) bool {
		//metricName := key.(string)
		mi := value.(*MetricInfo)
		mi.mu.RLock()
		//rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s | %s |", mi.Name, mi.Type, mi.Unit, mi.Help, strings.Join(mi.Labels, ",")))
		rows = append(rows, fmt.Sprintf("%s|%s|%s|%s|%s|%s", mi.SubsystemId, mi.Name, mi.Type, mi.Unit, mi.Help, strings.Join(mi.Labels, ",")))
		mi.mu.RUnlock()
		return true
	})
	//header := "| Name | Type | Unit | Help | Labels |"
	//border := "| --- | --- | --- | --- | --- |"
	if len(rows) == 0 {
		return nil
	}
	//dumpData := strings.Join([]string{header, border, strings.Join(rows, "\n")}, "\n")
	dumpData := strings.Join(rows, "\n")
	filename := fmt.Sprintf("/tmp/%s.md", jobName)
	return ioutil.WriteFile(filename, []byte(dumpData), 0644)
}

func Log(msg string, args ...interface{}) {
	fmt.Printf("%s %s\n", time.Now().String(), fmt.Sprintf(msg, args...))
}

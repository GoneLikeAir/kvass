package scrape

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestNewMetricInfo(t *testing.T) {
	r := require.New(t)

	// 测试创建 MetricInfo
	mi := NewMetricInfo("test_metric")
	r.NotNil(mi)
	r.Equal("test_metric", mi.Name)
	r.NotNil(mi.ExistsLabels)
	r.NotNil(mi.Labels)
	r.Equal(0, len(mi.Labels))
}

func TestInitMetricCollector(t *testing.T) {
	r := require.New(t)

	// 测试不启动后台任务
	mc := InitMetricCollector("", false)
	r.NotNil(mc)
	r.Equal("", mc.configPath)
	r.Equal(int32(0), mc.currency.Load())

	// 测试启动后台任务
	mc2 := InitMetricCollector("", true)
	r.NotNil(mc2)
}

func TestMetricInfoCollector_GetTargetInfo(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		targetInfo: sync.Map{},
	}

	// 测试没有找到目标信息
	info := mc.GetTargetInfo("localhost", "/metrics")
	r.Nil(info)

	// 测试找到目标信息
	targetInfo := &TargetInfo{
		SubsystemId: "test_subsystem",
		Labels:      labels.FromStrings("env", "test"),
		UpdateTime:  time.Now(),
	}
	mc.targetInfo.Store("localhost/metrics", targetInfo)

	info = mc.GetTargetInfo("localhost", "/metrics")
	r.NotNil(info)
	r.Equal("test_subsystem", info.SubsystemId)
}

func TestMetricInfoCollector_getSubsystem(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{}

	// 测试各种 subsystem 字段
	testCases := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name: "subsystem",
			labels: map[string]string{
				"subsystem": "test1",
			},
			expected: "test1",
		},
		{
			name: "subsystem_id",
			labels: map[string]string{
				"subsystem_id": "test2",
			},
			expected: "test2",
		},
		{
			name: "subsystem_name",
			labels: map[string]string{
				"subsystem_name": "test3",
			},
			expected: "test3",
		},
		{
			name: "subsystemId",
			labels: map[string]string{
				"subsystemId": "test4",
			},
			expected: "test4",
		},
		{
			name: "subsystemName",
			labels: map[string]string{
				"subsystemName": "test5",
			},
			expected: "test5",
		},
		{
			name: "no subsystem",
			labels: map[string]string{
				"other": "value",
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mc.getSubsystem(tc.labels)
			r.Equal(tc.expected, result)
		})
	}
}

func TestMetricInfoCollector_syncTargetInfoOnce(t *testing.T) {
	r := require.New(t)

	// 创建临时配置文件
	tmpFile, err := ioutil.TempFile("", "test_config_*.yaml")
	r.NoError(err)
	defer os.Remove(tmpFile.Name())

	configContent := `
scrape_configs:
  - job_name: 'test_job'
    static_configs:
      - targets: ['localhost:9090']
        labels:
          __address__: 'localhost:9090'
          __metrics_path__: '/metrics'
          subsystem: 'test_subsystem'
          env: 'test'
`

	_, err = tmpFile.WriteString(configContent)
	r.NoError(err)
	tmpFile.Close()

	mc := &MetricInfoCollector{
		targetInfo: sync.Map{},
		configPath: tmpFile.Name(),
	}

	// 测试同步目标信息
	mc.syncTargetInfoOnce()

	// 验证目标信息已存储
	info := mc.GetTargetInfo("localhost:9090", "/metrics")
	r.NotNil(info)
	r.Equal("test_subsystem", info.SubsystemId)
}

func TestMetricInfoCollector_discoverOnce(t *testing.T) {
	r := require.New(t)

	// 创建临时任务文件
	tmpFile, err := ioutil.TempFile("", "task_*.json")
	r.NoError(err)
	defer os.Remove(tmpFile.Name())

	tasks := []*CollectTask{
		{
			JobName:        "test_job1",
			ScrapeInterval: model.Duration(time.Second * 30),
		},
		{
			JobName:        "test_job2",
			ScrapeInterval: model.Duration(time.Minute),
		},
	}

	taskData, err := json.Marshal(tasks)
	r.NoError(err)

	err = ioutil.WriteFile("/tmp/task.json", taskData, 0644)
	r.NoError(err)

	mc := &MetricInfoCollector{
		waiting:    sync.Map{},
		collecting: sync.Map{},
	}

	// 测试发现任务
	mc.discoverOnce()

	// 验证任务已添加到等待队列
	_, ok := mc.waiting.Load("test_job1")
	r.True(ok)

	_, ok = mc.waiting.Load("test_job2")
	r.True(ok)

	// 验证临时文件已被删除
	_, err = os.Stat("/tmp/task.json")
	r.True(os.IsNotExist(err))
}

func TestMetricInfoCollector_NeedCollect(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		collecting: sync.Map{},
	}

	// 测试没有正在收集的任务
	r.False(mc.NeedCollect("test_job"))

	// 测试有正在收集的任务
	task := &CollectTask{
		JobName: "test_job",
	}
	mc.collecting.Store("test_job", task)

	r.True(mc.NeedCollect("test_job"))
}

func TestMetricInfoCollector_checkTaskOnce(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		collecting: sync.Map{},
		waiting:    sync.Map{},
		currency:   atomic.NewInt32(0),
	}

	// 添加一个已完成的任务
	oldTask := &CollectTask{
		JobName:        "old_task",
		ScrapeInterval: model.Duration(time.Second),
		StartTime:      time.Now().Add(-time.Minute * 5), // 5分钟前开始
	}
	mc.collecting.Store("old_task", oldTask)
	mc.currency.Add(int32(1))

	// 添加一个等待中的任务
	waitingTask := &CollectTask{
		JobName:        "waiting_task",
		ScrapeInterval: model.Duration(time.Second),
	}
	mc.waiting.Store("waiting_task", waitingTask)

	// 测试检查任务
	mc.checkTaskOnce()

	// 验证旧任务已被移除
	_, ok := mc.collecting.Load("old_task")
	r.False(ok)

	// 验证等待任务已被移到收集队列
	_, ok = mc.collecting.Load("waiting_task")
	r.True(ok)

	_, ok = mc.waiting.Load("waiting_task")
	r.False(ok)

	// 验证计数器
	r.Equal(int32(1), mc.currency.Load())
}

func TestMetricInfoCollector_AddMethods(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		data: sync.Map{},
	}

	jobName := "test_job"
	metricName := "test_metric"

	// 测试 AddHelp
	mc.AddHelp(jobName, metricName, "test help")

	// 测试 AddUnit
	mc.AddUnit(jobName, metricName, "test unit")

	// 测试 AddType
	mc.AddType(jobName, metricName, "counter")

	// 测试 AddSubsystemInfo
	mc.AddSubsystemInfo(jobName, metricName, "test_subsystem")

	// 测试 AddLabels
	lset := labels.FromStrings("label1", "value1", "label2", "value2")
	mc.AddLabels(jobName, metricName, lset)

	// 验证数据已存储
	jobData := mc.loadOrInitJobData(jobName)
	mi, ok := jobData.Load(metricName)
	r.True(ok)

	metricInfo := mi.(*MetricInfo)
	metricInfo.mu.RLock()
	defer metricInfo.mu.RUnlock()

	r.Equal("test help", metricInfo.Help)
	r.Equal("test unit", metricInfo.Unit)
	r.Equal("counter", metricInfo.Type)
	r.Equal("test_subsystem", metricInfo.SubsystemId)
	r.Contains(metricInfo.Labels, "label1")
	r.Contains(metricInfo.Labels, "label2")
}

func TestMetricInfoCollector_loadOrInitJobData(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		data: sync.Map{},
	}

	jobName := "test_job"

	// 测试初始化新作业数据
	jobData := mc.loadOrInitJobData(jobName)
	r.NotNil(jobData)

	// 验证数据已存储
	loadedData, ok := mc.data.Load(jobName)
	r.True(ok)
	r.Equal(jobData, loadedData.(*sync.Map))

	// 测试获取已存在的作业数据
	jobData2 := mc.loadOrInitJobData(jobName)
	r.Equal(jobData, jobData2)
}

func TestMetricInfoCollector_loadOrInitMetricInfo(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		data: sync.Map{},
	}

	jobName := "test_job"
	metricName := "test_metric"

	// 测试初始化新的指标信息
	mi := mc.loadOrInitMetricInfo(jobName, metricName)
	r.NotNil(mi)
	r.Equal(metricName, mi.Name)

	// 验证数据已存储
	jobData := mc.loadOrInitJobData(jobName)
	loadedMI, ok := jobData.Load(metricName)
	r.True(ok)
	r.Equal(mi, loadedMI.(*MetricInfo))

	// 测试获取已存在的指标信息
	mi2 := mc.loadOrInitMetricInfo(jobName, metricName)
	r.Equal(mi, mi2)
}

func TestMetricInfoCollector_dumpJobMetrics(t *testing.T) {
	r := require.New(t)

	mc := &MetricInfoCollector{
		data: sync.Map{},
	}

	jobName := "test_job"

	// 添加一些指标数据
	mc.AddHelp(jobName, "metric1", "help1")
	mc.AddUnit(jobName, "metric1", "unit1")
	mc.AddType(jobName, "metric1", "counter")
	mc.AddSubsystemInfo(jobName, "metric1", "subsystem1")

	lset := labels.FromStrings("label1", "value1")
	mc.AddLabels(jobName, "metric1", lset)

	mc.AddHelp(jobName, "metric2", "help2")
	mc.AddUnit(jobName, "metric2", "unit2")
	mc.AddType(jobName, "metric2", "gauge")
	mc.AddSubsystemInfo(jobName, "metric2", "subsystem2")

	// 测试转储作业指标
	err := mc.dumpJobMetrics(jobName)
	r.NoError(err)

	// 验证文件已创建
	filename := "/tmp/" + jobName + ".md"
	content, err := ioutil.ReadFile(filename)
	r.NoError(err)

	// 验证内容
	contentStr := string(content)
	r.Contains(contentStr, "metric1")
	r.Contains(contentStr, "metric2")
	r.Contains(contentStr, "help1")
	r.Contains(contentStr, "help2")
	r.Contains(contentStr, "unit1")
	r.Contains(contentStr, "unit2")
	r.Contains(contentStr, "counter")
	r.Contains(contentStr, "gauge")
	r.Contains(contentStr, "subsystem1")
	r.Contains(contentStr, "subsystem2")
	r.Contains(contentStr, "label1")

	// 清理
	os.Remove(filename)
}

func TestLog(t *testing.T) {
	// 测试 Log 函数
	Log("test message %s", "arg")
	// 这个测试主要是确保函数不会 panic
}

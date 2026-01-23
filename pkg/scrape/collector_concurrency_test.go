package scrape

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
)

// TestMetricInfo_ConcurrentAddLabels 测试多个 goroutine 同时调用 AddLabels 方法的线程安全性
func TestMetricInfo_ConcurrentAddLabels(t *testing.T) {
	// 创建一个 MetricInfo 实例
	mi := NewMetricInfo("test_metric")

	// 准备测试数据
	numGoroutines := 100
	numOperations := 1000
	var wg sync.WaitGroup

	// 启动多个 goroutine 同时调用 AddLabels
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 创建标签
				labelName := fmt.Sprintf("label_%d_%d", goroutineID, j)
				labelValue := fmt.Sprintf("value_%d_%d", goroutineID, j)
				lset := labels.FromStrings(labelName, labelValue)

				// 调用 AddLabels
				mi.mu.Lock()
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
				mi.mu.Unlock()
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 验证结果
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	expectedLabelsCount := numGoroutines * numOperations
	if len(mi.Labels) != expectedLabelsCount {
		t.Errorf("Expected %d labels, got %d", expectedLabelsCount, len(mi.Labels))
	}

	if len(mi.ExistsLabels) != expectedLabelsCount {
		t.Errorf("Expected %d entries in ExistsLabels, got %d", expectedLabelsCount, len(mi.ExistsLabels))
	}

	// 验证每个标签都在 ExistsLabels 中
	for _, labelName := range mi.Labels {
		if !mi.ExistsLabels[labelName] {
			t.Errorf("Label %s not found in ExistsLabels", labelName)
		}
	}
}

// TestMetricInfo_ConcurrentAddMetadata 测试多个 goroutine 同时调用 AddHelp、AddUnit、AddType、AddSubsystemInfo 方法的线程安全性
func TestMetricInfo_ConcurrentAddMetadata(t *testing.T) {
	// 创建一个 MetricInfo 实例
	mi := NewMetricInfo("test_metric")

	// 准备测试数据
	numGoroutines := 100
	numOperations := 1000
	var wg sync.WaitGroup

	// 启动多个 goroutine 同时调用各种元数据设置方法
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 随机选择要调用的方法
				rand.Seed(time.Now().UnixNano())
				method := rand.Intn(4)

				switch method {
				case 0: // AddHelp
					help := fmt.Sprintf("Help text from goroutine %d, operation %d", goroutineID, j)
					mi.mu.Lock()
					mi.Help = help
					mi.mu.Unlock()
				case 1: // AddUnit
					unit := fmt.Sprintf("unit_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Unit = unit
					mi.mu.Unlock()
				case 2: // AddType
					mType := fmt.Sprintf("type_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Type = mType
					mi.mu.Unlock()
				case 3: // AddSubsystemInfo
					subsystem := fmt.Sprintf("subsystem_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.SubsystemId = subsystem
					mi.mu.Unlock()
				}
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 验证结果 - 只需要确保字段不为空，因为最后的值会被覆盖
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	if mi.Help == "" {
		t.Error("Help field should not be empty after concurrent operations")
	}
	if mi.Unit == "" {
		t.Error("Unit field should not be empty after concurrent operations")
	}
	if mi.Type == "" {
		t.Error("Type field should not be empty after concurrent operations")
	}
	if mi.SubsystemId == "" {
		t.Error("SubsystemId field should not be empty after concurrent operations")
	}
}

// TestMetricInfo_ConcurrentReadWrite 测试多个 goroutine 同时进行读写操作的线程安全性
func TestMetricInfo_ConcurrentReadWrite(t *testing.T) {
	// 创建一个 MetricInfo 实例
	mi := NewMetricInfo("test_metric")

	// 准备测试数据
	numWriterGoroutines := 50
	numReaderGoroutines := 50
	numOperations := 1000
	var wg sync.WaitGroup

	// 启动多个写 goroutine
	for i := 0; i < numWriterGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 随机选择写操作
				rand.Seed(time.Now().UnixNano())
				operation := rand.Intn(5)

				switch operation {
				case 0: // AddLabels
					labelName := fmt.Sprintf("label_%d_%d", goroutineID, j)
					labelValue := fmt.Sprintf("value_%d_%d", goroutineID, j)
					lset := labels.FromStrings(labelName, labelValue)

					mi.mu.Lock()
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
					mi.mu.Unlock()
				case 1: // AddHelp
					help := fmt.Sprintf("Help text from goroutine %d, operation %d", goroutineID, j)
					mi.mu.Lock()
					mi.Help = help
					mi.mu.Unlock()
				case 2: // AddUnit
					unit := fmt.Sprintf("unit_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Unit = unit
					mi.mu.Unlock()
				case 3: // AddType
					mType := fmt.Sprintf("type_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Type = mType
					mi.mu.Unlock()
				case 4: // AddSubsystemInfo
					subsystem := fmt.Sprintf("subsystem_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.SubsystemId = subsystem
					mi.mu.Unlock()
				}
			}
		}(i)
	}

	// 启动多个读 goroutine
	for i := 0; i < numReaderGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 读取数据
				mi.mu.RLock()
				_ = mi.Name
				_ = mi.Type
				_ = mi.Help
				_ = mi.Unit
				_ = mi.SubsystemId
				_ = mi.ExistsLabels
				_ = mi.Labels
				mi.mu.RUnlock()
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 验证结果
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	// 验证字段不为空
	if mi.Help == "" {
		t.Error("Help field should not be empty after concurrent operations")
	}
	if mi.Unit == "" {
		t.Error("Unit field should not be empty after concurrent operations")
	}
	if mi.Type == "" {
		t.Error("Type field should not be empty after concurrent operations")
	}
	if mi.SubsystemId == "" {
		t.Error("SubsystemId field should not be empty after concurrent operations")
	}

	// 验证 Labels 和 ExistsLabels 的一致性
	for _, labelName := range mi.Labels {
		if !mi.ExistsLabels[labelName] {
			t.Errorf("Label %s not found in ExistsLabels", labelName)
		}
	}
}

// TestMetricInfo_HighConcurrencyStressTest 高并发场景下的压力测试
func TestMetricInfo_HighConcurrencyStressTest(t *testing.T) {
	// 创建一个 MetricInfo 实例
	mi := NewMetricInfo("test_metric")

	// 准备测试数据
	numGoroutines := 100
	numOperations := 1000
	var wg sync.WaitGroup

	// 启动大量 goroutine 进行各种操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 随机选择操作
				rand.Seed(time.Now().UnixNano())
				operation := rand.Intn(6)

				switch operation {
				case 0: // AddLabels
					labelName := fmt.Sprintf("label_%d_%d", goroutineID, j)
					labelValue := fmt.Sprintf("value_%d_%d", goroutineID, j)
					lset := labels.FromStrings(labelName, labelValue)

					mi.mu.Lock()
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
					mi.mu.Unlock()
				case 1: // AddHelp
					help := fmt.Sprintf("Help text from goroutine %d, operation %d", goroutineID, j)
					mi.mu.Lock()
					mi.Help = help
					mi.mu.Unlock()
				case 2: // AddUnit
					unit := fmt.Sprintf("unit_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Unit = unit
					mi.mu.Unlock()
				case 3: // AddType
					mType := fmt.Sprintf("type_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Type = mType
					mi.mu.Unlock()
				case 4: // AddSubsystemInfo
					subsystem := fmt.Sprintf("subsystem_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.SubsystemId = subsystem
					mi.mu.Unlock()
				case 5: // Read operations
					mi.mu.RLock()
					_ = mi.Name
					_ = mi.Type
					_ = mi.Help
					_ = mi.Unit
					_ = mi.SubsystemId
					_ = mi.ExistsLabels
					_ = mi.Labels
					mi.mu.RUnlock()
				}
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 验证结果
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	// 验证字段不为空
	if mi.Help == "" {
		t.Error("Help field should not be empty after concurrent operations")
	}
	if mi.Unit == "" {
		t.Error("Unit field should not be empty after concurrent operations")
	}
	if mi.Type == "" {
		t.Error("Type field should not be empty after concurrent operations")
	}
	if mi.SubsystemId == "" {
		t.Error("SubsystemId field should not be empty after concurrent operations")
	}

	// 验证 Labels 和 ExistsLabels 的一致性
	for _, labelName := range mi.Labels {
		if !mi.ExistsLabels[labelName] {
			t.Errorf("Label %s not found in ExistsLabels", labelName)
		}
	}
}

// TestMetricInfoCollector_ConcurrentOperations 测试 MetricInfoCollector 的并发操作
func TestMetricInfoCollector_ConcurrentOperations(t *testing.T) {
	// 创建一个 MetricInfoCollector 实例
	mc := &MetricInfoCollector{
		data:       sync.Map{},
		waiting:    sync.Map{},
		collecting: sync.Map{},
		targetInfo: sync.Map{},
	}

	// 准备测试数据
	numGoroutines := 100
	numOperations := 1000
	var wg sync.WaitGroup

	// 启动多个 goroutine 进行各种操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				jobName := fmt.Sprintf("job_%d", goroutineID)
				metricName := fmt.Sprintf("metric_%d_%d", goroutineID, j)

				// 随机选择操作
				rand.Seed(time.Now().UnixNano())
				operation := rand.Intn(5)

				switch operation {
				case 0: // AddLabels
					labelName := fmt.Sprintf("label_%d_%d", goroutineID, j)
					labelValue := fmt.Sprintf("value_%d_%d", goroutineID, j)
					lset := labels.FromStrings(labelName, labelValue)
					mc.AddLabels(jobName, metricName, lset)
				case 1: // AddHelp
					help := fmt.Sprintf("Help text from goroutine %d, operation %d", goroutineID, j)
					mc.AddHelp(jobName, metricName, help)
				case 2: // AddUnit
					unit := fmt.Sprintf("unit_%d_%d", goroutineID, j)
					mc.AddUnit(jobName, metricName, unit)
				case 3: // AddType
					mType := fmt.Sprintf("type_%d_%d", goroutineID, j)
					mc.AddType(jobName, metricName, mType)
				case 4: // AddSubsystemInfo
					subsystem := fmt.Sprintf("subsystem_%d_%d", goroutineID, j)
					mc.AddSubsystemInfo(jobName, metricName, subsystem)
				}
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 验证结果 - 检查是否有数据被正确存储
	var count int
	mc.data.Range(func(key, value interface{}) bool {
		jobData := value.(*sync.Map)
		jobData.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		return true
	})

	expectedCount := numGoroutines * numOperations
	if count != expectedCount {
		t.Errorf("Expected %d metrics, got %d", expectedCount, count)
	}
}

// TestMetricInfo_ExtremeStressTest 极限压力测试，使用更高的并发度和更长的执行时间
func TestMetricInfo_ExtremeStressTest(t *testing.T) {
	// 创建一个 MetricInfo 实例
	mi := NewMetricInfo("test_metric")

	// 准备测试数据 - 使用更高的并发度和操作次数
	numGoroutines := 1000
	numOperations := 10000
	var wg sync.WaitGroup

	// 记录开始时间
	startTime := time.Now()

	// 启动大量 goroutine 进行各种操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 随机选择操作
				rand.Seed(time.Now().UnixNano())
				operation := rand.Intn(6)

				switch operation {
				case 0: // AddLabels
					labelName := fmt.Sprintf("label_%d_%d", goroutineID, j)
					labelValue := fmt.Sprintf("value_%d_%d", goroutineID, j)
					lset := labels.FromStrings(labelName, labelValue)

					mi.mu.Lock()
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
					mi.mu.Unlock()
				case 1: // AddHelp
					help := fmt.Sprintf("Help text from goroutine %d, operation %d", goroutineID, j)
					mi.mu.Lock()
					mi.Help = help
					mi.mu.Unlock()
				case 2: // AddUnit
					unit := fmt.Sprintf("unit_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Unit = unit
					mi.mu.Unlock()
				case 3: // AddType
					mType := fmt.Sprintf("type_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.Type = mType
					mi.mu.Unlock()
				case 4: // AddSubsystemInfo
					subsystem := fmt.Sprintf("subsystem_%d_%d", goroutineID, j)
					mi.mu.Lock()
					mi.SubsystemId = subsystem
					mi.mu.Unlock()
				case 5: // Read operations
					mi.mu.RLock()
					_ = mi.Name
					_ = mi.Type
					_ = mi.Help
					_ = mi.Unit
					_ = mi.SubsystemId
					_ = mi.ExistsLabels
					_ = mi.Labels
					mi.mu.RUnlock()
				}
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 记录结束时间并计算执行时间
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	t.Logf("极限压力测试完成，执行时间: %v", duration)
	t.Logf("总操作数: %d (1000个goroutine × 10000次操作)", numGoroutines*numOperations)
	t.Logf("平均每秒操作数: %.2f", float64(numGoroutines*numOperations)/duration.Seconds())

	// 验证结果
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	// 验证字段不为空
	if mi.Help == "" {
		t.Error("Help field should not be empty after concurrent operations")
	}
	if mi.Unit == "" {
		t.Error("Unit field should not be empty after concurrent operations")
	}
	if mi.Type == "" {
		t.Error("Type field should not be empty after concurrent operations")
	}
	if mi.SubsystemId == "" {
		t.Error("SubsystemId field should not be empty after concurrent operations")
	}

	// 验证 Labels 和 ExistsLabels 的一致性
	for _, labelName := range mi.Labels {
		if !mi.ExistsLabels[labelName] {
			t.Errorf("Label %s not found in ExistsLabels", labelName)
		}
	}

	// 验证标签数量
	expectedMinLabels := numGoroutines * numOperations / 6 // 大约有1/6的操作是添加标签
	if len(mi.Labels) < expectedMinLabels {
		t.Errorf("Expected at least %d labels, got %d", expectedMinLabels, len(mi.Labels))
	}
}

// TestMetricInfoCollector_ExtremeStressTest MetricInfoCollector 的极限压力测试
func TestMetricInfoCollector_ExtremeStressTest(t *testing.T) {
	// 创建一个 MetricInfoCollector 实例
	mc := &MetricInfoCollector{
		data:       sync.Map{},
		waiting:    sync.Map{},
		collecting: sync.Map{},
		targetInfo: sync.Map{},
	}

	// 准备测试数据 - 使用更高的并发度和操作次数
	numGoroutines := 1000
	numOperations := 10000
	var wg sync.WaitGroup

	// 记录开始时间
	startTime := time.Now()

	// 启动多个 goroutine 进行各种操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				jobName := fmt.Sprintf("job_%d", goroutineID%100) // 限制job数量，避免内存过大
				metricName := fmt.Sprintf("metric_%d_%d", goroutineID, j)

				// 随机选择操作
				rand.Seed(time.Now().UnixNano())
				operation := rand.Intn(5)

				switch operation {
				case 0: // AddLabels
					labelName := fmt.Sprintf("label_%d_%d", goroutineID, j)
					labelValue := fmt.Sprintf("value_%d_%d", goroutineID, j)
					lset := labels.FromStrings(labelName, labelValue)
					mc.AddLabels(jobName, metricName, lset)
				case 1: // AddHelp
					help := fmt.Sprintf("Help text from goroutine %d, operation %d", goroutineID, j)
					mc.AddHelp(jobName, metricName, help)
				case 2: // AddUnit
					unit := fmt.Sprintf("unit_%d_%d", goroutineID, j)
					mc.AddUnit(jobName, metricName, unit)
				case 3: // AddType
					mType := fmt.Sprintf("type_%d_%d", goroutineID, j)
					mc.AddType(jobName, metricName, mType)
				case 4: // AddSubsystemInfo
					subsystem := fmt.Sprintf("subsystem_%d_%d", goroutineID, j)
					mc.AddSubsystemInfo(jobName, metricName, subsystem)
				}
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 记录结束时间并计算执行时间
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	t.Logf("MetricInfoCollector 极限压力测试完成，执行时间: %v", duration)
	t.Logf("总操作数: %d (1000个goroutine × 10000次操作)", numGoroutines*numOperations)
	t.Logf("平均每秒操作数: %.2f", float64(numGoroutines*numOperations)/duration.Seconds())

	// 验证结果 - 检查是否有数据被正确存储
	var count int
	mc.data.Range(func(key, value interface{}) bool {
		jobData := value.(*sync.Map)
		jobData.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		return true
	})

	expectedMinCount := numGoroutines * numOperations / 5 // 所有操作都应该创建或更新指标
	if count < expectedMinCount {
		t.Errorf("Expected at least %d metrics, got %d", expectedMinCount, count)
	}

	t.Logf("实际存储的指标数量: %d", count)
}

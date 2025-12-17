# MetricInfo 并发安全使用指南

## 概述

本指南为开发者提供关于如何正确使用 MetricInfo 的并发安全特性的详细说明。MetricInfo 是 Kvass 项目中用于存储指标元数据的核心结构体，它通过 `sync.RWMutex` 实现了线程安全，允许多个读操作并发执行，而写操作需要独占访问。

## 并发安全保证

### 线程安全机制

MetricInfo 使用 `sync.RWMutex` 读写锁来保护并发访问：

```go
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
```

### 锁策略

- **读锁（RLock）**：允许多个 goroutine 同时读取数据
- **写锁（Lock）**：独占访问，阻止所有其他读写操作
- **细粒度锁**：每个 MetricInfo 实例都有自己的锁，减少锁竞争

## 正确的使用模式

### 1. 直接访问 MetricInfo 字段

当直接访问 MetricInfo 字段时，必须手动加锁：

```go
// 读取操作 - 使用读锁
mi.mu.RLock()
defer mi.mu.RUnlock()

name := mi.Name
metricType := mi.Type
help := mi.Help
unit := mi.Unit
subsystemId := mi.SubsystemId
labels := mi.Labels
existsLabels := mi.ExistsLabels

// 使用读取的数据...
fmt.Printf("Metric: %s, Type: %s\n", name, metricType)
```

```go
// 写入操作 - 使用写锁
mi.mu.Lock()
defer mi.mu.Unlock()

mi.Help = "New help text"
mi.Unit = "seconds"
mi.Type = "counter"
mi.SubsystemId = "subsystem_123"
```

### 2. 使用 MetricInfoCollector 的安全方法

MetricInfoCollector 提供了线程安��的方法来操作 MetricInfo，推荐使用这些方法：

```go
// 添加帮助信息 - 内部已处理并发安全
collector.AddHelp(jobName, metricName, "This is a help text")

// 添加单位信息 - 内部已处理并发安全
collector.AddUnit(jobName, metricName, "seconds")

// 添加标签信息 - 内部已处理并发安全
labels := labels.FromStrings("env", "production", "service", "api")
collector.AddLabels(jobName, metricName, labels)

// 添加子系统信息 - 内部已处理并发安全
collector.AddSubsystemInfo(jobName, metricName, "auth_service")

// 添加类型信息 - 内部已处理并发安全
collector.AddType(jobName, metricName, "gauge")
```

### 3. 批量操作模式

对于需要多个字段的批量操作，建议使用单个锁来减少锁竞争：

```go
// 正确的批量写入模式
mi.mu.Lock()
defer mi.mu.Unlock()

mi.Help = "Comprehensive help text"
mi.Unit = "milliseconds"
mi.Type = "histogram"
mi.SubsystemId = "database_service"

// 批量添加标签
newLabels := []string{"env", "service", "version"}
for _, label := range newLabels {
    if _, exists := mi.ExistsLabels[label]; !exists {
        mi.Labels = append(mi.Labels, label)
        mi.ExistsLabels[label] = true
    }
}
```

```go
// 正确的批量读取模式
mi.mu.RLock()
defer mi.mu.RUnlock()

// 一次性读取所有需要的数据
metricData := struct {
    Name         string
    Type         string
    Help         string
    Unit         string
    SubsystemId  string
    Labels       []string
    ExistsLabels map[string]bool
}{
    Name:         mi.Name,
    Type:         mi.Type,
    Help:         mi.Help,
    Unit:         mi.Unit,
    SubsystemId:  mi.SubsystemId,
    Labels:       make([]string, len(mi.Labels)),
    ExistsLabels: make(map[string]bool),
}

// 复制切片以避免在锁外访问
copy(metricData.Labels, mi.Labels)
for k, v := range mi.ExistsLabels {
    metricData.ExistsLabels[k] = v
}

// 在锁外使用数据
processMetricData(metricData)
```

## 避免的常见错误

### 1. 忘记加锁

```go
// 错误示例 - 直接访问字段而不加锁
func printMetricInfo(mi *MetricInfo) {
    fmt.Printf("Name: %s, Type: %s\n", mi.Name, mi.Type) // 竞态条件！
}

// 正确做法
func printMetricInfo(mi *MetricInfo) {
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    fmt.Printf("Name: %s, Type: %s\n", mi.Name, mi.Type)
}
```

### 2. 锁不匹配

```go
// 错误示例 - 使用读锁进行写操作
func updateMetricType(mi *MetricInfo, newType string) {
    mi.mu.RLock() // 应该使用 Lock()
    defer mi.mu.RUnlock()
    mi.Type = newType // 竞态条件！
}

// 正确做法
func updateMetricType(mi *MetricInfo, newType string) {
    mi.mu.Lock()
    defer mi.mu.Unlock()
    mi.Type = newType
}
```

### 3. 锁的粒度过大

```go
// 错误示例 - 锁的粒度过大，包含不必要的操作
func processAndPrintMetric(mi *MetricInfo) {
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    // 只需要读取的操作
    name := mi.Name
    metricType := mi.Type
    
    // 耗时的处理操作（不需要锁保护）
    result := expensiveCalculation(name, metricType)
    
    // 打印操作（不需要锁保护）
    fmt.Printf("Result: %s\n", result)
}

// 正确做法 - 最小化锁的范围
func processAndPrintMetric(mi *MetricInfo) {
    // 只在必要时加锁
    mi.mu.RLock()
    name := mi.Name
    metricType := mi.Type
    mi.mu.RUnlock()
    
    // 耗时操作在锁外执行
    result := expensiveCalculation(name, metricType)
    
    // 打印操作
    fmt.Printf("Result: %s\n", result)
}
```

### 4. 死锁风险

```go
// 错误示例 - 可能导致死锁
func updateTwoMetrics(mi1, mi2 *MetricInfo, newType string) {
    mi1.mu.Lock()
    mi2.mu.Lock() // 如果其他 goroutine 以相反顺序获取锁，可能导致死锁
    defer mi1.mu.Unlock()
    defer mi2.mu.Unlock()
    
    mi1.Type = newType
    mi2.Type = newType
}

// 正确做法 - 使用一致的锁顺序
func updateTwoMetrics(mi1, mi2 *MetricInfo, newType string) {
    // 确保总是以相同的顺序获取锁
    first, second := mi1, mi2
    if mi1.Name > mi2.Name { // 使用某种确定性顺序
        first, second = mi2, mi1
    }
    
    first.mu.Lock()
    second.mu.Lock()
    defer first.mu.Unlock()
    defer second.mu.Unlock()
    
    mi1.Type = newType
    mi2.Type = newType
}
```

## 性能考虑

### 1. 读多写少场景

MetricInfo 的读写锁设计特别适合读多写少的场景：

```go
// 高效的并发读取模式
func readMetricConcurrently(mi *MetricInfo, numReaders int) {
    var wg sync.WaitGroup
    
    for i := 0; i < numReaders; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            // 多个读操作可以并发执行
            mi.mu.RLock()
            defer mi.mu.RUnlock()
            
            // 读取操作
            _ = mi.Name
            _ = mi.Type
            _ = mi.Help
            
            // 模拟一些处理
            time.Sleep(time.Millisecond * 10)
        }(i)
    }
    
    wg.Wait()
}
```

### 2. 批量操作优化

对于频繁的写操作，建议批量处理以减少锁竞争：

```go
// 高效的批量标签添加
func addLabelsBatch(mi *MetricInfo, newLabels []string) {
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    // 批量处理，减少锁获取次数
    for _, label := range newLabels {
        if _, exists := mi.ExistsLabels[label]; !exists {
            mi.Labels = append(mi.Labels, label)
            mi.ExistsLabels[label] = true
        }
    }
}
```

### 3. 避免锁内耗时操作

```go
// 错误示例 - 在锁内执行耗时操作
func processMetricWithHeavyComputation(mi *MetricInfo) {
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    
    // 耗时的网络请求或复杂计算
    result := heavyNetworkCall(mi.Name)
    processData(result)
}

// 正确做法 - 将数据复制到锁外处理
func processMetricWithHeavyComputation(mi *MetricInfo) {
    mi.mu.RLock()
    name := mi.Name
    metricType := mi.Type
    mi.mu.RUnlock()
    
    // 在锁外执行耗时操作
    result := heavyNetworkCall(name)
    processData(result, metricType)
}
```

## 最佳实践建议

### 1. 使用 MetricInfoCollector 的安全方法

尽可能使用 MetricInfoCollector 提供的线程安全方法，而不是直接操作 MetricInfo：

```go
// 推荐：使用 MetricInfoCollector 的方法
collector.AddLabels(jobName, metricName, labels)
collector.AddHelp(jobName, metricName, helpText)

// 避免：直接操作 MetricInfo（除非必要）
mi := collector.loadOrInitMetricInfo(jobName, metricName)
mi.mu.Lock()
// 直接操作...
mi.mu.Unlock()
```

### 2. 最小化锁的持有时间

```go
// 好的做法：最小化锁的持有时间
func getMetricSummary(mi *MetricInfo) string {
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    
    // 只复制必要的数据
    name := mi.Name
    metricType := mi.Type
    labelCount := len(mi.Labels)
    
    // 在锁外进行字符串格式化
    return fmt.Sprintf("%s (%s) with %d labels", name, metricType, labelCount)
}
```

### 3. 使用 defer 确保锁释放

```go
// 好的做法：使用 defer 确保锁释放
func updateMetricSafely(mi *MetricInfo, newHelp, newUnit string) {
    mi.mu.Lock()
    defer mi.mu.Unlock() // 确保在任何情况下都会释放锁
    
    mi.Help = newHelp
    mi.Unit = newUnit
    
    // 即使这里发生 panic，锁也会被正确释放
    if someCondition() {
        panic("unexpected condition")
    }
}
```

### 4. 避免在锁内调用外部代码

```go
// 错误示例：在锁内调用可能阻塞的外部代码
func processMetricWithCallback(mi *MetricInfo, callback func(string) error) error {
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    
    // 回调函数可能阻塞或执行未知操作
    return callback(mi.Name) // 危险！
}

// 正确做法：先获取数据，再调用回调
func processMetricWithCallback(mi *MetricInfo, callback func(string) error) error {
    mi.mu.RLock()
    name := mi.Name
    mi.mu.RUnlock()
    
    // 在锁外调用回调
    return callback(name)
}
```

## 常见问题和解决方案

### 1. 如何安全地复制 MetricInfo？

```go
// 安全的 MetricInfo 复制方法
func copyMetricInfo(src *MetricInfo) *MetricInfo {
    src.mu.RLock()
    defer src.mu.RUnlock()
    
    dst := &MetricInfo{
        Name:         src.Name,
        Type:         src.Type,
        Help:         src.Help,
        Unit:         src.Unit,
        SubsystemId:  src.SubsystemId,
        ExistsLabels: make(map[string]bool),
        Labels:       make([]string, len(src.Labels)),
    }
    
    // 深拷贝 ExistsLabels
    for k, v := range src.ExistsLabels {
        dst.ExistsLabels[k] = v
    }
    
    // 深拷贝 Labels
    copy(dst.Labels, src.Labels)
    
    return dst
}
```

### 2. 如何实现原子性的多字段更新？

```go
// 原子性的多字段更新
func updateMetricAtomically(mi *MetricInfo, update MetricUpdate) {
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    // 所有更新在同一个锁内完成，保证原子性
    if update.Help != nil {
        mi.Help = *update.Help
    }
    if update.Unit != nil {
        mi.Unit = *update.Unit
    }
    if update.Type != nil {
        mi.Type = *update.Type
    }
    if update.SubsystemId != nil {
        mi.SubsystemId = *update.SubsystemId
    }
    if update.Labels != nil {
        for _, label := range update.Labels {
            if _, exists := mi.ExistsLabels[label]; !exists {
                mi.Labels = append(mi.Labels, label)
                mi.ExistsLabels[label] = true
            }
        }
    }
}

type MetricUpdate struct {
    Help        *string
    Unit        *string
    Type        *string
    SubsystemId *string
    Labels      []string
}
```

### 3. 如何处理高并发场景下的性能问题？

```go
// 高并发场景下的性能优化
type MetricCache struct {
    cache sync.Map // 缓存处理后的结果
}

func (mc *MetricCache) GetProcessedMetric(mi *MetricInfo, processor func(*MetricInfo) string) string {
    // 使用指标名称作为缓存键
    if result, ok := mc.cache.Load(mi.Name); ok {
        return result.(string)
    }
    
    // 缓存未命中，处理并缓存结果
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    
    result := processor(mi)
    mc.cache.Store(mi.Name, result)
    return result
}
```

## 示例代码

### 1. 完整的并发安全使用示例

```go
package main

import (
    "fmt"
    "sync"
    "time"
)

// ConcurrentMetricProcessor 演示如何安全地并发处理 MetricInfo
type ConcurrentMetricProcessor struct {
    collector *MetricInfoCollector
    cache     sync.Map
}

func NewConcurrentMetricProcessor(collector *MetricInfoCollector) *ConcurrentMetricProcessor {
    return &ConcurrentMetricProcessor{
        collector: collector,
    }
}

// ProcessMetricsConcurrently 并发处理多个指标
func (cmp *ConcurrentMetricProcessor) ProcessMetricsConcurrently(jobName string, metricNames []string) {
    var wg sync.WaitGroup
    
    for _, metricName := range metricNames {
        wg.Add(1)
        go func(name string) {
            defer wg.Done()
            cmp.processSingleMetric(jobName, name)
        }(metricName)
    }
    
    wg.Wait()
}

// processSingleMetric 处理单个指标
func (cmp *ConcurrentMetricProcessor) processSingleMetric(jobName, metricName string) {
    // 获取 MetricInfo（通过 collector 的安全方法）
    mi := cmp.collector.loadOrInitMetricInfo(jobName, metricName)
    
    // 安全地读取指标信息
    mi.mu.RLock()
    name := mi.Name
    metricType := mi.Type
    help := mi.Help
    labelCount := len(mi.Labels)
    mi.mu.RUnlock()
    
    // 在锁外进行耗时处理
    summary := fmt.Sprintf("Metric: %s, Type: %s, Help: %s, Labels: %d", 
        name, metricType, help, labelCount)
    
    // 缓存结果
    cmp.cache.Store(name, summary)
    
    fmt.Printf("Processed metric: %s\n", name)
}

// UpdateMetricsBatch 批量更新指标
func (cmp *ConcurrentMetricProcessor) UpdateMetricsBatch(jobName string, updates []MetricUpdate) {
    // 按指标分组以减少锁竞争
    metricUpdates := make(map[string][]MetricUpdate)
    for _, update := range updates {
        metricUpdates[update.MetricName] = append(metricUpdates[update.MetricName], update)
    }
    
    var wg sync.WaitGroup
    for metricName, updates := range metricUpdates {
        wg.Add(1)
        go func(name string, upd []MetricUpdate) {
            defer wg.Done()
            cmp.updateSingleMetric(jobName, name, upd)
        }(metricName, updates)
    }
    
    wg.Wait()
}

// updateSingleMetric 更新单个指标的所有字段
func (cmp *ConcurrentMetricProcessor) updateSingleMetric(jobName, metricName string, updates []MetricUpdate) {
    mi := cmp.collector.loadOrInitMetricInfo(jobName, metricName)
    
    // 使用单个锁进行所有更新
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    for _, update := range updates {
        if update.Help != nil {
            mi.Help = *update.Help
        }
        if update.Unit != nil {
            mi.Unit = *update.Unit
        }
        if update.Type != nil {
            mi.Type = *update.Type
        }
        if update.SubsystemId != nil {
            mi.SubsystemId = *update.SubsystemId
        }
        if update.Labels != nil {
            for _, label := range update.Labels {
                if _, exists := mi.ExistsLabels[label]; !exists {
                    mi.Labels = append(mi.Labels, label)
                    mi.ExistsLabels[label] = true
                }
            }
        }
    }
}

type MetricUpdate struct {
    MetricName  string
    Help        *string
    Unit        *string
    Type        *string
    SubsystemId *string
    Labels      []string
}
```

### 2. 高并发场景下的性能测试示例

```go
// BenchmarkConcurrentAccess 并发访问性能测试
func BenchmarkConcurrentAccess(b *testing.B) {
    mi := NewMetricInfo("test_metric")
    
    // 预填充一些数据
    mi.mu.Lock()
    mi.Help = "Test metric help"
    mi.Type = "gauge"
    mi.Unit = "seconds"
    mi.mu.Unlock()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // 并发读取
            mi.mu.RLock()
            _ = mi.Name
            _ = mi.Type
            _ = mi.Help
            _ = mi.Unit
            mi.mu.RUnlock()
        }
    })
}

// BenchmarkConcurrentUpdates 并发更新性能测试
func BenchmarkConcurrentUpdates(b *testing.B) {
    mi := NewMetricInfo("test_metric")
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            // 并发写入
            mi.mu.Lock()
            mi.Help = fmt.Sprintf("Help text %d", i)
            mi.Unit = fmt.Sprintf("unit_%d", i)
            mi.mu.Unlock()
            i++
        }
    })
}
```

## 总结

MetricInfo 的并发安全设计为开发者提供了强大的多线程支持，但正确使用这些特性需要遵循一定的原则：

1. **理解锁机制**：明确读写锁的使用场景和限制
2. **最小化锁范围**：减少锁的持有时间，提高并发性能
3. **使用安全方法**：优先使用 MetricInfoCollector 提供的线程安全方法
4. **避免常见错误**：注意死锁、竞态条件等并发问题
5. **性能优化**：针对具体场景选择合适的并发策略

通过遵循本指南的建议和最佳实践，开发者可以安全、高效地使用 MetricInfo 的并发安全特性，构建稳定可靠的监控系统。
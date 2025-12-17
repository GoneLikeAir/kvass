# MetricCollector 功能和机制文档

## 概述

MetricCollector（MetricInfoCollector）是 Kvass 项目中的一个核心组件，负责收集、管理和存储 Prometheus 指标的元数据信息。它的主要作用是在指标抓取过程中收集指标的详细信息，包括指标名称、类型、帮助信息、单位和标签等，并将这些信息持久化存储，为后续的指标分析和管理提供支持。

MetricCollector 的重要性体现在以下几个方面：

1. **指标元数据管理**：集中管理所有指标的元数据，提供统一的查询接口
2. **自动化信息收集**：在指标抓取过程中自动收集指标信息，无需手动配置
3. **任务调度**：管理指标收集任务的调度和执行
4. **目标信息同步**：定期同步 Prometheus 配置中的目标信息

## 核心功能

MetricCollector 提供以下核心功能：

### 1. 指标信息收集
- 收集指标名称、类型、帮助信息、单位和标签
- 支持动态添加和更新指标信息
- 自动去重和合并标签信息

### 2. 任务管理
- 发现和管理指标收集任务
- 控制并发收集任务的数量（最多5个）
- 监控任务执行状态和超时处理

### 3. 目标信息同步
- 定期从 Prometheus 配置文件中同步目标信息
- 解析和提取子系统信息
- 缓存目标标签和元数据

### 4. 数据持久化
- 将收集的指标信息导出为 Markdown 文件
- 按任务名称组织输出文件
- 支持自定义输出格式

## 数据结构

### MetricInfoCollector

```go
type MetricInfoCollector struct {
    data       sync.Map      // 存储所有作业的指标信息
    waiting    sync.Map      // 存储等待执行的任务
    collecting sync.Map      // 存储正在执行的任务
    targetInfo sync.Map      // 存储目标信息
    currency   *atomic.Int32 // 当前并发任务计数器
    configPath string        // Prometheus 配置文件路径
}
```

### MetricInfo

```go
// MetricInfo 存储指标的元数据信息，包括名称、类型、帮助信息、单位、子系统ID和标签等。
// 该结构体是线程安全的，使用 sync.RWMutex 保护并发访问。
// 读操作可以并发执行，写操作需要独占访问。
// 每个 MetricInfo 实例都有自己的锁，采用细粒度锁策略，减少锁竞争。
type MetricInfo struct {
    Name         string            // 指标名称
    Type         string            // 指标类型（counter, gauge, histogram等）
    Help         string            // 指标帮助信息
    Unit         string            // 指标单位
    SubsystemId  string            // 子系统ID
    ExistsLabels map[string]bool   // 已存在的标签集合，用于去重
    Labels       []string          // 标签名称列表
    // mu 用于保护 MetricInfo 结构体的并发访问
    // 使用读写锁允许多个读操作并发执行，写操作需要独占访问
    mu sync.RWMutex
}
```

### CollectTask

```go
type CollectTask struct {
    JobName        string         // 作业名称
    ScrapeInterval model.Duration // 抓取间隔
    StartTime      time.Time      // 开始时间
}
```

### TargetInfo

```go
type TargetInfo struct {
    SubsystemId   string         // 子系统ID
    SubsystemName string         // 子系统名称
    Labels        labels.Labels  // 标签集合
    UpdateTime    time.Time      // 更新时间
}
```

## 工作流程

### 1. 初始化流程

```go
func InitMetricCollector(configPath string, runTask ...bool) *MetricInfoCollector
```

1. 创建 MetricInfoCollector 实例，初始化各个 sync.Map 和计数器
2. 如果 runTask 参数为 true（默认），启动三个后台协程：
   - `taskDiscovering()`: 每30秒发现新任务
   - `checkTask()`: 每20秒检查任务状态
   - `syncTargetInfo()`: 每3分钟同步目标信息

### 2. 任务发现流程

```go
func (c *MetricInfoCollector) taskDiscovering()
func (c *MetricInfoCollector) discoverOnce()
```

1. 每30秒检查 `/tmp/task.json` 文件是否存在
2. 如果文件存在，读取并解析任务列表
3. 将新任务添加到等待队列（waiting sync.Map）
4. 删除任务文件

### 3. 任务检查与执行流程

```go
func (c *MetricInfoCollector) checkTask()
func (c *MetricInfoCollector) checkTaskOnce()
```

1. 每20秒检查正在执行的任务
2. 对于超时任务（超过抓取间隔的3倍），将其从执行队列移除，并导出指标信息
3. 从等待队列中取出任务（最多并发5个），移动到执行队列
4. 更新 `/tmp/hasTaskRunning` 文件，标记是否有任务在运行

### 4. 目标信息同步流程

```go
func (c *MetricInfoCollector) syncTargetInfo()
func (c *MetricInfoCollector) syncTargetInfoOnce()
```

1. 每3分钟读取 Prometheus 配置文件
2. 解析配置中的静态配置
3. 提取目标地址、路径和标签信息
4. 识别子系统ID（从多个可能的标签字段中获取）
5. 将目标信息存储到 targetInfo sync.Map 中

### 5. 指标信息收集流程

MetricCollector 提供多个方法来收集不同类型的指标信息：

```go
// 添加帮助信息
// 线程安全：是，使用写锁保护对 MetricInfo.Help 字段的并发访问
func (c *MetricInfoCollector) AddHelp(jobName, metricName, helpInfo string)

// 添加单位信息
// 线程安全：是，使用写锁保护对 MetricInfo.Unit 字段的并发访问
func (c *MetricInfoCollector) AddUnit(jobName, metricName, unit string)

// 添加标签信息
// 线程安全：是，使用写锁保护对 MetricInfo.Labels 和 MetricInfo.ExistsLabels 字段的并发访问
// 会过滤掉已存在的标签和 __name__ 标签
func (c *MetricInfoCollector) AddLabels(jobName, metricName string, lset labels.Labels)

// 添加子系统信息
// 线程安全：是，使用写锁保护对 MetricInfo.SubsystemId 字段的并发访问
func (c *MetricInfoCollector) AddSubsystemInfo(jobName, metricName string, subsystemId string)

// 添加指标类型
// 线程安全：是，使用写锁保护对 MetricInfo.Type 字段的并发访问
func (c *MetricInfoCollector) AddType(jobName, metricName, mType string)
```

这些方法都通过 `loadOrInitMetricInfo` 函数来获取或创建 MetricInfo 实例，然后更新相应的字段。所有方法都是线程安全的，可以在并发环境中安全使用。

### 6. 数据导出流程

```go
// dumpJobMetrics 将指定作业的指标信息导出到文件
// 线程安全：是，使用读锁保护对 MetricInfo 各字段的并发访问
// 导出完成后会从内存中删除该作业的数据
func (c *MetricInfoCollector) dumpJobMetrics(jobName string) error
```

1. 从 data sync.Map 中获取指定作业的所有指标信息
2. 将每个指标信息格式化为字符串行
3. 将所有行合并为字符串
4. 写入 `/tmp/{jobName}.md` 文件
5. 从 data sync.Map 中删除该作业的数据

## API 参考

### MetricInfoCollector 方法

#### 初始化方法

```go
// InitMetricCollector 初始化 MetricCollector 实例
// 线程安全：是
// 参数：
//   - configPath: Prometheus 配置文件路径
//   - runTask: 是否启动后台任务（可选，默认为 true）
// 返回值：
//   - *MetricInfoCollector: MetricCollector 实例
func InitMetricCollector(configPath string, runTask ...bool) *MetricInfoCollector
```

#### 指标信息操作方法

```go
// AddHelp 为指定作业和指标添加帮助信息
// 线程安全：是，使用写锁保护对 MetricInfo.Help 字段的并发访问
// 参数：
//   - jobName: 作业名称
//   - metricName: 指标名称
//   - helpInfo: 帮助信息
func (c *MetricInfoCollector) AddHelp(jobName, metricName, helpInfo string)

// AddUnit 为指定作业和指标添加单位信息
// 线程安全：是，使用写锁保护对 MetricInfo.Unit 字段的并发访问
// 参数：
//   - jobName: 作业名称
//   - metricName: 指标名称
//   - unit: 单位信息
func (c *MetricInfoCollector) AddUnit(jobName, metricName, unit string)

// AddLabels 为指定作业和指标添加标签信息
// 线程安全：是，使用写锁保护对 MetricInfo.Labels 和 MetricInfo.ExistsLabels 字段的并发访问
// 会过滤掉已存在的标签和 __name__ 标签
// 参数：
//   - jobName: 作业名称
//   - metricName: 指标名称
//   - lset: 标签集合
func (c *MetricInfoCollector) AddLabels(jobName, metricName string, lset labels.Labels)

// AddSubsystemInfo 为指定作业和指标添加子系统信息
// 线程安全：是，使用写锁保护对 MetricInfo.SubsystemId 字段的并发访问
// 参数：
//   - jobName: 作业名称
//   - metricName: 指标名称
//   - subsystemId: 子系统ID
func (c *MetricInfoCollector) AddSubsystemInfo(jobName, metricName string, subsystemId string)

// AddType 为指定作业和指标添加类型信息
// 线程安全：是，使用写锁保护对 MetricInfo.Type 字段的并发访问
// 参数：
//   - jobName: 作业名称
//   - metricName: 指标名称
//   - mType: 指标类型
func (c *MetricInfoCollector) AddType(jobName, metricName, mType string)
```

#### 任务管理方法

```go
// NeedCollect 检查指定作业是否正在收集
// 线程安全：是，使用 sync.Map 的并发安全特性
// 参数：
//   - jobName: 作业名称
// 返回值：
//   - bool: 是否正在收集
func (c *MetricInfoCollector) NeedCollect(jobName string) bool

// GetTargetInfo 获取目标信息
// 线程安全：是，使用 sync.Map 的并发安全特性
// 参数：
//   - host: 主机地址
//   - path: 指标路径
// 返回值：
//   - *TargetInfo: 目标信息，如果不存在则返回 nil
func (c *MetricInfoCollector) GetTargetInfo(host, path string) *TargetInfo
```

#### 内部方法

```go
// loadOrInitMetricInfo 加载或初始化 MetricInfo 实例
// 线程安全：是，使用 sync.Map 的并发安全特性
// 参数：
//   - jobName: 作业名称
//   - metricName: 指标名称
// 返回值：
//   - *MetricInfo: MetricInfo 实例
func (c *MetricInfoCollector) loadOrInitMetricInfo(jobName, metricName string) *MetricInfo

// dumpJobMetrics 将指定作业的指标信息导出到文件
// 线程安全：是，使用读锁保护对 MetricInfo 各字段的并发访问
// 导出完成后会从内存中删除该作业的数据
// 参数：
//   - jobName: 作业名称
// 返回值：
//   - error: 错误信息
func (c *MetricInfoCollector) dumpJobMetrics(jobName string) error
```

### MetricInfo 方法

#### 直接访问方法（需要手动加锁）

```go
// 读取操作示例 - 使用读锁
// 线程安全：需要手动加锁
func (mi *MetricInfo) ReadFields() (name, metricType, help, unit, subsystemId string, labels []string, existsLabels map[string]bool) {
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    
    return mi.Name, mi.Type, mi.Help, mi.Unit, mi.SubsystemId, mi.Labels, mi.ExistsLabels
}

// 写入操作示例 - 使用写锁
// 线程安全：需要手动加锁
func (mi *MetricInfo) UpdateFields(help, unit, metricType, subsystemId string) {
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    mi.Help = help
    mi.Unit = unit
    mi.Type = metricType
    mi.SubsystemId = subsystemId
}

// 批量添加标签 - 使用写锁
// 线程安全：需要手动加锁
func (mi *MetricInfo) AddLabelsBatch(newLabels []string) {
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    for _, label := range newLabels {
        if _, exists := mi.ExistsLabels[label]; !exists {
            mi.Labels = append(mi.Labels, label)
            mi.ExistsLabels[label] = true
        }
    }
}
```

### 并发安全注意事项

1. **MetricInfoCollector 的公共方法都是线程安全的**，可以直接在并发环境中使用
2. **直接访问 MetricInfo 字段需要手动加锁**，使用 `mi.mu.RLock()` 进行读操作，`mi.mu.Lock()` 进行写操作
3. **建议使用 defer 确保锁释放**，避免死锁
4. **最小化锁的持有时间**，将耗时操作放在锁外执行
5. **批量操作时使用单个锁**，减少锁竞争

## 关键特性

### 1. 并发安全
- 使用 sync.Map 实现线程安全的数据存储
- 使用 atomic.Int32 实现线程安全的计数器
- MetricInfo 结构体使用 sync.RWMutex 实现细粒度的读写锁保护
- 所有公共方法都是并发安全的
- 读操作可以并发执行，写操作需要独占访问
- 每个 MetricInfo 实例都有自己的锁，减少锁竞争

### 1.1 MetricInfo 并发安全机制

MetricInfo 使用 `sync.RWMutex` 读写锁来保护并发访问：

- **读锁（RLock）**：允许多个 goroutine 同时读取数据
- **写锁（Lock）**：独占访问，阻止所有其他读写操作
- **细粒度锁**：每个 MetricInfo 实例都有自己的锁，减少锁竞争

### 1.2 线程安全方法说明

MetricInfoCollector 提供的线程安全方法：

```go
// AddHelp 为指定作业和指标添加帮助信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Help 字段的并发访问
func (c *MetricInfoCollector) AddHelp(jobName, metricName, helpInfo string)

// AddUnit 为指定作业和指标添加单位信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Unit 字段的并发访问
func (c *MetricInfoCollector) AddUnit(jobName, metricName, unit string)

// AddLabels 为指定作业和指标添加标签信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Labels 和 MetricInfo.ExistsLabels 字段的并发访问
// 会过滤掉已存在的标签和 __name__ 标签
func (c *MetricInfoCollector) AddLabels(jobName, metricName string, lset labels.Labels)

// AddSubsystemInfo 为指定作业和指标添加子系统信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.SubsystemId 字段的并发访问
func (c *MetricInfoCollector) AddSubsystemInfo(jobName, metricName string, subsystemId string)

// AddType 为指定作业和指标添加类型信息
// 该方法是线程安全的，使用写锁保护对 MetricInfo.Type 字段的并发访问
func (c *MetricInfoCollector) AddType(jobName, metricName, mType string)

// dumpJobMetrics 将指定作业的指标信息导出到文件
// 该方法是线程安全的，使用读锁保护对 MetricInfo 各字段的并发访问
// 导出完成后会从内存中删除该作业的数据
func (c *MetricInfoCollector) dumpJobMetrics(jobName string) error
```

### 1.3 直接访问 MetricInfo 的并发安全模式

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

### 1.4 性能影响分析

MetricInfo 的并发安全设计对性能的影响：

1. **读多写少场景优化**：
   - 读写锁允许多个读操作并发执行，适合指标信息读取频繁的场景
   - 读操作不会相互阻塞，显著提高并发读取性能

2. **写操作性能**：
   - 写操作需要独占访问，会阻塞所有其他读写操作
   - 建议批量更新多个字段以减少锁竞争

3. **内存开销**：
   - 每个 MetricInfo 实例都有一个 RWMutex，增加少量内存开销
   - 相比全局锁，细粒度锁策略减少了锁竞争，提高了整体性能

4. **锁竞争优化**：
   - 每个 MetricInfo 实例独立加锁，不同指标的更新不会相互影响
   - 使用 sync.Map 存储作业数据，进一步减少锁竞争

### 1.5 并发安全最佳实践

1. **优先使用 MetricInfoCollector 的安全方法**：
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

2. **最小化锁的持有时间**：
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

3. **批量操作优化**：
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

### 2. 自动化任务管理
- 自动发现和执行收集任务
- 自动处理任务超时和清理
- 限制并发任务数量，防止资源耗尽

### 3. 灵活的子系统识别
- 支持从多个标签字段中识别子系统ID
- 包括：subsystem, subsystem_id, subsystem_name, subsystemId, subsystemName

### 4. 标签去重机制
- 使用 ExistsLabels map 记录已存在的标签
- 避免重复添加相同标签

### 5. 资源管理
- 任务完成后自动清理内存中的数据
- 定期同步目标信息，保持数据新鲜度

## 代码示例

### 1. 初始化 MetricCollector

```go
// 初始化 MetricCollector，指定 Prometheus 配置文件路径
collector := InitMetricCollector("/etc/prometheus/prometheus.yml")

// 初始化但不启动后台任务
collector := InitMetricCollector("/etc/prometheus/prometheus.yml", false)
```

### 2. 添加指标信息（并发安全方式）

```go
// 添加指标类型 - 线程安全
collector.AddType("prometheus", "up", "gauge")

// 添加指标帮助信息 - 线程安全
collector.AddHelp("prometheus", "up", "Prometheus target is up")

// 添加指标单位 - 线程安全
collector.AddUnit("prometheus", "scrape_duration_seconds", "seconds")

// 添加指标标签 - 线程安全
labels := labels.FromMap(map[string]string{
    "instance": "localhost:9090",
    "job":      "prometheus",
})
collector.AddLabels("prometheus", "up", labels)

// 添加子系统信息 - 线程安全
collector.AddSubsystemInfo("prometheus", "up", "monitoring")
```

### 2.1 并发安全的批量操作示例

```go
// 批量更新指标信息 - 推荐方式
func updateMetricBatch(collector *MetricInfoCollector, jobName, metricName string) {
    // 使用单个锁进行所有更新
    mi := collector.loadOrInitMetricInfo(jobName, metricName)
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    // 批量更新多个字段
    mi.Help = "Updated help text"
    mi.Unit = "milliseconds"
    mi.Type = "histogram"
    mi.SubsystemId = "updated_subsystem"
    
    // 批量添加标签
    newLabels := []string{"env", "service", "version"}
    for _, label := range newLabels {
        if _, exists := mi.ExistsLabels[label]; !exists {
            mi.Labels = append(mi.Labels, label)
            mi.ExistsLabels[label] = true
        }
    }
}

// 并发读取示例
func readMetricConcurrently(collector *MetricInfoCollector, jobName, metricName string) {
    mi := collector.loadOrInitMetricInfo(jobName, metricName)
    
    // 使用读锁进行并发安全的读取
    mi.mu.RLock()
    defer mi.mu.RUnlock()
    
    // 读取所有需要的数据
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
    
    // 深拷贝数据以在锁外使用
    copy(metricData.Labels, mi.Labels)
    for k, v := range mi.ExistsLabels {
        metricData.ExistsLabels[k] = v
    }
    
    // 在锁外处理数据
    processMetricData(metricData)
}
```

### 3. 检查任务状态

```go
// 检查指定作业是否正在收集
isCollecting := collector.NeedCollect("prometheus")

// 获取目标信息
targetInfo := collector.GetTargetInfo("localhost:9090", "/metrics")
if targetInfo != nil {
    fmt.Printf("Subsystem: %s\n", targetInfo.SubsystemId)
    fmt.Printf("Labels: %s\n", targetInfo.Labels.String())
}
```

### 4. 创建收集任务

创建 `/tmp/task.json` 文件，内容如下：

```json
[
    {
        "jobName": "prometheus",
        "scrapeInterval": "15s"
    },
    {
        "jobName": "node",
        "scrapeInterval": "30s"
    }
]
```

MetricCollector 会自动发现并执行这些任务。

### 5. 高并发场景下的性能优化示例

```go
// 高并发处理器示例
type ConcurrentMetricProcessor struct {
    collector *MetricInfoCollector
    cache     sync.Map
}

func NewConcurrentMetricProcessor(collector *MetricInfoCollector) *ConcurrentMetricProcessor {
    return &ConcurrentMetricProcessor{
        collector: collector,
    }
}

// 并发处理多个指标
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

// 处理单个指标
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
```

## 迁移指南

### 从旧版本升级的注意事项

如果您正在从旧版本的 MetricCollector 升级到支持并发安全的新版本，请注意以下事项：

#### 1. API 兼容性

- **向后兼容**：所有现有的 MetricInfoCollector 方法（AddHelp, AddUnit, AddLabels 等）保持向后兼容
- **内部变更**：MetricInfo 结构体新增了 `mu sync.RWMutex` 字段，但对外接口保持不变
- **线程安全保证**：新版本提供线程安全保证，旧版本在并发环境下可能出现竞态条件

#### 2. 代码迁移建议

**无需修改的代码**：
```go
// 这些代码无需修改，已经是线程安全的
collector.AddType("prometheus", "up", "gauge")
collector.AddHelp("prometheus", "up", "Prometheus target is up")
collector.AddUnit("prometheus", "scrape_duration_seconds", "seconds")
collector.AddLabels("prometheus", "up", labels)
collector.AddSubsystemInfo("prometheus", "up", "monitoring")
```

**需要检查的代码**：
```go
// 如果您的代码直接访问 MetricInfo 字段，需要添加锁保护
// 旧版本代码（可能存在竞态条件）
mi := collector.loadOrInitMetricInfo(jobName, metricName)
fmt.Printf("Metric: %s, Type: %s\n", mi.Name, mi.Type) // 不安全！

// 新版本代码（线程安全）
mi := collector.loadOrInitMetricInfo(jobName, metricName)
mi.mu.RLock()
defer mi.mu.RUnlock()
fmt.Printf("Metric: %s, Type: %s\n", mi.Name, mi.Type) // 安全
```

#### 3. 性能考虑

- **内存开销**：每个 MetricInfo 实例增加约 24 字节的内存开销（RWMutex）
- **CPU 开销**：读写操作增加少量 CPU 开销，但在高并发场景下性能更好
- **锁竞争**：细粒度锁策略减少了锁竞争，提高了并发性能

#### 4. 并发场景优化

如果您在多线程环境中使用 MetricCollector，建议进行以下优化：

```go
// 批量操作优化
func updateMultipleFields(collector *MetricInfoCollector, jobName, metricName string) {
    mi := collector.loadOrInitMetricInfo(jobName, metricName)
    mi.mu.Lock()
    defer mi.mu.Unlock()
    
    // 在单个锁内完成所有更新
    mi.Help = "Updated help"
    mi.Unit = "seconds"
    mi.Type = "counter"
    mi.SubsystemId = "new_subsystem"
}

// 并发读取优化
func readMultipleMetrics(collector *MetricInfoCollector, jobName string, metricNames []string) {
    var wg sync.WaitGroup
    results := make([]MetricData, len(metricNames))
    
    for i, metricName := range metricNames {
        wg.Add(1)
        go func(index int, name string) {
            defer wg.Done()
            mi := collector.loadOrInitMetricInfo(jobName, name)
            
            mi.mu.RLock()
            results[index] = MetricData{
                Name:   mi.Name,
                Type:   mi.Type,
                Help:   mi.Help,
                Unit:   mi.Unit,
                Labels: make([]string, len(mi.Labels)),
            }
            copy(results[index].Labels, mi.Labels)
            mi.mu.RUnlock()
        }(i, metricName)
    }
    
    wg.Wait()
    // 处理结果...
}

type MetricData struct {
    Name   string
    Type   string
    Help   string
    Unit   string
    Labels []string
}
```

#### 5. 测试建议

升级后，建议进行以下测试：

1. **并发安全测试**：
   ```go
   func TestConcurrentAccess(t *testing.T) {
       collector := InitMetricCollector("/tmp/test.yml", false)
       jobName := "test_job"
       metricName := "test_metric"
       
       var wg sync.WaitGroup
       
       // 并发写入
       for i := 0; i < 10; i++ {
           wg.Add(1)
           go func(id int) {
               defer wg.Done()
               collector.AddHelp(jobName, metricName, fmt.Sprintf("Help %d", id))
           }(i)
       }
       
       // 并发读取
       for i := 0; i < 10; i++ {
           wg.Add(1)
           go func() {
               defer wg.Done()
               mi := collector.loadOrInitMetricInfo(jobName, metricName)
               mi.mu.RLock()
               _ = mi.Help
               mi.mu.RUnlock()
           }()
       }
       
       wg.Wait()
   }
   ```

2. **性能基准测试**：
   ```go
   func BenchmarkConcurrentReads(b *testing.B) {
       collector := InitMetricCollector("/tmp/test.yml", false)
       jobName := "test_job"
       metricName := "test_metric"
       
       // 预填充数据
       collector.AddHelp(jobName, metricName, "Test help")
       collector.AddType(jobName, metricName, "gauge")
       
       b.ResetTimer()
       b.RunParallel(func(pb *testing.PB) {
           for pb.Next() {
               mi := collector.loadOrInitMetricInfo(jobName, metricName)
               mi.mu.RLock()
               _ = mi.Help
               _ = mi.Type
               mi.mu.RUnlock()
           }
       })
   }
   ```

#### 6. 常见问题解决

**问题1：升级后出现死锁**
- **原因**：可能是在锁内调用了外部代码或获取了其他锁
- **解决**：确保锁的范围最小化，避免在锁内调用外部代码

**问题2：性能下降**
- **原因**：可能是频繁的锁竞争或锁的粒度过大
- **解决**：使用批量操作，减少锁的获取次数

**问题3：内存使用增加**
- **原因**：每个 MetricInfo 实例增加了 RWMutex
- **解决**：这是正常的开销，通过更好的并发性能来补偿

## 使用场景

### 1. 指标文档生成
MetricCollector 可以自动收集指标元数据并生成 Markdown 格式的文档，适用于：
- 自动生成指标文档网站
- 为团队提供最新的指标参考
- 监控指标变更和演进

### 2. 指标质量监控
通过收集指标元数据，可以：
- 检测缺少帮助信息的指标
- 识别没有单位的指标
- 发现标签使用不一致的问题

### 3. 多租户环境管理
在多租户环境中，可以：
- 按子系统或团队组织指标
- 跟踪不同团队的指标使用情况
- 提供指标所有权信息

### 4. 监控系统集成
与现有监控系统集成：
- 作为指标发现服务的组件
- 为自动化配置提供元数据支持
- 支持动态监控配置生成

### 5. 指标治理
在企业级指标治理中：
- 建立指标命名规范
- 跟踪指标类型分布
- 监控指标标签使用情况

## 总结

MetricCollector 是一个功能强大且灵活的指标元数据收集器，它通过自动化的方式收集、管理和存储 Prometheus 指标的详细信息。其设计充分考虑了并发安全、资源管理和自动化需求，适用于各种规模的监控环境。

### 主要特性总结

1. **并发安全设计**：
   - MetricInfo 结构体使用 `sync.RWMutex` 实现细粒度的读写锁保护
   - 所有 MetricInfoCollector 的公共方法都是线程安全的
   - 读操作可以并发执行，写操作需要独占访问
   - 每个 MetricInfo 实例都有自己的锁，减少锁竞争

2. **高性能并发访问**：
   - 读写锁设计特别适合读多写少的场景
   - 细粒度锁策略减少了锁竞争，提高了整体性能
   - 支持批量操作优化，减少锁获取次数

3. **完善的 API 设计**：
   - 提供线程安全的高级 API，推荐直接使用
   - 支持直接访问 MetricInfo，但需要手动加锁
   - 详细的 API 文档和使用示例

4. **向后兼容性**：
   - 所有现有 API 保持向后兼容
   - 升级过程平滑，无需修改现有代码
   - 提供详细的迁移指南和最佳实践

5. **自动化任务管理**：
   - 自动发现和执行收集任务
   - 自动处理任务超时和清理
   - 限制并发任务数量，防止资源耗尽

6. **灵活的数据管理**：
   - 支持多种指标类型和元数据
   - 自动去重和合并标签信息
   - 按任务名称组织输出文件

### 使用建议

1. **优先使用 MetricInfoCollector 的安全方法**，避免直接操作 MetricInfo
2. **最小化锁的持有时间**，将耗时操作放在锁外执行
3. **使用批量操作**减少锁竞争，提高性能
4. **在高并发场景下**，考虑使用缓存和并发处理模式
5. **升级时参考迁移指南**，确保代码的线程安全

通过合理使用 MetricCollector 的并发安全特性，可以显著提高指标管理的效率和质量，为监控系统的建设和维护提供有力支持。无论是在单线程环境还是高并发场景下，MetricCollector 都能提供稳定、高效的指标元数据管理服务。
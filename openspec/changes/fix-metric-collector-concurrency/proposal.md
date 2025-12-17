# Change: 修复MetricCollector并发读写问题

## Why
MetricCollector组件中的MetricInfo结构体存在并发读写问题，可能导致数据竞争和程序崩溃，影响系统的稳定性和可靠性。

## What Changes
- 为MetricInfo结构体添加互斥锁保护所有字段的并发访问
- 修改所有访问MetricInfo字段的方法，确保线程安全
- 保持现有API接口不变，确保向后兼容性

## Impact
- 受影响的规格: metric-collector
- 受影响的代码: pkg/scrape/collector.go, pkg/scrape/metric_info.go
- **BREAKING**: 无破坏性变更，仅内部实现修改
## Context
MetricCollector是Kvass项目中的核心组件，负责收集、管理和存储Prometheus指标的元数据信息。在高并发环境下，MetricInfo结构体存在并发读写问题，可能导致数据竞争和程序崩溃。

### 当前问题
1. MetricInfo结构体内部的map（ExistsLabels）存在并发读写问题
2. Labels切片存在并发修改问题
3. 其他字段（Help、Type、Unit等）存在并发修改问题

## Goals / Non-Goals
- Goals:
  - 确保MetricInfo结构体的线程安全
  - 保持现有API接口不变
  - 最小化性能影响
  - 提供可靠的并发访问机制

- Non-Goals:
  - 重新设计MetricCollector的整体架构
  - 修改现有的API接口
  - 引入新的依赖库

## Decisions
- Decision: 为MetricInfo结构体添加互斥锁（sync.RWMutex）保护所有字段的并发访问
  - Why: RWMutex允许多个读操作并发执行，只有在写操作时才需要独占访问，适合读多写少的场景
  - Implementation: 在MetricInfo结构体中添加mutex字段，并在所有访问方法中添加适当的锁操作

- Decision: 使用细粒度锁而不是全局锁
  - Why: 细粒度锁可以减少锁竞争，提高并发性能
  - Implementation: 每个MetricInfo实例都有自己的锁，而不是使用全局锁

- Decision: 保持现有API接口不变
  - Why: 确保向后兼容性，减少对现有代码的影响
  - Implementation: 在现有方法内部添加锁操作，不改变方法签名

## Risks / Trade-offs
- 性能影响: 锁操作会带来一定的性能开销，但相比数据竞争的风险是可接受的
- 复杂性增加: 需要确保所有访问路径都正确使用锁，避免死锁
- 内存开销: 每个MetricInfo实例都需要额外的锁对象

## Migration Plan
1. 添加互斥锁字段到MetricInfo结构体
2. 修改所有访问MetricInfo字段的方法，添加适当的锁操作
3. 更新所有调用MetricInfo方法的代码
4. 编写并发测试用例验证线程安全性
5. 进行性能测试确保性能影响在可接受范围内

## Open Questions
- 是否需要考虑使用原子操作来优化某些简单字段的访问？
- 是否需要添加锁超时机制以防止潜在的死锁？
- 是否需要添加监控指标来跟踪锁的使用情况？
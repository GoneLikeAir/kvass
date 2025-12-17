# Project Context

## Purpose
Kvass 是一个 Prometheus 横向自动扩缩容解决方案，旨在解决大规模监控场景下 Prometheus 的性能瓶颈问题。它通过动态分片技术将监控目标分配到多个 Prometheus 实例中，实现数千万时间序列的支持（适用于数千个 Kubernetes 节点的环境）。Kvass 的核心目标是提供轻量级、易于使用的 Prometheus 扩展方案，无需修改原有 Prometheus 配置文件，支持根据实际目标负载进行智能分片，并提供全局数据视图。

## Tech Stack
- **主要语言**: Go 1.15+
- **核心框架**: 
  - Prometheus (监控数据收集和存储)
  - Kubernetes (容器编排和服务发现)
  - Thanos (全局数据查询和长期存储)
- **关键依赖**:
  - github.com/prometheus/prometheus
  - k8s.io/api, k8s.io/apimachinery, k8s.io/client-go
  - github.com/gin-gonic/gin (Web API)
  - github.com/spf13/cobra (CLI)
  - github.com/sirupsen/logrus (日志)
  - golang.org/x/sync (并发控制)
- **部署方式**: Kubernetes StatefulSet
- **数据存储**: Prometheus TSDB + 可选远程存储 (如 InfluxDB)

## Project Conventions

### Code Style
- **命名约定**: 
  - 包名使用小写单词，不使用下划线或驼峰
  - 公共函数和变量使用 PascalCase
  - 私有函数和变量使用 camelCase
  - 常量使用全大写字母加下划线
- **文件组织**: 
  - 每个主要功能模块一个包
  - 测试文件以 `_test.go` 结尾
  - 接口定义通常放在 `types.go` 文件中
- **注释规范**: 
  - 公共函数必须有注释说明
  - 复杂逻辑需要行内注释
  - 使用 godoc 格式编写包和函数注释

### Architecture Patterns
- **微服务架构**: 采用 Coordinator 和 Sidecar 分离的架构模式
- **事件驱动**: 使用 channel 和 goroutine 实现组件间的异步通信
- **接口抽象**: 核心组件通过接口定义，便于扩展和测试
- **策略模式**: 支持多种分片策略和副本管理策略 (Kubernetes 和静态)
- **观察者模式**: 配置变更通过回调机制通知各组件

### Testing Strategy
- **单元测试**: 每个包都有对应的测试文件，使用 Go 标准测试框架
- **集成测试**: 通过 `utils/test` 包提供测试辅助工具
- **测试覆盖率**: 使用 `go test -coverprofile` 生成覆盖率报告
- **模拟测试**: 对外部依赖 (如 Kubernetes API) 使用模拟对象
- **测试数据**: 测试配置和数据放在 `testdata` 目录中

### Git Workflow
- **分支策略**: 
  - `master` 主分支保持稳定
  - 功能分支从 `master` 分出，完成后合并回 `master`
- **提交规范**: 
  - 提交信息使用英文，格式为 "type: description"
  - 类型包括: feat, fix, docs, style, refactor, test, chore
- **代码审查**: 所有代码变更需要经过代码审查
- **版本管理**: 使用语义化版本号，通过 Git 标签标记版本

## Domain Context
Kvass 专注于解决大规模监控场景下的 Prometheus 扩展性问题，主要应用于:

1. **大规模 Kubernetes 集群监控**: 支持数千个节点的集群监控
2. **高基数指标场景**: 处理数千万时间序列的监控数据
3. **云原生环境**: 与 Kubernetes 生态系统深度集成
4. **多租户监控**: 通过分片实现不同租户的监控隔离

**核心概念**:
- **Shard**: Prometheus 分片实例，负责处理部分监控目标
- **Coordinator**: 中央控制器，负责服务发现、目标分配和分片管理
- **Sidecar**: 边车组件，为每个 Prometheus 分片生成特定配置
- **Target**: 监控目标，可以是任何暴露指标的端点
- **Series**: 时间序列，Prometheus 中的基本数据单元

## Important Constraints
1. **性能约束**: 
   - 每个 Prometheus 分片建议最大处理 750,000 个时间序列
   - 内存使用建议为 8GB (对应 750,000 series)
   - 目标迁移需要至少 3 个采集周期确保数据连续性

2. **可靠性约束**:
   - 分片迁移过程中不能丢失监控数据
   - Coordinator 故障不能影响现有分片的正常工作
   - Sidecar 必须持久化存储目标信息以防重启

3. **兼容性约束**:
   - 必须兼容 Prometheus 配置格式
   - 支持所有 Prometheus 服务发现机制
   - 与 Thanos 等生态组件无缝集成

4. **扩展性约束**:
   - 支持动态扩缩容，无需重启整个系统
   - 分片数量可配置，支持最小和最大分片数限制
   - 支持多副本部署以��高可用性

## External Dependencies
1. **Kubernetes**: 
   - 用于服务发现和分片管理
   - 通过 client-go 库与 API Server 交互
   - 依赖 RBAC 权限管理

2. **Prometheus**: 
   - 核心监控数据收集和存储引擎
   - 使用其配置格式和服务发现机制
   - 通过 HTTP API 与 Prometheus 交互

3. **Thanos** (可选):
   - 提供全局数据查询能力
   - 支持数据长期存储和查询
   - 通过 Sidecar 模式集成

4. **远程存储** (可选):
   - 支持 InfluxDB 等远程存储系统
   - 通过 Prometheus 的 remote write 机制集成

5. **监控目标**:
   - 各种暴露指标的端点
   - 支持 HTTP/HTTPS 协议
   - 支持基本认证和 Bearer Token 认证

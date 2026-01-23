# Project Context

## Purpose
Kvass 是一个 Prometheus 横向自动扩缩容/分片解决方案。Coordinator 负责服务发现、目标调度和分片管理；Sidecar 为每个 Prometheus shard 生成只包含其目标的配置文件，并代理抓取请求以统计 series。面向大规模监控场景（数千万时间序列、数千节点），无需修改原 Prometheus 配置即可根据实际目标负载动态分片，并可与 Thanos 等组件组合提供全局数据视图。

## Tech Stack
- **语言/运行时**: Go 1.22.0（toolchain `go1.23.8`）
- **依赖管理**: Go Modules；`go.mod` 通过 `replace` 指向 `staging/src/github.com/promethues/prometheus`
- **核心组件/库**:
  - Prometheus `config`/`discovery`/`scrape`（服务发现与配置解析）
  - Kubernetes `client-go`（StatefulSet shard 管理）
  - `gin-gonic/gin`（HTTP API）
  - `spf13/cobra`（CLI）
  - `sirupsen/logrus` + `promlog`（日志）
  - `gin-contrib/pprof`, `cssivision/reverseproxy`（调试与反向代理）
- **部署与交付**: Kubernetes（`deploy/demo/` 示例清单，StatefulSet 管理 shards）；Dockerfile 打包二进制
- **数据存储与查询**: Prometheus TSDB；可选 remote write；可与 Thanos 集成

## Project Conventions

### Code Organization
- `cmd/kvass/`: CLI 入口与子命令（`coordinator`, `sidecar`）
- `pkg/`: 核心逻辑
  - `pkg/coordinator/`, `pkg/sidecar/`, `pkg/shard/`（`kubernetes`/`static`）
  - `pkg/scrape/`, `pkg/discovery/`, `pkg/prom/`, `pkg/target/`, `pkg/utils/`
- `staging/src/github.com/promethues/prometheus/`: Prometheus 源码（通过 `replace` 引入）
- `deploy/demo/`: Kubernetes 示例部署清单
- `docs/`: 设计/计划文档

### Code Style
- 遵循 Go 标准风格（仓库未提供单独 `fmt` 目标）
- 包名使用小写；导出/非导出命名遵循 Go 约定（PascalCase/camelCase）
- 测试文件以 `_test.go` 结尾
- 类型/接口定义常集中在 `types.go`

### Architecture Patterns
- **组件分离**: Coordinator 负责服务发现与调度，Sidecar 负责配置注入与代理
- **分片管理策略**: `pkg/shard/` 提供 Kubernetes 与静态文件两种实现
- **回调式配置链路**: `ConfigManager.AddReloadCallbacks` 与 `TargetsManager.AddUpdateCallbacks` 传播变更
- **HTTP API + 反向代理**: Sidecar 通过 Gin 提供 API，并对非 API 路径转发到 Prometheus

### Build & Test
- 依赖下载: `make dep`
- 构建二进制: `make build`（输出 `kvass`）
- 单元测试: `make test`（`go test -short`）
- 覆盖率: `make test-coverage`
- Lint/Vet: `make lint`（golint）, `make vet`
- 清理: `make clean`
- 变更日志: `make clog`（git-chglog 生成 `CHANGELOG.md`）

### Testing Strategy
- 以 Go 标准测试框架为主，单元测试覆盖 `pkg/` 各模块
- 通过 `go test -short` 与 `-coverprofile` 输出覆盖率
- 测试辅助工具集中在 `pkg/utils/test/`

### Git Workflow
- 未发现额外的分支/提交规范文件；默认遵循团队约定
- `CHANGELOG.md` 通过 `make clog` 自动生成/更新

## Domain Context
Kvass 主要解决大规模监控场景下的 Prometheus 扩展性问题，关注点包括:

1. **大规模 Kubernetes 集群监控**: 数千节点、数千万时间序列
2. **动态分片与扩缩容**: 按目标实际负载分配到 shards
3. **多副本场景**: 通过 StatefulSet 管理多个 Prometheus 副本
4. **全局视图**: 与 Thanos 或远程存储组合获得全局查询能力

**核心概念**:
- **Shard**: 单个 Prometheus 分片实例
- **Coordinator**: 统一的服务发现、目标分配与分片管理
- **Sidecar**: 为分片生成配置并代理抓取
- **Target**: 监控目标端点
- **Series**: 时间序列，分片决策的主要指标

## Important Constraints
1. **性能约束**:
   - `--shard.max-series` 控制单分片最大 series（默认 1,000,000）
   - 推荐上限约 750,000 series/分片（内存约 8GB）
   - 目标迁移需至少 3 个抓取周期保证数据连续性

2. **可靠性约束**:
   - 分片迁移过程中避免监控数据丢失
   - Coordinator 故障不影响现有分片正常抓取
   - Sidecar 必须持久化 `store.path`（避免重启丢失目标）

3. **兼容性约束**:
   - 兼容 Prometheus 配置与服务发现机制
   - 与 Thanos 等生态组件集成

4. **扩展性约束**:
   - 支持动态扩缩容与分片上下限控制（`--shard.min-shard`/`--shard.max-shard`）

## External Dependencies
1. **Kubernetes**:
   - `client-go` 与 RBAC 权限
   - StatefulSet 管理 Prometheus shards

2. **Prometheus**:
   - 使用 Prometheus 配置与服务发现实现
   - 通过 HTTP API 与 Prometheus 交互

3. **Thanos** (可选):
   - 全局查询视图与长期存储

4. **远程存储** (可选):
   - 通过 Prometheus `remote_write` 接入

5. **服务发现生态**:
   - 通过 Prometheus 内置 SD（kubernetes、consul、file、dns 等）

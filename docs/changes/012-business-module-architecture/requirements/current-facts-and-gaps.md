# 当前事实与能力缺口

## 1. 证据快照

- 复核日期：2026-08-14。
- 代码基线：`2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`。
- 本文只把代码、测试和当前权威文档共同支持的内容标记为“已实现”；目标目录与接口均标记为“目标设计”。

## 2. 已实现事实

### 2.1 入口、装配与生命周期

- `cmd/app/main.go` 先建立 Kernel baseline logger，再建立配置 Loader、Kernel Runtime 和 `internal/kernel/composition.Compose`。
- 有参数时只进入可选 CLI 路径；无参数时建立 Host，并把 Kernel 作为第一个 Participant。
- `internal/kernel/app.Plan` 使用显式 `Add`、typed `Binding/Input`、`Replace` 和 `Freeze`；没有包扫描、运行时 Resolver、反射式业务对象图或通用 DAG。
- Kernel Start 按顺序完成 Stage、Build、Start、Ready 和 Publish；失败时按反序回滚。
- Host/Supervisor 按注册顺序启动 Participant，按反序停止；长任务有 owner、取消和等待边界。
- Reload 先做 `RestartRequired` 预检，再构造候选、排空、提交、恢复并清理旧代；候选失败保留旧代。

### 2.2 当前 Capability

`composition.Capabilities` 当前交付 Logger、Clock、ID Generator、Validator、Database Access、Cache Access、I18n Translator、Storage Access、默认配置管理器和可选 CLI App。

| 能力 | 当前形态 | 资源/重载要点 |
| --- | --- | --- |
| Logger | 稳定 Manager/facade | baseline 必须先存在，配置成功后才能替换 |
| Clock / ID / Validator | Fixed + Direct | 普通稳定对象，无外部资源 |
| Database | Configured + Leased Access | Kernel 拥有 GORM/SQL 资源，调用方只能在回调内借用 |
| Cache | Configured backend + 业务 typed Client | Kernel 拥有远端后端；业务 Client 自己 Close；配置变更需重启 |
| I18n | 稳定 Translator facade | 候选资源成功后实例换代 |
| Storage | Configured + borrowed Client | Kernel 拥有 Manager，调用方无共享 Close 权 |

### 2.3 已有业务相关底座

- `pkg/httpx` 已用项目自有 Router、Handler、Context、Middleware 和 Server 隔离 chi/net/http，并实现 Recovery、RequestID、AccessLog、安全 Header、CORS、BodyLimit、RateLimiter。
- `pkg/cli` 已隔离 Cobra/Bubble Tea，提供 App、CommandSpec、Context 等项目契约。
- `pkg/fault` 已有稳定错误分类并保留 cause。
- `pkg/database` 有基础 Repository 工具、事务和迁移能力；这些是 Adapter 原语，不等于业务 Repository 已存在。
- `pkg/logger`、`pkg/i18n`、`pkg/cache`、`pkg/storage` 均可作为业务边界依赖，但必须继续遵守 owner 与 Lease 约束。

## 3. 明确未实现

- 没有任何真实业务模块、领域模型、用例 Service、业务 Repository port 或数据库 Adapter。
- 没有业务 HTTP Handler、路由注册、HTTP 配置、监听器或进入 Host 的 HTTP Participant；默认进程不提供 HTTP 服务。
- 没有业务 CLI Command，也没有“启动 Kernel 资源后执行一次业务命令再关闭”的运行模式。
- 没有模块清单、重复路由/命令检测、跨模块依赖规范或模块架构测试。
- 没有保证 Kernel 和业务/HTTP 配置使用同一初始 Snapshot 的编排接口。
- 没有业务对象图热重建、动态路由更新、远程模块、消息总线或进程外插件。

## 4. 现有边界中的风险

- `pkg/httpx.RequestID` 允许 nil 生成器时自行调用 UUID fallback。目标业务装配必须始终显式注入 ID Generator，后续实施应评估删除隐藏 fallback，而不是让模块依赖该偶然行为。
- `httpx.Server.Start` 是阻塞调用；若直接塞入 `Participant.Start`，要么阻塞 Host 启动，要么 goroutine 中的早期监听错误无法同步报告。目标需要 listener/readiness 边界。
- 当前 CLI 在 Kernel Start 前执行，这对 `config init` 正确，却不能直接承载依赖 Database/Cache 的业务命令。
- 当前 Kernel 在 Start 内加载配置；若业务图在 Start 前另读配置，会产生双读取和撕裂快照。目标设计把“单一启动快照”列为前置改造。
- 业务 Cache typed Client 的 Close 由创建者负责；未来模块装配若遗漏 owner，会泄漏清理 goroutine。

## 5. 事实、推断与目标的界线

| 类别 | 示例 | 本文处理 |
| --- | --- | --- |
| 已实现事实 | Kernel 首 Participant、Database Access Lease、无 HTTP Listener | 由当前源码复核 |
| 能力缺口 | 无业务 Service/Repository/Handler | 由目录、符号和调用方搜索复核 |
| 架构推断 | 业务图不应进入 Kernel Plan | 基于现有 Plan 定位与外部研究得出 |
| 目标设计 | `internal/business/<module>`、HTTP contribution、单 Snapshot 启动 | 只在 012 文档中提出，尚未确认 |
| 未决决策 | 首个真实模块、公开 API 精确命名 | 必须在实施前由真实需求确认 |

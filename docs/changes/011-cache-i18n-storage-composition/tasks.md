# 任务账本：Cache、I18n 与 Storage 装配

## 1. 确认门禁

- 当前方案状态：**已完成**。
- 当前 Git 基线：`main@d69233a`；本地相对当前 `origin/main` ahead 1。
- 工作树开始前已有未跟踪 `tmp/`，不属于 011；实施、验证和提交都必须排除。
- 用户已在方案报告后的后续消息中明确要求开始 `011-cache-i18n-storage-composition`；`CACHE-001` 至 `VER-001` 已获实施授权。
- 确认后若公开接口、配置结构、第三方依赖、组件边界、重载策略、外部探针或迁移范围实质变化，必须重新回到待确认。

## 2. 任务清单

| ID | 任务 | 工作量 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `DOC-001` | 建立 011 四件套与变更导航 | S | 无 | 事实、需求、设计、任务、门禁和验收矩阵完整 | 已完成 |
| `CACHE-001` | 收敛 Cache 后端资源与配置 | L | 用户确认 | disabled/Redis typed config；go-redis 创建、Ping、Close 和安全错误由单一所有者治理 | 已完成 |
| `CACHE-002` | 建立可关闭的 typed Client 生命周期 | M | `CACHE-001` | 清理 goroutine 有 cancel/wait；Close 幂等；无 GC finalizer 所有权依赖 | 已完成 |
| `CACHE-003` | 建立 Cache App 与泛型注入入口 | L | `CACHE-001`、`CACHE-002` | 稳定 Access；`NewClient[T]` 不泄漏 Store；RestartRequired/disabled/租约测试通过 | 已完成 |
| `I18N-001` | 建立 I18n Configured App | M | 用户确认 | Config/Defaults、稳定 Translator facade、候选加载、热换与失败保旧通过 | 已完成 |
| `STO-001` | 收敛对象 Storage 配置与探针 | M | 用户确认 | Manager-only typed config、唯一 local probe、安全远端校验和关闭错误完整 | 已完成 |
| `STO-002` | 建立 Storage App 稳定访问边界 | L | `STO-001` | Primary/Local/Object route、borrowed Client、逃逸失效、换代/排空/关闭通过 | 已完成 |
| `CMP-001` | 扩展 composition 固定清单与 Capabilities | M | `CACHE-003`、`I18N-001`、`STO-002` | 三项显式 Add；失败零安装；CLI 不打开资源；默认顺序稳定 | 已完成 |
| `CFG-001` | 同步默认配置、示例与入口行为 | M | `CMP-001` | config init/example/env key/本地默认启动与安全凭据说明一致 | 已完成 |
| `TST-001` | 补齐组件与跨组件生命周期验收 | L | `CFG-001` | package/app/composition/reload/process 成功、失败、取消、race 证据完整 | 已完成 |
| `DOC-002` | 同步当前权威文档 | M | `TST-001` | 根 README、Kernel/App/pkg 文档只描述已实现行为，011 保持历史证据定位 | 已完成 |
| `VER-001` | 全量验证、Diff 审阅与独立提交 | L | `DOC-002` | 全量门禁通过；只暂存 011；独立 commit；不 push | 已完成 |

`S/M/L` 只表示相对工作量，不是时间承诺。

## 3. 实施顺序

```text
DOC-001 -> 用户确认
  -> CACHE-001 -> CACHE-002 -> CACHE-003
  -> I18N-001
  -> STO-001 -> STO-002
  -> CMP-001 -> CFG-001 -> TST-001 -> DOC-002 -> VER-001
```

Cache、I18n、Storage 三条组件实现可以分别完成定向测试，但只有三者全部进入同一 FrozenPlan、通过跨组件事务测试并同步权威文档后，011 才能完成。

## 4. 验收矩阵

| 领域 | 必需证据 |
| --- | --- |
| Cache resource | disabled/Redis 校验、Ping、Close、敏感字段不泄漏、无第三方类型外露 |
| Cache typed client | 泛型读写、TTL/tag、显式 Close、清理 goroutine 退出、并发 race |
| Cache reload | section 变化返回 RestartRequired；同轮其他组件无 Build/Commit 副作用 |
| I18n | 默认/文件资源、语言匹配、缺失策略、稳定 facade、成功换代、失败保旧 |
| Storage | route、borrowed 失效、local 真读写、S3-compatible HTTP 合约、Ready/Stop/错误聚合 |
| Composition | 固定顺序、Capabilities、Defaults、Freeze/Install 原子性、CLI 无资源副作用 |
| Host | 默认无需外部服务；启动、Watch、Reload、反向 Stop 和取消边界正确 |
| Delivery | build/test/race/vet/tidy/no-CGO、config init、smoke、文档链接、边界搜索、diff-check、独立暂存审阅 |

## 5. 方案轮证据（2026-08-14）

- `git status --short --branch`：`main...origin/main [ahead 1]`，仅有预存未跟踪 `tmp/`。
- `docs/changes` 当前最高有效序号为 010，索引明确下一个任务序号为 011。
- 代码搜索确认 composition 当前只装配 Logger、Clock、ID Generator、Validator、Database；三项能力没有 App Definition 或 Capabilities 字段。
- 代码搜索确认仓库外没有 Cache/I18n 业务调用方；Storage 只有 `pkg/foundation_test.go` 使用局部文件工具。
- `pkg/cache` 当前 Redis client 由调用方创建和关闭，typed Client 没有 Close；第三方 go-cache 的清理 janitor 只有私有 stop，并依赖 finalizer。
- `pkg/i18n` 当前构造时加载资源，Translator 不泄漏第三方类型；`MessageFS` 是 `fs.FS`，不能直接成为文件配置字段。
- `pkg/storage` 当前 Manager 拥有 local/object client，但对外 Client 含 Close；文件工具还包含 watcher 和 `RemoveAll`，不适合作为全局 Capabilities。
- 定向基线 `go test ./internal/kernel/app/... ./internal/kernel/composition/... ./pkg/cache/... ./pkg/i18n/... ./pkg/storage/...` 通过。
- 本轮未修改实现、未运行服务、未连接 Redis/S3/MinIO、未暂存、未提交、未 push。

## 6. 确认与实施轮（2026-08-14）

- 用户在 011 方案报告后的后续消息中明确要求开始方案计划，当前任务进入实施阶段。
- 实施前基线仍为 `main@d69233a`；暂存区为空，工作树只有 011 方案文档和预存未跟踪 `tmp/`。
- 已启用仓库 `git-commit-task-changes` 技能；最终只精确暂存 011 文件，不 push、不改写历史。

## 7. 完成判定

只有以下条件全部满足，011 才能标记完成：

- 三项能力只通过当前 App Plan 和 composition 进入进程，没有第二套装配或配置路径；
- Cache、I18n、Storage 的公开能力、资源所有权和重载语义与本方案一致；
- 默认启动不依赖外部缓存或对象存储，disabled 也不伪装成功；
- 成功、失败、取消、排空、关闭和跨组件事务均有真实测试证据；
- 当前权威文档已同步，不把目标设计写成已实现；
- 全量验证通过，未执行项和外部系统限制如实记录；
- 只提交 011 可确认归属的文件，不夹带 `tmp/` 或其他已有修改，不 push。

## 8. 实施与验证证据（2026-08-14）

- Cache App 实现 disabled/Redis typed 配置、Ready Ping、Kernel 私有 Redis Close、稳定 Access、`NewClient[T]` 和 `RestartRequired`；typed Client 的 L1 清理任务具备 cancel/wait 与幂等 Close。
- I18n App 输出身份稳定的 `pkg/i18n.Translator` facade；配置成功时换代，缺失消息文件候选失败时保留旧 Translator。
- Storage App 只治理对象 `StorageManager`；route 借用 Client 不含 Close 且回调后失效；local Ready 使用唯一对象完成 Put/Get/Exists/Delete 并保留清理错误。
- 跨组件集成证明 I18n/Storage 同轮成功提交；非法 I18n 候选不创建 Storage 候选目录；Cache section 变化在副作用前拒绝包含其他变化的整轮 Reload。
- 进程级 smoke 使用临时 SQLite 与本地 Storage，证明默认能力无需 Redis、S3 或 MinIO即可启动并在取消后优雅退出；`config init` 同步生成五段默认配置。
- 实际通过：`go mod tidy -diff`、`go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...`、`CGO_ENABLED=0 go build ./...`、40 个变更文档相对链接检查、边界/旧符号搜索和 `git diff --check`。

## 9. 未执行项与剩余风险

- 未连接真实 Redis、S3 或 MinIO；Redis 使用 miniredis 验证资源和 typed Client，S3-compatible 继续由现有 `httptest` 合约覆盖，远端服务的网络、权限和部署配置不在本地验证结论内。
- I18n 消息文件内容变化仍不会被独立监听；Cache 配置变化仍明确要求重启。这两项是已确认边界，不是静默降级。
- 本任务没有新增生产依赖，没有修改或提交预存 `tmp/`，没有 push。

## 10. Commit

- Commit：以承载本记录的 `011` 最终提交为准；短哈希和标题在交付报告中记录。
- Push：未执行。

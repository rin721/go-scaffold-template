# 任务账本：Kernel 内置 Logger 的可选 App 替换

## 1. 确认门禁

- 当前方案状态：**已完成**。
- 当前事实基线：`main@10e1bf33a1471ae0935ec23b1a0a6013d7234085`。
- 用户已在方案报告后的后续消息中明确要求落实 007；`APP-001` 至 `VER-001` 已获实施授权。
- 确认后只实施本账本任务；公共 API、依赖、模块边界、默认选择或替换事务实质变化时，状态回到待确认。
- 方案阶段不暂存、不提交；确认实施后，方案文档、实现、测试和权威文档同步进入同一任务提交，除非用户明确要求不提交。

## 2. 任务清单

| ID | 任务 | 工作量 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `DOC-001` | 建立 007 方案与导航 | S | 无 | 四份文档完整，索引可达，状态待确认，文档校验通过 | 已完成 |
| `APP-001` | 建立最小 typed Replacement 契约 | M | 用户确认 | `ReplacementDefinition[T]` 与 `app.Replace` 类型分离；target 校验、唯一性、失败原子性完成 | 已完成 |
| `APP-002` | 补齐 Plan/Replacement 边界测试 | M | `APP-001` | Add/Replace 隔离、零值/跨 Plan/顺序/重复/Freeze/原子性测试通过 | 已完成 |
| `LOG-001` | 收敛 Kernel 内置 Logger facade | M | `APP-001` | Kernel 提供只读 Logger 与 typed target 契约；Manager 并发和动态 With 语义保持 | 已完成 |
| `LOG-002` | 把 Logger App 改为显式 Replacement | M | `APP-001`, `LOG-001` | `loggerapp.Replacement` 只创建 replacement 节点；旧 Access/Definition/Activation 入口删除 | 已完成 |
| `CMP-001` | 实现 composition 可选选择 | M | `LOG-002` | 内置 Binding 始终加入；零值 baseline-only；显式 configured replacement；未知值原子失败 | 已完成 |
| `ENT-001` | 迁移默认应用入口 | S | `CMP-001` | CLI/服务都显式选择 configured replacement；生命周期直接依赖稳定 Logger | 已完成 |
| `TST-001` | 验证生命周期、配置与并发 | L | `APP-002`, `ENT-001` | baseline/replace/reload/failure/restore/close/defaults/config init/race 场景通过 | 已完成 |
| `DOC-002` | 同步当前权威文档并清理旧描述 | M | `TST-001` | README、Kernel/App/Logger 说明与实现一致；旧符号和独立 Access 描述归零 | 已完成 |
| `VER-001` | 完整审阅、验证和任务提交 | M | `DOC-002` | Diff 聚焦；build/test/race/vet/diff-check 结果记录；只提交 007 文件 | 已完成 |

工作量：`S` 为局部变更，`M` 为跨包契约或测试，`L` 为生命周期与并发整体验收。

## 3. 实施顺序

```text
DOC-001
  -> 用户确认
  -> APP-001 -> APP-002
  -> LOG-001 -> LOG-002
  -> CMP-001 -> ENT-001
  -> TST-001 -> DOC-002 -> VER-001
```

不得先改 composition 再用临时 bool、旧 `loggerapp.Access` 或 `WithActivation` 兼容层过渡。实施必须在同一任务内完成调用方迁移和旧入口删除。

## 4. 逐轮证据

### 方案轮（2026-08-13）

- 已确认 Git 分支为 `main`，HEAD 为 `10e1bf33a1471ae0935ec23b1a0a6013d7234085`，方案建立前工作区无已跟踪改动。
- 已从根 README、Kernel/App 主题文档、004/006 历史记录和当前源码核对现状。
- 已确认 Manager 已是稳定动态 facade，实际替换隐藏在 Logger App 的 `WithActivation`，而 composition 使用普通 `app.Add` 且无条件选择 Logger App。
- 已把 target Binding 与实际 replacement target 收敛为同一 typed 实例，组件构造函数不再另收可能身份错配的 Manager。
- 实施测试确认 baseline-only 只免除 Logger 配置，完整 composition 仍受 Database 必填配置约束；验收表述已收紧，未改变 Database 行为。
- 完成审计发现仅收窄字段静态类型仍可能经类型断言取得 Manager 控制方法；实现改为稳定只读 view 与 typed target 分离，并增加不泄漏 `Replace/Restore` 的测试。
- 已确认现有空目录 `docs/changes/007-app-component-logger-injection` 没有文件、未被 Git 跟踪，也未被 ignore 规则覆盖，因此可作为本任务目录。
- 已验证 007 四件套、变更索引和相对 Markdown 链接；`git diff --check`、尾随空白及文件末尾换行检查通过。
- 未修改任何实现文件，未执行构建、Go 测试、服务启动、暂存、提交或推送。

### 确认轮（2026-08-13）

- 用户在方案报告后明确要求“落实 `007-app-component-logger-injection` 方案”，当前方案状态更新为已确认。
- 实施范围固定为 `APP-001` 至 `VER-001`，不引入方案非目标中的通用 Catalog、多级覆盖或业务能力。

### 实施轮（2026-08-13）

- 新增 `ReplacementDefinition[T]`、`ManagedConfiguredReplacement` 与 `app.Replace`，typed target 必须来自同一 Plan 的更早 Binding；同一 target 只允许一个 replacement，失败不占用 ID 或 target。
- Kernel 内置 Logger 使用 `kernel.logger` typed target；Manager 提供动态类型不含控制方法的稳定只读 view，`Capabilities.Logger` 不泄漏 `Replace/Restore/Close`。
- Logger App 单轨迁移为 `logger.configured` replacement，旧 `loggerapp.Access`、`Definition(manager)`、普通 Add 和 `WithActivation` 已删除。
- composition 增加 `KernelBuiltinLogger` 与 `ConfiguredLoggerReplacement`；零值只保留 baseline，`cmd/app` 在 CLI 和服务模式都显式选择 configured replacement。
- 测试覆盖 cross-plan/zero/duplicate/frozen/失败原子性、baseline、candidate 失败、首次发布、Reload Commit 边界、Stop Restore、动态 With、在途写入与 Replace 同步、Defaults、未知选择及入口生命周期。
- 权威 README 已同步；004/006 变更记录保留历史原文。

### 验证轮（2026-08-13）

- `gofmt`：007 修改的全部 Go 文件已格式化。
- `go mod tidy`：执行后 `go.mod/go.sum` 无变化。
- `go build ./cmd/app`：通过；Windows 生成的 `app.exe` 已移出仓库到系统临时目录。
- `go test ./...`：通过。
- `go test -race ./...`：通过。
- `go vet ./...`：通过。
- 实际 `go run ./cmd/app config init --output <temp> --force`：通过，生成顺序为 Logger、Database。
- 旧生产符号搜索：`WithActivation`、`LoggingManager()`、`loggerapp.Access`、`loggerapp.Definition`、旧 `composeLogger` 无当前 Go 源码残留。
- `git diff --check` 与 Markdown 相对链接校验：通过。
- Commit：`feat: 实现 Kernel 内置 Logger 可选替换`（本任务提交；短哈希由提交后核验并在交付报告中给出）。

## 5. 实施记录模板

确认后每一轮在此追加：

- 已完成任务 ID 与文件范围；
- 关键 API/语义证据；
- 执行命令及真实结果；
- 未执行项、失败、剩余风险；
- 若创建任务提交，记录 Commit ID。

## 6. 完成判定

只有同时满足以下条件才能把本任务标为已完成：

- Kernel baseline 与 configured replacement 两条路径均真实可用；
- 替换意图在组件类型和 Plan 操作中显式可见，普通 Add 不能伪装；
- `Capabilities.Logger` 单一稳定，旧 Access 和隐藏 Activation 入口已删除；
- 失败、Reload、Stop、并发和资源关闭语义通过测试；
- 当前权威文档已同步，任务记录不冒充当前 API 入口；
- 所有适用验证通过，没有把未执行检查描述为已通过；
- 实施范围没有引入通用 Catalog、业务能力或外部副作用。

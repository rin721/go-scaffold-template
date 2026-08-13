# 任务账本：配置重载与生命周期修复

## 1. 确认门禁

- 当前方案状态：**已完成**。
- 当前 Git 基线：`main@139d437e4407583f6a71afd17808e149a9663d72`，与本地 `origin/main` 一致。
- 009 是独立任务，不属于 008；当前工作树的 008 未提交改动只作为必须保护和兼容的事实。
- 用户已在方案报告后的后续消息中明确要求“开始修复009方案”；`ENT-001` 至 `VER-001` 已获实施授权。
- 实施完成后按仓库规则把 009 文档、实现、测试和权威文档作为一个独立任务提交；不 push、不改写历史。
- 确认后若公共 API、监听失败策略、配置优先级、热换边界、依赖或外部副作用实质变化，状态回到待确认并重新报告。

## 2. 任务清单

| ID | 任务 | 工作量 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `DOC-001` | 建立独立 009 四件套与导航 | S | 无 | 根因、范围、设计、任务、门禁和验收矩阵完整 | 已完成 |
| `ENT-001` | 默认服务入口显式启用 Watch | S | 用户确认 | 服务注册 watcher；CLI 不注册；错误使用稳定 Logger 分类上报 | 已完成 |
| `WCH-001` | 建立 watcher ready 与 reconciliation | M | `ENT-001` | 初始 Load 到目录注册之间无丢更新窗口；无第二套 Reload | 已完成 |
| `WCH-002` | 补齐文件事件与退出测试 | M | `WCH-001` | Write/rename/remove-create、防抖、基础设施失败、取消无泄漏 | 已完成 |
| `DB-001` | 真实 SQLite Database 换代验收 | L | `WCH-001` | v1/v2 标记证明稳定 Access 切换；旧代/当前代各关闭一次 | 已完成 |
| `DB-002` | Database 失败保旧与恢复 | M | `DB-001` | 非法/不可用候选保旧；后续有效候选恢复；在途租约排空 | 已完成 |
| `LOG-001` | Logger 与跨组件整轮原子性 | L | `WCH-001` | facade 成功切换、失败保旧、同轮失败不部分提交、Stop 恢复 baseline | 已完成 |
| `LFC-001` | 当前其他能力生命周期矩阵 | M | `ENT-001` | Direct 能力身份稳定；CLI、Participant、Supervisor 启停边界通过 | 已完成 |
| `CFG-001` | 配置优先级与安全诊断 | M | `WCH-001` | Env 覆盖语义明确；日志无 Snapshot/DSN；错误分类真实 | 已完成 |
| `DOC-002` | 同步当前权威文档 | M | 上述实现任务 | 默认 Watch、优先级、失败、退出和非目标与代码一致 | 已完成 |
| `VER-001` | 全量验证、Diff 审阅与独立任务提交 | L | `DOC-002` | build/test/race/vet/smoke/diff-check；只提交 009 归属增量 | 已完成 |

`S/M/L` 只表示相对工作量。009 不接管 008 的任务 ID、实现状态或最终提交。

## 3. 实施顺序

```text
DOC-001 -> 用户确认
  -> ENT-001 -> WCH-001 -> WCH-002
  -> DB-001 -> DB-002
  -> LOG-001 -> LFC-001 -> CFG-001
  -> DOC-002 -> VER-001
```

Database/Logger 集成测试必须走完整 Loader、composition、Kernel、Host 和 Watch 链路；不得通过直接调用 `Reload` 跳过本次入口缺陷。Kernel 单元测试仍用于精确失败阶段和状态机验证。

## 4. 验收矩阵

| 领域 | 必需证据 |
| --- | --- |
| Root cause | 入口空 `HostOptions` 的回归测试；服务 watcher 注册，CLI 不注册 |
| Watch | ready reconciliation、启动窗口、Write/rename/remove-create、防抖、错误继续、取消退出 |
| Database | 真实 SQLite v1/v2、稳定 Access、候选 Ready、租约排空、旧代与当前代关闭 |
| Logger | 稳定 facade、成功/失败切换、同轮原子性、Stop 恢复 baseline |
| Config | file -> env、effective digest、覆盖字段不误报、无 DSN/Snapshot 泄漏 |
| Lifecycle | Kernel/Participant/task 顺序、Direct 不重建、启动补偿、Stop 错误链 |
| Delivery | build、unit、race、vet、真实 smoke、旧描述搜索、diff-check、独立暂存审阅 |

## 5. 当前证据

### 方案轮（2026-08-14）

- 用户明确纠正本任务为独立 009；已在工作计划、文档状态和任务 ID 中与 008 分离。
- `git status` 显示 `main` 与本地 `origin/main` 都在 `139d437`，工作树存在 008 的已跟踪和未跟踪改动；本轮未改写这些实现。
- 代码定位确认 `cmd/app/main.go` 使用 `kernel.NewHost(runtime, kernel.HostOptions{}, ...)`；`internal/kernel/host.go` 只有 `Watch != nil` 才注册 `kernel-config-watch`。
- 用户日志没有 `kernel reload completed` 或 reload error，只有初始启动与 Ctrl+C 停止，和上述代码路径一致。
- 已核对 `Kernel.Reload`、managed component、Lease、Host、WatchFiles 与 Supervisor：通用候选、排空、提交、回滚、清理和反向停止机制存在。
- 已确认应用/Database 验收缺口：`cmd/app` 测试没有文件热变更场景；Database App 测试只覆盖定义边界与单租约事务，没有真实 SQLite 跨配置换代。
- 已确认 watcher 启动窗口：初始 Snapshot Load 与 fsnotify 目录注册之间没有 ready/reconciliation，极短窗口内的文件变化可能丢失。
- 已确认 Source 优先级为 File 后 Env；存在 `APP_DATABASE__*` 覆盖时，文件同字段变化可能不改变 effective Database 配置。
- 只读基线验证通过：`go test ./internal/kernel/... ./cmd/app ./pkg/supervisor ./pkg/database`、`go vet ./...`、`git diff --check`。这些结果包含当前 008 工作树，且不代表 009 修复已实现。
- 未启动服务、未修改用户配置、未执行真实 reload smoke、未运行 race、未暂存、未提交、未 push。

### 确认与实施轮（2026-08-14）

- 用户在独立 009 方案报告后明确要求“开始修复009方案”，实施范围固定为 `ENT-001` 至 `VER-001`。
- 启用提交技能后复核：HEAD 仍为 `139d437`，暂存区为空；008 的既有修改路径已记录并继续受保护。

### 实施与验证轮（2026-08-14）

- `cmd/app` 服务模式显式传入 `WatchOptions`；reload 边界按 rejected、restart-required、committed-cleanup 分类记录，只输出安全 `error_type`，不记录原始错误、Snapshot 或 DSN。CLI 分支仍在 Host 构造前返回。
- `config.WatchFiles` 增加 `WatchCallbacks`；全部父目录注册成功后先发送 ready reconciliation，Write/Create/Rename/Remove 事件继续防抖并只投递通知。Kernel 复用容量 1 的变化队列和 `operationMu` 串行执行原有 `Reload`，没有第二套应用路径。
- 测试覆盖 ready 顺序、连续写防抖、rename-save、remove-create、缺失父目录、启动窗口 reconciliation、单次失败继续监听、watcher 基础设施失败触发 application/Kernel 反向停止，以及取消后无残留 Host。
- 完整 composition + Host + 真实 SQLite 集成覆盖：v1 初始代；Logger 候选已准备但非法 Database 候选使整轮保旧；后续有效 Logger+Database 候选同轮提交到 v2；稳定 facade/Access 不重新注入；v1 在 reload 后释放、v2 在 Stop 后释放。
- Kernel 事件测试确认 v1、v2 每代只执行一次 Stop；Database 在途租约排空超时与旧代恢复沿用既有用例并通过。
- Clock、ID Generator、Validator 在 reload 前后保持同一 direct output 且继续可用；application Participant 只 Start/Stop 各一次。环境变量覆盖测试确认 effective Database section digest 不随被覆盖的文件字段变化。
- 定向测试重复 10 轮通过；Kernel/config/composition/cmd 定向 race 通过。
- 当前完整工作树执行 `go build ./cmd/app`、`go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...`、`go mod tidy -diff`、`git diff --check` 均通过。
- 在系统临时目录从纯 `HEAD@139d437` 导出仓库、只叠加 009 Go 文件后，定向测试通过，证明独立 009 commit 不依赖未提交 008 文件才能编译；旧 HEAD 不支持 SQLite，真实 SQLite 用例按 typed `ErrInvalidDriver` 明确 Skip，当前完整工作树上该用例真实通过。
- 独立进程 smoke 使用系统临时目录，不读取或修改用户 `config.yaml`/`.data`：同一进程完成 Logger+Database 同轮换代、非法 Driver 候选保旧、有效候选恢复、三代 SQLite 创建；发送 Windows `CTRL_BREAK_EVENT` 后 application 与 Kernel 正常停止，退出码为 0，日志未泄露 Driver/DSN。
- Markdown 相对链接校验通过。PostgreSQL/MySQL 真连接不属于本缺陷必需验收，本轮未执行，不能用 SQLite 结果代替。
- `go build` 产生的仓库根 `app.exe` 已移动到 smoke 临时目录，工作树没有 009 生成物。执行环境拒绝删除两个精确的系统临时验证目录；它们不在仓库内，最终交付中如实报告。
- 009 独立暂存将按精确 pathspec 和重叠文件 hunk 执行；暂存 Diff、Commit 标题与短哈希由提交后核验并在交付报告中给出，不在本文件预写未知哈希。

## 6. 实施记录模板

确认后每轮追加：

- 已实施的 009 任务 ID 与精确文件/hunk；
- 与 008 重叠文件的归属判断和保护证据；
- 生命周期事件、实例切换、关闭次数与错误分类证据；
- 命令、真实结果、未执行项与剩余风险；
- 独立暂存 Diff 和最终 Commit ID。

## 7. 完成判定

只有以下条件全部满足，009 才能标记完成：

- 默认服务真正监听文件并封闭 watcher 注册窗口；
- Database/Logger 使用当前完整应用链路完成成功换代、失败保旧、恢复与停止验证；
- 其他当前能力的生命周期或无生命周期语义有明确证据；
- 配置优先级和安全诊断不误导、不泄密；
- 当前权威文档与实现同步，不把 008 目标写成现状；
- 全量验证通过，未执行项如实记录；
- 只提交 009 可确认归属的改动，不夹带 008，也不 push 或改写历史。

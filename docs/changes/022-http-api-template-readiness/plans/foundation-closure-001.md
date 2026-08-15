# FOUNDATION-CLOSURE-001：剩余 Foundation 单轨闭环计划

## 1. 状态与授权边界

- 当前状态：**已确认并实施完成**。
- 研究依据：[R008](../research/R008-remaining-foundation-closure/report.md)。
- 基线：`3a936a5`；实施前必须复核 HEAD、worktree 和本文文件影响。
- 单轨关系：本计划替换未启动的 `FOUNDATION-ACCEPTANCE-001` 与 `FOUNDATION-RECONCILIATION-001`，不保留第二套当前施工计划。
- 确认依据：用户在本 goal 中明确授予对 AGENTS 二次确认门禁的临时全流程例外，要求连续完成研究、计划、实施、验证和提交。
- 当前允许：仅实施 `FCL-001..008`，执行本地验证，显式暂存本任务文件并创建一个 Conventional Commit。
- 始终禁止：推送、部署、启动常驻服务、修改真实配置、访问外部系统或把 HTTP 产品治理混入本计划。

## 2. 目标与完成定义

在不改变 Kernel/Coordinator/Supervisor 分工、不新增依赖和公共 API 的前提下完成当前已证明同步 HTTP/CLI profile 的 Foundation 闭环：

1. preflight-only `RestartRequired` 可以由后续完整有效且不再要求重启的候选安全解除；
2. degraded cleanup debt 仍永久 fail-closed 到进程重启，不被上述恢复路径误清；
3. Host、Service/CLI、真实本地资源和 process diagnostics 的缺失跨层证据补齐；
4. `FND-CONFIG-*`、`FND-ASSEMBLY-*`、`FND-LIFECYCLE-*`、`FND-RUNTIME-*`、`FND-DIAGNOSTICS-*`、`FND-GOV-*` 与 `FND-ACCEPT-001..005` 在 current profile 中全部有可定位证据；
5. 022 单轨标记 `Foundation-closed(current synchronous HTTP/CLI profile)`，同时继续阻断未评估的 Runner/Health/new resource/long-lived connection profile；
6. 完整 validation、Diff 审计和本任务 commit 成功。

## 3. 冻结设计

### 3.1 RestartRequired 恢复语义

`RestartRequired` 分成两类，不增加新公开类型：

| 状态来源 | 是否产生副作用 | 后续候选行为 |
| --- | --- | --- |
| application binding 或 Kernel component preflight 要求重启 | 否；不得 Build/Commit | 继续允许 Loader + 全 owner Validate + Kernel preflight；仍要求重启则保持 latch，恢复候选成功则清除 |
| committed previous cleanup failure | 是；新 generation 已提交且旧 owner 未终结 | `LifecycleDegraded + CleanupRequired` 继续阻断 Reload，不自动恢复 |

实现保持 `Coordinator.Reload(ctx)` 公共签名和 `app.ErrRestartRequired` 不变：

1. `LifecycleRunning` 即使 diagnostics 中已有 `RestartRequired`，仍允许加载下一候选；
2. Loader/owner validation 失败时保持当前 generation 与既有 latch，不清除；
3. application 或 Kernel preflight 再次要求重启时保持 latch，且 Stage 之后 Build/Commit 为零；
4. 候选相对当前有效 generation 不再要求重启，并完整通过现有 reload 事务后，才原子清除 latch；
5. `LifecycleDegraded`、`CleanupRequired` 或 incomplete ownership 继续返回 blocked；
6. 清除只影响 diagnostics/reload admission，不伪造 config generation，也不改变当前 Snapshot。

不得通过“收到任意文件事件就清 flag”、比较原始文件文本或绕过全 owner validation 实现恢复。

### 3.2 验收证据策略

不复制三个已实施计划的测试。现有测试继续作为分层证据，只新增当前缺失的跨层断言：

- Coordinator：latch 建立、持续、无效候选保持、恢复候选解除、恢复后 hot reload、degraded 不可恢复；
- Host：实际 uncooperative Participant/Task 经 Supervisor 后在唯一 `ProcessDiagnostics` 中仍能定位 owner/kind/phase/state/budget；测试结束必须显式释放 test double，不能留下 goroutine；
- process current profile：Service cancellation 后 HTTP listener 可重绑、SQLite 文件可重命名/删除，CLI 结束后同一资源可再次取得所有权；
- existing evidence mapping：candidate/retired/current cleanup、terminal attempt cache、HTTP force、fsnotify/logger/storage/cache release、Env shape conflict、Service/CLI 顺序、architecture boundary。

### 3.3 Foundation-closed 的精确边界

通过标签仅为：

```text
Foundation-closed(current synchronous HTTP/CLI profile)
```

它包含同步 HTTP request/response、one-shot CLI、startup Participant、现有 Kernel capability、SQLite/local Storage/disabled Cache 的默认本地组合，以及现有可选资源的 hermetic contract tests。

它不包含 external PostgreSQL/MySQL/Redis/S3 runtime、WebSocket/hijacked、后台 consumer/scheduler、动态 Health、新共享资源、management operation、部署第二次信号、Linux runtime 或 release portability。

## 4. 文件影响

### 4.1 非文档实施文件

| 文件 | 计划改动 |
| --- | --- |
| `internal/kernel/coordinator.go` | 分离 preflight restart latch 与 degraded cleanup block；仅在完整成功候选后清 latch |
| `internal/kernel/kernel_test.go` | 增加 restart latch 持续/恢复/不误清与 recovery 后 reload 测试 |
| `internal/kernel/host_test.go` | 增加实际 Host -> ProcessDiagnostics pending/failed/budget 跨层测试 |
| `cmd/app/main_test.go` | 增加 Service/CLI 正常退出后的 listener 与 SQLite 物理所有权断言 |

实际实现复用了现有 test harness，没有新增 production package、依赖、脚本或 CI。

### 4.2 文档与研究

| 文件 | 计划改动 |
| --- | --- |
| `README.md`、`internal/kernel/README.md` | 只在实现通过后同步当前 restart recovery 与 Foundation profile 边界 |
| `docs/changes/022-http-api-template-readiness/{README.md,requirements.md,design.md,acceptance.md,tasks.md}` | 单轨 Program、十一门结果、业务解锁和逐轮证据 |
| `docs/changes/022-http-api-template-readiness/research/README.md` | R008 作为当前 Foundation 研究入口 |
| `R002/R004 metadata.yaml` | 标记被 R008 supersede，正文保留历史不重写 |
| `docs/changes/README.md` | 实施后同步 022 当前状态 |
| 本计划 | 回写每个 FCL 结果、验证、commit 与剩余场景风险 |

## 5. 稳定任务清单

| Task ID | 依赖 | 工作 | 完成条件 | 当前状态 |
| --- | --- | --- | --- | --- |
| `FCL-001` | 本 goal 明确确认 | 修改 Coordinator restart latch 判定和成功清除点 | 公共 API/错误不变；degraded cleanup 仍阻断；只有完整成功候选清除 | 已完成 |
| `FCL-002` | `FCL-001` | 补 restart reconciliation 状态机与事务测试 | 建立、重复、invalid、revert、恢复后 reload、degraded 六类通过；Build/Commit side effect 受断言 | 已完成 |
| `FCL-003` | `FCL-001` | 补实际 Host diagnostics 跨层故障测试 | Participant/Task owner、phase、pending/failed、budget 在 Host authority 可定位且测试无 goroutine 泄漏 | 已完成 |
| `FCL-004` | 无 | 补 Service/CLI 当前 profile 物理释放断言 | cancel/operation return 后 listener 可重绑、SQLite 文件可重新取得所有权；Windows 文件锁语义被真实验证 | 已完成 |
| `FCL-005` | `FCL-002..004` | 执行 Foundation evidence mapping 与旧原语/边界残留审计 | 每个 FND/FND-ACCEPT ID 对应代码/test/command；无第二 owner、locator、万能 Close/retry 假实现 | 已完成 |
| `FCL-006` | `FCL-005` | 刷新当前权威文档、R002/R004 关系和 022 单轨状态 | current profile 与未验证场景分离；旧两个未启动 Program 无当前 authority 残留 | 已完成 |
| `FCL-007` | `FCL-001..006` | 完成目标/full/race/vet/build/cross-build/文档/Diff 验证 | 第 7 节适用命令全部通过；Windows tidy/WSL 限制如实保留给 portability | 已完成 |
| `FCL-008` | `FCL-007` | 审阅并精确暂存本任务文件，创建 Conventional Commit | staged diff 仅本任务、无凭据/产物；commit 后 worktree 状态如实报告；不 push | 随本任务提交完成 |

## 6. 精确测试矩阵

### 6.1 Coordinator reconciliation

1. application-owned section 变化返回 `ErrRestartRequired`、`Applied=false`、generation/digest 不变、latch=true；
2. Kernel `RestartRequired` component 变化同样在 Build/Commit 前返回；
3. 第二个仍要求重启的候选继续返回 typed error，latch 不清；
4. 无效 File/Env/owner 候选返回原错误，latch 不清；
5. 恢复到 current effective config 后 Reload 成功，latch 清除，generation 不伪增；
6. 恢复候选同时包含合法 hot-swappable 变化时，只按现有事务一次提交，并在成功后清 latch；
7. committed cleanup failure 的 degraded/cleanup owner 即使文件恢复仍 blocked，不能借本路径清除；
8. watcher 在 restart-required 后继续接受恢复事件，不需要重启 watcher。

### 6.2 Host diagnostics

1. short shared shutdown budget 下 uncooperative Participant 进入 participant/pending/stop；
2. uncooperative Task 进入 task/pending/stop；
3. Host `Ready=false`、process 不是 clean stopped，budget deadline/exhausted 与 Supervisor 一致；
4. 释放 test double 后 goroutine 完成，测试自身不泄漏；不虚构 Go goroutine kill。

### 6.3 Current-profile resource release

1. Service ready 后正常请求可达；cancel 后 `process.run` 有界返回；
2. 原 HTTP address 可重新 `net.Listen`；
3. SQLite 文件可在 Windows 上 rename 后再打开，证明 pool owner 已释放；
4. one-shot CLI 每次完整 Start/operation/reverse Stop，连续 invocation 与 Service 可共享同一 SQLite 数据；
5. 复用现有 logger/fsnotify/file Storage/Redis test TCP/HTTP force 测试，不要求真实外部服务。

## 7. 验证与静态门禁

确认实施后按顺序执行：

```text
gofmt -w <本任务修改的 Go 文件>
go test ./internal/kernel/... -count=1
go test ./cmd/app ./internal/composition -count=1
go test ./pkg/supervisor ./pkg/httpx ./pkg/database ./pkg/logger ./pkg/storage ./pkg/cache/... -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; go build ./...   # 在独立命令进程中，不污染后续环境
git diff --check
```

另执行：

- 生产 Go 源码搜索 `WithStop`、`pkg/resource`、`Close(force bool)`、service locator/registry 与旧 diagnostics authority；
- 当前文档搜索 `FOUNDATION-ACCEPTANCE-001`、`FOUNDATION-RECONCILIATION-001`，只允许 R008/本计划解释被替换关系，或既有研究/已完成施工计划保留当时的历史停止线；
- Markdown 相对链接检查必须排除 fenced 与 inline code；
- `git status --short --branch`、完整 diff、`git diff --stat`、`git diff --check`、staged diff 和敏感信息审计。

`go mod tidy -diff` 在当前 Windows checkout 只因 `go.sum` CRLF/LF 返回 1，且本计划不改依赖与行尾；记录为 `PORTABILITY-001` 证据，不以改写 `go.sum` 或 `.gitattributes` 冒充 Foundation 修复。当前无可运行 WSL distro，Linux runtime 不声明通过；只执行 cross-build，并保留 Ubuntu CI 作为仓库现有远端门禁定义，不访问外部系统查询状态。

## 8. 验收映射

| 门禁 | 主要证据 |
| --- | --- |
| `FND-CONFIG-001..003` / `FND-ACCEPT-004` | 已实施 Config tests + `FCL-002` invalid candidate 保持 latch |
| `FND-ASSEMBLY-001..003` | architecture/composition tests + `FCL-005` 残留审计 |
| `FND-LIFECYCLE-001..008` / `FND-ACCEPT-001/002` | lifecycle plan tests + `FCL-004` process physical release |
| `FND-RUNTIME-001/002` / `FND-DIAGNOSTICS-001..003` / `FND-ACCEPT-003` | diagnostics plan tests + `FCL-003` actual Host authority；003 以“没有未授权运行中 operation”通过 |
| `FND-GOV-001..004` / `FND-ACCEPT-005` | `FCL-002..007`、全量/race/vet/build/cross-build、docs/architecture/Diff gate |
| 十一门与业务解锁 | `FCL-006` 刷新 acceptance；只解锁 current synchronous HTTP/CLI profile |

## 9. 非目标

- 不增加新公共 API、依赖、状态容器、后台 reconciliation goroutine、retry queue 或 management endpoint。
- 不自动恢复 `LifecycleDegraded`、cleanup debt、terminal-failed 或 forced owner。
- 不把 `RestartRequired` 改成 hot reload，也不提交候选中的 restart-only section。
- 不接入真实 PostgreSQL/MySQL/Redis/S3，不实现 WebSocket/hijacked registry。
- 不修改 `.github/workflows`、`.gitattributes`、`go.mod`、`go.sum` 或全仓行尾。
- 不实现 API authority、protocol/security/observability/delivery/release。
- 不设计新业务模块，不扩大 `module.Contribution`。

## 10. 必须退回研究并重新确认的变化

出现以下任一事实时停止实施：

- 安全恢复必须改变 `Coordinator.Reload` 公共签名、`app.ErrRestartRequired` 或配置优先级；
- 需要新增依赖、持久 recovery state、management transport 或后台 goroutine；
- 发现 cleanup debt 也会被当前恢复路径误清，且无法在现有 state 中窄修复；
- current profile 物理释放失败暴露新的 owner/close 语义，而不是缺测试；
- 需要修改数据库、HTTP、module、部署或外部系统边界。

## 11. 停止线与确认结果

用户在本 goal 中明确授予对 AGENTS 二次确认门禁的临时全流程例外，本文已经确认并实施。`FCL-001..008`、验证和 commit 完成即停止；不 push、不部署、不访问外部系统。

## 12. 实施结果与证据

### 12.1 代码与测试

- `Coordinator.Reload` 只对 `LifecycleDegraded/CleanupRequired` 保持永久 block；已有 preflight restart latch 不再阻止加载下一候选。
- `completeReload` 在成功事务的同一锁内发布 config generation/current Snapshot 并清除 restart latch；无效/重复 restart 候选不会调用该路径。
- Kernel 测试覆盖 Kernel 与 application-owned restart、重复候选、owner validation error、恢复候选、恢复同时 hot swap、watcher rename 发布和 degraded 不误清。
- Host 测试通过实际 Supervisor 路径证明不合作 Participant/Task 在共享 budget 后仍以 typed pending responsibility 出现在唯一 `ProcessDiagnostics`，并在测试结尾释放 owner goroutine。
- 进程测试在 Service/CLI 正常退出后重新绑定 HTTP address，并在 Windows 上往返 rename SQLite 文件，证明 listener 与数据库 pool 的物理所有权已经释放。
- watcher 新测试最初使用原地写入，在 `-count=5` 中真实捕获到 truncate 中间候选；已改为临时文件 + rename，受影响包 `-count=10` 稳定通过。

### 12.2 验证

以下检查在 Windows amd64、Go 1.25.7 上通过：

```text
go test ./internal/kernel/... -count=1
go test ./cmd/app ./internal/composition -count=1
go test ./pkg/supervisor ./pkg/httpx ./pkg/database ./pkg/logger ./pkg/storage ./pkg/cache/... -count=1
go test ./internal/kernel ./cmd/app -count=10
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
GOOS=linux GOARCH=amd64 go build ./...
```

一次把 full test、race 与多个重叠包测试并行启动的探针返回失败；隔离提取时没有复现，随后 full test、race 和受影响包 10 轮均独立通过，因此最终门禁不使用跨进程重叠验证。旧原语搜索、Markdown/metadata、`git diff --check` 与 staged scope 在提交前完成。

本地没有可运行 WSL distro；未声明 Linux runtime。Windows `go mod tidy -diff` 的 CRLF/LF 全文件差异继续归入 `PORTABILITY-001`，本任务未改依赖、CI、`.gitattributes` 或行尾政策。

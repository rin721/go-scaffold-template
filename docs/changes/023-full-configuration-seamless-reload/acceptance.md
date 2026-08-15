# 023 验收台账

## 1. 结论

`RLD-001..013` 已有当前代码与先前稳定工作区的定向测试证据。用户于 2026-08-15 批准跳过 Linux amd64 真实 Service runtime 和真实 Redis backend/tag namespace 切换；两项保持“未验证”，不再等待环境，也不使用 cross-compile/mock 冒充通过。`RLD-014` 当前仍被 024 未提交 Todo/API 改动造成的编译失败和本机 Go 临时目录空间不足阻断，`RLD-015` 因而不能宣称整项竣工。所有提交均为本地提交，本任务不执行 push。

实现主体位于 `56ce851 feat(runtime): switch immutable application generations`。后续 ListenerHub 背压、结构化失败诊断、动态 active work/resource 诊断和 Windows sharing violation 精确测试随 `86c2aca feat(api): establish strict OpenAPI transport` 一并落盘；该提交还包含 024 的 API 工作，因此它只能作为实现落点证据，不能作为 023 的独立纯净提交证据，也不得改写历史拆分。

## 2. 环境

| 项目 | 实际环境 |
| --- | --- |
| 宿主 | Windows amd64，PowerShell |
| Go | 当前仓库 toolchain；`go build`、`go vet` 与 race detector 可运行 |
| Linux | WSL 功能存在，但没有已安装且可运行的 distribution；Docker、Podman、QEMU 均不可用；用户批准跳过，状态保持未验证 |
| Redis | 本机没有可启动的真实 Redis server；用户批准跳过，未用 miniredis 冒充真实后端验收 |
| Git | `main`；本任务不 push、不 rebase、不 amend |

## 3. 十一项需求验收

| # | 状态 | 证据与边界 |
| --- | --- | --- |
| 1 | 部分通过 | `TestEachConfigurationSectionCreatesOneCompleteGeneration` 覆盖七节单改，`TestAllConfigurationSectionsCommitOneGeneration` 覆盖组合候选；Windows 同 PID 真实进程验证 Todo 从允许 10 字符切为上限 5，HTTP 从 `201` 变为 `400`，PID 始终为 `60468`，日志显示 generation `1 -> 2`。真实 Redis section 经用户批准跳过，保持未验证。 |
| 2 | 通过 | `TestAllConfigurationSectionsCommitOneGeneration` 断言组合修改只提交一个新 generation；GenerationCoordinator 测试覆盖 no-op、candidate failure 与 cleanup debt。 |
| 3 | 部分通过 | `TestListenerHubContinuousRequestsSurviveSameAddressCommit`、`TestApplicationGenerationReloadsTodoAndHTTPWithoutRestart` 与先前 `go test -race` 通过；Windows 真实进程验证同地址生效；Linux 真实持续请求经用户批准跳过，保持未验证。 |
| 4 | 部分通过 | ListenerHub 的新地址 prepare/abort、old request pinning 和 retire 测试通过；尚缺 Windows/Linux 双平台的真实地址迁移脚本证据。 |
| 5 | 通过 | `TestApplicationGenerationPinsOldDatabaseUntilRetire` 证明旧请求固定旧 DB、新请求使用新 DB；Database reload 在 commit 前执行只读 schema readiness。 |
| 6 | 未验证（批准跳过） | generation-owned Cache Client/L1/tag index 已实现并有生命周期测试；真实 Redis backend/tag namespace 切换由用户批准跳过，未用内存实现或 mock 替代。 |
| 7 | 通过 | `TestApplicationGenerationPinsOldDatabaseUntilRetire` 同时证明旧在途请求使用旧 Policy，新请求使用新 Policy。 |
| 8 | 通过 | stable file、GenerationCoordinator prepare failure、listener bind failure、schema readiness 与 Service reload reporter 测试证明失败保留 current。 |
| 9 | 通过（Windows） | stable-file 用例覆盖原地写、atomic replace、连续变化、权限/非普通文件、取消；`stable_file_windows_test.go` 精确覆盖 Windows sharing violation errno。 |
| 10 | 通过（自动化） | GenerationCoordinator 测试覆盖 cleanup debt、force stop、结构化 phase/owner；ListenerHub 覆盖长请求 pinning、active connection、Stop/Wait 和无丢弃背压。Windows 真实进程收到 Ctrl+C 后记录 application stopping 与资源停止。 |
| 11 | 未完成 | 先前稳定工作区的定向 test/race、`go vet ./...`、`go build ./cmd/app` 与 Linux/amd64 cross-compile 已通过；Linux 真实 runtime 经用户批准跳过并保持未验证。当前复跑因 024 未提交 Todo/API 改动编译失败，并叠加 C 盘 Go 临时目录空间不足，不能记为通过。 |

## 4. 已执行命令与结果

以下结果来自 2026-08-15 当前工作区；cross-compile 只证明 Linux 编译，不等价于 Linux runtime：

```powershell
go test -mod=mod ./internal/kernel/config ./internal/kernel ./internal/composition ./pkg/httpx -count=1
go test ./pkg/httpx -run 'ListenerHub' -count=20
go test ./internal/composition -run 'ApplicationGeneration|AllConfiguration|ServiceRuntime' -count=10
go test ./internal/kernel/config -run 'StableFile|WatchRecovers' -count=20
go test ./internal/composition -run 'ApplicationGenerationPinsOldDatabase' -count=10
go test -race ./internal/kernel/... ./pkg/httpx ./internal/composition ./internal/module/todo/... -count=1
go vet ./...
go build ./cmd/app
$env:GOOS='linux'; $env:GOARCH='amd64'; go test -exec='cmd.exe /d /c rem' ./... -count=1
```

上述命令是在 023 实现落盘后的稳定工作区通过，并非当前脏工作区的最新绿灯。2026-08-15 当前复跑出现：024 未提交 Todo 变更导致 `service.go` 的 `undefined: todo` 和 `model.New` 参数数量不匹配；全量测试还出现 Go linker `There is not enough space on the disk`，并有并行运行时 Windows watcher 删除文件遭 sharing violation。023 不越界修改这些 024 源码，也不把这轮失败记为通过。

## 5. Windows 真实进程证据

1. 从当前代码构建临时 `app.exe`，临时配置监听 `127.0.0.1:50502`。
2. 启动后 PID 为 `60468`；初始 Todo 标题 10 字符的 POST 返回 `201`。
3. 把临时配置的 `todo.titleMaxRunes` 从 `120` 改为 `5`。
4. 同一 PID 的新 POST 返回 `400`；日志记录 `generation reload completed`、generation `2`、`changed_sections=["todo"]`，监听地址不变。
5. Ctrl+C 后日志记录 `application stopping` 与所有 generation owner 停止；没有遗留 app 进程。

临时目录 `C:\Users\xiaol\AppData\Local\Temp\go-scaffold-023-5b1f0c4844194b6abb70f2c50bdf7f32` 尚未删除：已执行严格路径校验的清理命令被当前执行策略在运行前拒绝，未改用更强或更宽的删除方式。

## 6. RLD 完成判定

| 任务 | 判定 | 当前证据 |
| --- | --- | --- |
| RLD-001 | 已实施 | 稳定双采样、瞬时错误分类、bounded retry 与跨平台专用文件；定向重复测试通过。 |
| RLD-002 | 部分验收 | ListenerHub 原型、背压、pending ownership、same-address 与 address-change 自动化通过；Linux 真实 runtime 经用户批准跳过，保持未验证。 |
| RLD-003 | 已实施 | GenerationFactory、状态机、latest-wins、Abort/Commit/Retire、cleanup debt 与 shutdown 测试通过。 |
| RLD-004 | 已实施 | Logger/Database/Cache/I18n/Storage typed slot、digest、引用计数与反向 journal 已落地；race 通过。 |
| RLD-005 | 已实施 | process baseline 与 generation target 分离，commit 切换及旧代引用终结已有测试。 |
| RLD-006 | 已实施 | 单 Snapshot 构造完整 capabilities、Todo、Router、Server 与 lifecycle owners。 |
| RLD-007 | 已实施，跨平台验收未闭环 | Service 已接 ListenerHub；持续请求、长请求、bind failure、shutdown 测试通过；Linux 真实 runtime 批准跳过。 |
| RLD-008 | 已实施 | Todo Policy/Repository/Handler/Router 按代构造，old/new DB 与 Policy pinning 测试通过。 |
| RLD-009 | 已实施，真实 Redis 验收未闭环 | Cache Client/L1/tag index/cleanup owner 已代际化；真实 Redis 运行证据经用户批准跳过，保持未验证。 |
| RLD-010 | 已实施 | Database/I18n/Storage 进入完整 generation，Database 执行只读 schema readiness。 |
| RLD-011 | 已实施 | watcher 只触发串行 GenerationCoordinator，组合修改一代、失败后恢复。 |
| RLD-012 | 已实施 | diagnostics 包含 generation、route、active work、resource build/reuse、phase/owner/type；错误链保留且日志脱敏。 |
| RLD-013 | 已实施 | 长期 Service 单轨使用 Application Generation；底层 Kernel 的 RestartRequired 仅保留给 one-shot/通用组件协议，不是 Service 七节 reload 入口。 |
| RLD-014 | 未完成 | Linux/真实 Redis 已批准跳过并标记未验证；当前全量测试仍被 024 并行改动和 Go 临时目录空间不足阻断。 |
| RLD-015 | 进行中 | 权威说明、验收台账与 Diff 审阅可完成；依赖 RLD-014，不能提前标记竣工。 |

## 7. 继续验收条件

Linux amd64 真实 runtime 与真实 Redis server 不再是本轮等待项，但必须继续标记未验证。要关闭 `RLD-014` 与 `RLD-015`，需要等待 024 工作区恢复可编译状态、为 Go 临时构建提供足够空间并重新执行本地全量测试；随后更新本台账、审阅只属于 023 的最终 Diff，并创建本地 Conventional Commit，仍不 push。

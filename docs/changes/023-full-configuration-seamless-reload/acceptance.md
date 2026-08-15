# 023 验收台账

## 1. 结论

`RLD-001..015` 已完成当前批准范围内的本地实施与验收。用户于 2026-08-15 批准跳过 Linux amd64 真实 Service runtime 和真实 Redis backend/tag namespace 切换；两项保持“未验证”，不再等待环境，也不使用 cross-compile/mock 冒充通过。Windows 外部进程七节组合重载、独立 HEAD 的定向/全量 test、race、vet、build 均通过。所有提交均为本地提交，本任务不执行 push。

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
| 1 | 通过（带批准的未验证项） | `TestEachConfigurationSectionCreatesOneCompleteGeneration` 覆盖七节单改，`TestAllConfigurationSectionsCommitOneGeneration` 覆盖组合候选；Windows PID `52696` 外部进程一次修改七节，日志显示 generation `1 -> 2` 和完整 `changed_sections`。真实 Redis 经用户批准跳过，保持未验证。 |
| 2 | 通过 | `TestAllConfigurationSectionsCommitOneGeneration` 断言组合修改只提交一个新 generation；GenerationCoordinator 测试覆盖 no-op、candidate failure 与 cleanup debt。 |
| 3 | 通过（带批准的未验证项） | `TestListenerHubContinuousRequestsSurviveSameAddressCommit`、`TestApplicationGenerationReloadsTodoAndHTTPWithoutRestart` 与 `go test -race` 通过；Windows 真实进程验证同地址生效；Linux 真实持续请求经用户批准跳过，保持未验证。 |
| 4 | 通过（Windows） | ListenerHub 的新地址 prepare/abort、old request pinning 和 retire 测试通过；Windows PID `52696` 从 `50611` 切到 `50612`，新地址先成功服务，旧地址停止接受连接。Linux 经用户批准跳过。 |
| 5 | 通过 | `TestApplicationGenerationPinsOldDatabaseUntilRetire` 证明旧请求固定旧 DB、新请求使用新 DB；Database reload 在 commit 前执行只读 schema readiness。 |
| 6 | 未验证（批准跳过） | generation-owned Cache Client/L1/tag index 已实现并有生命周期测试；真实 Redis backend/tag namespace 切换由用户批准跳过，未用内存实现或 mock 替代。 |
| 7 | 通过 | `TestApplicationGenerationPinsOldDatabaseUntilRetire` 同时证明旧在途请求使用旧 Policy，新请求使用新 Policy。 |
| 8 | 通过 | stable file、GenerationCoordinator prepare failure、listener bind failure、schema readiness 与 Service reload reporter 测试证明失败保留 current。 |
| 9 | 通过（Windows） | stable-file 用例覆盖原地写、atomic replace、连续变化、权限/非普通文件、取消；`stable_file_windows_test.go` 精确覆盖 Windows sharing violation errno。 |
| 10 | 通过（自动化） | GenerationCoordinator 测试覆盖 cleanup debt、force stop、结构化 phase/owner；ListenerHub 覆盖长请求 pinning、active connection、Stop/Wait 和无丢弃背压。Windows 真实进程收到 Ctrl+C 后记录 application stopping 与资源停止。 |
| 11 | 通过（带批准的未验证项） | detached HEAD `3d363d5` 的定向 test、`go test ./... -count=1`、race、`go vet ./...`、`go build ./cmd/app` 全部通过；Linux 真实 runtime 经用户批准跳过并保持未验证。 |

## 4. 已执行命令与结果

以下结果来自 2026-08-15 detached HEAD `3d363d5` 的独立验证 worktree；`GOTMPDIR` 指向 D 盘任务专用临时目录，以避开 C 盘空间不足。cross-compile 只证明 Linux 编译，不等价于 Linux runtime：

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

定向命令、`go test ./... -count=1`、race、vet、build 均通过。主工作区复跑曾因 024 未提交 Todo/API 改动和 C 盘空间不足失败；因此最终验证使用同一提交的独立 worktree 隔离用户改动，未 stash、reset 或修改 024 源码。

## 5. Windows 真实进程证据

1. 从当前代码构建临时 `app.exe`，临时配置监听 `127.0.0.1:50502`。
2. 启动后 PID 为 `60468`；初始 Todo 标题 10 字符的 POST 返回 `201`。
3. 把临时配置的 `todo.titleMaxRunes` 从 `120` 改为 `5`。
4. 同一 PID 的新 POST 返回 `400`；日志记录 `generation reload completed`、generation `2`、`changed_sections=["todo"]`，监听地址不变。
5. Ctrl+C 后日志记录 `application stopping` 与所有 generation owner 停止；没有遗留 app 进程。

随后在 detached HEAD `3d363d5` 进行了七节组合外部进程验收：

1. PID `52696` 初始监听 `127.0.0.1:50611`，10 字符 Todo POST 返回 `201`。
2. 一次修改 `logger/database/cache/i18n/storage/http/todo`；新 SQLite 目标预置了当前 schema，Cache 保持 disabled 但切换 generation-owned namespace。
3. 日志只记录一次 `application generation reload completed`，generation 为 `2`，`changed_sections` 精确包含七节，bound address 为 `127.0.0.1:50612`。
4. PID 仍为 `52696`；新地址的相同 POST 按新 Todo Policy 返回 `400`，旧地址停止接受连接。
5. Ctrl+C 后记录 `application stopping` 与全部 Kernel owner 停止，无遗留 app 进程。

临时目录 `C:\Users\xiaol\AppData\Local\Temp\go-scaffold-023-5b1f0c4844194b6abb70f2c50bdf7f32` 尚未删除：已执行严格路径校验的清理命令被当前执行策略在运行前拒绝，未改用更强或更宽的删除方式。

独立验证 worktree `D:\coder\rin721\agent-scaffold\work\go-scaffold-template-023-verify` 也尚未删除：验证完成后的严格清理命令被执行策略在运行前拒绝。进程已停止；目录只包含 detached HEAD、生成的 `app.exe`、忽略的 `config.yaml` 与 `.data` 验收数据。按安全规则未重试更强删除。

## 6. RLD 完成判定

| 任务 | 判定 | 当前证据 |
| --- | --- | --- |
| RLD-001 | 已实施 | 稳定双采样、瞬时错误分类、bounded retry 与跨平台专用文件；定向重复测试通过。 |
| RLD-002 | 已完成（带批准的未验证项） | ListenerHub 原型、背压、pending ownership、same-address 与 address-change 自动化/Windows 进程验收通过；Linux 真实 runtime 经用户批准跳过，保持未验证。 |
| RLD-003 | 已实施 | GenerationFactory、状态机、latest-wins、Abort/Commit/Retire、cleanup debt 与 shutdown 测试通过。 |
| RLD-004 | 已实施 | Logger/Database/Cache/I18n/Storage typed slot、digest、引用计数与反向 journal 已落地；race 通过。 |
| RLD-005 | 已实施 | process baseline 与 generation target 分离，commit 切换及旧代引用终结已有测试。 |
| RLD-006 | 已实施 | 单 Snapshot 构造完整 capabilities、Todo、Router、Server 与 lifecycle owners。 |
| RLD-007 | 已完成（带批准的未验证项） | Service 已接 ListenerHub；持续请求、长请求、bind failure、shutdown 与 Windows 地址迁移通过；Linux 真实 runtime 批准跳过。 |
| RLD-008 | 已实施 | Todo Policy/Repository/Handler/Router 按代构造，old/new DB 与 Policy pinning 测试通过。 |
| RLD-009 | 已完成（带批准的未验证项） | Cache Client/L1/tag index/cleanup owner 已代际化；disabled backend 的 generation namespace 切换通过，真实 Redis 经用户批准跳过并保持未验证。 |
| RLD-010 | 已实施 | Database/I18n/Storage 进入完整 generation，Database 执行只读 schema readiness。 |
| RLD-011 | 已实施 | watcher 只触发串行 GenerationCoordinator，组合修改一代、失败后恢复。 |
| RLD-012 | 已实施 | diagnostics 包含 generation、route、active work、resource build/reuse、phase/owner/type；错误链保留且日志脱敏。 |
| RLD-013 | 已实施 | 长期 Service 单轨使用 Application Generation；底层 Kernel 的 RestartRequired 仅保留给 one-shot/通用组件协议，不是 Service 七节 reload 入口。 |
| RLD-014 | 已完成（带批准的未验证项） | 独立 HEAD 的 Windows 外部进程、定向/全量 test、race、vet、build 通过；Linux/真实 Redis 已批准跳过并标记未验证。 |
| RLD-015 | 已完成 | README/配置示例/任务/验收台账同步，旧 Cache 重启说明删除，链接与 staged Diff 门禁通过，仅创建本地 Conventional Commit，未 push。 |

## 7. 后续可选补验

Linux amd64 真实 runtime 与真实 Redis server 不再是本轮完成条件，但必须继续标记未验证。若后续提供环境，可补跑两项并更新台账；在补验成功前不得把 cross-compile、disabled backend 或 mock 描述成真实运行证据。

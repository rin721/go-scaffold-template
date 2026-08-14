# R021：go-scaffold2 底层闭环实施快照

## 1. 结论

012 的 FND-001..010 与 GOV-F-001..004 已按“保留 Kernel 资源平面、补齐上层控制闭环”的单轨方案实施。旧 CLI contribution、服务 Capabilities 中的 Configuration/CLI、Kernel 自行调用 Loader 的 Start/Reload 入口，以及旧 HTTP Start/Shutdown 生命周期均已删除；没有保留兼容双轨，也没有新增第二个容器或运行时。

基础闭环完成只满足 `AC-BIZ-GATE-001`。当前没有用户确认的真实 actor、业务不变量、数据/事务 owner、入站协议和验收数据，因此 `AC-BIZ-GATE-002`、BIZ-D 与 VSL 继续阻塞。

## 2. 当前运行链

```text
args 非空
  -> ComposeBootstrap
  -> section bindings + DefaultManager + frozen CLI registry
  -> config init/help/parse
  -> 不创建 Kernel、resource、listener 或 goroutine

args 为空
  -> Loader(File -> Env)
  -> Kernel + service capabilities
  -> Coordinator.Prepare: Load 一次 + 全 section strict validate
  -> HTTP config 从同一 candidate 解码
  -> Host/Supervisor: Coordinator -> application -> HTTP Start
  -> blocking HTTP Run + watcher
  -> all required runners acknowledged -> process ready
  -> signal/runtime failure -> ready false -> cancel -> reverse Stop -> bounded Wait
```

默认 Service 使用 application-owned `http` section 和 `http.NotFoundHandler`。它真实绑定配置地址并接受请求，但没有业务路由；未匹配请求返回 404。

## 3. 契约关闭证据

| 主题 | 当前实现 | 代表测试 |
|---|---|---|
| Bootstrap/CLI | `ComposeBootstrap` 只构造六个配置节、DefaultManager 与 CLI；registry 首次 Run 冻结，校验完整树 identity/flag/mode/side-effect/positionals | `TestComposeBootstrapGeneratesAllServiceSectionsWithoutKernel`、`TestRegistryRejectsNestedIdentityFlagAndContractConflictsBeforeRun`、`TestExecuteCoversSuccessConfigAndCancellationExitCodes` |
| Default/Config | `config.Binding` 把 defaults 与同一 strict validator 绑定；Source 必须非空唯一，值域深拷贝，merge shape、YAML/JSON duplicate、unknown/type 均失败 | `TestLoaderMergesSourcesAndRedactsSecrets`、`TestFileSourceRejectsDuplicateJSONAndYAMLKeys`、`TestSnapshotDecodeRejectsUnknownAndCrossTypeValues`、`TestExampleConfigSatisfiesCurrentServiceBindings` |
| 单一候选 | Coordinator 是 Loader 唯一进程调用者；Prepare 的候选同时供应 Kernel 与 HTTP；application section 先做 RestartRequired preflight | `TestReloadRequiresRestartForApplicationOwnedSection`、`TestReloadRestartRequiredHasNoPartialSideEffects`、`TestHostWatchReconcilesChangeMadeBeforeWatcherReady` |
| Supervisor | Participant/runner 非空唯一；runner ready ack 后才进程 ready；error 或意外 nil 触发取消；反向 Stop 与 Wait 共用总期限 | `TestSupervisorTreatsEarlyNilCompletionAsFailure`、`TestSupervisorBoundsUncooperativeRunnerAndReportsOwner`、`TestSupervisorWaitsForRunnerReadyAcknowledgement` |
| HTTP | Server Start 预绑定，Run 阻塞 Serve，Stop 执行 Shutdown/Close/Wait；默认进程测试请求未注册路径得到 404 | `TestServerStartReportsBindFailureSynchronously`、`TestServerStopWaitsForActiveRequest`、`TestProcessServiceModeStartsDefaultCapabilitiesWithoutExternalServices` |
| State/Reload | Coordinator 与 Supervisor 分别提供配置代际和进程监督快照；Host 汇总 Ready/Health；terminal drain 不 Resume，committed cleanup 进入 degraded 并阻断 reload | `TestHostReadinessAndHealthFollowSupervisedLifecycle`、`TestTerminalDrainTimeoutDoesNotResumeOrForceCloseActiveLease`、`TestReloadCleanupErrorKeepsCommittedCandidate` |
| Governance | 解析 `go list -json ./...` 的生产 import graph；合法/违规 fixture 证明规则可执行，不以目录 grep 代替 import 事实 | `TestProductionPackageGraphRespectsCompositionBoundaries`、`TestPackageGraphRulesAcceptLegalFixtureAndRejectViolations` |

## 4. 验证记录

2026-08-14 在 Windows/PowerShell、本仓库当前工作树执行：

- `go test ./...`：通过；包含默认 Service listener/404/取消停止、当前 `config.example.yaml` strict binding 和 package graph 门禁。
- `go test -race ./...`：通过；包含 Supervisor、Coordinator、HTTP Stop 与 diagnostics 并发路径。
- `go vet ./...`：通过。
- `go build ./cmd/app`：通过；生成的本地 `app.exe` 已在确认其精确路径和文件类型后清理，未纳入任务范围。
- `go test ./cmd/app -run TestProcessServiceModeStartsDefaultCapabilitiesWithoutExternalServices -count=1`：通过；覆盖默认 Service listener、404 与取消停止。

- 21 份 research metadata 的 YAML、唯一 ID、report 与 supersedes 双向关系：通过。
- 012 Markdown 的 94 个非代码本地链接：通过。
- 旧 `WithCLI`、`CLIContracts`、`Options.CLI`、`Capabilities.CLI/Configuration`、公开 `Kernel.Start/Reload` 和旧 `NewHost(runtime, ...)` 搜索：生产代码零残留。
- `git diff --check`：通过；完整 Diff 与 Git 范围在提交前再次审阅。

## 5. 明确边界

- 没有增加业务 Handler、Service、Repository、Model、Route contribution、业务 Command 或 Module SDK。
- 没有创建管理 HTTP 端点；`Host.Health` 是 application seam 与测试入口。
- 没有承诺 hijacked/WebSocket connection handoff、跨平台无条件 crash durability、自动 degraded 修复或远程依赖产品验收。
- HTTP 与 Cache 配置当前都是 `RestartRequired`；需要 listener handoff 或动态切换时必须由真实需求触发新方案。

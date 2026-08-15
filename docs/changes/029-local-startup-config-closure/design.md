# 设计：本地启动与配置闭环

## 1. 总体方案

```text
applicationOwnedConfigurationBindings()
  -> Bootstrap config init
  -> Service GenerationCoordinator
  -> Migration CLI validation
  -> Todo CLI Coordinator

generated config
  -> db migrate up
  -> Service ready
  -> graceful stop
```

本任务不设计第二套配置投影或宽松解析器。所有模式继续共享一个正式配置文件，只是 one-shot command 不构造无关运行资源。

## 2. 配置绑定单轨

在 `internal/composition` 建立未导出的 application-owned binding 构造函数，按稳定顺序返回：

```text
auth -> migration -> todo -> management -> observability
```

调用方不得再手写这组 slice：

- `Application.Run` 把该集合交给 `ComposeBootstrap`；
- `newServiceRuntime` 把该集合交给 `kernelcomposition.ConfigurationBindings`；
- `executeMigration` 使用完整集合校验正式配置，只构造 Database/Migration 资源；
- `prepareTodo` 在 Kernel 已登记的底层 section 之外注册同一 application-owned 集合。

HTTP 继续由底层 `ConfigurationBindings` 或 Kernel composition 拥有，不能在 helper 中重复。函数返回新 slice，调用方不能修改共享全局状态。

## 3. 严格语义

`ValidateCandidate` 继续校验完整 known roots 与每个 owner：

- generated `management`/`observability` 必须合法并被 Migration 识别；
- 任意真正未知 root 继续失败；
- tracing 默认 `enabled=false`，空 endpoint 合法；
- tracing 启用后沿用 HTTPS 或 insecure HTTP loopback 规则。

本任务不增加“Migration 忽略其他 section”的例外。若未来需要独立 deployment config projection，必须先研究文件边界、来源优先级和漂移风险。

## 4. 进程闭环测试

在 `cmd/app/main_test.go` 新增单一用户旅程测试：

1. `config init --output <temp>/config.yaml`；
2. 用隔离 env prefix 覆盖 SQLite、Storage、business/management loopback 临时地址；
3. 执行 `db migrate status`，确认命令不因合法 section 失败；
4. 执行 `db migrate up` 并再次 `status`；
5. 无参数启动 Service，等待 `/readyz` 或等价 readiness；
6. 访问一个业务端点证明业务 listener 已提交；
7. 取消，等待干净退出，验证两个地址可重新 bind、SQLite 可 rename。

补充窄测试：

- application-owned binding ID/path 顺序唯一；
- Migration 接受 Management/Observability，拒绝 `unknownRoot`；
- generator 默认 tracing disabled；
- command error 保留具体 owner/stage。

测试不得使用仓库根 `.data`、固定 `8080/9090` 或用户 `config.yaml`。

## 5. 文档信息架构

### 5.1 根 README

根 README 调整为：

1. 项目是什么；
2. 五分钟本地启动；
3. 成功判据与停止；
4. “我要做什么”文档地图；
5. 一段当前架构摘要。

详细配置字段、模块内部、reload、release 和历史任务移出根 README，改为链接。

### 5.2 当前 authority

- `docs/getting-started/local-development.md`：唯一当前本地启动流程。
- `docs/configuration/README.md`：唯一当前配置来源、优先级、section 和校验说明。
- `docs/operations/migration-and-rollback.md`：生产 migration/rollback authority，不复制本地 onboarding。
- `internal/**/README.md`、`pkg/**/README.md`：实现与扩展说明。
- `docs/changes/**`：历史研究、计划与证据。

### 5.3 配置示例

`config.example.yaml` 顶部改为“字段参考”：

- 首次本地启动优先运行 `config init`；
- 如人工复制示例，仍必须执行 migration；
- 不再写“复制后直接启动”。

## 6. 唯一启动流程

```powershell
go run ./cmd/app config init
go run ./cmd/app db migrate up
go run ./cmd/app
```

成功判据包括：

- 日志出现 generation started/application ready；
- business listener 默认 `127.0.0.1:8080`；
- management `/readyz` 默认 `127.0.0.1:9090`；
- `Ctrl+C` 后出现 draining/stopped 且进程退出。

`db migrate status` 放在“检查/排障”而不是首次启动强制前置。已有 `config.yaml` 时不建议直接 `--force`；先编辑当前文件，或用 `--output` 生成到临时路径进行比较。

## 7. 排障模型

| 错误片段 | owner | 处理 |
| --- | --- | --- |
| `open config.yaml` | 配置来源 | 运行 `config init` 或确认 cwd/`--output` |
| `default configuration target exists` | 生成器 | 不盲目 force；编辑现有文件或生成到其他路径比较 |
| `unknown config section` | binding contract | generated 合法 section 命中即产品回归；自定义 root 则删除或实现 owner |
| migration version/dirty/completion | Migration | 按运维 authority 前滚或补 owner，不由 Service 自动修复 |
| trace endpoint | Observability | 默认关闭；启用时提供合法 HTTPS，或仅 loopback 使用 insecure HTTP |
| address already in use | Listener | 释放占用或通过 env 覆盖地址 |

## 8. 文件影响

预计修改：

- `internal/composition` 的 binding helper、Bootstrap、Service、Migration、Todo 路径与测试；
- `cmd/app/main_test.go`；
- 根 `README.md`、`docs/README.md`；
- 新增 `docs/getting-started/local-development.md`、`docs/configuration/README.md`；
- `config.example.yaml`、`docs/operations/migration-and-rollback.md`、必要模块 README；
- 029 状态与实施证据。

预计不修改公共 CLI、配置 key/default、`go.mod`/`go.sum`、migration SQL、Database schema、OpenAPI、业务实现、部署和 release 文件。

## 9. 重新确认触发器

出现以下任一情况必须更新研究/计划并重新确认：

- 需要改变配置文件格式、key、默认值、env precedence 或 unknown section 语义；
- 需要让 Migration 忽略其他正式 section 或引入独立配置投影；
- 需要新增依赖、外部服务、固定端口或真实远端资源；
- 需要修改 schema/migration SQL、公共 CLI、HTTP API 或认证语义；
- 无法用共享 binding helper 消除 producer/consumer 漂移。

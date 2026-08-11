# 实施任务：Logger Capability 注入

## 确认状态

- 用户确认：2026-08-11 已明确确认当前 004 方案并要求执行。
- 当前阶段：已完成。
- 实施门禁：已满足；本轮只能执行 LOG-001 至 VER-001。

## 使用规则

- 本清单是后续实施的唯一任务账本；需求语义以 [requirements.md](requirements.md) 为准，接口与事务语义以 [design.md](design.md) 为准。
- 只实施已确认任务 ID；目标、公共接口、依赖、边界或外部副作用发生实质变化时，恢复“待确认”并重新报告。
- 方案阶段不暂存、不提交。确认后，任务文档、实现、测试和权威文档形成一个任务 commit，不 push。

## 进度总览

- 方案文档：`1 / 1` 点。
- 实现与验证：`25 / 25` 点。
- 总进度：`26 / 26` 点。

## 任务清单

### 方案基线

- [x] **DOC-001（1 点）** 建立 004 的 README、requirements、design 和任务账本，并登记变更索引。
  - 验收：当前状态明确为待确认；目标、非目标、接口、事务和验证计划一致。

### Logger 资源契约

- [x] **LOG-001（5 点）** 拆分 `pkg/logger.Logger` 与 `Resource`，实现无 I/O 配置校验和可关闭 sink 所有权。
  - 依赖：当前方案确认。
  - 验收：Close 幂等；Sync 和全部关闭错误完整保留；stdout/stderr 不被关闭；旧 Logger/Sync 调用迁移归零。
- [x] **LOG-002（3 点）** 实现并发安全的 Kernel logging manager 和动态 With。
  - 依赖：LOG-001。
  - 验收：baseline 必填；替换、恢复、字段继承和并发 race 测试通过。

### Kernel 与 Capability

- [x] **KRN-001（4 点）** 增加 ActivationHooks 和 Kernel 成功状态日志，迁移所有 Kernel Options 调用方。
  - 依赖：LOG-002。
  - 验收：激活只发生在提交区；失败候选不可见；停止前 Deactivate；返回错误不重复记录。
- [x] **CAP-001（4 点）** 实现 Logger Definition、typed Config、Defaults、私有 Resource Instance 和窄 Access。
  - 依赖：LOG-001、LOG-002、KRN-001。
  - 验收：ID/路径固定；默认值来自 pkg；业务回调不可访问 Close；Build/Stop/Activate 时序有测试。
- [x] **CMP-001（2 点）** 按 Logger、Database 顺序组合并扩展 Capabilities 和默认配置聚合。
  - 依赖：CAP-001。
  - 验收：成功返回两个稳定 Access；CLI 配置包含两个有序段；失败不返回部分能力。

### 应用与文档

- [x] **APP-001（3 点）** 让应用入口拥有 baseline Resource，并增加使用 Logger Access 的生命周期 Participant。
  - 依赖：CMP-001。
  - 验收：配置化 logger 记录启动/停止；stderr 和退出码语义不变；baseline 明确关闭。
- [x] **DOC-002（1 点）** 同步根说明、Kernel/Capability 和 logger 权威文档。
  - 依赖：APP-001。
  - 验收：文档只描述真实实现；基线、接管、资源所有权、配置和业务注入示例一致。

### 验证与提交

- [x] **VER-001（2 点）** 执行完整验证、审阅 Diff 并创建单一任务 commit。
  - 依赖：LOG-001 至 DOC-002 全部完成。
  - 必须执行：`gofmt`、`go mod tidy` 差异检查、`go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...`、实际 `config init`、架构残留搜索和 `git diff --check`。
  - 验收：只暂存 004 任务文件；提交信息符合仓库惯例；不 push；如实记录未执行项和风险。

## 逐轮执行记录

| 轮次 | 日期 | 完成任务 | 本轮点数 | 累计点数 | 验证 | Commit | 剩余风险 |
| --- | --- | --- | ---: | ---: | --- | --- | --- |
| 1 | 2026-08-11 | DOC-001 | 1 | 1 / 26 | 004 四件套集合、相对链接和状态检查通过；`git diff --check` 通过；仅 Markdown 变更 | 未暂存、未提交 | LOG-001 至 VER-001 尚未实施；等待用户确认当前 004 方案。 |
| 2 | 2026-08-11 | LOG-001、LOG-002、KRN-001、CAP-001、CMP-001、APP-001、DOC-002、VER-001 | 25 | 26 / 26 | `gofmt`、`go mod tidy` 无依赖差异、`go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...`、真实 `config init`、48 个 Markdown 相对链接、架构残留搜索、`git diff --check` 全部通过 | `feat: 注入 Kernel Logger Capability` | 无已知剩余风险；未连接外部 Database，按任务范围未执行长期服务探测；未 push。 |

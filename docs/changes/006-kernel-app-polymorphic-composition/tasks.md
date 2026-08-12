# 实施任务：Kernel App 多态装配基础

## 确认状态

- 用户确认：尚未确认当前 006 方案。
- 当前阶段：方案待确认。
- 实施门禁：未满足；只能维护本任务文档和必要导航。
- 当前允许完成：`DOC-001`。
- 用户后续明确确认本方案后，才允许执行 `APP-001` 至 `VER-001`。

## 使用规则

- 本清单是 006 后续实施的唯一任务账本；需求语义以 [requirements.md](requirements.md) 为准，接口和状态机语义以 [design.md](design.md) 为准。
- 只实施已确认任务 ID。目标、公共 API、依赖、模块边界、重载事务或迁移范围发生实质变化时，恢复“待确认”并重新报告。
- 方案阶段不修改源码、测试、配置、依赖或生成物；用户于 2026-08-12 明确要求把当前纯文档变更独立提交并推送，该 Git 授权不构成实施确认。
- 确认后，实施、测试和随真实代码变化产生的文档更新作为一个完整任务变更审阅与提交；未经用户要求不 push。
- 每轮实施结束追加执行记录；只有实现、测试、文档和对应验收证据齐全时才勾选任务。

## 进度总览

- 方案文档：`1 / 1` 点。
- 实现、迁移与验证：`0 / 43` 点。
- 总进度：`1 / 44` 点。

## 任务清单

### 方案基线

- [x] **DOC-001（1 点）** 建立 006 的 README、requirements、design、tasks，并登记变更索引。
  - 验收：状态为待确认；当前事实、目标 API、范围、迁移、失败语义和验证计划一致；只修改 Markdown。

### App 核心契约与 Plan

- [ ] **APP-000（1 点）** 用独立最小编译探针验证目标泛型 API 的 Go 1.25 可表达性。
  - 依赖：用户确认 006。
  - 验收：typed Add/Input、异构 Plan、Fixed/Leased 均无需调用方 `any`/反射/类型断言；失败则停止实施、更新方案并重新确认，不提交占位生产 API。
- [ ] **APP-001（4 点）** 实现 `internal/kernel/app` 的 ID、Definition、Fixed/Managed 构造源和可选小契约。
  - 依赖：APP-000。
  - 验收：Fixed 不伪造配置；Configured 严格 Decode/Validate；Defaults、CLI、Starter、Ready、Stopper、Activation 均独立可选；非法策略组合在 Add 前失败。
- [ ] **APP-002（5 点）** 实现 `Added/Binding/Input`、有序 Plan、Freeze 和启动期 typed Input 解码。
  - 依赖：APP-001。
  - 验收：无 Get/Resolve；重复 ID、零值/跨 Plan/前向 Input、Freeze 后 Add 均失败；循环无法表达；失败不改变 Plan。
- [ ] **APP-003（4 点）** 实现 Fixed Direct 与 Managed Leased exposure。
  - 依赖：APP-001、APP-002。
  - 验收：Direct 输出普通接口；Leased 输出稳定 Access；内部实例、Kernel Handle 和 Close 权不泄漏；typed/race 测试通过。

### Kernel 安装、生命周期与重载

- [ ] **KRN-001（5 点）** 实现 Frozen Plan 原子 Install 和按顺序初始启动/反向失败清理。
  - 依赖：APP-002、APP-003。
  - 验收：Install 只接受 created/empty Kernel；失败无部分状态；依赖方在前置 Output 发布后构造；可选生命周期无空 Hook。
- [ ] **KRN-002（5 点）** 把 Reload 单轨迁移为 Plan 节点编排，并实现 `NoReload`、候选先准备、反向 drain、提交和回滚。
  - 依赖：KRN-001。
  - 验收：旧实例在候选准备期间可用；排空/取消/超时/清理错误完整；成功后立即反向清理旧代；并发 race 通过。
- [ ] **KRN-003（3 点）** 实现 `RestartRequired` 预检、typed 结果和无副作用语义。
  - 依赖：KRN-002。
  - 验收：同轮含 RestartRequired 时不应用任何组件；当前实例、摘要和入口不变；错误可识别并含全部 Component ID。

### 组件单轨迁移

- [ ] **CMP-001（4 点）** 把 Logger、Database 从 `internal/kernel/capability` 迁移到 `internal/kernel/app`。
  - 依赖：APP-003、KRN-001、KRN-002。
  - 验收：Logger baseline/Activation/Resource Close 不退化；Database Ping 使用 Ready，Access 移除 Close 权，Kernel 私有实例继续负责释放；两个组件使用 Leased + KernelInstanceSwap；配置和 Defaults 语义不变。
- [ ] **CMP-002（3 点）** 为 Clock、ID Generator、Validator 建立 Fixed Direct app Definition 和契约测试。
  - 依赖：APP-003。
  - 验收：输出分别为现有项目接口；无 Access.Use、ConfigPath、Defaults、CLI 或生命周期；实现替换契约测试通过。
- [ ] **CMP-003（4 点）** 重构 composition 为本地 Plan、Freeze、Defaults/CLI 校验和最后原子 Install。
  - 依赖：CMP-001、CMP-002、KRN-003。
  - 验收：固定清单为 Logger、Clock、ID Generator、Validator、Database；Capabilities 完整；失败返回零值且 Kernel 不变；默认文档仍只有 Logger、Database 且顺序不变。

### 单轨清理与文档

- [ ] **CLN-001（2 点）** 删除旧 capability 目录、Definition/Registration/Handle/Access/InstanceHooks API 和全部旧引用。
  - 依赖：CMP-003。
  - 验收：旧路径、类型、import、测试、说明和逐项 Register 搜索归零；不保留 alias、legacy 或兼容分支。
- [ ] **DOC-002（1 点）** 同步根 README、docs 入口、Kernel/App 与相关 pkg 权威说明，并更新 002 研究状态边界。
  - 依赖：CLN-001。
  - 验收：当前实现与后续 Native/Handoff/观察期目标明确分开；示例与真实 API 一致；不设计业务层。

### 验证与提交

- [ ] **VER-001（2 点）** 执行完整验证、审阅最终 Diff 并创建单一 006 实施 commit。
  - 依赖：APP-001 至 DOC-002 全部完成。
  - 必须执行：`gofmt`、`go mod tidy` 差异检查、`go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...`、实际 `config init`、Markdown 相对链接、架构残留搜索和 `git diff --check`。
  - 验收：只暂存本任务实现、测试和随真实代码变化产生的文档更新；提交信息符合仓库惯例；不 push；未连接真实 Database 的边界如实记录。

## 逐轮执行记录

| 轮次 | 日期 | 完成任务 | 本轮点数 | 累计点数 | 验证 | Commit | 剩余风险 |
| --- | --- | --- | ---: | ---: | --- | --- | --- |
| 1 | 2026-08-12 | DOC-001 | 1 | 1 / 44 | 006 四件套、变更索引、UTF-8、Markdown 相对链接、范围检查和 `git diff --check` | 按用户当前指令作为纯文档变更提交并推送 | APP-000 至 VER-001 尚未实施；Git 交付授权不构成实施确认，仍等待用户明确确认当前 006 方案。 |

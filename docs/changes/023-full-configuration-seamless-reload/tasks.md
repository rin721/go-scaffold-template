# 任务：全配置无感重载

## 1. 状态与授权

- 研究门禁：已通过。
- 计划状态：**已取代**；024 是当前唯一施工 authority。
- 实施授权：023 的既有授权已被用户后续确认的 024 `ONE-001..025` 单轨范围吸收，不再单独执行 `RLD-*`。
- 基线：`e251b73518a457ec97c529d067ddfffe77be203a`。
- 当前允许动作：仅作为 024 的历史证据与需求映射读取。
- 当前禁止动作：以 023 名义继续并行施工，以及任何未获授权的外部发布。

若实施证据触发第 6 节条件，必须停止实施、恢复“待确认”并重新报告。

## 2. 任务依赖

```text
RLD-001 ─┐
         ├─> RLD-003 -> RLD-004 -> RLD-006 -> RLD-007 ─┐
RLD-002 ─┘                         ├-> RLD-005          ├-> RLD-011 -> RLD-012 -> RLD-013 -> RLD-014 -> RLD-015
                                   ├-> RLD-008          │
                                   ├-> RLD-009          │
                                   └-> RLD-010 ─────────┘
```

`RLD-002` 是首个可证伪门禁。原型不能证明语义时，不继续大规模迁移，退回研究并重新确认。

## 3. 实施任务

### RLD-001 稳定配置读取

- 工作量：M
- 依据：R001、`CFG-001`
- 内容：实现受 context/deadline 约束的稳定双采样；分类 Windows sharing violation、atomic rename 暂时不存在与永久错误；保持 File < Env 和 strict decode。
- 完成条件：原地写、rename、锁文件、连续保存、非法 YAML、权限/类型错误测试通过；旧 generation 在失败时不变。

### RLD-002 ListenerHub 可证伪原型与契约

- 工作量：L
- 依据：R002、ADR-002、`HTTP-001/002`
- 内容：先在 `pkg/httpx` 证明 physical listener、虚拟 route、Serve-ready、pending dispatch barrier、背压、Stop 唤醒、地址迁移和错误传播。
- 完成条件：Windows/Linux 的 same-address 持续请求和地址变化原型通过；无连接静默丢弃、无 owner 外 goroutine、无 data race。失败则记录证据并退回研究。

### RLD-003 Generation 状态机与协议

- 工作量：L
- 依赖：RLD-001、RLD-002
- 内容：建立 `GenerationFactory`、prepared/current/retiring、candidate Abort、不可失败 Commit、cleanup debt、latest-wins 与 shutdown 状态机。
- 完成条件：并发表驱动测试覆盖所有合法/非法转换；任一候选失败保持 current；cleanup debt 阻断后续 reload。

### RLD-004 Typed Resource Slot 与终结 journal

- 工作量：L
- 依赖：RLD-003
- 内容：为 Logger、Database、Cache、I18n、Storage 建立显式 typed key、digest、Ready、引用计数和反向 Close；不引入万能 registry。
- 完成条件：未变资源复用，变化资源独立；Abort/retire/shutdown 的所有 owner 与错误均保留；race 测试通过。

### RLD-005 Logger 基线与 generation target

- 工作量：M
- 依赖：RLD-004
- 内容：保留不可丢失的 baseline diagnostics；configured business logger 在 commit 切换，旧请求仍可写旧 sink，引用归零后关闭。
- 完成条件：候选 logger 失败仍可诊断；level/output 改变只影响新代；无 sink 提前关闭或日志错误递归。

### RLD-006 完整 Composition Factory

- 工作量：L
- 依赖：RLD-004
- 内容：从单一 Snapshot 构造 capabilities、Todo Repository/Policy/Service/Handler/Router、HTTP server/route 和 journal；完成 owner Ready。
- 完成条件：构造顺序和依赖显式；业务代码不接触 Kernel/registry/Close；candidate 不在 Ready 前发布。

### RLD-007 HTTP generation handoff

- 工作量：L
- 依赖：RLD-006
- 内容：把 Service HTTP 生命周期迁到 ListenerHub；实现同地址 route commit、地址变更先 bind、旧 `http.Server.Shutdown` 排空和进程 Stop。
- 完成条件：全部当前 `ServerConfig` 字段在新连接代生效；持续请求、长请求、bind failure、shutdown 与诊断测试通过。

### RLD-008 Todo 对象图代际化

- 工作量：M
- 依赖：RLD-006
- 内容：每代重建 immutable Policy、Service、Handler 和 Router；Repository 固定该代 Database；移除只 validate 不生效的旧 binding。
- 完成条件：旧在途请求使用旧 Policy/DB，新请求使用新 Policy/DB；不存在共享可变 Todo config。

### RLD-009 Cache 客户端代际隔离

- 工作量：L
- 依赖：RLD-006
- 内容：typed Client、L1、tag index 和 cleanup goroutine 归 generation 所有；底层 remote resource 可按 digest 复用。
- 完成条件：backend/namespace 变化不命中旧 L1；旧请求排空前可完成；goroutine 有 Stop/Wait owner。

### RLD-010 Database、I18n、Storage 集成

- 工作量：M
- 依赖：RLD-006
- 内容：把现有 component replacement 迁入完整 generation；Database 增加只读 schema readiness，明确外部数据迁移边界。
- 完成条件：新资源 Ready 前不提交；缺 schema/目标不可达拒绝候选；旧请求固定旧资源。

### RLD-011 Watcher 与完整 reload 事务接线

- 工作量：L
- 依赖：RLD-007、RLD-008、RLD-009、RLD-010
- 内容：watcher 只触发 GenerationCoordinator；整合 no-op、latest-wins、deadline、Abort、Commit、retire 和 shutdown。
- 完成条件：组合修改只提交一代；拒绝后 watcher 可恢复；无半提交、无无界队列或 retired generation。

### RLD-012 诊断与可观测性

- 工作量：M
- 依赖：RLD-011
- 内容：统一 attempt/generation/digest/changed sections/phase/routes/active work/reuse/cleanup debt；脱敏错误保留 owner 与 error chain。
- 完成条件：日志不再只显示笼统 `*errors.errorString`；diagnostics 可区分 rejected、active、retiring、degraded。

### RLD-013 单轨删除旧 reload

- 工作量：L
- 依赖：RLD-012
- 内容：迁移全部调用方后删除长期 Service 的 section-level Coordinator、`RestartRequired` 策略、旧 application binding、Cache 跨代入口和失效测试/日志/文档。
- 完成条件：搜索证明无旧入口或永久兼容分支；CLI invocation 模式保持；当前权威文档只描述新路径。

### RLD-014 完整验证与运行验收

- 工作量：XL
- 依赖：RLD-013
- 内容：执行七节单改/组合改、失败注入、并发、长请求、地址变化、cleanup debt、Windows/Linux 真实进程验收以及 test/race/vet/build。
- 完成条件：`requirements.md` 11 项验收逐项记录命令、环境和证据；未执行项明确标记，不以单元测试代替进程验收。

### RLD-015 文档、Diff 与提交闭环

- 工作量：M
- 依赖：RLD-014
- 内容：同步 README、架构、配置、组件开发、运行与故障说明；更新 023 状态、逐轮证据和 Commit；审阅完整 Diff，只暂存本任务文件并创建 Conventional Commit。
- 完成条件：相对链接、`git diff --check` 和旧语义残留检查通过；提交不包含用户无关修改；不自动 push。

## 4. 计划验证命令

实际包路径在 RLD-002/RLD-003 后复核；目标门禁包括：

```powershell
go test ./internal/kernel/... ./internal/composition ./pkg/httpx ./pkg/cache/... ./internal/module/todo/... -count=1
go test ./... -count=1
go test -race ./internal/kernel/... ./pkg/httpx ./internal/composition ./internal/module/todo/... -count=1
go vet ./...
go build ./cmd/app
git diff --check
```

还必须执行 Windows amd64 与 Linux amd64 的真实 Service runtime 脚本，持续请求并修改全部配置。若当前机器不具备 Linux/race 环境，只能报告未验证，不能声明完成。

## 5. 计划阶段现有证据

- 2026-08-15：基线 `e251b735` 后，定向测试除一个 Windows watcher 用例外通过。
- 失败用例：`TestWatchRecoversAfterRestartRequiredCandidateIsRestored` 在读取临时配置时返回 sharing violation；单独 `-count=10` 可立即复现。
- 结论：022 的 latch 恢复修复不等于文件稳定读取，RLD-001 必须先解决保存窗口的瞬时不可读。
- 计划阶段不修改 Go，不启动 Service，不暂存、不提交。

## 6. 重新确认条件

命中 `design.md` 第 13 节任一条件，或任务工作量、公共 API、依赖、模块边界、配置契约、数据迁移、外部副作用发生实质变化时：

1. 停止实施；
2. 回到研究并补充档案；
3. 更新 requirements/design/tasks；
4. 将状态恢复为“待确认”；
5. 提交新计划报告并等待用户再次确认。

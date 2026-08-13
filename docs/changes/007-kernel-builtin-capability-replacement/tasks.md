# 实施任务：Kernel 内置能力槽位与显式替换体系

## 确认状态

- 方案文档修正确认：**已完成**，仅授权 `DOC-001`。
- 非文档实施确认：**已确认**（2026-08-13）。
- 当前阶段：已完成。
- 实施门禁：已满足；按任务依赖顺序实施 `BLT-001` 至 `VER-001`。

## 使用规则

- 本清单是 007 后续实施的唯一任务账本；需求以 [requirements.md](requirements.md) 为准，接口和事务语义以 [design.md](design.md) 为准。
- 用户必须在新版 007 需求报告之后明确确认，才能实施 `FRM-001` 至 `VER-001`；此前对旧 Logger fallback 方案的讨论不构成确认。
- 只实施已确认任务 ID。若公共接口、Role 清单、策略、所有权、事务边界或首期范围发生实质变化，恢复“待确认”并重新报告。
- 方案阶段不暂存、不提交。确认后，方案、实现、测试和权威文档作为一个任务变更提交；未经用户要求不 push。

## 进度总览

- 方案修正：`1 / 1` 项完成。
- 实现、测试、权威文档与验证：`12 / 12` 项完成。
- 当前实现、测试、权威文档和交付验证已完成。

## 任务清单

### 方案基线

- [x] **DOC-001** 单轨修正 007 四件套与变更索引。
  - 验收：目录和标题统一为“Kernel 内置能力槽位与显式替换体系”；确定 `internal/kernel/builtin` 命名、Config/Logger/CLI baseline Definition、Assembly 阶段与可见性；明确三个替换场景、固定结论和确认门禁；只修改 Markdown。

### 通用框架

- [x] **BLT-001** 建立 `internal/kernel/builtin` 封闭 catalog、`BuiltinDefinition` 和 Kernel Assembly。
  - 依赖：用户确认新版 007。
  - 验收：内置组件声明统一进入 `builtin/<name>`；Assembly 按 Bootstrap、PreStart、Runtime 阶段构造和反序清理；每个 Role 有 baseline Definition；KernelOnly/AppVisible 边界不能绕过；普通 App 不能动态创建或查找 Role。

- [x] **BLT-002** 组件化 Config、Logger、CLI 三项 baseline。
  - 依赖：BLT-001。
  - 验收：Config/Logger 为 RequiredActivation Bootstrap，CLI 为 SelectedActivation PreStart；Config/CLI 为 KernelOnly、Logger 为 AppVisible；生产 baseline 由 Assembly 拥有；Config/CLI 机制包不再承担隐式构造职责。

- [x] **FRM-001** 建立 Kernel 封闭 `BuiltinRole` 和 root Binding。
  - 依赖：BLT-002。
  - 验收：Role 包含 typed target、阶段、激活方式、可见性、策略和所有权；AppVisible output 单独返回 typed Binding；生产 Plan 由 Assembly 建立；跨阶段倒序依赖失败。

- [x] **FRM-002** 增加 `Spec`、`ReplacementDefinition` 和显式 `app.Replace`。
  - 依赖：FRM-001。
  - 验收：Add 与 Replace 类型和返回值分离；replacement 不发布独立 Binding；不使用 marker、反射、字符串 qualifier 或弱类型容器。

- [x] **FRM-003** 完成 Plan/Freeze 治理校验。
  - 依赖：FRM-002。
  - 验收：未知/跨 Plan Role、多 replacer、策略冲突、声明顺序、重复 ID、ConfigPath 相等或父子重叠均失败且无部分写入。

- [x] **FRM-004** 将 replacement 接入 Kernel 生命周期事务。
  - 依赖：FRM-003。
  - 验收：初始发布、候选准备、slot 排空、提交、恢复、回滚、旧代关闭和 baseline 所有权满足设计；取消和错误链完整。

### Logger 纵向切片

- [x] **LOG-001** 将 Logger 注册为首个 `RuntimeTransaction + AssemblyOwnedBaseline` Role，并建立 `pkg/logger.Access`。
  - 依赖：FRM-004。
  - 验收：Kernel 和普通消费者只依赖 root Access；baseline 从早期启动到最终清理可用；不暴露 Manager、Resource 或控制面。

- [x] **LOG-002** 将 Logger App 单轨迁移为 `Replacement(spec)` 与 `Instance(spec)`。
  - 依赖：LOG-001。
  - 验收：两入口共用构建实现但拥有独立生命周期；删除 Manager 注入、旧 Activation 替换和旧 App 私有 Access；Resource 只关闭一次。

### 多实例场景

- [x] **DB-001** 将 Database 改为 `Spec + Input[pkglogger.Access]` 并支持 db1/db2。
  - 依赖：LOG-002。
  - 验收：db1 跟随 root；db2 只使用 logging.db2 Binding；独立日志不可用时显式失败且不 fallback；Database 不导入具体 Logger App 或 Kernel 控制面。

- [x] **TST-001** 覆盖三个组合场景及事务失败路径。
  - 依赖：BLT-001 至 DB-001。
  - 验收：builtin 阶段/激活、baseline-only、main replacement、main 与 independent 共存，以及启动失败、运行期失败、取消、回滚、排空、关闭顺序和 race 均有确定性证据。

### 权威文档与交付

- [x] **DOC-002** 同步根 README、Kernel/App 权威说明和配置示例。
  - 依赖：TST-001。
  - 验收：只描述实际已实现行为；Provide/Replace/Decorate 边界、Role 清单、三个场景和所有权清晰；没有旧 Logger fallback 现行说明。

- [x] **VER-001** 完整验证、Diff 审阅并创建单一 007 commit。
  - 依赖：BLT-001 至 DOC-002 全部完成。
  - 必须执行：定向测试、格式检查、`go mod tidy` 无依赖差异、build、全量 test、race、vet、Markdown 链接、架构搜索和 `git diff --check`。
  - 验收：只暂存本任务文件；不 push；逐项记录命令、结果、Commit 和剩余风险。

## 逐轮执行记录

| 轮次 | 日期 | 完成任务 | 验证 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-13 | DOC-001：将旧 Logger fallback 规划单轨修正为通用内置能力槽位与显式替换体系 | 四件套、变更索引、相对链接、旧标题残留和 `git diff --check` | 未提交 | BLT-001 至 VER-001 尚未确认和实施；目标 API 均不是当前代码事实。 |
| 2 | 2026-08-13 | DOC-001：增加 `internal/kernel/builtin`、Config/Logger/CLI baseline Definition 与 Assembly 阶段模型 | 四件套语义、相对链接、旧所有权术语和 `git diff --check` | 未提交 | BLT-001 至 VER-001 尚未确认和实施；Config/CLI replacement 仍是非目标。 |
| 3 | 2026-08-13 | BLT-001 至 DOC-002：建立封闭 builtin catalog 与 Assembly；组件化 Config/Logger/CLI；实现 Role/root Binding、Spec、Replace 与事务；迁移 Logger/Database 多实例；同步权威文档 | 定向测试与全量 `go test ./...` 通过；三个装配场景、换代与治理错误已有测试 | 待提交 | 正在执行 race、vet、build、tidy、链接、架构搜索和完整 Diff 审阅；Config/CLI replacement 仍是非目标。 |
| 4 | 2026-08-13 | VER-001：完成格式、依赖、构建、测试、race、vet、链接、架构残留和 Diff 检查 | `gofmt -w .`；`go mod tidy` 无依赖差异；`go build ./...`、`go test ./...`、`go test -race ./...`、`go vet ./...`、Markdown 链接、架构搜索、`git diff --check` 全部通过 | 本提交 | 未 push；Config/CLI replacement 与 Decorate 仍是明确非目标。 |

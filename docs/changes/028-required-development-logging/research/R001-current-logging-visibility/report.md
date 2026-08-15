# R001 当前日志能力、启动可见性与开发门禁复核

## 1. 研究问题

本报告回答四个问题：项目是否缺少日志能力；启动时为什么只看到少量输出；“开发环境输出 debug 到 error”应怎样定义；怎样把日志使用变成可执行的开发要求，而不是一句无法验收的口号。

## 2. 方法与范围

研究快照为 `a700d25248b483bb5cb844ee235d4e9bbb6b37d4`，初始工作树 clean。检查范围包括：

1. `pkg/logger` 的项目契约、zap 封装、默认配置、级别过滤、字段与资源所有权；
2. `cmd/app -> internal/composition -> GenerationCoordinator/Kernel -> Supervisor` 的启动、重载和停止链；
3. HTTP access/recovery、Auth security audit 与 reload reporter 等真实消费者；
4. `config.example.yaml`、根文档、开发指南、Logger/Kernel 文档和既有 004/007 记录；
5. 生产 `.go` 文件中的 `Debug/Info/Warn/Error` 调用点及相应测试。

按照计划阶段门禁，本轮没有启动 Service、写数据库、绑定端口或修改实现，因此没有把用户观察冒充为本轮运行复现。静态调用链、默认配置和测试证据已经足以解释现象并形成计划；真实进程输出列入确认后的验收。

## 3. 当前事实

### 3.1 Logger 能力已经存在且边界正确

`pkg/logger.Logger` 暴露 `Debug`、`Info`、`Warn`、`Error` 和 `With`，具体 zap 类型没有泄漏。`Resource` 单独拥有 `Sync/Close` 和文件 sink。`internal/kernel/logging.Manager` 强制持有 baseline，并向消费者提供不含替换权和关闭权的稳定 facade；配置化 Logger 只在 generation 提交后接管，候选失败或停止后恢复 baseline。

因此问题不是“没有 Logger”或“没有注入”，也不需要再造第二套日志框架、全局 logger 或新 Adapter。

### 3.2 开发默认会过滤 Debug

`pkg/logger.DefaultEnvironment` 是 `development`，但 `DefaultLevel` 是 `info`。`pkg/logger.New(nil)` 使用这组默认值；`internal/kernel/app/logger` 的 Defaults 也来自同一个 `DefaultConfig()`。`config.example.yaml` 明确写着 `environment: development` 与 `level: info`。

`LevelDebug` 的真实语义是输出 debug 及以上；`LevelInfo` 会过滤 debug。因此即使代码调用 `Debug`，默认开发启动也不会显示它。

### 3.3 生产日志事件本身很少

当前生产调用点呈现以下分布：

- `Debug`：只有 Kernel 的 `kernel reload unchanged`；当前完整 Application Generation 的 no-op reload 没有对应日志。
- `Info`：Kernel start/reload/stop、Application Generation start/reload、application start/stop、HTTP request completed、security decision。
- `Warn`：没有真实业务调用点；只有 Logger/Manager 方法定义。
- `Error`：reload rejected/cleanup debt、HTTP panic/request failure。

长期 Service 初次启动虽然会在 generation 提交后输出 `application generation started` 和 `application started`，但配置加载、配置校验、候选编号、资源 build/reuse、migration compatibility、两个 listener 的 bind/serve-ready、Supervisor ready 等阶段没有连续日志。`startImmutableComponent` 还会为多个底层资源创建小 Kernel，其成功日志只有 `components` 数量，缺少 owner/component identity，信息密度低。

### 3.4 启动失败主要依赖最终 stderr

`cmd/app.execute` 在 `process.run` 返回错误后向 stderr 写一次完整错误。Kernel 与 GenerationCoordinator 多数失败路径保留错误链并向上返回，不逐层打印，这是正确的“单一错误边界”基础；但当前没有一个结构化 Service 失败事件记录阶段、owner、generation 和错误类型。

不能简单让每层同时 `Error(err)`：那会重复日志并可能把 DSN、路径、Token 或第三方错误文本泄漏。目标应是在 application operation 边界记录一次低敏结构化分类，同时保留最终 stderr 的单行人类可读错误。

### 3.5 文档没有形成开发门禁

`pkg/logger/README.md` 说明 API、配置和注入方式；`AGENTS.md` 规定错误日志不能重复、不能泄密；应用模块开发指南要求评估失败与诊断。但当前没有文档统一规定：

- 哪些生命周期、外部边界和状态变化必须记录；
- Debug/Info/Warn/Error 的选择标准；
- 谁是唯一错误日志 owner；
- 必填结构化字段、关联字段与禁止字段；
- 新模块怎样在研究、设计、任务和验收中证明日志覆盖；
- `Noop` 只能用于测试，不得出现在 production composition。

所以“有 Logger API”没有转化为“开发必须使用日志”的可审查规则。

## 4. 判断与方案比较

### 4.1 采用：保留现有能力，补默认、事件和治理

现有 zap 封装、baseline、replacement、资源所有权和稳定 facade 已满足基础要求。最小正确路线是：

1. development 默认阈值改为 `debug`，production 未显式 level 时保持 `info`；
2. 在现有 application/Kernel owner 边界补结构化启动阶段、ready、reload、stop 和失败分类；
3. 将可恢复的 reload rejection 定为 `Warn`，cleanup debt、启动失败和服务不可用定为 `Error`；
4. 为 HTTP access error 按结果分类，避免把所有预期 4xx 当成 Error；
5. 建立唯一开发日志规范、模块评估项和自动化回归门禁。

### 4.2 拒绝：成功启动固定打印四个级别

“debug 到 error 都应该打印”应解释为开发阈值允许四级事件输出，而不是每次启动各造一条。成功启动没有可恢复异常或失败，打印 `Warn/Error` 会污染告警、误导 operator，并使等级失去语义。

确认后的验收会分别触发健康启动、可恢复候选拒绝和启动失败场景，证明四级在适当事件上可见。

### 4.3 拒绝：每层记录完整错误

逐层记录会造成重复、放大日志量并泄露底层错误文本。继续使用错误链向上返回；只在决定运行策略的边界记录一次低敏字段，完整错误仍由最终 stderr 或受控诊断呈现。

### 4.4 暂缓：异步日志、轮转和远程采集

当前问题是本地可见性与开发治理，不是 sink 能力。异步队列、日志轮转、OTLP Logs、集中检索、采样或审计存储需要独立场景、资源 owner 和外部选型，本任务不引入。

## 5. 目标级别语义

| 级别 | 目标语义 | 当前任务示例 |
| --- | --- | --- |
| `Debug` | 开发诊断细节，不表示异常 | invocation mode、startup phase、generation candidate、resource build/reuse、no-op reload |
| `Info` | operator 关心的成功里程碑 | generation committed、business/management listener ready、application ready、graceful stop |
| `Warn` | 操作未达预期但旧能力仍可服务或请求可恢复 | candidate reload rejected while previous generation remains active、限流/过载等可恢复拒绝 |
| `Error` | 当前操作失败、服务不可用、panic 或 cleanup debt | initial startup failed、runtime owner failed、HTTP 5xx/panic、committed cleanup debt |

日志级别是最低输出阈值，不是“只输出这一种级别”。development `debug` 会允许四级；production `info` 默认过滤 Debug，但保留 Info/Warn/Error，并允许 operator 显式调整。

## 6. 事件与字段边界

启动事件由真实 owner 记录，不另建事件总线：

- application entry/composition：`application`、`mode`、`phase`，不记录原始 CLI args；
- GenerationCoordinator：`attempt`、`generation`、`phase`、`changed_sections`、build/reuse owner；
- listener/ready：业务与 management 的已绑定地址、ready 状态；
- failure boundary：`error_type`、`cause_type`、`phase`、`owner`、`generation`，不记录完整错误文本或配置快照；
- HTTP：`method`、`operation`、`request_id`、`trace_id`、`duration`、结果分类，不记录原始 URL、Authorization、body 或 subject。

稳定字段由语义 owner 就近定义；不建立无归属的全局 constants 包，也不把第三方 zap 字段暴露给上层。

## 7. 文档与门禁判断

应新增 `docs/development/logging.md` 作为开发日志唯一当前 authority，覆盖必记事件、级别、字段、错误 owner、敏感信息、注入、测试和反例。其他文档只链接或补所在流程的短门禁：

- `docs/README.md` 与根 `README.md` 增加入口；
- `docs/development/application-module-development.md` 的能力评估增加日志计划与验收；
- `pkg/logger/README.md` 继续只做 API/资源使用说明并链接开发规范；
- `AGENTS.md` 增加“新增运行 owner/外部边界必须有日志设计与验证”的稳定红线；
- architecture test 禁止 production 使用 `logger.Noop()` 或绕过项目 Logger 直接创建 zap/global logger。

“每个函数必须打日志”不可执行也会制造噪音，不能成为门禁。纯 Model、值对象和无副作用算法可以不记录；生命周期 owner、外部 I/O 边界、状态转换、可恢复异常和最终失败策略边界必须有日志方案。

## 8. 适用性、局限与未知

结论适用于当前单进程 Service、Application CLI 与 copy-owned 脚手架。CLI 的人机输出仍由 `pkg/cli` 管理，不应把 help、prompt 或命令结果伪装成运行日志；CLI 若拥有外部 I/O 或长期资源，仍要按同一日志规范记录 operational event。

本轮未运行真实 Service，所以尚未确认终端颜色、具体时间戳或 Windows 控制台呈现；这些不影响默认阈值和调用点缺口判断。确认实施后的进程级测试必须捕获实际 stdout/stderr，并覆盖成功、可恢复拒绝和失败。

## 9. 对当前任务的影响与研究门禁

028 不需要新依赖、公共 Logger 方法或第二套日志组件。实施范围集中在默认配置、application/Kernel/HTTP 事件分类、文档和验证。Logger 契约、资源所有权、baseline/replacement 和 configuration generation 边界保持不变。

关键问题已有代码、配置、测试和文档证据；事实与目标判断已分离，剩余终端呈现只影响实施验收，不妨碍形成计划。研究门禁通过。

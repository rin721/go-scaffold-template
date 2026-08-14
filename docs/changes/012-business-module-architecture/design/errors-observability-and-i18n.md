# 错误、日志、可观测性与 I18n

## 1. 基础闭环优先级

业务 Presenter/I18n 规则在业务门禁通过前冻结。本轮先建立进程级错误和诊断闭环：primary termination cause、runner/owner、phase、generation、candidate digest、last reload/cleanup、failed/degraded 与未完成排空必须可识别，并且只在 process owner 这一策略边界记录一次。

Kernel candidate `Ready` 不能直接映射为进程 readiness。`pkg/health` 当前未接入生产，目标只复用其有界 checker/聚合原语；lifecycle state 由 application coordinator 拥有。readiness 在 drain/关键 runner failure 时先撤销，liveness 不因任意外部依赖短暂失败而制造重启风暴。所有诊断输出必须脱敏。

## 2. 错误分层

错误从内到外保留稳定分类和原因链：

1. Domain/Application 返回业务 reason、项目 `fault.Code` 或可识别 sentinel/type，不包含本地化文案和协议状态。
2. Adapter 把第三方错误转换为项目分类，并通过 `%w` 或等价机制保留原因。
3. HTTP/CLI Presenter 决定公开状态、稳定错误码、可公开字段与 I18n message ID。
4. 只有真正决定重试、拒绝、退出或响应策略的边界记录一次错误日志。

当前 `pkg/fault` 已提供 invalid argument、not found、conflict、unavailable、timeout、canceled 和 internal 等基础分类，以及 cause/retryable 信息。模块可以定义有命名空间的业务 reason，但不能复制一套互不兼容的基础错误系统。

## 3. 错误映射

映射必须是确定且可测试的：

| 项目分类 | HTTP 语义 | CLI 语义 | 日志级别原则 |
|---|---|---|---|
| invalid argument | 客户端输入错误 | 参数/输入失败 | 通常不记 error |
| not found | 资源不存在 | 未找到 | 取决于是否异常 |
| conflict | 状态冲突 | 业务冲突 | 结构化业务信息 |
| canceled | 请求取消 | 命令取消 | 避免噪声 |
| timeout/unavailable | 依赖失败 | 临时失败 | 记录依赖与重试属性 |
| internal/unknown | 安全的内部错误 | 内部失败 | 带 cause 的边界日志 |

表中不冻结具体 HTTP 数字状态和 CLI exit code；这些公共协议值要与首个真实用例一起确认。Presenter 对未知错误必须安全降级为 internal，绝不回显堆栈、SQL、DSN、Token 或文件内部路径。

## 4. I18n 所有权

- 领域和 application 只输出稳定 reason/message ID 所需参数，不输出面向人的本地化句子。
- message ID 使用模块命名空间，避免不同模块碰撞。
- HTTP 从经过验证的语言协商结果选 locale；CLI 从明确 flag/环境策略选 locale。
- Presenter 调用 Kernel I18n Translator，翻译失败时遵循明确的安全 fallback，并保留诊断错误；不得返回空成功响应。
- message data 只包含允许公开的字段，禁止把原始错误或敏感对象直接传给模板。

当前 I18n 资源通过配置显式列出文件路径，不存在模块自动扫描。首个模块实施前必须在以下方案中确认一种：继续显式聚合文件，或设计可审计的 embed/资源合并并适配 reload。未决前不得声称模块自带消息资源已被自动发现。

## 5. 日志

使用项目 Logger 契约和稳定 facade，不在业务模块直接创建 Zap 或标准全局 Logger。日志字段需要有稳定所有者：

- 边界：request ID、operation、module、actor/tenant 的安全标识；
- Adapter：dependency、operation、elapsed、retryable；
- 生命周期：owner/runner、generation、candidate digest、phase、state、reload/cleanup outcome。

默认不记录完整请求/响应体、Authorization/Cookie、DSN、Token、Secret、私有对象或 SQL 参数。业务拒绝不应在每层重复打印。调用取消和健康检查噪声应按策略降低级别。

## 6. Request ID 与时间

Request ID 必须由可信 Middleware 创建或校验，并随 context 传给 Service/Adapter 用于诊断；它不是业务主键。缺少注入的 IDGenerator 时应用构造失败，不能沿用隐藏随机 fallback。

访问耗时和业务时间分别使用注入的 Clock；当前 `pkg/httpx` AccessLog 直接调用系统时间是目标差距。测试应能用固定 Clock 断言时序字段。

## 7. 可观测性边界

当前仓库已实现结构化 Logger 与 HTTP Request ID 基础，但没有经代码验证的 tracing/metrics 业务集成。本文不把“可观测性”写成已实现能力。

基础闭环最低证据是：

- 启动、ready、drain、停止、runner failure 和 cleanup degraded 有结构化状态/日志；
- generation、candidate digest 与 reload outcome 可关联；
- diagnostics 的并发读取、确定输出和敏感字段脱敏通过测试。

业务门禁解除后的首个垂直切片再补充：HTTP/CLI 操作、模块和 Request ID 可关联，Adapter 延迟与失败类别可诊断。
- 敏感字段脱敏测试通过。

只有出现明确 SLO、追踪或指标需求后，才设计项目自有 Telemetry 契约、传播格式、采样和 exporter 生命周期；不得让业务 Service 直接依赖某个 SDK。

## 8. Recovery

HTTP Recovery 是最后安全网，不是普通错误处理路径。它应：

- 捕获 panic，生成安全 internal 响应；
- 记录一次含 Request ID 的受控诊断；
- 不把 panic 文本和堆栈返回客户端；
- 不吞掉进程级不可恢复状态或破坏资源一致性。

后台 goroutine 的 panic 由其 owner/Supervisor 策略处理，不能假设 HTTP Recovery 能覆盖。

## 9. 验证要求

- 基础错误表驱动测试覆盖 `errors.Is/As`、primary/runtime/cancel/timeout/drain/stop/wait/cleanup 错误合并与 owner/phase。
- 状态测试覆盖 ready/draining/failed/degraded、committed cleanup 和脱敏 diagnostics。
- Presenter 测试覆盖不同 locale、缺失 message、非法模板数据和不泄密。
- 日志 capture 测试证明同一失败只在策略边界记录，并验证敏感字段不存在。
- 固定 Clock/IDGenerator 测试验证 Request ID 和耗时可控。
- 文档与实现均不得把尚未接入的 metrics/tracing 描述为已完成。

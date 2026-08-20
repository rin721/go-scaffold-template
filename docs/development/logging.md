# 开发日志规范

日志是当前项目必备的运行能力。Service 的生命周期、外部 I/O 边界、状态切换、可恢复异常和最终失败策略必须可通过结构化日志诊断；纯 Model、值对象和无副作用算法不要求机械打印。

Logger 的 API、配置和资源所有权见 [`pkg/logger`](../../pkg/logger/README.md)。本文只规定开发者何时必须记录、由谁记录、使用什么级别和字段，以及如何验证。

## 1. 获取 Logger

- 只依赖项目的 `pkg/logger.Logger` 或使用方定义的更窄接口，不导入 zap 类型。
- Logger 由 composition root 显式注入；业务函数不得创建全局 Logger、第二套 sink 或隐藏默认实现。
- Kernel 托管的 Logger 是稳定 facade。消费者不能取得 `Replace`、`Restore`、`Sync` 或 `Close`。
- `logger.Noop()` 只允许测试使用，production 构造不得以 Noop 掩盖缺失依赖。
- 构造函数确实需要日志能力时必须拒绝 nil，不能静默跳过。

## 2. 必须记录的边界

以下 owner 必须在自己的决策边界提供日志设计和测试：

| 边界 | 必记事件 |
| --- | --- |
| Participant、runner、listener、Server | Start/Ready、draining、Stop、意外退出和清理失败 |
| 配置与代际协调 | load、prepare、commit、no-op、reject、retire、cleanup debt |
| 外部 I/O Adapter | 最终重试结果、可恢复降级、超时/取消分类；不逐次泄露底层 payload |
| HTTP/CLI 边界 | operation outcome、稳定关联 ID、最终失败分类；CLI 人机输出不冒充日志 |
| 业务安全与审计边界 | 低敏 actor/resource hash、decision 和 outcome |
| 状态机 | operator 关心的状态转换和不可恢复失败 |

不要逐函数、逐行记录，也不要在高频循环重复打印同一状态。业务输入校验错误由协议边界统一呈现时，下层 Service 只返回可识别错误，不再重复记录。

当前已落地 owner 的最低要求如下：

| owner | 必记事件 | 级别 |
| --- | --- | --- |
| `application` / generation | 启动、ready、drain、commit、reload reject、cleanup debt、最终 service failure | 成功里程碑 Info；候选拒绝 Warn；终结失败或 cleanup debt Error |
| `execution` | 外部依赖 degraded/recovered、异步执行记录持久化失败 | Degraded/异步记录失败 Warn；恢复 Info |
| `migration` | `db.migrate.status` / `db.migrate.up` 的 start、completed、failed | start Debug；兼容完成 Info；dirty/incompatible Warn；operation failed Error |
| `messaging-provider` | provider state change、RabbitMQ decode reject、连接/拓扑确定性失败 | 状态正常 Info；恢复中/失败/Envelope reject Warn |
| `messaging-consumer` | Execution admission 变化失败、delivery defer/retry/dead-letter | defer/retry/admission 失败 Warn；dead-letter Error |
| `scheduler` | start/drain/stop、协调降级/恢复相关状态、task failure、fatal state | 生命周期 Info/Debug；可恢复协调与单次失败 Warn；fatal Error |
| `management` | probe fail、protected operation reject、diagnostics/build operation failed | 4xx/不通过探针 Warn；5xx 或 operation 终结失败 Error |

成功健康轮询、standby 竞争未获租约、publisher 正常 confirm、completed duplicate 和正常优雅取消不应额外打印高频 Info/Warn。

## 3. 级别

日志级别表示最低输出阈值。development 默认 `debug`，因此 Debug、Info、Warn、Error 都可见；production 未显式配置时默认 `info`。

| 级别 | 使用条件 | 示例 |
| --- | --- | --- |
| `Debug` | 开发诊断细节，不表示异常 | startup phase、candidate、resource build/reuse、no-op reload |
| `Info` | operator 关心的成功里程碑 | generation committed、listener ready、application ready/stopped、成功请求 |
| `Warn` | 操作未达预期但旧能力仍服务或请求可恢复 | reload candidate rejected、4xx、rate limit、受控 overload |
| `Error` | 当前操作失败、服务不可用或进入 degraded | initial startup failure、5xx/panic、runner failure、cleanup debt |

健康启动不得为了“覆盖四级”伪造 Warn/Error。取消和正常优雅关停不是 Error。

## 4. 唯一错误 owner

错误必须完整向上返回，只有决定处理策略的边界记录一次：

- 选择退出进程的 application operation boundary；
- 选择保留旧 generation 的 reload reporter；
- 选择进入 degraded 的 cleanup boundary；
- 选择 HTTP status/Problem 的 transport boundary。

下层只增加错误上下文并保留原因链。禁止“记录后返回、上层再次记录完整错误”的逐层重复。结构化运行日志优先记录 `error_type`、`cause_type`、`owner`、`phase` 和稳定错误码，不直接记录未经审查的 `err.Error()`。

## 5. 结构化字段

字段由语义 owner 就近定义，名称保持稳定：

- 生命周期：`application`、`owner`、`phase`、`attempt`、`generation`；
- 资源：`built_resources`、`reused_resources`；
- listener：`bound_address`、`management_bound_address`；
- reload：`changed_sections`、`previous_generation`、`current_generation`；
- HTTP：`method`、`operation`、`request_id`、`trace_id`、`duration`、`status`、`status_class`、`error_code`；
- 失败：`error_type`、`cause_type`。
- messaging：`provider`、`driver`、`consumer`、`route`、`contract`、`message_id`、`disposition`、`delivery_count`、`desired`；
- scheduler：`task`、`state`、`generation`；
- migration：`operation`、`current_version`、`target_version`、`dirty`、`empty`、`compatible`；

禁止记录密码、Token、Secret、完整 DSN、Authorization、Cookie、原始 CLI args、完整 URL/query、请求/响应 body、完整 Config/Snapshot、subject 或业务对象。需要关联身份或资源时使用已有低敏 hash，不自行散列可逆或低熵秘密。

字段值必须控制基数。动态错误文本、用户输入和任意 map 不能成为长期标签或稳定字段。

## 6. 开发步骤

新增模块、Adapter 或运行 owner 时，在研究和设计中回答：

1. 哪些状态变化和外部结果需要 operator 观察；
2. 哪个边界是每个事件的唯一 owner；
3. 每个事件的 level、稳定字段和关联 ID；
4. 哪些值敏感或高基数，如何排除；
5. 取消、重试、降级和最终失败如何分级；
6. 用什么测试证明事件存在、顺序正确、没有重复或泄漏。

纯逻辑不需要 Logger 时写明“不适用及原因”，不要为通过检查增加空调用。

## 7. 验证

- 单元测试使用 `logger.TestLogger` 断言 level、message、数量和事件顺序。
- 需要验证真实编码、过滤或脱敏时，把 `pkg/logger.Resource` 输出到测试临时文件并读取结果。
- 生命周期测试覆盖健康启动、no-op、成功 reload、候选拒绝、cleanup debt、取消和停止。
- HTTP 测试覆盖成功、4xx、429、受控 503、5xx 和 panic，并验证 request/trace correlation。
- 进程级测试只使用临时目录、loopback 临时端口和临时资源，结束后证明端口与文件句柄已释放。
- production 源码不得调用 `logger.Noop()`，`cmd/internal` 不得导入标准全局 logger，zap 只能存在于 `pkg/logger` 实现边界。
- production 源码不得使用 `logger.String("error", err.Error())` 记录原始错误文本；应使用稳定 `error_type`、`cause_type` 或经过审查的错误码。

构建通过不能替代日志语义、脱敏或真实过滤验证。

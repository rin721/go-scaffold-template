# 041 日志体系补齐与治理需求

## 目标

在不重建日志框架、不制造噪声、不泄露敏感信息的前提下，补齐当前项目关键运行路径的结构化日志与测试门禁，使日志能和 Trace、Execution Record、Health、Diagnostics 一起回答：

```text
发生了什么 -> 发生在哪里 -> 为什么发生 -> 影响了什么
-> 系统采取了什么动作 -> 最终是否恢复
```

## 功能要求

| ID | 要求 |
| --- | --- |
| REQ-001 | 保留现有 `pkg/logger`、Kernel baseline/replacement 和 `docs/development/logging.md` 作为日志能力与规范 authority，不新增平行 Logger 或全局 sink。 |
| REQ-002 | 修复 Execution 异步记录失败日志，禁止记录原始 `err.Error()`，改为 owner、phase、error_type、cause_type、overflow/state 等低敏字段。 |
| REQ-003 | 为 `db migrate status/up` one-shot operation 增加结构化日志，记录 operation、phase、outcome、current/target/dirty/compatible、error_type 和 cleanup result；CLI JSON/stdout 继续只做人机/机器输出。 |
| REQ-004 | 为 Messaging Consumer 的非成功 disposition 增加低敏事件，覆盖 defer、retry、dead-letter、decode reject、handler panic/timeout 和 Execution backend unavailable。 |
| REQ-005 | Messaging Provider、Consumer admission 和 Consumer disposition 日志必须能关联 application generation、provider、consumer、route、message_id 或 trace_id；不得记录 payload、Broker URI、headers 全量、subject、body 或原始错误文本。 |
| REQ-006 | Management health/diagnostics 的关键异常 outcome 需要有低敏 owner 日志，覆盖 readiness fail/warn、diagnostics 读取失败和 management operation 失败；健康轮询成功路径不得高频刷 Info。 |
| REQ-007 | Scheduler 现有日志保持“状态变化和失败优先”，不得为每次正常 tick、并发 skip 或高频队列变化制造噪声；补足必要字段和测试保护。 |
| REQ-008 | 明确 Debug/Info/Warn/Error 在 execution、schedule、messaging、migration、management 中的级别选择，并保证取消和正常关停不误报 Error。 |
| REQ-009 | 补齐日志测试门禁：断言关键事件 level、message、字段、顺序、数量、去重和敏感信息排除；架构搜索继续禁止 production `logger.Noop()`、直接 zap/global logger 和原始错误文本日志。 |
| REQ-010 | 同步更新开发日志规范、execution/schedule/messaging/operations 文档和 041 任务证据；当前有效规范回到主题文档，`docs/changes/041` 只保存任务证据。 |

## 非目标

- 不引入 OTLP Logs、集中日志平台、采样策略、日志轮转或审计日志持久化。
- 不改变 `pkg/logger.Logger` 公共接口，不把 zap 类型暴露给业务层。
- 不把每个函数、每个成功 CRUD、每次健康轮询、每次 scheduler tick 或每次成功消息 ack 都打印为 Info。
- 不用日志替代 metrics、trace、Execution Record、Health 或 Diagnostics。
- 不接入真实 Redis/数据库 execution backend，不改变 RabbitMQ topology 策略，不新增外部服务。
- 不修改数据库 migration SQL、OpenAPI 契约或业务语义。

## 验收标准

- `go test` 覆盖本轮修改包的日志事件和低敏字段。
- `go test ./... -count=1`、`go vet ./...`、`go build ./cmd/app`、`go mod tidy -diff`、`gofmt -l .`、`git diff --check` 通过，或如实记录与本任务无关的既有失败。
- 搜索确认 production 源码没有新增 `logger.Noop()`、直接 `zap.New*`、标准库全局 logger、`fmt.Print` 运行日志或 `logger.String("error", err.Error())` 这类原始错误文本日志。
- 真实进程 smoke 能在临时目录、loopback 端口和 SQLite 下观察健康启动、HTTP request、migration one-shot、正常 shutdown 关键日志。
- 定向测试覆盖 Execution async error、Messaging Consumer disposition、Provider/admission generation 字段、management failure outcome、Scheduler 日志保护。
- 文档说明和真实实现一致，不把未实施能力写成已完成。


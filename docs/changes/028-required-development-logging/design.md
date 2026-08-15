# 设计：开发日志基线与启动可见性

## 1. 设计结论

保留现有单一日志链：

```text
cmd/app
  -> pkg/logger baseline Resource
  -> internal/kernel/logging.Manager stable facade
  -> configured Logger in Application Generation
  -> composition injects pkg/logger.Logger
  -> lifecycle / HTTP / module-owned boundary events
```

本任务只改变默认级别、事件覆盖、结果分级和开发治理，不新增 Logger 接口、全局变量、Service Locator、异步日志管线或第三方依赖。

## 2. 默认级别

默认解析必须区分环境：

| Environment | 未显式 level | 目的 |
| --- | --- | --- |
| `development` | `debug` | 本地默认看见诊断细节以及 Info/Warn/Error |
| `production` | `info` | 默认抑制高频 Debug，保留运行里程碑和异常 |

`DefaultConfig()` 继续代表完整默认 development 配置，因此输出 `development + debug`。按 Environment 补空字段时，level 与 encoding/caller/stacktrace 一样由环境解析；调用方显式设置 level 后不得被环境覆盖。

入口 baseline 先于配置加载，使用 development/debug 的安全 console 配置，以便早期阶段可见。它只输出本设计允许的低敏事件；配置化 Logger 一旦 generation 提交就接管输出和阈值。

预计修改：

- `pkg/logger/defaults.go`、`builder.go`、相关测试；
- `internal/kernel/app/logger` Defaults 与测试；
- `config.example.yaml` 和 `cmd/app` 默认配置 golden/测试。

如果当前 `Config.Level` 零值无法在不破坏显式语义的情况下表达环境默认，实施必须先用定向测试冻结“缺失”和“显式 level”的区别；不得让 production 静默变成 debug，也不得新增旧常量 alias。

## 3. 启动事件模型

### 3.1 事件 owner

| Owner | 负责事件 | 不负责 |
| --- | --- | --- |
| application entry | mode 选择、Service operation 最终结果分类 | 底层资源细节、原始 CLI args |
| GenerationCoordinator | load/prepare/commit/retire 状态、generation identity、整体 ready | 每个第三方 Client 的内部调试 |
| generation factory/resource pool | capability build/reuse 摘要 | 重复记录同一 Kernel 成功事件 |
| listener/server owner | business/management bind 与 serve-ready | 业务 operation 结果 |
| Supervisor/application lifecycle | application ready、draining、stopped、runner terminal failure | 重复底层返回的完整错误链 |
| HTTP boundary | request outcome、operation/request/trace correlation | body、Authorization、业务对象或完整 URL |
| module boundary | 安全审计或模块独有的重要状态变化 | 已由 HTTP/lifecycle 记录的同一事实 |

### 3.2 启动顺序

健康 Service 的目标输出顺序按真实状态约束，而非硬编码时间：

```text
Debug application service selected
Debug generation load started
Debug generation prepare started
Debug resources prepared (built/reused owners)
Debug listener readiness started
Info  application generation started (generation + two addresses)
Info  application ready
```

停止顺序：

```text
Info application draining
Debug generation retirement started
Debug resources released
Info application stopped
```

不得为了让每级都出现，在健康路径增加 Warn/Error。Warn/Error 由第 5 节的真实异常场景产生。

### 3.3 避免低价值重复

当前每个 immutable resource 使用小 Kernel 启动，`kernel started components=N` 可能重复且不能识别 owner。实施应选择以下单轨之一并通过测试固定：

1. 为 Kernel lifecycle 日志增加稳定 component IDs/owner 字段，并让 generation 摘要不重复同一成功事实；或
2. 将小 Kernel 的逐实例成功日志降为 Debug，由 GenerationCoordinator 统一输出一次 build/reuse 摘要。

优先选择第 2 条：应用级 operator 事件由 GenerationCoordinator 汇总，小 Kernel 细节留在 Debug。不得同时保留多条等价 Info。

## 4. 结构化字段

按 owner 就近定义稳定字段，不建立全局杂物包：

- application：`application`、`mode`；
- lifecycle：`owner`、`phase`、`attempt`、`generation`；
- resources：`built_resources`、`reused_resources`；
- listener：`bound_address`、`management_bound_address`；
- reload：`changed_sections`、`previous_generation`、`current_generation`；
- failure：`error_type`、`cause_type`，只在已知安全时增加有限状态码；
- HTTP：`method`、`operation`、`request_id`、`trace_id`、`duration`、`status_class`。

禁止字段：原始 args、完整 URL/query、Header、Authorization、Cookie、request/response body、完整 Config/Snapshot、DSN、密码、Token、Secret、subject、业务对象和未经审查的 `err.Error()`。

## 5. 级别与失败边界

### 5.1 Reload

- snapshot 未变化：Debug；
- 新 generation 成功提交且旧代清理成功：Info；
- load/prepare/commit 候选失败但旧代继续 active：Warn，只记录 phase/owner/type；
- 新 generation 已提交但 retire/cleanup 失败、进入 degraded：Error。

`reloadErrorReporter` 是这组异步结果的唯一日志 owner。GenerationCoordinator 只更新 diagnostics 并返回错误，不再打印同一失败。

### 5.2 初始启动和 Supervisor

`runService` 或等价 application operation boundary 负责一次结构化失败分类。正常 context cancellation 和用户触发的 graceful shutdown不记 Error。结构化事件只记录 error type、phase、owner/generation；`cmd/app.execute` 继续向 stderr 输出一次完整人类可读错误。

这不是重复完整错误：日志用于机器关联且低敏，stderr 用于当前调用者。下层 Kernel、factory、resource 和 participant 只返回错误链。

### 5.3 HTTP

`AccessLog` 在统一 HTTP 边界根据最终可识别结果分级：

- 2xx/3xx：Info；
- 预期客户端拒绝与受控限流/过载：Warn；
- 5xx、panic、未知内部错误：Error。

若当前中间件位置拿不到最终 status，实施必须先把 outcome 分类放在能够观察最终结果的现有边界，不允许猜测状态或重复安装第二个 access middleware。取消/超时保持可识别，不能统一改写为内部 Error。

## 6. 开发日志 authority

新增 `docs/development/logging.md`，章节固定覆盖：

1. 为什么日志是必备能力以及适用/不适用位置；
2. 显式注入和资源 owner；
3. 四级选择矩阵与事件示例；
4. 生命周期、外部 I/O、HTTP/CLI、重试/降级和最终失败边界；
5. 结构化字段、关联 ID、敏感信息和基数限制；
6. 错误去重与“记录或返回”的责任；
7. 测试与代码审查清单。

导航和短门禁同步到：

- `README.md`、`docs/README.md`；
- `docs/development/application-module-development.md`；
- `pkg/logger/README.md`；
- `AGENTS.md`。

同一规则只在 authority 详细展开，其他文件使用链接，避免形成第二套现行规范。

## 7. 可执行门禁

在现有 architecture tests 增加以下通用检查：

1. production `.go` 文件不得调用 `logger.Noop()`；测试可以使用。
2. `internal/**` 和 `cmd/**` 不得直接导入 `go.uber.org/zap` 或创建标准库全局 logger；第三方实现只留在 `pkg/logger`。
3. application/runtime owner 的构造依赖 Logger 时必须拒绝 nil，不能静默跳过。
4. 新日志不得直接使用稳定敏感字段名或记录完整 Config/Snapshot/HTTP body；静态检查只能覆盖确定模式，语义仍由测试与 review 清单保证。

不建立“每个 package 必须有日志调用”的机械门禁；它会迫使纯逻辑产生噪音，也无法证明日志质量。

## 8. 测试设计

### 8.1 Logger 默认与过滤

- development 缺省配置写入四个级别并断言四条都可见；
- production 缺省配置写入四级并断言 Debug 被过滤、其余可见；
- 显式 level 覆盖环境默认；非法 level 继续失败；
- baseline 与 configured replacement 切换后各自阈值正确。

### 8.2 生命周期事件

使用 `logger.TestLogger` 和确定性 fake generation/resource/listener 验证：

- 健康启动只出现 Debug/Info，关键 phase、owner、generation 和地址齐全且顺序合法；
- no-op/success/rejected/degraded reload 分别为 Debug/Info/Warn/Error；
- cancellation/graceful stop 不出现 Error；
- 同一失败不被 application、coordinator、resource 三层重复记录。

### 8.3 HTTP 与脱敏

- success/client rejection/overload/internal failure/panic 对应正确级别；
- request_id/trace_id/operation 保持；
- Authorization、query、body、subject、DSN 和注入的 secret error text 不出现在输出。

### 8.4 真实进程 smoke

确认后新增或扩展 `cmd/app` 黑盒测试，在测试拥有的临时目录中：

1. 生成 development/debug 配置和临时 SQLite；
2. 使用 loopback 临时端口启动、等待 business/management ready；
3. 捕获 stdout/stderr，验证 Debug/Info 启动链且无 Warn/Error；
4. 写入稳定非法候选，验证一条 Warn 且旧代继续服务；
5. 单独使用初始非法配置验证一条低敏 Error 与最终 stderr；
6. 发送取消并验证 draining/stopped，清理进程、端口和临时目录。

测试不得依赖固定端口、外部 Database/Redis/S3/OTLP 或网络。

## 9. 文件影响

预计修改：

- `pkg/logger/defaults.go`、`builder.go`、`logger_test.go`、`README.md`；
- `internal/kernel/app/logger/logger.go` 及测试；
- `internal/kernel/kernel.go`、`generation.go` 及相关测试；
- `internal/composition/application.go`、`service.go`、`generation.go`、`generation_resources.go` 及测试；
- `pkg/httpx/production_middleware.go` 及测试；
- `cmd/app/main_test.go`、`config.example.yaml`；
- architecture tests；
- `README.md`、`docs/README.md`、新增 `docs/development/logging.md`、应用模块开发指南、`AGENTS.md`；
- 028 状态与实施证据。

预计不修改 `logger.Logger` 方法集合、`go.mod`、`go.sum`、OpenAPI、Database schema、业务 Model/Service、部署和 release 文件。

## 10. 重新确认触发器

出现以下任一情况必须停止实施、更新研究与计划并重新确认：

- 需要修改 `logger.Logger` 公共方法或暴露 zap 类型；
- 需要新增/升级依赖、异步日志、远程 sink、采样或日志轮转；
- 需要改变 production 配置兼容、HTTP 对外行为或错误 stderr 契约；
- 需要记录原始错误、配置、请求 body 或身份数据才能满足诊断；
- 需要启动外部服务、固定端口、真实远端资源或执行部署写入；
- 无法在现有 HTTP 边界取得可靠 outcome，只能重复安装日志中间件。

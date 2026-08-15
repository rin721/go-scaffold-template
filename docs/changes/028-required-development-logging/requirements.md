# 需求：开发日志基线与启动可见性

## 1. 依据

- `R001`：Logger 能力与强制 baseline 已实现；开发默认 `info` 会过滤 Debug，启动阶段日志稀疏，production 没有真实 Warn 调用，开发文档也没有统一日志门禁。
- 004/007：保留项目自有 Logger 契约、入口拥有 baseline、配置化 replacement 提交后接管、失败恢复 baseline 的既有单轨设计。

## 2. 目标

让开发者在默认本地配置下看到从 Debug 到 Error 的所有适用事件，让 Service 从入口、配置、候选构造、listener ready、运行、重载到停止形成可诊断且低噪的结构化日志链，并把日志设计与验证纳入项目开发规范。

“从 Debug 到 Error 都能打印”表示 development 默认最低级别为 `debug`，四个级别都不会被阈值过滤；不要求健康启动伪造 Warn/Error。

## 3. 功能要求

### `REQ-001` 环境化默认级别

- development 未显式配置 level 时使用 `debug`；production 未显式配置时使用 `info`。
- `config init` 和 `config.example.yaml` 的默认 development 配置写出 `level: debug`。
- 显式 `debug/info/warn/error` 继续严格生效；未知级别确定失败，不静默回退。
- 入口 baseline 在配置读取前必须允许 Debug 诊断，配置化 Logger 提交后按当前 generation 配置接管。

### `REQ-002` 启动与停止事件链

Service 至少记录以下真实事件：

- `Debug`：选择 Service mode、开始配置加载、开始候选准备、资源 build/reuse 摘要、开始 listener/ready 门禁；
- `Info`：完整 generation 已提交、业务与 management listener 已 ready、Application 已 ready、开始优雅停止、停止完成；
- `Error`：初始配置加载、候选准备、listener/ready、Supervisor task 或清理失败的唯一低敏分类事件。

事件必须带 owner/phase/generation 等可关联字段，不输出原始配置、完整错误文本、CLI args 或凭据。

### `REQ-003` 重载与运行结果分级

- no-op reload 使用 `Debug`；成功提交使用 `Info`。
- 候选被拒但旧 generation 继续健康服务使用 `Warn`。
- 已提交但旧代清理失败、进入 degraded 或运行 owner 失败使用 `Error`。
- HTTP access log 按结果分级：成功使用 `Info`，可恢复拒绝使用 `Warn`，5xx、panic 或未知失败使用 `Error`；认证审计保持独立低敏事件。

### `REQ-004` 唯一错误记录边界

- 错误继续完整向上返回并保留原因链。
- 决定“失败退出、保留旧代、进入 degraded、返回 5xx”等策略的边界负责日志；下层不得重复打印同一错误。
- 结构化错误事件只记录受控分类字段。`cmd/app` 最终 stderr 继续提供一次人类可读错误，不把同一完整错误链再次写入结构化日志。
- 取消和正常关停不得误记为 Error。

### `REQ-005` 开发日志规范

新增 `docs/development/logging.md` 作为唯一当前 authority，必须明确：

- 必须记录的 owner、边界和状态变化；
- Debug/Info/Warn/Error 选择表与反例；
- Logger 的显式注入、`With` 和结构化字段约定；
- request/trace/generation/component 等关联字段；
- 密码、Token、DSN、Authorization、body、原始配置、个人数据等禁止项；
- 错误去重、取消、重试、采样和高频日志约束；
- 单元、集成和进程级日志验收方法。

根 README、文档索引、应用模块开发指南、`pkg/logger` API 文档和 `AGENTS.md` 只增加必要入口或门禁，不复制整篇规范。

### `REQ-006` 新开发门禁

- 新增模块、外部 I/O Adapter、Participant、runner、listener 或状态机时，研究与设计必须说明日志 owner、事件、级别、字段、敏感信息和验证。
- 纯 Model、值对象和无副作用算法不强制打日志；禁止逐函数、逐行、成功循环刷屏。
- production composition 不得使用 `logger.Noop()`，不得直接创建 zap/global logger 绕过项目契约。
- Logger 必须通过已有 composition 显式注入；调用方不得创建第二套 Logger 或关闭共享 Resource。

## 4. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 可见 | 默认 development 场景可观察 Debug 和 Info；受控异常场景分别可观察 Warn 和 Error |
| 低噪 | 健康启动不产生 Warn/Error；循环与每请求日志不重复记录同一事实 |
| 可关联 | 生命周期事件至少包含 owner/phase，代际与请求事件包含对应 generation/request/trace 字段 |
| 安全 | 结构化失败日志不含完整错误文本、DSN、Token、Authorization、body、原始配置或 subject |
| 单轨 | 继续使用 `pkg/logger` 和现有 Manager/replacement，不引入全局 logger、第二后端或兼容层 |
| 可验证 | 默认、级别、事件顺序、分类、去重、脱敏和 production Noop 均有自动化证据 |

## 5. 范围

### 包含

- development/production 默认 level 语义和默认配置示例；
- Application、GenerationCoordinator、底层 component、listener、Supervisor 边界的启动/停止/失败日志；
- reload 与 HTTP 结果的级别校正；
- 当前权威开发日志规范及文档导航；
- Logger、composition、HTTP、进程与 architecture 测试。

### 不包含

- 替换 zap、改变 `logger.Logger` 方法集合或暴露第三方类型；
- 日志轮转、异步队列、远程采集、OTLP Logs、集中检索或采样平台；
- 审计日志持久化、合规留存或业务事件总线；
- 给每个函数、纯 Model 或成功循环机械加日志；
- 修改公开 HTTP API、数据库 schema、业务规则或部署配置；
- push、tag、Release、部署或外部服务写入。

## 6. 验收标准

1. `logger.New(nil)` 和默认 development 配置允许 Debug/Info/Warn/Error；production 缺省 level 过滤 Debug 并允许 Info/Warn/Error。
2. `config init` 与 `config.example.yaml` 默认输出 `development + debug`，文档清楚说明 production 默认/建议为 `info`。
3. 进程级健康启动输出连续、可关联的 Debug/Info 事件，包含 generation、业务 listener、management listener 和 ready 里程碑，不产生 Warn/Error。
4. 稳定非法 reload 输出一条 Warn 并明确旧 generation 保持 active；cleanup debt 输出一条 Error。
5. 初次启动失败输出一条低敏结构化 Error 分类和一条最终 stderr，不在结构化日志复制完整错误或秘密。
6. HTTP 成功、可恢复拒绝、5xx/panic 分别按 Info/Warn/Error 记录，request_id 与 trace_id 保持关联。
7. 文档 authority、模块评估和 Agent 规则明确“何时必须使用 Logger”，production code 不出现 `logger.Noop()` 或直接 zap/global logger。
8. 定向测试、全量 test/race/vet/build/tidy、Markdown 链接与 `git diff --check` 通过；进程 smoke 只使用临时目录、loopback 临时端口和临时 SQLite。

## 7. 确认要求

这是包含源码、配置和测试实现的非纯文档计划。只有用户在本计划报告后的后续消息明确确认 028 当前方案，才能实施 `GOV-001` 至 `VER-001`。若实施需要改变 Logger 公共方法、第三方依赖、配置 schema 兼容、错误输出契约、外部采集或部署副作用，必须退回研究并重新确认。

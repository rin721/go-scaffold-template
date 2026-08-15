# 028 开发日志基线与启动可见性

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，证据为 `R001`。
- 计划状态：用户已在计划报告后的独立消息确认，`GOV-001..VER-001` 已实施并验证。
- 当前结果：development 缺失日志级别时采用 `debug`，production 采用 `info`；Service、Generation、reload、停止和 HTTP outcome 已形成分级结构化日志链，开发规则与架构门禁同步生效。
- 外部副作用：进程 smoke 只在测试拥有的临时目录启动短生命周期进程、绑定 loopback 临时端口并创建临时 SQLite 数据，测试结束后已清理；未连接外部服务，未 push、tag、发布 Release 或部署。

## 问题

项目已经有强制 Kernel baseline、配置化 Logger replacement、`Debug/Info/Warn/Error` 契约和 HTTP access/security audit 日志，但当前开发默认阈值是 `info`，会过滤 `debug`。长期 Service 的生产日志调用又集中在少数成功里程碑、HTTP 请求和重载失败上；配置加载、候选准备、资源构造与 Ready、listener 就绪等启动阶段没有形成可读事件链，失败大多只在最终 stderr 边界显示一行错误。

项目文档目前说明了 Logger API、注入、资源所有权和敏感信息约束，却没有一份当前权威的开发日志规范，也没有要求新增模块或运行 owner 明确日志事件、级别、字段和唯一错误记录边界。

## 计划结论

本计划不重做 Logger，也不引入新日志依赖。实施采用以下单轨方案：

1. 开发默认配置改为 `debug`，使 `Debug`、`Info`、`Warn`、`Error` 都具备输出资格；生产默认保持 `info`。
2. 为 Service 启动、代际构造、Ready、listener、配置重载和停止建立少而明确的结构化事件链。
3. 按事件结果选择级别，不在成功启动时伪造 `Warn` 或 `Error`：细节用 `Debug`，正常里程碑用 `Info`，可恢复异常用 `Warn`，失败或 degraded 用 `Error`。
4. 最终 stderr 保留人类可读错误；结构化失败日志只记录低敏错误分类和阶段，不重复输出完整错误链。
5. 新增 `docs/development/logging.md` 作为唯一当前 authority，并由根文档、开发指南、Logger API 文档和 `AGENTS.md` 链接或引用。
6. 用日志契约测试、启动/失败/重载场景测试和生产代码 `Noop` 门禁阻止回退。

## 阅读顺序

1. [R001 当前日志链与可见性复核](research/R001-current-logging-visibility/report.md)
2. [需求](requirements.md)
3. [设计](design.md)
4. [任务与确认状态](tasks.md)

本目录是任务级计划和实施证据，不替代当前 [开发日志规范](../../development/logging.md) 或 [Logger API 说明](../../../pkg/logger/README.md)。

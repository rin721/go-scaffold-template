# 025 业务模块边界收口

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，见 [R001](research/R001-current-business-module-boundary/report.md)。
- 计划状态：已确认并实施完成。
- 当前授权：用户已在计划报告后的后续消息中明确确认“确认实施 025 当前方案”，授权 `GOV-001` 至 `VER-001` 的本地实施、验证和聚焦提交；不授权 push、tag、Release、部署或外部写入。

## 目标

把“业务能力默认收口在 `internal/module/<name>`，只有经过能力评估证明为跨业务复用且由进程统一选择的底层资源才进入 Kernel Capability”落实为当前代码边界和自动门禁。

首轮收口当前唯一反例 Todo HTTP：全局 OpenAPI authority 与纯生成协议代码继续留在应用级位置，所有手写 Todo Handler、DTO 映射、错误呈现和请求身份端口迁回 Todo 模块；composition 只连接模块完成品并安装业务 HTTP Handler。

## 实施结果

- Todo strict Handler、DTO 映射、错误呈现、request metadata 与协议测试已经收口到 `internal/module/todo/binding/http`；旧顶层 Todo transport 已删除。
- Todo HTTP binding 只依赖 Todo-owned `RequestAccess`；composition Adapter 连接 Auth Principal/Service，Todo 不导入 Auth。
- Todo 长期 Service 使用显式 `HTTPModule`，同时返回 Service、Handler 与 contribution；application Router 只安装完成 Handler。
- `cmd/app` 生产入口只依赖标准库与 `internal/composition`；基线日志、Kernel logging manager 和退出码映射由 application composition 拥有。
- package graph 已增加 module owner、entrypoint 和跨模块反向依赖门禁，正反 fixture 与真实 production graph 均通过。
- OpenAPI authority、生成协议、Todo HTTP/CLI 行为、配置、依赖和 Application Generation 生命周期保持不变。

## 阅读顺序

1. [研究索引](research/README.md)
2. [R001 当前业务模块边界](research/R001-current-business-module-boundary/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务](tasks.md)

## 范围摘要

- 收口 Todo 手写 HTTP Adapter 与测试。
- 以 Todo-owned 窄端口隔离 Auth 主体类型，跨模块映射只留在 composition root。
- 让 Todo 的 HTTP profile 返回已经构造完成的 Handler；应用 Router 不再构造 Todo transport。
- 移除 `cmd/app` 对 Ops 内部 Model 的直接依赖。
- 加固 package graph，禁止 composition 之外的包导入其他业务模块内部包。
- 同步当前权威文档并删除旧路径、旧说明和兼容入口。
- 不机械补齐没有真实需求的 Handler、Repository、binding 或配置目录。

## 确认门禁

用户已确认并完成 025 当前方案。实施和验证证据见 [任务账本](tasks.md)；本任务没有授权 push、tag、Release、部署或外部写入。

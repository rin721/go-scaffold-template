# R002：Kratos layout 与 Wire

## 研究问题

检查真实生成 composition、业务分层和 App lifecycle，判断哪些结构可以迁移，哪些依赖不应引入。

## 源码证据

在快照 `94dbfcc...` 的 `cmd/server/wire_gen.go` 中，生成的 `wireApp` 是一条普通 Go 构造链：`NewData` 返回 data 与 cleanup，随后构造 Repo、Usecase、Service、gRPC/HTTP Server，最后 `newApp`。依赖图在编译期成为普通调用代码，不存在请求期间按名字查找。

layout 把业务规则、数据实现和协议服务分在 `internal/biz`、`internal/data`、`internal/service`。这种横向分层适合单服务模板，但在拥有多个业务能力的模块化单体中，若直接复制会把所有 Service/Repo 聚集成全局目录；012 只吸收“内层接口 + 外层实现”的方向，顶层改按业务能力纵向组织。

Kratos `App` 快照 `668db92...` 收集多个 Server，使用 errgroup 启动并在 context/信号驱动下停止；服务注册与注销也纳入 App 生命周期。它说明“构造对象”和“启动 Server”应是两个阶段，cleanup/Stop 需要 owner。

## 可迁移经验

- composition root 显式连接 Data/Repo/Usecase/Service/Server。
- inner business interface 由需求驱动，外层 data 实现它。
- 构造函数可以返回 cleanup，并由顶层决定执行顺序。
- Server 生命周期不应隐藏在 Handler 或业务 Service 中。
- 生成代码最终仍应可读成普通 Go 构造链。

## 不直接采用

Wire 项目已经归档，当前仓库没有需要通过新依赖解决的大型静态图。引入 Wire 会增加生成工具、`wireinject` build tag、生成物同步和维护门禁，而手工 composition 已符合现状。

Kratos layout 的 `ServiceContext`/分层命名也不是本仓库目录权威；其 App 生命周期没有当前 Kernel candidate reload 语义，不能替代 Kernel/Host。

## 对 012 的结论

采用“普通构造链 + outer cleanup + business port/data adapter”的结构性经验。保持手工装配并让 Kernel 管资源、Host 管 Participant；业务模块按能力纵向组织。未来对象图显著增长时，可重新评估生成器，但必须证明手工图的真实维护成本，不能默认选择已归档 Wire。

## 局限

本报告检查官方源码快照而非在本仓库运行 Kratos 示例。上游 App 解决的是 Kratos Server 进程生命周期，不包含当前项目的配置候选事务。

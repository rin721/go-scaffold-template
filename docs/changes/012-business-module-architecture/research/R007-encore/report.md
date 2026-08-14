# R007：Encore

## 研究问题

确认 Encore 的“模块/服务图”来自什么机制，评估静态发现是否比手工 composition 更适合本仓库。

## 源码与文档事实

Encore 把一个 Go package 识别为服务，并从受约束的 API 声明与资源声明派生应用模型。官方仓库包含 parser/compiler、Go runtime 和大量 e2e 测试；`v2/parser` 对 API、infra resource 和源码关系进行解析，不只是运行时反射。

官方 SQL 示例在服务 package 中使用 `sqldb.NewDatabase` 声明数据库资源，平台据此提供本地/云基础设施、迁移和连接。教程展示服务 API 调用、数据库和分布式追踪由工具链统一理解。

因此 Encore 的“自动装配”并非一小段 Go Registry，而是编译器、运行时、CLI、基础设施和部署平台共同形成的产品能力。

## 优势

- 服务/API/资源图可由源码自动派生并用于开发、部署和观测。
- 平台可以统一数据库迁移、Secret、tracing 和环境差异。
- 编译期分析比任意 runtime scanning 更早发现部分错误。
- 对接受其编程模型的团队，开发与运维路径高度集成。

## 不适合当前仓库的原因

当前目标是普通 Go 脚手架上的显式 composition 与项目自有 Capability。采用 Encore 意味着接受专用 parser/compiler/runtime/CLI 和资源声明语法，边界远超新增一个库。

package-level 资源声明即使由编译器治理，也与本仓库“资源由明确构造/owner 注入、禁止隐式初始化”的当前规则不同。它也会重叠 Kernel 配置、Database、Host 和 reload 语义。

## 对 012 的结论

不采用 Encore 工具链或自动服务发现。吸收的原则是：若未来生成/分析工具存在，它应从静态源码契约生成可审查图并具备强验证，而不是 runtime `init` 扫描。

当前对象图规模不足以证明专用编译平台成本。用普通 Go 手工 composition、typed contribution validator 和 import tests 就能满足需求。

## 局限

本报告只分析架构机制和适用边界，不评价 Encore 的云产品、价格或完整生产能力。上游快速演进时需要刷新版本事实。

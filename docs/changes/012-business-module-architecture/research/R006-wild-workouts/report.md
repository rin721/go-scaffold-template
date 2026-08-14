# R006：Wild Workouts

## 研究问题

检查项目作者的官方系列文章与源码，关注业务边界、Repository 所有权、持久化模型和事务表达，而非复制示例业务。

## 已验证模式

Wild Workouts 将应用/领域逻辑与 HTTP/gRPC、Firestore/SQL Adapter 分离。业务层定义自己需要的 Repository 接口，基础设施实现依赖该接口方向；测试可替换为内存实现而不启动完整进程。

Repository 文章展示了把事务包在 update closure 中：Repository 读取聚合，把领域更新函数应用在事务边界内，再提交。作者明确反对通过 context 或 Middleware 隐式携带事务，因为那会把技术细节泄漏到用例并让入口绑定。

示例也区分领域对象和数据库结构，转换逻辑留在 Adapter。composition 在入口显式选择具体实现，而不是让业务层创建数据库客户端。

## 适合迁移的原则

- Repository 接口由 application 使用方定义，方法使用业务语言。
- 领域对象与 persistence Record 显式转换。
- 事务边界由用例语义表达，可使用闭包让底层 Tx 不逃逸。
- HTTP/gRPC 只是调用同一应用服务的 Adapter。
- 小而明确的 fake port 使业务测试不依赖基础设施。

## 需要调整的部分

该样本是架构教学项目，不是通用框架或当前模板。其具体目录、领域和 Repository closure 形状不能直接成为本仓库公共 API。

当前项目已有 `Database Access.Use/WithinTx` 与 borrowed Client 约束。012 必须让 module-specific Repository/UnitOfWork 适配现有回调，而不是另建连接/事务系统。

文章中的 CQRS/事件等后续主题不因引用该项目自动进入范围；首个真实用例没有证据时保持不实现。

## 对 012 的结论

把 caller-owned port、domain/record 分离和显式 transaction closure 作为业务内层核心原则。单 Repository 原子操作可封装事务；多 Repository 用例定义模块专用 UnitOfWork port，由 Database Adapter 用 `WithinTx` 实现。禁止 Tx context、Handler 事务和共享具体 Repository。

## 局限

原则经过源码示例验证，但未在当前仓库运行该应用。数据库错误、隔离级别和性能必须由首个真实业务适配与合约测试确定。

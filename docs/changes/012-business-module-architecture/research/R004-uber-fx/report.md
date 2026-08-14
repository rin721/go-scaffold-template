# R004：Uber Fx

## 研究问题

从官方文档和源码确认 Fx 的依赖图、Module、value group 与生命周期语义，作为手工装配的对照组。

## 已验证机制

Fx 使用 constructor/`Provide` 建立运行时类型图，通过 `Invoke` 触发对象构造。`fx.Module` 为一组 option 提供命名作用域，官方建议模块即使不依赖 Fx container 也应能够被普通构造和测试。

Lifecycle 允许 constructor 追加 Hook。`Start` 按 Hook 追加顺序执行 `OnStart`，失败会停止已启动 Hook；`Stop` 按反序执行 `OnStop`。这是明确而成熟的资源所有权模式。

Value groups 可让多个 constructor 向同类型集合贡献值，适合 route/handler/plugin 集合；但官方文档明确 group 内顺序不保证。顺序敏感时必须额外建模，不能依赖注册文件顺序。

## 优势

- 大型对象图可以按 Module 组合，减少顶层手写连接代码。
- constructor、依赖校验和生命周期 Hook 有统一机制。
- graph validation、日志与测试支持成熟。
- value group 能表达多实现集合，适合大规模可插拔贡献。

## 与当前约束的冲突

当前仓库已经有明确 Kernel Plan/Host 生命周期，而且要求依赖从 composition 代码直接可见。再引入 Fx 会形成第二个运行时容器和第二套生命周期语义；类型缺失、重复和可选依赖的一部分错误会延后到 runtime graph 构建。

Value group 的无序语义不适合需要确定 middleware/participant/command 顺序的 contribution。`fx.In` 参数对象虽然缓解长参数列表，也可能成为万能依赖集合。业务代码若直接保存 Lifecycle/Container，会违反边界。

## 对 012 的结论

不引入 Fx/Dig。吸收其两个设计原则：

1. 生命周期 Hook 必须有 owner，Start 正序、Stop 反序，失败停止已启动项。
2. Module 应能脱离装配框架，用普通构造函数独立测试。

012 的 typed contribution 必须集中校验且显式排序，不能复制 value group 的隐式集合。若未来对象图规模出现量化痛点，应先评估可读的静态代码生成，再单独决策 runtime DI。

## 局限

本报告不评价 Fx 的性能或完整生态，只比较依赖可见性、模块组合和生命周期语义。上游版本快照变化后需复核 API。

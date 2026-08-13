# 开发设计：Web 框架基线建设

## 1. 状态声明

本设计是已暂停的历史目标，不是已实现架构。Database 设计已经迁入 [010 design](../010-database-gorm-boundary/design.md)，当前使用方式以对应主题文档为准。

## 2. 原目标结构

```text
cmd/app
  -> Kernel Frozen Plan
     -> foundation capabilities
     -> generated plugin factories
     -> web server
  -> Host supervised runners
```

原设计要求：

- Kernel 是唯一治理与装配入口，插件内部使用普通构造函数；
- Plugin registry 在开发/CI 生成，运行时不扫描、不使用 Resolver 或 Service Locator；
- Web 在全部依赖准备完成后监听，Runner 异常必须传播到 Host；
- Password、Mail 和业务持久化通过项目自有契约注入；
- User domain/application 不导入 Kernel 或第三方基础设施类型；
- 所有停止、租约排空、错误和敏感信息处理具有明确边界。

## 3. 当前实现边界

上述 Web、Plugin、Runner、Password、Mail 和 User 设计尚未落地。当前仓库已经落地的 Database 结构、失败语义、文件影响和验证策略只在 010 中维护，不在本文件复制。

## 4. 后续处理

后续实现应按独立任务逐项设计，不一次恢复整个 008。若公共 API、依赖、生命周期、网络监听、数据模型或外部副作用发生变化，必须重新确认。

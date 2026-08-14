# 产品需求：业务模块架构

## 1. 背景

仓库已经具备底层能力契约、Kernel App 组件、显式有序 Plan、配置候选事务、稳定 Capability facade、Host 生命周期和可选 CLI，但还没有业务 Model、Service、Repository、Handler、HTTP 监听器或业务命令。继续增加业务能力前，需要先定义一条能被新开发者复用、又不破坏当前 Kernel 边界的唯一开发路径。

本任务不是把主流框架拼接进仓库，而是回答三个问题：

1. 当前代码真实提供了什么，哪些能力仍然缺失？
2. 代表性 Go 项目怎样组织业务模块、装配依赖和管理生命周期，各自成本是什么？
3. 哪个目标模型最适合本仓库，并且怎样分阶段落地、验证和迁移？

详细事实见 [current-facts-and-gaps.md](requirements/current-facts-and-gaps.md)，逐项验收见 [acceptance-matrix.md](requirements/acceptance-matrix.md)。

## 2. 目标

- 建立按业务能力纵向组织的模块边界，明确 Model、Service、Repository port、数据库 Adapter、HTTP Handler、CLI Command 和模块装配各自职责。
- 保持依赖显式、方向稳定：业务规则不依赖 Kernel、HTTP、CLI、GORM、Redis、Cobra 或其他第三方具体类型。
- 定义唯一 composition root，将 Kernel Capabilities、业务模块、HTTP 路由、CLI 命令和 Host Participant 显式连接。
- 定义配置加载、构造、启动、就绪、运行、重载、停止和失败回滚的完整顺序与资源所有权。
- 明确跨模块调用、事务、缓存、日志、I18n、错误分类与边界映射规则。
- 建立可检索、可复核、可判断新旧关系的研究档案规范，并按该规范归档本次研究。
- 给出新模块黄金路径、实施任务、依赖、工作量、验证门禁、迁移风险与未决项。

## 3. 范围

### 3.1 包含

- 进程内模块化单体的业务模块组织与显式装配。
- HTTP 和 CLI 两类入站 Adapter。
- Database、Cache、I18n、Logger、Clock、ID、Validator、Storage 等当前 Capability 的使用边界。
- 启动配置的一致快照、HTTP Server Participant、业务命令运行模式等必要的目标改造。
- 模块内同步调用与显式跨模块 port。
- 架构测试、契约测试、生命周期测试、文档门禁和首个真实垂直切片验收。

### 3.2 不包含

- 当前轮的任何非文档实现、代码生成器、服务启动、数据库迁移或外部系统写入。
- 未经真实需求证明的消息总线、Saga、CQRS、Event Sourcing、服务发现、远程模块或动态插件。
- 用假业务、空 Handler、内存假 Repository 或 TODO 冒充首个业务模块。
- 将所有模块改造成微服务，或承诺未来可无成本拆分。
- 业务对象热重建、动态增删路由或运行时依赖查找。
- 允许业务代码绕过项目契约直接创建数据库、缓存、日志或第三方客户端。

## 4. 核心需求

### 4.1 模块结构

- 顶层按业务能力命名，不建立全局 `handlers`、`services`、`models`、`repositories` 横向杂物目录。
- 领域模型不携带 HTTP、CLI、配置或 ORM 序列化标签；协议 DTO、持久化 Record 与领域模型显式转换。
- Service 负责用例编排和业务不变量；Repository 接口由使用它的业务模块定义；具体实现依赖并满足该接口。
- 模块不得导入 composition、Kernel Runtime 或其他模块的内部 Adapter。

### 4.2 装配与注册

- 只有进程 composition root 能选择模块、实现和启动 Profile。
- 构造函数只声明最小依赖；禁止 `map[string]any`、万能依赖对象、Service Locator、全局 Registry、反射扫描和 `init` 自注册。
- 路由、命令和 Participant 作为已经绑定依赖的 typed contribution 被集中验证；它们不承载依赖解析功能。
- Kernel Plan 保持底层 App 组件平面，不接纳普通业务 Service、Handler 或 Repository。

### 4.3 入站边界

- HTTP Handler 只做协议解析、校验、认证上下文提取、Service 调用、错误/响应映射和本地化，不直接访问 Repository。
- 全局技术 Middleware、模块策略 Middleware 和业务不变量必须分层；事务不能由通用 HTTP Middleware 隐式包裹。
- CLI Command 是与 HTTP 并列的入站 Adapter，调用同一 Service，不调用 HTTP Handler。
- Bootstrap 命令与依赖业务资源的 Application 命令必须有不同运行语义，不能让当前 `config init` 因业务装配而打开资源。

### 4.4 基础设施与跨模块

- Repository Adapter 只能在 Lease/回调边界内使用当前 Database Capability，不缓存或泄漏共享 Client/Tx。
- Cache 是显式 Decorator 或模块协作者，命中、未命中、失效和后端不可用语义可测试；禁止静默改变业务成功语义。
- I18n 只在 HTTP/CLI 等呈现边界把稳定错误码映射为消息；领域层不返回本地化文本。
- 跨模块依赖由调用方定义窄 port，通过构造注入；禁止共享对方 Repository、穿透内部包或用无业务语义事件绕过依赖。

### 4.5 生命周期与配置

- 初始启动的 Kernel 与业务/HTTP 配置必须来自同一不可变快照，避免同一进程两次读取产生撕裂视图。
- 所有构造和贡献冲突尽量在监听端口前失败；网络监听失败必须能在 Host Start 阶段同步报告。
- 启动顺序为 Kernel 资源先、模块 Participant 后、HTTP 最后；停止严格反向。
- 初版业务对象图、路由表和命令表在进程内不可变；影响它们的配置变更明确返回 `RestartRequired`。
- 每个 goroutine、Client、缓存清理器、Listener 和 Server 都有唯一 owner、取消、等待与关闭错误语义。

## 5. 质量要求

- 失败保留错误链，取消、超时、业务错误和资源清理错误可区分；日志不泄漏 Secret、Token、完整 DSN 或私有数据。
- 公共接口使用项目自有类型；第三方类型只留在 Adapter 内。
- 新模块可独立进行 Service 单元测试、Adapter 合约测试和入站边界测试，不要求启动全进程。
- 架构门禁能检查禁用 import、重复路由/命令、未关闭资源、文档链接和实现/设计状态混淆。
- 不新增依赖，除非实施前按维护度、许可证、安全、兼容、稳定性、测试性和替换成本重新评估并获得确认。

## 6. 成功判定

本方案只有在后续实施满足下列条件时才能转为已完成：

- 一个来自真实需求的业务模块按目标路径端到端通过 HTTP 或 Application CLI 验收；
- Kernel 与业务对象图边界、配置快照、启动/停止顺序和资源所有权有可执行证据；
- Service 与领域模型不依赖 HTTP/CLI/Kernel/第三方基础设施；
- 业务 Repository port、数据库 Adapter、事务和缓存语义经成功/失败/取消测试；
- 当前权威文档同步为真实行为，旧入口与冲突规范被迁移或删除；
- 所有确认范围内任务完成并形成单一任务提交，未夹带预存 `tmp/`。

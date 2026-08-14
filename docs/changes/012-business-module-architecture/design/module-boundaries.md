# 模块边界与依赖规则

> 当前状态：**候选约束，业务延伸门禁解除前冻结**。本文件只保留已经有外部证据支持的依赖方向，不确认目录、公开 contribution API 或首个模块形态。底层当前入口见 [../design.md](../design.md)，CLI/Config/生命周期以其列出的三份契约文档为准。

## 1. 解锁后的候选目录语义

下列目录是目标语义，不要求在没有真实业务时预创建空目录：

```text
internal/
├─ business/
│  ├─ contracts.go                 # 仅放跨模块都需要的 typed contribution 契约
│  └─ <business-capability>/
│     ├─ domain/                   # 实体、值对象与不变量；确有领域行为时才建立
│     ├─ application/              # 用例、命令/查询/结果、调用方拥有的 port
│     ├─ adapter/
│     │  ├─ database/              # 持久化 Record、转换与 Repository 实现
│     │  ├─ http/                  # DTO、Handler、路由贡献
│     │  └─ cli/                   # Command、参数与输出映射
│     └─ module.go                 # 本模块局部纯装配；不创建底层资源
└─ composition/                    # 进程唯一 composition root
```

包名必须使用真实业务语言，例如库存、结算或身份，而不是 `common`、`utils`、`core` 等无所有权名称。若用例没有独立领域模型，允许由 `application` 直接拥有清晰的数据类型；不得为了目录对称制造空层。

## 2. 各层职责

### 2.1 Domain

- 表达业务实体、值对象、不变量和纯领域错误。
- 不知道持久化格式、HTTP/CLI 协议、配置键、日志实现或 Kernel。
- 不携带 GORM、JSON、表单等 Adapter 标签。
- 时间、ID 等外部变化来源通过 application 注入的项目契约传入，不在内部调用全局函数。

### 2.2 Application

- 一个公开操作对应一个有业务语义的用例方法，而非按 CRUD 模板机械生成。
- 使用明确的 Command、Query 和 Result，区分缺失、空值和零值。
- 定义自己需要的 Repository、跨模块 port、Clock、ID 等最小接口。
- 负责授权后的用例编排、事务意图和业务失败分类；不解析 HTTP，也不输出本地化文案。
- 接口只有在形成边界价值时建立；模块内部纯局部协作者可使用具体类型。

### 2.3 Adapter

- 把外部协议或技术能力转换为 application 所需契约。
- 第三方类型限制在 Adapter 内；向内返回项目/业务自有类型。
- 在不改变错误分类的前提下补充操作上下文，并保留 `Unwrap` 错误链。
- 数据库 Adapter 拥有表 Record 与 Domain 转换；HTTP/CLI Adapter 各自拥有 DTO。

### 2.4 Module assembly

`module.go` 接收已经由 composition root 选定的最小 Capability 和跨模块依赖，构造本模块 Service 与 Adapter，并返回已经绑定的 contribution。它不得：

- 读取配置文件或环境变量；
- 创建第二套数据库、缓存、日志或 HTTP Server；
- 通过扫描或 Registry 发现依赖；
- 在构造函数中访问数据库、网络或启动 goroutine；
- 导入其他模块的内部 Adapter。

### 2.5 Composition root

composition root 是唯一可以同时知道 Kernel Capability、各应用模块和进程运行模式的位置。它选择实现、传递显式依赖、合并 contribution、执行冲突校验并建立 Host。它不能演变成业务逻辑中心或万能依赖对象。

## 3. 模型分离

同一概念允许在不同边界有不同模型，但必须显式转换：

| 模型 | 所有者 | 目的 | 禁止内容 |
|---|---|---|---|
| Domain Entity / Value | `domain` | 不变量与业务行为 | HTTP、CLI、ORM 标签 |
| Command / Query / Result | `application` | 用例输入输出 | 第三方客户端、协议上下文 |
| HTTP DTO | HTTP Adapter | 请求/响应协议 | Repository、数据库 Record |
| CLI Args / View | CLI Adapter | 参数解析与终端输出 | HTTP DTO、Cobra 类型外泄 |
| Persistence Record | Database Adapter | 表结构与扫描 | 作为领域对象跨层传播 |

转换失败属于其所在边界：协议转换返回输入错误，持久化转换返回数据损坏或内部错误，领域构造返回业务约束错误。禁止通过一个带全部标签的“万能 Model”减少转换代码。

## 4. 依赖规则

允许的关键方向：

```text
composition -> module assembly
module assembly -> adapters + application
http/cli/database/cache adapter -> application
application -> domain
application -> caller-owned narrow ports
```

禁止的方向：

- `domain` 或 `application` 导入 `internal/kernel`、`pkg/httpx`、`pkg/cli`、GORM、Redis、Cobra、Chi。
- 模块 A 导入模块 B 的 `adapter`、Repository 实现或持久化 Record。
- Handler 直接调用 Repository，或 Repository 返回 HTTP DTO。
- 任何业务包导入 `internal/composition`。
- 通过 `context.Context` 塞入数据库事务、Service Locator 或无类型依赖。

`context.Context` 只传播取消、截止时间和边界元数据；业务依赖仍由构造参数提供。

## 5. Typed contribution

模块返回的 contribution 只表达可集中安装的完成品：

- 稳定且唯一的 Module ID；
- 已绑定 Handler 的 Route；
- 已绑定执行函数的 CLI Command；
- 有 owner 的生命周期 Participant 或 Cleanup。

它不是 DI 容器输入，不包含 `Provider`、`map[string]any` 或运行时构造回调。composition root 在开放监听器前校验：

- Module ID 唯一；
- HTTP method 与规范化 path 组合唯一；
- CLI 完整命令路径和 alias 唯一；
- Participant 名称唯一且顺序可确定；
- 每个 Cleanup 的 owner 和停止阶段明确。

具体 Go 类型名和字段将在实施前按首个真实用例定稿；本文只冻结语义与不变量。

## 6. 跨模块契约

调用方模块定义它真正需要的窄 port，例如“读取结算所需的账户状态”，而不是依赖提供方完整 Service 接口。Go 的结构化接口允许提供方具体 Service 在不导入调用方的情况下满足该 port，composition root 负责连接。

若两边模型不同，由调用方外层 Adapter 转换。禁止共享另一模块 Repository、数据库表模型或内部事件以规避显式依赖。只有真实异步业务语义、交付保证和观测要求出现后，才单独设计消息契约。

## 7. 可执行门禁

后续实施至少需要以下自动检查：

- 禁用 import 检查覆盖 domain/application 到 Kernel、协议和第三方基础设施的依赖；
- composition 之外禁止应用模块互相导入内部 Adapter；
- contribution 重复项在构造验证阶段失败；
- Service 单元测试不启动 Kernel 或外部资源；
- Adapter 合约测试覆盖转换、错误链、取消和资源边界。

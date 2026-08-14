# HTTP 与 CLI 入站边界

> 当前状态：HTTP listener/Serve/Shutdown 的进程生命周期与 Bootstrap CLI 属于基础闭环；CLI mode、registry、默认配置和退出契约以 [cli-and-default-config-contracts.md](cli-and-default-config-contracts.md) 为准。Handler、Route、业务 Command 与公开错误协议在业务延伸门禁通过前均为候选约束，不得实施占位 API。

## 1. 基础闭环与业务边界的分界

当前只确认一个进程级 HTTP owner：预绑定 listener、受监督阻塞 Serve、运行错误回传、readiness、graceful Shutdown/Close/Wait。它不得依赖任何虚构业务模块。路由贡献、Handler 和 CLI Service 复用只有在真实用例确认后才细化。

## 2. 共同原则（业务门禁后）

HTTP Handler 和 CLI Command 都是 application Service 的入站 Adapter。二者可以有不同协议模型、认证方式和输出格式，但必须复用同一业务用例，不得互相调用，也不得各自复制业务规则。

```text
HTTP DTO -> HTTP Handler ─┐
                          ├-> Application Service -> ports
CLI Args -> CLI Command ──┘
```

Adapter 只依赖所需 Service 方法，不能接收整个 Module、Capabilities 或万能 `ServiceContext`。

## 3. HTTP Handler

Handler 的完整职责是：

1. 读取 path/query/header/body 并进行协议级大小与格式校验。
2. 从可信中间件结果提取认证主体、租户和 Request ID；不信任客户端伪造值。
3. 把 DTO 显式转换为 application Command/Query。
4. 调用一个清晰的 Service 用例并传递请求 `context.Context`。
5. 把稳定 Result 转换为响应 DTO。
6. 在唯一呈现边界映射错误状态、错误码和本地化消息。

Handler 不直接调用 Repository、Database/Cache Access、GORM 或跨模块内部对象；不启动 goroutine；不记录随后还会被上层重复记录的同一错误。

## 4. Middleware 分层

### 4.1 全局技术 Middleware

作用于所有路由，并由应用入口按固定顺序安装：Recovery、可信 Request ID、访问日志、安全 Header、请求体限制、CORS（真实需求存在时）等。顺序需要测试，例如 Recovery 必须覆盖后续链，Request ID 必须在 AccessLog 前建立。

当前 `pkg/httpx` 的 Request ID 在生成器缺失时静默回退，AccessLog 直接读取系统时间。目标实现需由 composition 注入 Kernel ID/Clock，缺失必需依赖时构造失败，不通过隐藏默认值改变诊断语义。

### 4.2 Route/Module policy Middleware

认证、授权、租户边界、限流等策略按 route contribution 显式声明或绑定。它们可以拒绝请求或向 context 放置经过验证的边界元数据，但不能通过 context 注入 Service、Repository 或事务。

### 4.3 业务不变量

库存、额度、状态转换等规则只属于 application/domain，不能放进 Middleware。数据库事务也不得由通用 HTTP Middleware 隐式包裹，因为其提交边界应由用例语义决定，并且同一用例还可能由 CLI 调用。

## 5. 路由贡献与冲突

每个模块提供已经绑定 Handler 的 typed Route。composition root 在启动监听器前：

- 规范化 HTTP method 和 path；
- 拒绝 method+path 重复；
- 校验参数命名和 prefix 规则；
- 固定全局与 route policy Middleware 顺序；
- 输出可诊断但不包含 Secret 的最终路由摘要。

不使用 `init` 注册、包扫描、字符串 Handler 名或运行时依赖查找。动态增删路由不在当前范围。

## 6. 响应与错误协议

响应映射由应用级 Error Presenter 统一治理，但模块拥有自己的稳定业务 reason/message ID。映射至少区分：

- 协议输入无效；
- 业务对象不存在或冲突；
- 依赖不可用与超时；
- 调用取消；
- 未知内部错误。

客户端只能收到允许公开的 reason 和安全字段；内部原因链进入一次边界日志。未知错误不得回显数据库、路径、DSN 或堆栈。具体状态码和响应 schema 要与首个真实 API 一起确认，本文不虚构公共协议。

## 7. CLI Command

CLI Adapter 负责：

- 使用项目 `pkg/cli` 契约声明命令、参数和 flag；
- 把参数转换为 application Command/Query；
- 传递取消和截止时间；
- 把 Result 映射为稳定的文本/结构化输出；
- 把业务错误映射为退出类别和本地化终端消息。

业务命令不得调用 HTTP Handler 或通过回环 HTTP 访问本进程。需要交互式 UI 时，Bubble Tea 只属于 CLI Adapter，Service 不感知终端组件。

## 8. Bootstrap 与 Application 命令

命令必须在解析前就能判断运行类别：

| 类别 | 示例语义 | Kernel | 业务模块 | HTTP |
|---|---|---:|---:|---:|
| Bootstrap | 生成/检查本地配置 | 不启动 | 不构造 | 不启动 |
| Application | 执行业务用例 | 启动所需资源 | 构造所需模块 | 不启动 |
| Service | 长期提供 API | 启动 | 构造 | 启动 |

当前 `config init` 保留 Bootstrap 语义。Application 命令只有真实需求出现时才进入详细设计；其 one-shot 完成语义必须与 Service runner 分离，在 Kernel 和必要 owner 就绪后执行，完成、取消或失败均进入统一反向停止。不能把它伪装成长期 Participant。

## 9. 验证要求

- 基础批次先验证端口预绑定、Serve runtime failure、Shutdown/Wait、readiness 与 Supervisor 不互锁。
- 业务门禁后，Handler 测试只使用 Service stub，覆盖解析、校验、取消、错误映射和 I18n。
- Middleware 顺序测试覆盖 panic、Request ID、日志字段、认证拒绝与敏感信息脱敏。
- Route validator 覆盖规范化后重复、非法 path 和顺序稳定性。
- CLI 测试覆盖参数错误、业务错误、取消、输出稳定性以及 Bootstrap 不启动资源。
- HTTP 与 CLI 对同一用例的契约测试证明业务结果一致，协议展示允许不同。

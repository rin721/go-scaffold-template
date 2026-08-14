# R005：CloudWeGo Hertz

## 研究问题

检查 Hertz Server、Spin/Shutdown 和官方示例目录，提炼 HTTP 入站边界与生命周期经验。

## 源码与示例事实

Hertz `Default` 创建 Engine 并安装 Recovery。`Spin` 启动 `Run` goroutine，等待系统信号或运行错误，再调用 `Shutdown`；优雅停止失败时使用 `Close`。该模式为单一 Hertz 应用提供便捷进程所有权。

官方 `hertz_gorm` 示例把代码分在 `biz/dal`、`handler`、`model`、`pack`、`router` 等目录；main 中先 `dal.Init()`，再创建默认 server、注册路由并 `Spin()`。生成路由能稳定地把 Handler 安装到 group。

## 可借鉴点

- Recovery 和 Middleware 在统一 Router 层安装。
- Route 生成/注册与 Handler 实现分离。
- Server 运行错误需要被进程 owner 感知。
- Shutdown 应有超时，并在必要时定义强制关闭语义。
- 协议 DTO 与数据库访问可以分开组织。

## 不适合直接复制

示例的 `dal.Init()` 和包级资源是教学便利，但会隐藏数据库 owner、错误路径和关闭顺序，违反当前项目禁止隐式初始化/全局客户端的规则。

`Spin` 自己捕获系统信号并管理 Server；当前仓库已有 Host/Supervisor，若直接调用会产生两个进程 owner。Hertz 的 package/global logger 和注册 hook 也不能替代项目 Logger/Capability。

示例顶层仍较横向，适合单个 demo，不足以定义多业务模块边界。

## 对 012 的结论

保留现有项目 `pkg/httpx` 契约和 Host，吸收统一 Middleware、路由贡献和运行错误上报。目标 HTTP Participant 必须在 Host 下预绑定 listener、受管启动 serve goroutine、等待退出并反向 Shutdown。

业务模块只贡献已绑定 Route，不创建 Server，不调用全局 DAL。是否更换 HTTP 框架不是 012 目标；当前没有证据证明 Chi/httpx 不满足需求。

## 局限

本报告检查官方源码和示例快照，不做 HTTP 性能基准，也不把示例目录当作生产架构规范。

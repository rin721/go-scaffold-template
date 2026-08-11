# 产品需求：Logger Capability 注入

## 背景与当前事实

仓库已有封装 zap 的 `pkg/logger`，业务代码可以显式构造并注入 `logger.Logger`；Kernel 当前只组合 Database Capability，尚未提供 Logger Definition、稳定 Access、配置聚合或热替换。Kernel 启动、配置加载和候选构造发生在配置化 logger 可用之前，因此不能把早期诊断依赖于一个尚未发布的 Capability。

当前 `pkg/logger.Logger` 同时暴露日志方法和 `Sync`，`zap.Config.Build` 打开的文件 sink 没有由项目契约明确关闭。若直接把该对象做成可重载资源，旧代实例可能只 flush 而未释放文件句柄，不满足 Kernel 的资源所有权要求。

## 目标与用户场景

- 应用装配者显式向 Kernel 注入一个始终可用的基线 logger，不依赖全局变量或隐式默认值。
- 业务构造函数通过稳定 Logger Access 使用当前已发布实例，不持有 Kernel Handle、Resource 或第三方 zap 类型。
- 运维人员可以修改 `logger` 配置并调用现有 `Kernel.Reload`；只有整轮配置事务成功时才看到新 logger。
- Kernel 在配置加载前、候选失败和停止配置化 logger 后仍有基线日志能力。
- 应用入口通过真实 Supervisor Participant 记录启动和停止，同时继续由最外层 stderr 边界处理最终错误和退出码。

## 功能需求

### Logger 与资源所有权

- `pkg/logger.Logger` 只暴露 `Debug`、`Info`、`Warn`、`Error` 和 `With`。
- 新增所有者专用 `Resource`，组合 `Logger` 并提供 `Sync`、`Close`；`New` 返回 `Resource`。
- 新增无 I/O 的 `ValidateConfig`，供 Capability Decode 阶段验证配置；验证不得打开文件或创建 logger。
- Resource 必须拥有构造时打开的文件 sink。`Close` 幂等，先 Sync 再关闭全部自有 sink；主错误和所有清理错误使用 `errors.Join` 保留。
- stdout 和 stderr 由进程拥有，Resource 不得关闭；保持当前已文档化的 stdout、stderr 和文件路径输出行为，不扩展第三方 sink scheme。
- 业务 Logger Access 不得暴露 `Resource.Close`，避免调用方关闭共享实例。

### Kernel 基线与配置化接管

- Kernel Options 必须显式接收非 nil logging manager；未提供时 `kernel.New` 失败，不静默使用 Noop 或创建隐藏默认 logger。
- manager 始终持有一个基线 `Logger`，并以并发安全方式委托日志调用。
- manager 的 `With` 必须返回动态视图；替换当前实例后，先前创建的子 logger 也应使用新实例并保留已绑定字段。
- Logger 候选完成 Build/Start 不代表发布。只有本轮全部受影响能力构造成功且旧租约排空后，Kernel 才激活配置化 logger。
- 初始启动失败、重载失败或回滚不得改变 manager 当前实例；成功提交后才切换，并在切换后释放旧 Logger Resource。
- Kernel 停止时，Database 等后注册能力先停止；Logger Capability 停止前必须让 manager 恢复基线，再关闭配置化 Resource。
- Kernel 只记录成功的启动、重载和停止状态。继续向调用方返回的错误不在 Kernel 内重复打印。

### Logger Capability 与配置

- Capability ID 和顶层配置路径固定为 `logger`。
- typed Config 映射现有 `pkg/logger.Config` 的 environment、level、encoding、outputPaths、errorOutputPaths、addCaller 和 addStacktrace。
- Decode 预填 `pkg/logger` 默认配置并调用 `ValidateConfig`；Build 创建新的 Resource；Stop 关闭 Resource。
- 默认配置契约沿用 `pkg/logger` 自有默认值。Encoding、AddCaller、AddStacktrace 未显式配置时继续由 Environment 推导，不把派生值固化成独立默认。
- Composition 固定按 Logger、Database 顺序登记并按相同顺序聚合默认配置；返回的 `Capabilities.Logger` 是稳定业务 Access。
- 启动前 `config init` 生成 Logger 和 Database 配置，但不构造、激活或关闭配置化 logger。

### 应用入口

- `cmd/app` 创建并拥有基线 Resource，构造 logging manager 后显式传给 Kernel。
- 服务模式增加一个排在 Kernel 之后的 Participant，通过 `Capabilities.Logger` 记录应用启动和停止；反向停止顺序保证停止日志发生在 Logger Capability 关闭前。
- CLI 模式不启动 Kernel 或应用生命周期 Participant，只生成包含 Logger 配置的默认文件。
- logger 尚未建立时的构造错误和 `Host.Run` 返回后的最终错误继续由入口写 stderr；CLI 稳定退出码和现有错误链保持不变。
- 基线 Resource 在进程函数退出前明确关闭；若关闭失败且当前仍是成功退出，则转为非零退出并向 stderr 报告一次。

## 验收标准

- 缺少 logging manager 时 `kernel.New` 明确失败；提供基线后，Kernel 在 Logger Capability 发布前后都能写日志。
- 成功启动或 Reload 后，manager 和业务 Access 都使用新配置；失败时继续使用旧实例且候选被关闭。
- 重载成功后旧 Resource 只关闭一次；Kernel 停止时先恢复基线，再关闭当前配置化 Resource。
- 旧 `With` 子 logger 在替换后使用新实例，且并发日志与替换通过 race 检查。
- `config init` 按 Logger、Database 顺序生成配置，派生选项保持 Environment 驱动。
- 应用服务模式通过配置化 logger 记录启动和停止；最终错误仍只由 stderr 边界输出一次。
- logger 文件 sink 被明确关闭；重复 Close 不重复释放，Sync 和多个 Close 错误均可识别。

## 非目标

- 不新增全局 logger、Service Locator、反射扫描、自动注册或业务依赖 DAG。
- 不启用文件配置 Watch，不增加 HTTP、RPC、后台任务或具体业务服务。
- 不接入日志轮转、远程采集、OTLP、审计存储或其他外部日志后端。
- 不把 zap 类型暴露到项目公共契约，不允许业务代码直接关闭 Kernel 托管的 Resource。

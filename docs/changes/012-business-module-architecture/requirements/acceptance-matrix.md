# 需求与验收矩阵

## 1. 场景

| 场景 | 预期结果 | 禁止结果 |
| --- | --- | --- |
| 新增业务模块 | 领域、Service、port、Adapter、HTTP/CLI 边界和 composition 位置明确 | 复制另一模块、全局 Registry、万能 Context |
| HTTP 请求 | Middleware 分层后进入 Handler，再调用 Service 和 Repository port | Handler 直接查库、领域返回 HTTP 状态码 |
| CLI 业务命令 | 启动明确需要的 Kernel/模块资源，调用同一 Service，完成后反向关闭 | 调用 HTTP Handler，或让 `config init` 打开数据库 |
| 数据库事务 | 用例层通过模块自有 UnitOfWork port 明确事务范围 | 通用 HTTP Middleware 隐式事务、Tx 逃逸 Lease |
| 缓存 | 显式 Decorator/协作者，有降级、失效和 owner 测试 | 远端失败静默返回伪造成功或旧数据 |
| 跨模块调用 | 调用方定义窄 port，composition 显式绑定 provider | 导入对方 Adapter/Repository，或绕过依赖方向 |
| 启动失败 | 监听前暴露构造/配置/冲突错误，已启动资源反向清理 | goroutine 吞掉 bind error、返回成功后立即退出 |
| 配置变化 | 受支持 Capability 原子换代；业务图/路由变化明确要求重启 | 局部热更新导致新旧业务对象混用 |

## 2. 架构验收

| ID | 验收项 | 必需证据 |
| --- | --- | --- |
| `AC-ARCH-001` | 业务包不导入 Kernel、composition、HTTP/CLI 或第三方基础设施 | import 边界测试与源码搜索 |
| `AC-ARCH-002` | Repository port 由业务使用方定义，Adapter 单向依赖该 port | 编译期接口断言、包依赖图、单元测试 |
| `AC-ARCH-003` | 唯一 composition root 显式选择全部模块和实现 | 组合测试、无 init/扫描/Resolve 搜索 |
| `AC-ARCH-004` | Kernel Plan 只含底层 App 组件 | FrozenPlan 快照与文档检查 |
| `AC-ARCH-005` | 路由、命令和 Participant contribution 可在运行前校验冲突 | 重复 ID/path/verb/command 失败测试 |

## 3. 生命周期验收

| ID | 验收项 | 必需证据 |
| --- | --- | --- |
| `AC-LIFE-001` | Kernel 与应用配置来自同一初始 Snapshot | 配置源变动并发测试或不可变 token 测试 |
| `AC-LIFE-002` | Kernel -> 模块 Participant -> HTTP 顺序启动，严格反向停止 | 事件序列测试 |
| `AC-LIFE-003` | 监听失败在 Start 返回；正常 Shutdown 等待请求与 goroutine | 端口占用、取消、超时测试 |
| `AC-LIFE-004` | Cache Client、Listener、Server 等 owner 唯一且 Close 幂等 | owner 表、泄漏/重复关闭测试 |
| `AC-LIFE-005` | 业务图相关配置变更返回 RestartRequired，不部分提交 | reload 事务测试 |

## 4. 业务边界验收

| ID | 验收项 | 必需证据 |
| --- | --- | --- |
| `AC-BIZ-001` | 领域模型与 HTTP DTO、CLI 输入、持久化 Record 分离 | 类型与转换测试 |
| `AC-BIZ-002` | Service 在纯单元测试中只依赖窄 fake/stub | 不启动 Kernel/HTTP/DB 的测试 |
| `AC-BIZ-003` | Adapter 保留错误链并映射基础设施错误 | errors.Is/As、取消、超时测试 |
| `AC-BIZ-004` | 跨模块只走显式业务 port | import 与 composition 绑定测试 |
| `AC-BIZ-005` | 首个垂直切片源于真实业务，不使用占位实体 | 已确认需求、端到端验收记录 |

## 5. 入站与质量验收

| ID | 验收项 | 必需证据 |
| --- | --- | --- |
| `AC-IN-001` | HTTP 技术/策略 Middleware 顺序固定且可测试 | Router/Handler 测试 |
| `AC-IN-002` | Handler/Command 共享 Service，但 DTO 与呈现逻辑独立 | 双入口用例测试 |
| `AC-IN-003` | 错误只在决定策略的边界记录一次，I18n 只在呈现边界 | 日志捕获与语言测试 |
| `AC-IN-004` | RequestID、Logger、Validator、I18n 均显式注入 | nil/fallback 禁止测试 |
| `AC-QUAL-001` | race、vet、build/test、no-CGO、文档链接、diff-check 全部通过 | 命令输出与任务账本证据 |
| `AC-QUAL-002` | 当前权威文档只描述已实现行为 | 文档状态审阅与旧符号搜索 |

## 6. 当前轮判定

本轮只验收文档：结构化研究档案存在、需求/设计/任务互相可追踪、相对链接有效、Markdown 无空白错误、改动范围只含 012 与必要导航。上述实现验收项全部保持“待确认/未执行”，不得描述为已通过。

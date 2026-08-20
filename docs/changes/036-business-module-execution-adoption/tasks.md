# 036 任务清单：业务模块接入 execution（Todo 落地）

任务 ID 稳定；状态：研究门禁通过；非文档实施待确认。

| ID | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- |
| DOC-001 | — | 新增 `docs/development/execution-capability.md` 权威接入文档并在模块开发指南建立入口 | 文档与实现一致；从开发指南可进入 | 已完成（纯文档） |
| TODO-001 | DOC-001 | `service` 新增执行窄 port `Executor`；注入到 `Service` | 契约编译；窄 port 收敛；不 import internal | 已完成 |
| TODO-002 | TODO-001 | `Complete` 把关键写操作（`repository.Save`）包装经 `Executor` 执行（幂等键 `todo:complete:<id>`，`PolicyName="todo"`） | 重复只执行一次；重试按策略；错误链保留 | 已完成 |
| ADAPTER-001 | TODO-002 | composition 用 `capabilities.Execution` 构造窄 port Adapter 并经 `todo.Dependencies.Executor` 注入（一次性 CLI + 长期 HTTP Service 两路径） | 装配成功；无反向/循环依赖 | 已完成 |
| CFG-001 | TODO-002 | 把 `execution.policies.todo` 写入配置并在共享配置契约注册 execution 节（`executionapp.Configuration()` + `ConfigurationBindings`） | 配置集中声明 + 校验；`db migrate` 等命令可识别；`PolicyName` 可解析 | 已完成 |
| TEST-001 | TODO-002, ADAPTER-001 | Service 窄 port 单测（重复只执行一次、可重试重试、executor 失败错误链）+ 装配测试 + `executor.Timeout` 回归测试 | 相关 `-race` 通过 | 已完成 |
| VER-001 | 全部 | 全量验证：build/vet/gofmt/test/tidy/diff-check | 全通过；提交 | 已完成 |

## 确认状态

- 研究门禁：已通过（R001）。
- 纯文档交付（DOC-001）：已完成。
- 非文档 Todo 接入：**已确认并完成**。范围：只接 `Complete`；`execution.policies.todo` 写入配置（方案 A，含共享配置契约注册 execution 节）；CLI 与 HTTP Service 双路径均接入。

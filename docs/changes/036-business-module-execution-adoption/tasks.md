# 036 任务清单：业务模块接入 execution（Todo 落地）

任务 ID 稳定；状态：研究门禁通过；非文档实施待确认。

| ID | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- |
| DOC-001 | — | 新增 `docs/development/execution-capability.md` 权威接入文档并在模块开发指南建立入口 | 文档与实现一致；从开发指南可进入 | 已完成（纯文档） |
| TODO-001 | DOC-001 | `service` 新增执行窄 port `Executor`；注入到 `Service` | 契约编译；窄 port 收敛；不 import internal | 待确认 |
| TODO-002 | TODO-001 | `Complete`（可选 Create）把关键写操作包装经 `Executor` 执行（幂等键 + `PolicyName="todo"`） | 重复只执行一次；重试按策略；错误链保留 | 待确认 |
| ADAPTER-001 | TODO-002 | composition 用 `capabilities.Execution` 构造窄 port Adapter 并经 `todo.Dependencies.Executor` 注入 | 装配成功；无反向/循环依赖 | 待确认 |
| CFG-001 | TODO-002 | 声明 `execution.policies.todo` 命名策略（应用默认或配置示例） | 配置集中声明 + 校验；`PolicyName` 可解析 | 待确认 |
| TEST-001 | TODO-002, ADAPTER-001 | Service 窄 port 单测（重复只执行一次、可重试重试、executor 失败错误链）+ 装配测试 | 相关 `-race` 通过 | 待确认 |
| VER-001 | 全部 | 全量验证：build/vet/gofmt/test/tidy/diff-check | 全通过；提交（按确认范围） | 待确认 |

## 确认状态

- 研究门禁：已通过（R001）。
- 纯文档交付（DOC-001）：直接完成。
- 非文档 Todo 接入（TODO-001…VER-001）：**待确认**。落实用例（Complete/Create）、是否改应用默认配置需用户确认后实施。

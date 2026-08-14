# 任务：HTTP API 成熟度缺口评估

## 1. 门禁状态

- 018 废除：已完成；只保留历史研究，不得实施。
- 019 研究门禁：已通过。
- 019 文档计划：已完成，属于纯文档交付。
- 非文档实施：未授权。下表 Future tasks 只是后续独立变更候选，不能从本轮请求推导实施授权。

## 2. 本轮任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RETIRE-018` | S | 用户决定 | 将 018 入口、权威文档、任务和研究标记为废除/历史 | 不再存在“待确认后实施 018”的有效入口 | 已完成 |
| `RES-001` | XL | 无 | 核验当前 HTTP 运行链、协议、数据、运营、CI 与脚手架形态 | R001 区分已实现、局部、已识别未设计和未设计 | 已完成 |
| `RES-002` | L | 无 | 用官方标准建立最小成熟度参考模型 | R002 给出适用边界，不绑定具体产品 | 已完成 |
| `PLAN-001` | XL | `RES-001`、`RES-002` | 建立缺口优先级、目标分层、需求和未来任务 | requirements/design/tasks 可复核且不声称已实现 | 已完成 |

## 3. 后续独立变更候选

每一项都必须新建 `docs/changes/<seq>/`、重新研究并在计划报告后的后续消息中确认。

| 候选 ID | 优先级 | 前置 | 目标 | 当前状态 |
| --- | --- | --- | --- | --- |
| `FORM-001` | P0 | 无 | ADR 决定 template/library/generator/组合产品形态与升级模型 | 未纳入实施 |
| `API-AUTHORITY-001` | P0 | `FORM-001` | 原型比较 spec-first 与 typed code-first，冻结单一 API authority | 未纳入实施 |
| `API-CONTRACT-001` | P0 | `API-AUTHORITY-001` | Operation、schema、route、OpenAPI、compatibility diff 单轨闭环 | 未纳入实施 |
| `PROBLEM-001` | P0 | `API-CONTRACT-001` | 统一 problem/validation/404/405/panic 协议并迁移 Todo | 未纳入实施 |
| `EDGE-POLICY-001` | P0 | `API-CONTRACT-001` | 请求预算、limits、CORS、trusted proxy、rate/overload 政策 | 未纳入实施 |
| `IDENTITY-POLICY-001` | P0 | 真实 actor + `API-CONTRACT-001` | Principal、认证 Adapter、资源授权、审计与测试 | 未纳入实施 |
| `MANAGEMENT-001` | P0 | 无 | management listener、startup/live/ready、diagnostics/build info | 未纳入实施 |
| `OBSERVABILITY-001` | P0 | `API-CONTRACT-001`、`MANAGEMENT-001` | OTel trace/metric/log correlation 与 cardinality policy | 未纳入实施 |
| `MIGRATION-001` | P0 | 生产部署场景 | versioned migration、lock、expand-contract 和独立命令/job | 未纳入实施 |
| `OUTBOUND-001` | P1 | 首个真实外部 API | 命名 client、budget、retry/breaker、telemetry、资源 owner | 未纳入实施 |
| `QUALITY-001` | P1 | `API-CONTRACT-001` | contract/fuzz/performance/leak/vulnerability 门禁 | 未纳入实施 |
| `DELIVERY-001` | P1 | `FORM-001`、`MANAGEMENT-001` | build metadata、容器、部署样例、SBOM、发布/回滚 | 未纳入实施 |
| `SCAFFOLD-DX-001` | P1 | `FORM-001`、主要 P0 | 创建/重命名/模块生成、golden、升级和移除示例 | 未纳入实施 |
| `ASYNC-001` | P2 | 真实异步用例 | 消息/任务/调度的交付、幂等、背压、排空 | 未纳入实施 |
| `TENANT-001` | P2 | 真实租户模型 | 身份、隔离、迁移、限额与审计 | 未纳入实施 |

## 4. 推荐下一步

第一批只启动两个研究型变更，不改生产运行链：

1. `FORM-001`：用一个仓库外消费样例验证脚手架应如何被创建和升级；
2. `API-AUTHORITY-001`：用 Todo 四个 operation 做 spec-first 与 typed code-first 最小原型比较。

两个决定会影响后续错误、认证、OpenAPI、观测、生成器和兼容测试。它们完成前不应先新增 Swagger 注解、JWT middleware 或另一套路由 Registry。

## 5. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RETIRE-018`、`RES-001`、`RES-002`、`PLAN-001` | HEAD `28fbc7a`；`go test ./...` 通过；HTTP/OpenAPI/OTel/Kubernetes/OWASP/Go 官方资料；文档链接/YAML/Diff 检查 | 本次纯文档提交 | 只有一个 Todo 模块；无真实 identity/deployment；未来公共协议均需独立确认 |

## 6. 停止条件

019 到差距评估和路线即完成。任何源码、配置、依赖、CI、生成器、容器、服务启动或外部写入，都必须建立新的变更目录并获得对应计划的后续明确确认。

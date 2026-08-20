# R001 文档审计矩阵

## 矩阵

| 范围 | 覆盖文件 | 发现 | 处理动作 |
| --- | --- | --- | --- |
| 根入口 | `README.md` | 已承担项目定位、五分钟启动和手册入口；符合 040 根入口职责。 | 小幅强化连续阅读路径和权威边界，不扩展为长手册。 |
| 项目手册 | `docs/README.md` | 已有生命周期分类，但仍像目录索引，缺少“使用能力 -> 开发业务 -> 接入基础设施 -> 排障运维”的连续路径说明。 | 改写为连续项目手册，按真实使用顺序展开。 |
| 配置 | `docs/configuration/README.md` | 配置 owner 表与代码事实基本一致，已覆盖 execution、scheduler、messaging、observability。 | 保留为配置 authority，由手册与运维入口链接。 |
| 架构 | `docs/architecture/README.md` | 内容较短，能链接 Kernel/module/pkg，但没有明确 composition、Application Generation、生命周期治理的阅读层级。 | 扩展为架构主题入口，收口局部 README 与历史边界。 |
| 开发 | `docs/development/README.md`、`docs/development/*.md` | 已覆盖模块、日志、execution、schedule、messaging；README 缺少 Binding/API contract 的显式顺序。 | 扩展开发闭环，明确 Binding 契约、API contract 和能力接入顺序。 |
| 运维 | `docs/operations/README.md`、`docs/operations/*.md` | README 只是列表，未收口排障、运行维护、未验证门禁。 | 改为运行路径入口，串联构建、迁移、发布、复制、安全、调度、消息和排障。 |
| API | `api/README.md` | 已明确 Go code-first 契约和 `contract-gen`，符合当前实现。 | 作为开发和手册 API contract authority 链接，不复制到其他文档。 |
| pkg 总览 | `pkg/README.md` | 能力清单含 `execution`，但没有链接 `pkg/execution/README.md`；包级 README authority 边界不够明确。 | 补齐 `execution` 局部 README 链接，增加包级 README 职责边界。 |
| pkg 局部 README | `pkg/**/README.md` | 多数保留局部用法；`pkg/execution` 缺 README；短 README 依赖主题文档。 | 新增 `pkg/execution/README.md`；短 README 保留局部边界并由主题文档承接。 |
| internal module | `internal/module/README.md`、`internal/module/**/README.md` | 模块边界说明较完整；承担部分项目级模块规范，但本身就是 architecture/development 链接的局部 authority。 | 保留局部实现细节，由 development/architecture 指向，不在其他 README 复制。 |
| internal kernel | `internal/kernel/README.md`、`internal/kernel/app/README.md` | 存在旧阶段说明：当前 Service 没有业务层；与 Todo/HTTP 当前实现冲突。 | 改为当前事实：业务层由 `internal/module` 与 Application Generation 装配，Kernel App 不替业务对象定义容器职责。 |
| 历史研究 | `docs/research/**` | 目标设计、尚未实现、未来等表述属于研究快照。 | 不改写历史，只在正式文档中明确研究不是当前规范。 |
| 任务记录 | `docs/changes/**` | 存在待确认、已废除、旧方案等历史状态。 | 不大规模移动或改写，更新 `docs/changes/README.md` 加入 040。 |

## 040 必修发现

| 问题类型 | 证据 | 处理 |
| --- | --- | --- |
| 代码已有能力但缺局部正式文档 | `pkg/execution/*.go` 存在，`pkg/execution/README.md` 不存在。 | 新增 `pkg/execution/README.md`，并从 `pkg/README.md` 与 `docs/README.md` 链接。 |
| 正式局部文档保留过期阶段边界 | `internal/kernel/README.md` 与 `internal/kernel/app/README.md` 描述当前 Service 没有业务 middleware/handler/service/repository/model。 | 改成当前事实：业务模块已存在，Kernel App 不负责业务对象图。 |
| 主题入口没有完整收口 | `docs/architecture/README.md`、`docs/development/README.md`、`docs/operations/README.md` 内容偏索引。 | 增加主题职责、连续阅读顺序、边界和维护约束。 |
| 包级 README 可能被误读为项目级入口 | `pkg/README.md` 与多个 `pkg/**/README.md` 包含能力说明。 | 明确 `pkg` 是局部能力索引，项目级开发/架构/运维回到 `docs/**` authority。 |

## 不处理项

- 历史 `docs/changes/**` 中的“待确认、已废除、旧方案、go-scaffold2”等词保留为历史证据。
- `docs/research/**` 中的目标设计和未实现说明保留为研究快照，不迁入当前操作手册。
- 不重新生成 OpenAPI，不修改 `api/openapi.yaml`。

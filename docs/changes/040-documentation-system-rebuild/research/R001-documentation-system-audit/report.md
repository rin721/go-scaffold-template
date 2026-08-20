# R001 文档体系审计与当前代码事实核对

## 研究问题

本研究回答：当前项目文档是否已经能以真实代码和实际能力为准，形成连续、单轨、可维护的项目手册；哪些文档需要在 040 中收敛、补齐或改回局部说明。

## 方法与范围

- 检查 Git 状态，确认实施前工作区干净，当前分支 `main...origin/main` ahead 3。
- 统计并审计根 README、`docs/**`、`api/README.md`、`internal/**/README.md`、`pkg/**/README.md`。全仓 Markdown 去重后为 353 个文件。
- 读取正式入口：根 README、`docs/README.md`、configuration、architecture、development、operations、api。
- 抽样读取局部 README：`internal/kernel`、`internal/kernel/app`、`internal/module`、`pkg/README.md` 与短缺的 `pkg/messaging`、`pkg/schedule`、`pkg/coordination`、`pkg/observability`。
- 静态核对代码事实：`cmd/app`、`internal/composition/application.go`、`configuration.go`、`http_contracts.go`、`service.go`、`internal/module/contracts.go`、`pkg/execution/*.go`、`go.mod`。

## 事实

- 当前 module path 是 `github.com/rin721/go-scaffold-template`，根 README 已使用新项目名。
- `cmd/app` 只进入 `composition.Run`；`Application.Run` 按参数数量选择 one-shot CLI 或长期 Service。
- 统一配置 owner 由 `internal/composition/configuration.go` 的 `applicationOwnedConfigurationBindings()` 集中返回，包含 auth、migration、todo、management 与 observability；Kernel composition 再补齐 logger、database、cache、i18n、storage、HTTP、execution、scheduler、messaging 等配置节。
- 长期 Service 由 `internal/composition/service.go` 创建 `GenerationCoordinator`、Application Generation factory、config watcher 与 Supervisor。
- HTTP 契约聚合点是 `internal/composition/http_contracts.go` 的 `applicationHTTPModules()`；当前公开业务 HTTP 模块为 Todo。
- 业务模块贡献通过 `internal/module/contracts.go` 的 `Contribution`、`ValidateContributions`、`ScheduleBindings` 和 `MessageBindings` 校验并聚合。
- `pkg/execution` 已存在完整 Go 实现，但没有 `pkg/execution/README.md`，而 `docs/README.md` 的底层能力清单也未链接该局部入口。
- `internal/kernel/README.md` 与 `internal/kernel/app/README.md` 仍存在当前 Service “没有业务 middleware、handler、service、repository 或 model”的阶段性表述；这与当前 Todo 垂直切片、HTTP route contribution 和 Application Generation 现实不一致。
- 039 已建立根 README -> 项目手册 -> 主题入口的初步闭环，但 `docs/README.md` 仍偏分类索引，operations 入口偏列表，缺少 040 要求的连续使用路径和维护约束。
- 历史 `docs/changes/**` 与 `docs/research/**` 中保留大量“目标设计、尚未、待确认、旧方案、已废除”等词是合理历史证据；问题不在于历史存在，而在于正式文档不能把这些内容当作当前操作说明。

## 推断

- 040 应优先重构正式入口和局部 README authority，而不是大规模移动历史文件。
- `docs/README.md` 应从生命周期目录进一步升级为连续阅读路径，并明确每个主题的完成边界。
- `docs/architecture/README.md`、`docs/development/README.md`、`docs/operations/README.md` 应承担主题收口，而包级 README 只保留局部实现、公开类型和主题链接。
- `pkg/execution/README.md` 是必须补齐的代码能力文档缺口。
- Kernel 局部 README 中的过期阶段边界必须改成当前事实，否则会误导读者以为业务层尚未建设。

## 对 040 的影响

- 建立 [审计矩阵](audit-matrix.md)，把发现、风险和处理动作显式记录。
- 修改正式入口文档，强化连续路径和 authority 边界。
- 补齐 `pkg/execution/README.md` 并在 `pkg/README.md`、`docs/README.md` 中可达。
- 修正 Kernel 局部 README 与当前 Application Generation/Todo 现实不一致的表述。
- 只改 Markdown，不改源码、配置、脚本或生成物。

## 局限与刷新触发器

本研究不验证运行时行为，不运行 Go 测试、构建或服务启动。若后续新增/删除能力、修改 composition root、改变配置 owner、重塑 Application Generation 或 module Contribution，需要刷新本审计。

# R001 当前文档拓扑与闭环缺口

## 研究问题

当前项目文档为什么呈现为分散片段，以及怎样形成连续项目手册？

## 方法与范围

本研究只检查当前仓库文档和目录结构，不评价源码实现是否正确，也不改变配置、CLI、HTTP 契约或运行行为。检查对象包括根 `README.md`、`docs/README.md`、`docs/changes/README.md`、`docs/research/README.md`、`docs/development/*.md`、`docs/operations/README.md`、`internal/kernel/README.md`、`internal/kernel/app/README.md`、`internal/module/README.md` 与 `pkg/README.md`。

## 当前事实

- 根 README 已包含五分钟启动、文档地图、架构摘要和 license，但文档地图把启动、配置、API、开发、Kernel、pkg、运维、变更记录和 AGENTS 放在同一层。
- `docs/README.md` 已区分当前使用说明、开发与架构主题、研究与历史，但没有形成从启动到交付的连续阅读顺序。
- `docs/development/` 已有应用模块、日志、execution、schedule 和 messaging 主题文档，但缺少目录入口。
- `docs/architecture/` 尚不存在；架构相关说明散落在 `internal/kernel/README.md`、`internal/kernel/app/README.md`、`internal/module/README.md`、`pkg/README.md` 和部分开发文档中。
- `docs/operations/README.md` 已是运维入口，但没有明确它在项目手册中的位置。
- `docs/changes/README.md` 已维护 001 到 038 的任务级历史索引，并声明下一个任务序号为 `039`。
- `docs/research/README.md` 已声明研究档案是阶段性快照，不替代根 README 或主题文档。

## 推断

- 主要问题不是缺少所有内容，而是缺少信息架构：入口没有把主题按项目生命周期串起来。
- 继续设置身份分流入口会让读者先判断入口归属，不能解决当前“这一块那一块”的体验。
- 更合适的整理方式是建立项目手册总目录，按启动、配置、架构、开发、能力、API、验证、运维、历史证据组织。
- 历史 `docs/changes/**` 目录不应大规模移动；只需要在总目录和变更索引中明确它们是证据账本，不是当前使用权威。

## 适用与不适用场景

本结论适用于当前文档体系整理、导航补强和 authority 边界澄清。不适用于源码架构调整、配置格式变更、CLI 行为变更、外部技术选型或发布流程改造。

## 局限与剩余未知

本研究没有逐篇审计所有主题文档内容是否仍与代码完全一致。若后续发现某个主题文档内容过期，应建立独立变更做代码事实复核和文档同步。

## 对当前任务的影响

本任务应新增少量导航和任务证据文档，重写根 README 与 `docs/README.md` 的入口结构，保持历史目录稳定，并通过链接检查和 `git diff --check` 验证文档改动。

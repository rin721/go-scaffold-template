# 039 文档体系闭环整理需求

## 目标

把当前项目文档整理为连续、闭环的项目手册。阅读顺序应自然覆盖启动、配置、架构、开发、能力接入、API、验证、运维、研究与变更历史，而不是让文档以零散链接或预设身份入口呈现。

## 范围

- 重写根 README 的文档入口和权威边界说明。
- 重写 `docs/README.md` 为项目手册总目录。
- 新增 `docs/development/README.md`，收口开发主题。
- 新增 `docs/architecture/README.md`，收口架构主题。
- 更新 `docs/operations/README.md`，让运维入口回到项目手册闭环。
- 新建本任务的 `docs/changes/039-documentation-system-closure/` 研究、需求、设计和任务账本。
- 更新 `docs/changes/README.md` 的任务索引与下一个序号。

## 非目标

- 不移动或重命名既有历史 `docs/changes/**` 目录。
- 不修改源码、配置、脚本、生成物、CLI、HTTP 契约或运行行为。
- 不新增按身份划分的阅读入口。
- 不把历史研究或任务设计提升为当前权威文档。

## 约束

- 文档、注释和交付说明以中文为主，技术标识符和命令保持英文。
- 根 README、`docs/README.md`、主题入口与历史证据之间必须形成可点击链路。
- 当前权威、研究快照和任务证据必须明确分层。
- 新增导航页只做主题收口，不复制大段既有正文。

## 验收标准

- 根 README 能在短路径中说明项目定位、启动命令、项目手册入口和文档权威边界。
- `docs/README.md` 能按项目生命周期连续组织现有文档。
- development、architecture、operations 三类主题入口互相回到项目手册闭环。
- `docs/changes/039-documentation-system-closure/` 包含 README、research、requirements、design 和 tasks。
- Markdown 本地链接检查通过。
- `git diff --check` 通过。
- 最终说明不声称 Go 测试、构建或服务启动已经通过。

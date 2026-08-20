# 039 文档体系闭环整理

状态：纯文档任务；研究门禁已通过，计划已确认，文档实施已完成。

## 范围

本变更把项目文档从分散入口整理为连续项目手册：根 README 保留项目定位与最短启动，`docs/README.md`
承接完整生命周期目录，development、architecture、operations 分别形成主题入口，`docs/changes/**` 和
`docs/research/**` 明确退回历史证据位置。

本任务不修改源码、配置、脚本、CLI、HTTP 契约或运行行为。

## 阅读顺序

1. [研究索引](research/README.md)：当前文档入口、缺口和整理结论。
2. [需求规格](requirements.md)：目标、范围、非目标和验收标准。
3. [设计方案](design.md)：信息架构、authority 边界和文件影响。
4. [任务清单](tasks.md)：稳定任务 ID、完成条件和验证证据。

## 当前实现结论

- 根 [README](../../../README.md) 只保留项目定位、五分钟启动、项目手册入口、架构摘要和文档权威边界。
- [项目手册](../../README.md) 按项目生命周期组织当前主题文档，不设置身份分流入口。
- [开发指南](../../development/README.md) 收口应用模块、日志、execution、schedule 和 messaging 开发主题。
- [架构说明](../../architecture/README.md) 收口 Kernel、Application Generation、composition、模块边界和 pkg 能力链路。
- `docs/changes/**` 与 `docs/research/**` 仅作为历史证据和研究快照，不替代当前主题文档。

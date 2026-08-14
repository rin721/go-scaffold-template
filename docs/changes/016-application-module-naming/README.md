# 016 应用模块命名迁移

## 状态

- 任务性质：非纯文档目录、包名与稳定 owner ID 重命名。
- 当前状态：**已完成实现与本地验证**。
- 研究与计划日期：2026-08-15。
- 代码基线：`7bbe4bc7c6979324844cc1f7c43384d69738936b`。
- 当前授权：用户在计划报告后的后续消息中明确要求“执行 `应用模块命名方案`”，授权实施 `PATH-001/SEM-001/GOV-001/DOC-001/VER-001`。

## 一句话结论

`internal/business` 能区分业务与 Kernel，但它只表达“不是基础设施”，没有表达当前一级子包是可独立装配、集中贡献入口的应用模块。按用户选择，现已单轨改为 `internal/module`，Todo 位于 `internal/module/todo`；Kernel 运行单元继续称 Component，底层依赖继续称 Capability，`module` 专指应用组合根选择的纵向模块。

## 实施结果

- `internal/business` 已整体迁移为 `internal/module`，没有保留 alias、转发包或第二套校验入口。
- 根契约包已改为 `module`，身份类型从 `ModuleID` 收敛为 `module.ID`。
- Todo 配置 owner 与 migration participant 已分别改为 `module.todo`、`module.todo.schema`。
- composition、package graph 门禁、当前 README 与 012/014/015 的可执行路径已经同步；历史任务目录名保持稳定。
- Todo 配置键、HTTP/CLI 契约、数据库 Schema、Kernel Plan 和第三方依赖没有改变。

## 阅读顺序

1. [研究档案](research/README.md)：当前职责、引用面和候选命名比较。
2. [requirements.md](requirements.md)：命名契约、范围、兼容影响和验收标准。
3. [design.md](design.md)：单轨迁移、稳定 ID、文档与验证策略。
4. [tasks.md](tasks.md)：稳定任务 ID、确认门禁与实施顺序。

## 确认边界

本次实施只覆盖 `PATH-001/SEM-001/GOV-001/DOC-001/VER-001`。Todo 配置路径、HTTP/CLI 契约、数据库 Schema、模块内部层次和 Kernel composition 均保持原有行为。

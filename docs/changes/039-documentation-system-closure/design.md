# 039 文档体系闭环整理设计

## 信息架构

文档采用项目生命周期顺序，而不是身份分流顺序：

```text
README.md
  -> docs/README.md
      -> 本地启动
      -> 配置与运行
      -> 架构与生命周期
      -> 应用开发
      -> 底层能力
      -> 验证与运维
      -> 研究与变更历史
```

根 README 是最短入口，只给出定位、启动命令和手册入口。`docs/README.md` 是完整项目手册总目录，负责把主题文档串成连续路径。

## 主题入口

- `docs/development/README.md` 收口模块开发、日志、execution、schedule、messaging，并链接架构、pkg、API 与运维。
- `docs/architecture/README.md` 收口 Kernel、Kernel App、Application Generation、composition、模块边界和 `pkg` 能力链路。
- `docs/operations/README.md` 保留原有运维主题，并声明它承接项目手册的验证与运维部分。

这些入口只做导航和边界说明，不复制已有主题文档的完整实现细节。

## Authority 边界

- 当前启动、配置、开发和运维，以根 README 与 `docs/README.md` 下的主题文档为准。
- `docs/changes/**` 保存任务级需求、设计、任务和验证证据，不替代当前主题文档。
- `docs/research/**` 保存阶段性研究快照，不把目标设计写成已实现能力。
- `pkg/**/README.md` 与 `internal/**/README.md` 是局部包说明，由主题文档链接进入，不作为全局入口。

## 文件影响

- 修改：`README.md`、`docs/README.md`、`docs/operations/README.md`、`docs/changes/README.md`。
- 新增：`docs/development/README.md`、`docs/architecture/README.md`。
- 新增任务账本：`docs/changes/039-documentation-system-closure/**`。

不修改任何 Go 源码、配置样例、脚本或生成物。

## 验证方案

- 使用本地 Markdown 链接检查确认新增和修改入口可达。
- 执行 `git diff --check`。
- 审阅 diff，确认没有混入源码、配置或运行行为变更。

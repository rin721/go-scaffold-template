# 032 研究档案

本目录保存 032 任务的研究记录。研究先于计划；任何结论必须能被源码、配置、测试与已确认决策中的证据支持，事实与推断分开标记。

## 索引

- [R001 当前 i18n 组件默认值与路径声明复核](R001-current-i18n-defaults-and-path/report.md)：复核 `kernel/app/i18n` 目前如何取默认配置、消息文件路径如何声明，以及 Todo/业务侧如何消费 Translator。
- [R002 pkg 通用库与 kernel/app 组件配置职责边界审计](R002-pkg-kernel-app-config-boundary/report.md)：审计全部 `kernel/app/*` 组件基于 `pkg/*` 封装时是否存在「直接复用通用库默认配置 / 把应由应用层声明的值交给通用库默认值」的问题。

## 记录要求

每份研究记录固定包含 `metadata.yaml` 与 `report.md`；检索从 metadata 进入，证据以 report 为准。

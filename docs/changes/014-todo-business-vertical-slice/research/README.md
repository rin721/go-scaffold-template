# 014 研究档案

本目录保存 Todo 作为首个真实业务垂直切片的任务级研究。研究只回答当前仓库是否具备实施条件、还缺少哪些契约，以及哪些既有能力必须复用；它不代表业务源码已经实现。

## 检索与复用

- 已检索 `docs/**/research/**/metadata.yaml`，命中 012 的业务模块架构、HTTP 生命周期、CLI/Config 契约和基础实施快照。
- 012 的 `R021` 在 2026-08-14 验证了当前 Kernel、HTTP、SQLite、CLI 和治理底座，但其刷新触发器包含“首个真实业务用例确认”，因此本任务重新核验当前 HEAD，不能只复用旧摘要。
- 当前技术栈已经固定使用 `net/http + chi`、项目 `pkg/httpx`、项目 `pkg/database + GORM`、Cobra Adapter 和显式 Kernel composition；本任务没有新的第三方选型缺口，因此不重复开展泛化框架调研。

## 记录

- [R001 当前 Todo 垂直切片可行性](R001-current-todo-vertical-slice-feasibility/report.md)：核验当前入口、配置、数据库、HTTP、CLI、生命周期和架构门禁，并给出 014 的实施影响。

## 当前结论

`R001` 已满足研究门禁：Todo 可以在不新增第三方依赖、不引入第二个 DI/生命周期容器的前提下完成真实 HTTP、SQLite、配置和 CLI 闭环。研究当时识别出的 business contribution、Schema migration owner、application one-shot 生命周期和业务边界测试缺口，已由本任务的已确认实现补齐；R001 报告仍保留为实施前快照证据。

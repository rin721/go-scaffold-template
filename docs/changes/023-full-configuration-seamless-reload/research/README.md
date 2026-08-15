# 023 研究索引

## 研究范围

本任务研究当前 `logger/database/cache/i18n/storage/http/todo` 七个配置节如何在同一长期 Service 进程中安全、原子且不中断请求地生效。研究同时覆盖文件稳定读取、完整应用代际、HTTP listener 交接、资源复用、旧代排空和外部数据连续性边界。

## 检索与复用

开始前已检索 `docs/**/research/**/metadata.yaml`。复用：

- `docs/research/002-kernel-app-manual-composition` 的 Reload 策略与未实现 Handoff 结论；
- 022-R003 的 Go/Caddy 运行模式；
- 022-R005 的逐资源终结与 HTTP Shutdown 边界；
- 022-R008 的 restart latch 与 current-profile closure。

上述记录没有回答跨 Kernel/application/HTTP 的完整 generation，因此新增两份 023 当前档案。

## 记录

1. [R001 当前全配置重载边界与失败链路](R001-current-reload-boundaries/report.md)：代码级追踪七个配置节、事务边界、Cache L1、Todo Policy、HTTP Server 和 Windows 文件读取竞态。
2. [R002 不可变应用代际与 Listener Handoff 模式](R002-generation-listener-patterns/report.md)：核对 Go/Caddy 官方主源，比较四条路径并推荐 Application Generation + TCP ListenerHub。

## 门禁结论

研究门禁已通过：当前限制、目标保证、可复用能力、必须替换的旧路径和跨平台验证条件已有可复核证据。研究通过只授权形成计划，不授权修改 Go、配置、测试、依赖或运行状态。

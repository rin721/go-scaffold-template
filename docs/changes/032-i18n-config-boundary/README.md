# 032 i18n 配置职责边界与集中声明

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，证据为 `R001`、`R002`。
- 计划状态：已确认并实施完成。
- 方向确认：用户已确认方向（不调整）。明确的例外——某个 `pkg/*` 的通用库**基础默认常量**（如 `redisstore.DefaultTagPrefix`）允许保留，仅用于底层组件封装装配时未声明该值即生效的基础默认；门禁只针对「应用组件默认配置整体复用 `pkg/*.DefaultConfig()`」这类应用策略默认值问题。cache 的 tag 前缀作为该例外保留。
- 当前授权：用户确认 032 方案，授权本地实施、验证与聚焦提交；不授权 push、tag、Release、部署或外部写入。
- 本轮交付：032 的源码、测试、架构门禁与权威文档同步；未 push。
- 外部副作用：无。不启动服务、不写数据库、不 push、不 tag、不 release。

## 问题

业务模块需要统一接入 i18n 国际化规范，但当前存在几类配置职责边界问题：

1. `internal/kernel/app/i18n` 在默认配置（`defaults{}.Defaults()`、`defaultConfig()`）中直接复用 `pkg/i18n.DefaultConfig()`（默认语言 zh-CN、缺失行为 error、消息文件为空），即应用层组件隐式依赖底层通用库的默认值。
2. i18n 消息文件路径没有统一为 `./locales`；`config.example.yaml` 中仅注释示例 `locales/messages.zh-CN.yaml`，组件内部也没有关于 `./locales` 的稳定集中声明。
3. `kernel/app/i18n` 涉及的字符串、可变值、默认配置（`ConfigPath`、默认语言、注释中出现的路径等）散落在 `i18n.go` 与注释中，未集中到一个文件统一声明。
4. 业务接入规范缺失：新增业务模块如何接入 i18n、新增/修改语言内容应在哪个文件维护，没有明确的当前权威说明，容易造成各模块自行形成不同接入方式。
5. 其他 `kernel/app/*` 组件（logger、database、cache、storage）在基于 `pkg/*` 封装时，也可能直接复用通用库默认值（如 `pkg/logger.DefaultConfig()`、`pkg/database.DefaultConfig()`、`redisstore.DefaultTagPrefix`），或把应由应用层/使用者显式声明的动态值交给通用库默认值。

## 计划结论

建立统一的「`pkg/*` 通用库 vs `kernel/app/*` 应用层组件」配置职责边界，并单轨落地：

- 通用库只提供通用能力与基础默认行为；**属于具体应用环境、组件装配、业务场景或运行时变化的配置，由对应 `kernel/app/*` 组件或使用者自行声明和提供**，不得隐式依赖底层通用库默认值。
- `internal/kernel/app/i18n`：在自己组件内集中声明全部字符串/默认值（路径、默认语言、默认缺失行为、默认消息文件），i18n 消息文件目录统一为 `./locales`；不再直接调 `pkg/i18n.DefaultConfig()` 作为应用默认值。
- 审计并对齐其他 `kernel/app/*`：`logger`、`database` 不再复用 `pkg/*.DefaultConfig()` 作为组件默认值；`cache` 不再直接使用 `redisstore.DefaultTagPrefix`；`storage`、`observability` 已各自集中声明，保持并纳入规范。
- 在业务接入文档（应用模块开发指南）中明确 i18n 接入规范与语言内容维护位置。
- 将「应用层配置不得隐式依赖通用库默认值」纳入项目门禁规范与架构测试，并持续保持。

## 阅读顺序

1. [R001 当前 i18n 组件默认值与路径声明复核](research/R001-current-i18n-defaults-and-path/report.md)
2. [R002 pkg 通用库与 kernel/app 组件配置职责边界审计](research/R002-pkg-kernel-app-config-boundary/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务与确认状态](tasks.md)

本记录是任务级计划，不替代 [应用模块开发指南](../../development/application-module-development.md)、[底层能力库](../../../pkg/README.md)、[Kernel App 组件开发](../../../internal/kernel/app/README.md) 或 [配置说明](../../configuration/README.md)。确认实施后必须同步这些当前权威文档。

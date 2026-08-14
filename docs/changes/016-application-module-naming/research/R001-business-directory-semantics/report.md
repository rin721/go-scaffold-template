# R001：Business 目录命名语义复核

## 1. 研究问题与范围

本研究回答：`internal/business` 是否准确表达当前目录的真实职责；改为用户选择的“模块”语义是否符合现有架构；迁移需要覆盖哪些代码、稳定标识和文档。

范围只包含业务切片根目录、根契约包、Todo 导入路径、owner ID、架构门禁和当前说明。Todo 的业务行为、内部 `model/service/repo/handler/binding/middleware` 分层、配置键、HTTP/CLI、数据库 Schema 与 Kernel 均不在本任务中改变。

## 2. 方法与既有研究复用

开始前检索了现有 research metadata，并复用：

- 012 `module-boundaries.md`：当前目标是按业务能力纵向组织，composition root 显式连接模块与底层能力；
- 012 R015：Go `internal` 不能单独表达模块内依赖方向，需要 package graph 门禁；
- 014 R001：Todo 已实现为包含 Model、Service、Repository、HTTP/CLI、Config 与 migration 的真实垂直切片。

随后核验当前 HEAD `7bbe4bc7c6979324844cc1f7c43384d69738936b` 的 `internal/business`、`internal/composition`、架构测试与引用。`go test ./...` 在 2026-08-15 通过。命名选择取决于本地职责与已有术语所有权，不依赖外部框架流行度，因此没有新增外部搜索。

## 3. 当前事实

### 3.1 目录实际承载的内容

`internal/business` 同时包含两类内容：

- 根包定义 `ModuleID`、`Route`、`Contribution` 和集中校验；
- `todo` 子目录纵向收口 Model、Service、Repository、Handler、HTTP/CLI/Config/Model binding、middleware 与局部装配。

它不是纯 Domain 层，也不是只放用例的 Application 层；它是一个由应用组合根整体选择和安装的纵向模块集合。

### 3.2 当前术语已经有明确所有者

- `module`：表示 Todo 的局部装配单元和贡献身份，已有 `module.go`、`Module`、`ModuleID`，也是 012/014 的主架构术语。
- `capability`：表示 Kernel/`pkg` 提供的 Logger、Database、Clock 等技术能力，已有稳定 facade 与 composition 语义。
- `component`：表示 Kernel Plan 中受生命周期和配置治理的运行单元，目录位于 `internal/kernel/app`。
- `domain`：只适合表达业务不变量，不能覆盖 Handler、Repository Adapter 和协议 binding。
- `business`：只给出“业务而非基础设施”的宽泛分类，无法说明目录按什么单位组织。

当前代码还有 `business.todo` 与 `business.todo.schema` owner ID、`business` 包名、错误文本和 `businessCorePackage` 门禁 helper。只移动目录而保留这些名称会制造新旧双轨语义。

### 3.3 引用与兼容面

迁移会触及：

- `internal/business/**` 全部文件及其包声明、内部导入；
- `internal/composition` 的 Todo、Database、Router 和 Bootstrap 装配；
- package graph fixture、前缀判定和错误文本；
- `business.todo`、`business.todo.schema` 诊断/重载 owner ID 及其测试；
- 根 README、当前模块说明、Kernel/`pkg` 导航和仍承担当前指导作用的 012/014/015 文档。

`internal` 导入路径不是仓库外公共 API，但 owner ID 可出现在诊断和 `RestartRequired` 结果中，所以它仍是可观测语义变化，必须在同一提交中迁移并验证。

## 4. 候选比较

| 候选 | 与当前职责的匹配 | 冲突或误导 | 结论 |
| --- | --- | --- | --- |
| `business` | 能与基础设施区分 | 过宽，只说明类别，不说明纵向切片单位 | 替换 |
| `domain` | 能表达业务核心 | 目录实际还包含协议、持久化与配置 Adapter | 拒绝 |
| `module` | 与现有 `Module`、`ModuleID`、Contribution 和显式 composition 一致 | 名称较宽，必须限定为 application module | **用户选择；推荐实施** |
| `capability` | 能表达能力 | 已由底层技术能力占用，会让业务与基础设施同名 | 拒绝 |
| `feature` | 表达面向用户能力的纵向切片 | 偏产品表述，且用户明确选择 module | 不采用 |

## 5. 结论与推断

采用单数 Go 包语义 `internal/module`，首个模块为 `internal/module/todo`。根包改为 `module`；为避免 `module.ModuleID` 重复，身份类型收敛为 `module.ID`，其余调用形式为 `module.Contribution` 和 `module.ValidateContributions`。Todo 内部的 `todo.Module` 继续表示局部装配结果。

这不是因为 `business` “过时”，而是因为 `module` 更直接回答当前架构问题：“应用组合根选择和安装的一级单元是什么”。术语边界固定为：`module` 是应用纵向模块，Kernel 运行单元是 Component，Logger/Database 等底层依赖是 Capability，业务不变量仍位于模块内部的 Model/Domain 角色。

稳定 owner ID 同步迁移为 `module.todo` 与 `module.todo.schema`。Todo 的 `ID = "todo"`、配置路径 `todo`、路由、命令和数据库对象保持不变。仓库不保留 `internal/business` alias、转发包或兼容常量。

完成的 012/014/015 变更目录名称是稳定历史记录，不重命名；其中仍承担当前使用指引的路径和导航改为 `internal/module`，逐轮历史证据保留当时命名并加清晰迁移说明，避免伪造历史。

## 6. 局限与刷新条件

本结论建立在当前只有 Todo 一个应用模块、模块内部纵向收口、Kernel 与 application composition 分离的事实之上。若未来改成多 bounded context、独立 Go module 或插件加载，应重新评估 `internal/module` 是否仍是最准确的一级语义，而不是把本命名无限外推。

# 需求：应用模块命名迁移

## 1. 目标与依据

依据 [R001](research/R001-business-directory-semantics/report.md)，当前 `internal/business` 实际保存由应用组合根选择的纵向模块，但 `business` 只表达与基础设施的分类关系。按用户选择，任务目标是把根语义单轨收敛为 `module`，同时让 Kernel Component、底层 Capability 和模块内部 Domain 各自保留明确职责。

## 2. 命名契约

- 根目录从 `internal/business` 改为 `internal/module`，Go 包名为 `module`。
- Todo 完整模块迁移到 `internal/module/todo`，内部 `model/service/repo/handler/binding/middleware` 结构不变。
- 根贡献 API 使用 `module.ID`、`module.Route`、`module.Contribution` 和 `module.ValidateContributions`；旧 `ModuleID` 不保留 alias。
- Todo 模块 ID 仍为 `todo`；配置 section path 仍为 `todo`。
- application owner ID 从 `business.todo` 改为 `module.todo`，migration participant 从 `business.todo.schema` 改为 `module.todo.schema`。
- 架构门禁 helper、错误文本、测试名和当前说明同步使用 Module 语义。
- Module 在本仓库指应用组合根显式选择的纵向业务单元，不代表 Go module、Kernel Component 或动态插件。

## 3. 单轨要求

- 删除 `internal/business` 旧目录，不保留 alias、转发包、兼容常量或第二套校验入口。
- 所有生产代码、测试和当前权威说明改用 `internal/module`。
- 完成后搜索旧 import path、旧 Go 包引用、旧 owner ID 和失效当前说明；历史变更编号与逐轮证据中的当时术语可以保留，但必须能被迁移说明明确识别为历史。

## 4. 非目标

- 不重命名稳定历史目录 `012-business-module-architecture`、`014-todo-business-vertical-slice` 和 `015-todo-route-middleware-example`。
- 不改变 Todo 的业务字段、不变量、Service API、Repository 语义或内部层次。
- 不改变 `todo` 配置键、环境变量、HTTP route、CLI command、错误 reason、数据库表或字段。
- 不改变 Kernel Plan、Capability facade、composition 模型、生命周期或第三方依赖。
- 不引入 Feature Flag、动态模块发现、Registry、代码生成或兼容层。

## 5. 兼容影响

`internal/business/...` 是仓库内部导入路径，仓库外调用方本就不能合法导入；仓库内调用方必须在同一任务中全部迁移。`business.todo` 和 `business.todo.schema` 会出现在诊断、重载结果或测试断言中，实施后变为 `module.*`；它们不作为配置键或持久化身份继续兼容。

## 6. 验收标准

- `go list ./...` 只出现 `internal/module` 应用模块包，不出现 `internal/business`。
- Todo HTTP、Application CLI、SQLite persistence、migration 和配置验证行为保持不变。
- package graph 门禁继续阻止 Module core 导入 Kernel、HTTP、CLI 和 Database 边界。
- `module.todo` 与 `module.todo.schema` 的 owner/participant 断言通过；旧 `business.*` ID 在当前实现中无残留。
- 当前 README 与模块开发说明统一解释 Module、Component、Capability、Domain 的不同语义。
- 本任务涉及的 Go 文件执行 `gofmt -l` 无输出；`go mod tidy -diff`、`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build -o NUL ./cmd/app`、文档链接和 `git diff --check` 通过。全仓 `gofmt -l .` 若仅因未修改文件的 Windows CRLF 或忽略目录输出，必须记录基线限制，不能借命名任务格式化范围外文件。

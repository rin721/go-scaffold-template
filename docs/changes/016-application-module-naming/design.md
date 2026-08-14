# 设计：应用模块命名迁移

## 1. 设计结论

把纵向业务模块根从 `internal/business` 单轨迁移到 `internal/module`。`module` 直接表达应用组合根选择和安装的单元；Kernel 生命周期单元继续称 Component，模块依赖的 `pkg`/Kernel 能力继续称 Capability。这个改名只校正语义，不改变运行架构。

## 2. 目标结构

```text
internal/
├── module/
│   ├── contracts.go
│   ├── contracts_test.go
│   ├── README.md
│   └── todo/
│       ├── model/
│       ├── service/
│       ├── repo/
│       ├── handler/
│       ├── middleware/
│       ├── binding/
│       ├── module.go
│       └── README.md
├── composition/
└── kernel/
```

依赖方向保持：

```text
cmd/app -> internal/composition -> internal/module/todo
                              \-> internal/kernel/composition

module/todo model <- service <- repo/handler <- binding <- module.go
```

## 3. Go 符号与稳定 ID

根 package declaration 从 `package business` 改为 `package module`。`ModuleID` 收敛为 `ID`，避免调用点出现重复的 `module.ModuleID`；`Contribution` 和 `Route` 继续准确表达模块装配输出。调用点从 `business.Contribution` 改为 `module.Contribution`。

同步替换：

| 当前 | 目标 | 说明 |
| --- | --- | --- |
| `internal/business` | `internal/module` | 根目录与 import path |
| `package business` | `package module` | 根契约包 |
| `business.ModuleID` | `module.ID` | 模块身份类型 |
| `business.*` | `module.*` | 其余 Go 限定名 |
| `business.todo` | `module.todo` | Todo 配置 owner/Capability ID |
| `business.todo.schema` | `module.todo.schema` | migration participant name |
| `businessCorePackage` | `moduleCorePackage` | package graph helper |
| business contribution/module/route/participant | module contribution/route/participant | 诊断文本 |

`ID("todo")` 的值不改变，避免把模块身份和 owner namespace 混为同一概念。

## 4. 迁移顺序

1. 整体迁移目录并更新根 package declaration、`ModuleID -> ID`、Todo 内部 imports 与所有测试 imports。
2. 更新 `internal/composition` 的 import alias、参数类型和诊断上下文。
3. 更新 owner ID、participant name 与相关断言；不保留旧常量。
4. 更新 package graph 的合法 fixture、Module core 识别函数和错误文本。
5. 更新根 README、`internal/module` 与 Todo README、Kernel/`pkg` 当前导航和仍被当作开发指南的变更文档。
6. 搜索旧路径/限定名/owner ID，执行全量验证后一次性提交计划、实现、测试和当前文档。

迁移必须在同一任务中闭合，不能先加入 `module` 再让 `business` 代理，也不能用 import alias 把旧 package 名长期伪装为新语义。

## 5. 文档与历史策略

- 当前权威说明统一使用“应用模块/纵向模块”，首次出现时与 Go module、Kernel Component、Capability 区分。
- `docs/changes/016-application-module-naming` 记录本次替代关系。
- 012/014/015 的目录名和逐轮实施证据保留，因为它们是稳定任务 ID 和当时事实。
- 这些历史文档中承担当前导航、开发步骤或代码链接的内容更新到 `internal/module`；保留的旧路径必须明确标注为 016 之前的历史快照，不能继续作为现行入口。

## 6. 文件影响

- 移动并修改 `internal/business/**` 为 `internal/module/**`。
- 修改 `internal/composition/*.go` 中所有相关导入、类型和诊断。
- 修改 `internal/kernel/composition/architecture_test.go` 的 Module 边界规则与 fixture。
- 更新根 README、`internal/kernel/README.md`、`pkg/README.md`、当前 Module/Todo README 和必要的 012/014/015 导航/说明。
- 更新 016 状态、任务证据和 `docs/changes/README.md`。
- 不修改 `go.mod`、`go.sum`、配置示例、HTTP/CLI 契约或数据库定义。

## 7. 验证设计

- 结构：`Test-Path internal/business` 为 false，`go list ./...` 只列出 `internal/module...`。
- 残留：在生产代码与当前说明中搜索 `internal/business`、`package business`、`business.`、`businessCorePackage`；历史命中逐项确认有明确快照语义。
- 架构：现有合法 fixture 改用 Module 路径，负向测试继续覆盖 Module core 越界。
- 行为：复用全量 Todo Model/Service/Repository/HTTP/CLI/进程测试，证明只有命名变化。
- 工程门禁：gofmt、tidy diff、test、race、vet、build、Markdown 本地链接与 Diff 检查。

若迁移暴露需要修改配置键、公开协议或数据库身份的新事实，停止实施并把任务退回“待确认”。

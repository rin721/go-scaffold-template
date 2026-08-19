# 需求：i18n 配置职责边界与集中声明（032）

## 1. 依据

- `R001`：`kernel/app/i18n` 默认配置直接复用 `pkg/i18n.DefaultConfig()`，消息文件目录未统一 `./locales`，字符串/默认值散落。
- `R002`：logger/database 复用 `pkg/*.DefaultConfig()`，cache 复用 `redisstore.DefaultTagPrefix`；storage/observability 已正确自声明。

## 2. 目标

建立并落地「`pkg/*` 通用库 vs `kernel/app/*` 应用层组件」的配置职责边界：

```text
pkg/*：通用能力 + 基础默认行为（库自身最低可用默认）
kernel/app/*：根据应用环境、装配、业务与运行时显式声明默认配置与常量
业务模块/使用者：按需显式传入动态值，不隐式依赖底层默认
```

重点：`kernel/app/i18n` 集中自声明全部默认配置，i18n 消息文件目录统一 `./locales`；同时对齐 logger/database/cache；并把边界纳入门禁规范与文档。

## 3. 术语

- **通用库默认值**：`pkg/*` 内部 `DefaultConfig()`/默认常量（库自己提供的最低可用默认）。
- **应用组件默认值**：`kernel/app/*` 组件内集中声明的默认配置/常量（应用层应根据实际需求显式声明）。
- **`./locales`**：i18n 消息文件目录统一约定，由 `kernel/app/i18n` 集中声明。

## 4. 功能要求

### `REQ-001` i18n 组件集中声明默认配置与路径

`internal/kernel/app/i18n` 必须在一个集中文件中统一声明以下值，供组件其他代码引用，不得散落：

- `ConfigPath`（配置节名）；
- 消息文件目录常量 `./locales`（LocalesDir）；
- 默认语言（对齐当前 zh-CN）；
- 默认缺失行为（对齐当前 error）；
- 默认消息文件列表（基于 `./locales` 的稳定默认）。

`defaults{}.Defaults()` 与 `defaultConfig()` 必须基于组件内集中声明，不得直接调用 `pkg/i18n.DefaultConfig()` 作为应用默认值。

### `REQ-002` 统一 i18n 消息文件路径

i18n 消息文件目录统一声明为 `./locales`；`kernel/app/i18n` 的默认 `MessageFiles` 与运行时 `packageConfig()` 使用该统一目录语义，`config.example.yaml` 与验收示例同步为 `./locales`。

### `REQ-003` 应用层不得隐式依赖通用库默认值（门禁边界）

以下约束纳入规范并持续保持：

- `kernel/app/*` 组件的默认配置/常量应在组件内集中自声明，默认配置路径不得直接调用 `pkg/*.DefaultConfig()` 或 `pkg/*` 内部默认常量。
- 属于应用环境、装配、业务或运行时变化的配置，由对应 `kernel/app/*` 组件或使用者显式声明与传入。
- 特例（确需继承通用库「通用最低默认」）必须有明确理由与审批路径，不能静默成立。

### `REQ-004` 对齐 logger / database / cache 默认配置来源

- `kernel/app/logger`：不再在 `defaults{}`/`defaultConfig()` 复用 `pkg/logger.DefaultConfig()`；组件内集中声明默认 environment/level/encoding。
- `kernel/app/database`：不再复用 `pkg/database.DefaultConfig()`；组件内集中声明默认值。
- `kernel/app/cache`：`TagPrefix` 默认值不再复用 `redisstore.DefaultTagPrefix`；在组件内集中声明默认 tag 前缀。

### `REQ-005` 保持 storage / observability 做法并纳入说明

storage 与 observability 已正确在组件内集中自声明默认配置；本次不改其默认值来源，但把它们作为规范示例写入权威文档，防止回退。

### `REQ-006` 业务 i18n 接入规范

更新应用模块开发指南，明确：

- 新增业务模块如何接入 i18n（通过注入的 `pkg/i18n.Translator`，message ID 前缀/命名约定，如 `todo.error.*`）；
- 新增或修改语言内容应在哪个文件维护（`./locales` 下按语言命名的消息文件，以及消息文件命名/扩展名规范）；
- 业务 handler 不应绕过统一 Translator 或自行读取 `pkg/i18n.Config` 默认值。

### `REQ-007` 文档与门禁同步

同步 `pkg/README.md` 边界说明、Kernel App 组件开发说明、应用模块开发指南、配置说明与 `config.example.yaml`，使权威文档只描述单轨现行边界；架构门禁（或规范检查）防止「`kernel/app/*` 默认配置直接调用 `pkg/*.DefaultConfig()`/默认常量」再次引入。

## 5. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 简单 | 组件默认配置都在各自 `kernel/app/*` 内一处集中声明 |
| 明确 | 应用组件默认值来源清晰，不隐式依赖 `pkg/*` 默认 |
| 统一 | i18n 消息文件目录统一为 `./locales`，无散落路径 |
| 可验证 | 门禁/架构测试阻止 `kernel/app` 复用 `pkg` 默认值；单元与集成测试通过 |

## 6. 范围

### 包含

- `kernel/app/i18n` 集中默认声明 + `./locales` 统一；
- `kernel/app/logger`、`database`、`cache` 默认值来源对齐；
- 业务 i18n 接入规范（应用模块开发指南）与配置说明、`pkg/README.md`、Kernel App 组件说明同步；
- 门禁/规范检查（架构测试或可执行检查）与 `config.example.yaml` 同步。

### 不包含

- 更换翻译引擎第三方或改变 `pkg/i18n.Translator` 契约；
- 引入 i18n 平台/远端翻译；
- 重构 Kernel 生命周期框架或 config Scheduler；
- 改变 `pkg/*` 通用能力契约本身；
- push、tag、Release、部署或数据库操作。

## 7. 验收标准

1. `kernel/app/i18n` 内单一文件集中声明 `ConfigPath`、`LocalesDir=./locales`、默认语言、默认缺失行为、默认消息文件；`defaults{}/defaultConfig` 不调用 `pkg/i18n.DefaultConfig()`。
2. `kernel/app/logger`、`database` 的 `defaults{}/defaultConfig` 不调用 `pkg/{logger,database}.DefaultConfig()`；`cache` 的 tag 前缀默认值来自组件内集中声明，不引用 `redisstore.DefaultTagPrefix`。
3. 业务接入规范（应用模块开发指南）明确新增模块 i18n 接入与 `./locales` 语言内容维护位置；handler 经统一 Translator 呈现，message ID 前缀约定清晰。
4. 门禁/架构检查阻塞「`kernel/app/*` 默认配置路径直接调用 `pkg/*.DefaultConfig()`/默认常量」；相关 fixture 通过。
5. 配置说明、`pkg/README.md` 边界、Kernel App 组件说明、`config.example.yaml` 与 `./locales` 语义一致（无旧路径 `locales/...` 或散落字符串残留）。
6. `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./... -count=1`、`go test -race`（受影响）、`go mod tidy -diff` 与 `git diff --check` 通过。

## 8. 确认要求

这是非纯文档实施计划：将修改 `kernel/app/*` 源码、测试、配置示例与文档。只有用户在本计划报告之后的后续消息明确确认 032 当前方案，才能开始实施。若实施中发现必须改变 `pkg/*` 通用能力契约、Kernel 生命周期/配置框架、Translator 契约或需要新增第三方依赖，必须退回研究并重新确认。

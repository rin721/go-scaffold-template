# R001 当前 i18n 组件默认值与路径声明复核

## 1. 研究问题

`internal/kernel/app/i18n` 目前如何取得默认配置，i18n 消息文件路径如何声明，业务模块如何消费 Translator；这些是否导致应用层隐式依赖 `pkg/i18n` 默认值，或使路径/常量散落？

## 2. 方法与范围

- 只读取仓库内源码、测试、配置示例与权威文档作为证据，不编写实现。
- 复核对象：`internal/kernel/app/i18n/`、`pkg/i18n/`、Todo handler/module、`config.example.yaml` 与 composition i18n 段。

## 3. 证据（事实）

### 3.1 kernel/app/i18n 的默认配置来源

`internal/kernel/app/i18n/i18n.go`：

- `Config` 定义 `DefaultLanguage`／`MessageFiles`／`MissingBehavior`（后者的类型直接引用 `pkgi18n.MissingBehavior`，`mapstructure:"missingBehavior"`）。
- `defaults{}.Defaults(ctx)` 调用 `values := pkgi18n.DefaultConfig()`，把 `DefaultLanguage`、`MessageFiles`、`MissingBehavior` 写入 config Object。
- `defaultConfig()` 同样 `values := pkgi18n.DefaultConfig()` 构造组件默认 `Config`。
- `Config.packageConfig()` 构造传给 `pkg/i18n.New` 的 `pkg/i18n.Config`：`DefaultLanguage`/`MessageFiles`/`MissingBehavior` 透传，`MessageFS: os.DirFS(".")` 固定。

结论（事实）：**应用层组件 kernel/app/i18n 直接复用 `pkg/i18n.DefaultConfig()` 作为应用默认值**（默认语言 zh-CN、缺失行为 error、消息文件 nil），即应用层配置隐式依赖底层通用库默认值。

### 3.2 消息文件路径

- `pkg/i18n` 的 `resolvedConfig` 默认 `MessageFS: os.DirFS(".")`（builder.go）；kernel/app 的 `packageConfig()` 也固定 `MessageFS: os.DirFS(".")`。
- `config.example.yaml` 的 i18n 段中，`messageFiles: []` 且注释示例为 `locales/messages.zh-CN.yaml`、`locales/messages.en.json`。
- 组件内未见统一声明的 `./locales` 常量或默认消息文件。

结论（事实）：**i18n 消息文件目录未在 kernel/app/i18n 内集中声明为 `./locales`**；默认 `MessageFiles` 为空，且路径仅出现在配置示例注释中。

### 3.3 常量/字符串集中度

- `ConfigPath = "i18n"` 定义在包级 const；默认语言/缺失行为/消息文件默认值均从 `pkg/i18n.DefaultConfig()` 透传。
- 组件内没有单独的文件集中声明「路径、默认配置、语义常量」；这些散布在 `i18n.go` 与配置注释中。

结论（事实）：字符串、可变值、默认配置未集中到单一文件。

### 3.4 业务消费路径

- `internal/module/todo/module.go` 的 `HTTPDependencies.Translator i18n.Translator`（这里 `i18n` 指 `pkg/i18n`）。
- `internal/module/todo/handler/handler.go` 持有 `translator i18n.Translator`，`NewHandler` 注入；`present` 用 `translator.Translate(language, i18n.Text("todo.error."+reason, ...))`。
- `internal/kernel/composition` 通过 `i18napp.Definition()` 产出 `pkgi18n.Translator`，注入 `Capabilities.I18n` 再传入 Todo。

结论（事实）：业务模块只消费稳定的 `pkg/i18n.Translator` facade，不直接接触 `pkg/i18n.Config` 默认值；接入面在 Todo handler 的 `present` 中（message ID 约定 `todo.error.*`）。

## 4. 推断

1. 当前默认值全部来自 `pkg/i18n.DefaultConfig()`，使「应用层组件该声明哪些配置」不清晰；这也是用户点名的核心问题。
2. `./locales` 路径应以 kernel/app/i18n 自己的常量集中声明，并作为默认 `MessageFiles` 的基准，避免散落注释与硬编码。
3. 业务接入规范缺失：新增模块应在哪个文件维护语言内容、如何接入 Translator，需在应用模块开发指南明确。
4. 由于多个 `kernel/app/*` 组件都复用 `pkg/*.DefaultConfig()`，该问题不是 i18n 独有，需整体审计（见 R002）。

## 5. 适用与不适用场景

- 适用：i18n 组件默认值/路径集中声明；业务 i18n 接入规范；pkg 与 kernel/app 配置边界。
- 不适用：更换翻译引擎、新增语言文件格式、构建 i18n 平台、改变 `pkg/i18n.Translator` 契约。

## 6. 局限与剩余未知

- 未运行测试/生成；当前为只读复核。
- `./locales` 的具体默认文件命名与语言集（zh-CN/en）需在设计阶段定案，并同步配置示例与验收。

## 7. 对 032 的影响

- 计划必须明确：kernel/app/i18n 内集中声明 `LocalesDir`、默认语言、默认缺失行为、默认消息文件；`packageConfig()` 用组件自有默认构造 `pkg/i18n.Config`；`defaults{}`/`defaultConfig()` 不再直接依赖 `pkg/i18n.DefaultConfig()`。
- 业务接入规范（应用模块开发指南）与配置说明需同步。
- 研究门禁通过。

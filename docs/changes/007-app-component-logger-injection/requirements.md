# 产品需求：Kernel 内置 Logger 的可选 App 替换

## 1. 背景与当前事实

当前 `main` 已有以下真实行为：

- `cmd/app` 创建并拥有一个基线 `pkg/logger.Resource`，再构造 `internal/kernel/logging.Manager`；`kernel.New` 强制要求该 Manager 非 nil。
- Manager 本身实现 `pkg/logger.Logger`，通过读写锁在 baseline 与 current 之间委托；旧 `With` 子 Logger 会跟随 `Replace/Restore`。
- `internal/kernel/app/logger.Definition(manager)` 构造配置化 Logger Resource，并通过普通 `app.Add` 加入 Plan。
- Logger App 在 `WithActivation` 中调用 `manager.Replace/Restore`，因此它实际上会接管 Kernel Logger，但这个意图没有出现在 Definition 类型、Plan 操作或 target Binding 中。
- `composition.Compose` 当前无条件加入 Logger App；`Capabilities.Logger` 是该 App 自己的 Leased Access。因此 composition 不能选择“只用 Kernel 基线”，也存在“内置 Manager facade + App Access”两个 Logger 使用入口。
- Logger App 被选择时会贡献 `logger` 默认配置；当前 `cmd/app config init` 生成 Logger、Database 两段。

本任务解决的是声明与选择语义，不重做 `pkg/logger`、Kernel 全部生命周期或通用能力治理框架。

## 2. 目标与用户场景

- Kernel 构造后即拥有一个不依赖配置文件的内置 Logger，早期诊断和正常能力调用使用同一个稳定 facade。
- composition 调用方可以只保留内置 Logger，不构造、不读取、不生成配置化 Logger 组件。
- composition 调用方也可以显式选择 `kernel/app/logger` 替换组件；代码审阅者只看组件构造和 Plan 装配即可确认替换意图与目标。
- 配置化实例成功发布、重载或停止时，稳定 facade 分别切换到新实例、继续使用旧实例或恢复 baseline；消费者不需要重新注入。
- 默认应用入口继续显式选择配置化替换，避免本任务无意改变已有配置文件和运行方式。

## 3. 功能需求

### 3.1 Kernel 内置 Logger

- Kernel Options 继续强制接收一个非 nil logging Manager，不增加隐藏 Noop 或隐式默认构造。
- Kernel 对普通消费者暴露不含替换权和关闭权的稳定 `pkg/logger.Logger`；同时只在 `internal/kernel` 边界提供由同一 Manager 实现的 typed replacement target。composition 把这个 target 作为固定内置能力加入本地 Plan，并取得 Binding。
- 内置能力必须有稳定、归属明确的 ID，不能与配置化替换组件 ID 混用。
- `Capabilities.Logger` 始终返回该稳定只读 facade，类型不随是否选择替换而变化；其动态类型也不得通过类型断言泄漏 `Replace/Restore`。
- 基线 Resource 仍由应用入口拥有和关闭；Manager 与 Plan 都不取得其关闭权。

### 3.2 显式替换声明

- `internal/kernel/app/logger` 的公开构造入口命名为 `Replacement`，返回与普通 `Definition` 不同的 typed `ReplacementDefinition[kernellogging.Target]`。
- 替换必须通过 `app.Replace(plan, targetBinding, replacement)` 登记，不能通过 `app.Add` 伪装成普通独立组件。
- target Binding 必须来自同一个 Plan 中已经加入的内置 Logger；零值、跨 Plan、尚未加入或类型不匹配的 target 必须失败。
- `app.Replace` 必须从 target Binding 解析并把同一个 target 实例传给 replacement；组件构造函数不得另收一个可与 Binding 身份错配的 Manager 或控制器。
- 同一个 target 在一份 Plan 中最多登记一个替换组件；失败不得留下 ID、Defaults、CLI、运行节点或 target 占用的部分状态。
- 替换组件保留自己的稳定组件 ID 和 `logger` ConfigPath，但不创建可供消费者使用的第二个 Binding 或 Access。
- 普通 `Definition` 与 `ReplacementDefinition` 单轨分离；替换意图不得继续仅隐藏在通用 `WithActivation` 回调中。

### 3.3 可选 composition

- `composition.Options` 使用专用有限取值类型选择 Logger 行为，禁止用含义不清的布尔参数或配置字符串控制。
- 零值选择只使用 Kernel 内置 Logger；显式选择值才加入配置化 Logger 替换组件。
- 未选择替换时：Plan 不含 Logger 运行节点，不解码 `logger` 配置，不贡献 `logger` Defaults，启动和停止都只使用 baseline。
- 选择替换时：替换节点保持在其他可能使用 Logger 的底层 App 组件之前；其 Defaults、配置校验、构造、热换和关闭语义继续生效。
- 未知选择值必须在安装 Plan 前返回错误，且返回零 `Capabilities`、不向 Kernel 安装部分计划。
- 当前 `cmd/app` 在 CLI 和服务模式都显式选择配置化替换，以保持 `config init` 仍生成 Logger、Database，并保持无参数服务模式使用配置化 Logger。

### 3.4 生命周期、失败与并发

- 初始启动只有在配置化 Resource 构造和就绪成功后才能替换 baseline；Decode、Build、Start 或 Ready 失败均保持 baseline。
- Reload 候选失败时继续使用当前有效 Logger；成功提交后才切换到新 Resource，再关闭旧 Resource。
- Kernel Stop 必须先把稳定 facade 恢复到 baseline，再关闭当前配置化 Resource；恢复和关闭错误语义不得被吞掉。
- Manager 的日志写入与 `With` 动态视图语义保持不变；替换必须等待正在执行的同步日志调用结束，且 race 检查无数据竞争。
- 替换组件的 Resource 所有权不泄漏给 `Capabilities` 或其他调用方；消费者只依赖 `pkg/logger.Logger`。
- Kernel 自身成功状态日志继续通过 Manager 输出；错误继续完整向 Host/入口返回，不增加逐层重复日志。

### 3.5 文档与单轨迁移

- 删除 `loggerapp.Access`、`Definition(manager)` 和普通 `app.Add` Logger 的当前入口与全部调用；不保留 alias、deprecated 包装或兼容分支。
- 更新根 README、Kernel/App 主题文档及必要的 Logger 用法说明，明确“内置稳定 facade”和“可选替换组件”的区别。
- 已完成的 004、006 变更记录保持历史事实，不重写为当前 API 权威；当前结论同步到主题文档。
- 搜索旧符号和旧说明，确认没有把配置化 Logger 继续描述为独立 Leased Capability。

## 4. 验收标准

- `composition.Compose(runtime, Options{})` 成功返回非 nil Logger，Plan 不含配置化 Logger，Defaults 不含 `logger`；Logger 在配置加载前即可使用。Database 仍按既有契约要求有效配置，本任务不把“Logger 无配置”扩大成“完整 Plan 无配置”。
- 显式选择配置化替换后，Plan 中可识别到“替换内置 Logger”的关系；`config init` 仍按 Logger、Database 顺序生成配置。
- `app.Add` 不能接收 Logger Replacement；`app.Replace` 拒绝零值、跨 Plan、错误顺序、类型不匹配和重复 target，并保证失败原子性。
- baseline、首次替换、成功 Reload、失败 Reload、Stop 恢复的日志目标和 Resource 关闭次数符合需求。
- 替换前取得的 Logger 和 `With` 子 Logger 在替换及恢复后继续跟随 Manager 当前目标。
- `cmd/app` 的 CLI 与服务路径都显式选择配置化替换；现有用户配置不因默认选择变化而失效。
- 相关单元测试、`go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...` 和 `git diff --check` 通过；未执行项必须如实报告。

## 5. 约束

- 不新增依赖，不暴露 zap 类型，不改变 `pkg/logger.Logger` 的业务方法集合。
- 不使用全局变量、`init` 注册、反射扫描、Service Locator、运行期 Resolver 或 `map[string]any` 表达替换关系。
- 替换选择发生在 composition；配置文件只配置已经被选择的组件，不能反向决定组件是否存在。
- 本任务不能弱化 Kernel baseline 的必填要求，也不能让配置化组件拥有或关闭 baseline。
- 源码注释和维护文档以中文为主，标识符保留英文。

## 6. 非目标

- 不建设适用于 Config、CLI、Clock、ID、Validator、Database 的通用内置能力 Catalog。
- 不实现同一 target 多个候选、优先级、覆盖栈、嵌套替换、Feature Flag、动态装卸或运行期切换实现类型。
- 不设计 HTTP、数据库、消息、健康检查、业务 Module 或业务对象 DI。
- 不改变文件 Watch、Kernel 整轮 Reload、Supervisor、CLI 命令或配置格式。
- 不接入新的 Logger 后端、轮转、远程采集、审计或 OTLP。
- 不提交、合并、rebase、push 或复用其他分支上的 007 实现。

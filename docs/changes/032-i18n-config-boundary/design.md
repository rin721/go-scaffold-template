# 设计：i18n 配置职责边界与集中声明（032）

## 1. 设计结论

单轨建立「`pkg/*` 通用库 vs `kernel/app/*` 应用层组件」配置职责边界，并聚焦 i18n 组件集中声明 + `./locales` 统一，同时对齐 logger/database/cache：

```text
pkg/*              -> 通用能力 + 基础默认行为（库自身最低可用默认，保持原样）
kernel/app/*       -> 在组件内集中声明应用默认配置/常量；默认配置路径不调 pkg DefaultConfig/默认常量
业务模块/使用者     -> 显式接入统一 Translator / 传入动态值
门禁               -> 架构/规范检查阻止 kernel/app 复用 pkg 默认
```

## 2. 为什么不是现有结构

当前 `kernel/app/i18n` 的 `defaults{}.Defaults()` 与 `defaultConfig()` 直接复用 `pkg/i18n.DefaultConfig()`，使应用层隐式依赖通用库默认；`./locales` 未集中声明；字符串/默认值散落。logger/database/cache 同理（复用 `pkg/*.DefaultConfig()`/`redisstore.DefaultTagPrefix`），storage/observability 已正确。

## 3. kernel/app/i18n 集中声明

在 `internal/kernel/app/i18n` 新增/收口一个集中文件（例如保留在 `i18n.go` 顶部或独立 `config.go`，按实施语义定名），集中声明：

```go
const (
    ID         app.ID = "i18n"
    // ConfigPath 是 Kernel 配置节名。
    ConfigPath = "i18n"
    // LocalesDir 是 i18n 消息文件目录（相对进程工作目录）。
    LocalesDir = "./locales"
)

const (
    defaultLanguage     = "zh-CN"
    defaultMissingBehavior = pkgi18n.MissingBehaviorError
)

var defaultMessageFiles = []string{
    "./locales/messages.zh-CN.yaml",
    // 其他默认消息文件按需加入
}
```

- `defaults{}.Defaults(ctx)` 与 `defaultConfig()` 改为基于上述集中默认值构造，**不再调用 `pkg/i18n.DefaultConfig()`**。
- `Config.packageConfig()` 用统一路径：默认 `MessageFiles` 兜底为 `defaultMessageFiles`，`MessageFS` 语义与 `./locales` 对齐（实现上保持 `fs.FS` 注入能力，但应用默认走 `./locales`）。
- `Config.MissingBehavior` 类型继续用 `pkg/i18n.MissingBehavior`（这是能力契约类型，不是默认值；保持契约不变）。

## 4. 对齐 logger / database / cache

- `kernel/app/logger`：在组件内集中声明默认 `environment/level/encoding` 常量，`defaults{}/defaultConfig` 不再调 `pkg/logger.DefaultConfig()`。
- `kernel/app/database`：在组件内集中声明默认值，`defaults{}/defaultConfig` 不再调 `pkg/database.DefaultConfig()`。
- `kernel/app/cache`：把 Redis `TagPrefix` 默认值移到组件内集中常量（例如 `defaultRedisTagPrefix`），不再引用 `redisstore.DefaultTagPrefix`；其余已自声明保持。
- storage/observability：默认值来源保持不变（已正确），仅纳入规范/文档作为示例。

## 5. 业务 i18n 接入规范（权威文档）

在 `docs/development/application-module-development.md` 增加「i18n 接入」小节，明确：

1. 业务模块通过注入的 `pkg/i18n.Translator` 消费（composition 注入 `Capabilities.I18n`），不自行创建 Translator 或读取 `pkg/i18n.Config` 默认值。
2. 消息 ID 命名约定：`<domain>.<error|catalog>.<key>`，如 `todo.error.title_required`；`kernel/app/i18n` 负责按语言加载 `./locales` 消息文件。
3. 新增/修改语言内容在 `./locales` 下维护：`messages.<lang>.yaml|yml|json`（lang 为 BCP 47，如 `zh-CN`、`en`）；新增语言即新增对应消息文件并在 `kernel/app/i18n` 默认消息文件列表/配置声明。
4. Handler 经统一 Translator 呈现错误，不绕过；缺翻译默认 `error` 策略（可配置 `use-id`）。

`config.example.yaml` 的 i18n 段同步为 `./locales` 语义（取消纯注释、给出稳定示例），配置说明相应更新。

## 6. 文件影响

预计实施范围：

- 修改 `internal/kernel/app/i18n/i18n.go`（集中默认 + `./locales` + `packageConfig`）与 `i18n_test.go`；
- 修改 `internal/kernel/app/logger/logger.go`、`internal/kernel/app/database/database.go`、`internal/kernel/app/cache/cache.go` 默认值来源；
- 修改 `config.example.yaml` i18n 段；
- 修改 `docs/development/application-module-development.md`（i18n 接入小节）、`pkg/README.md`（边界）、`docs/configuration/README.md`、`internal/kernel/app/README.md`；
- 修改 `internal/kernel/composition/architecture_test.go` 或新增门禁检查（阻塞 `kernel/app` 默认配置路径调用 `pkg/**DefaultConfig`/默认常量）；
- 同步 032 变更证据与 `docs/changes/README.md` 索引。

预计不修改 `pkg/*` 通用能力契约、Kernel/config 框架、Translator 契约、迁移、数据库 schema、Host/listener。

## 7. 失败语义

- 默认消息文件缺失：`pkg/i18n.New` 构造失败 -> 组件 build 失败 -> 候选 generation abort，旧代保留。
- 门禁检查发现 `kernel/app` 仍复用 `pkg` 默认：测试失败，阻止提交。
- 配置示例与实现不一致：文档/集成测试失败，阻止提交。
- 行为回归：单元/集成测试与 reload 测试失败，阻止提交。

## 8. 架构/规范门禁

新增或修改可执行检查（不依赖路径白名单/注释）：

- `kernel/app/{i18n,logger,database,cache}` 的默认配置构造/`Defaults` 不得调用 `pkg/{i18n,logger,database}/DefaultConfig()` 或 `pkg/**/Default*` 默认常量（含 `redisstore.DefaultTagPrefix`）。
- 组件内集中常量（`ConfigPath`/`LocalesDir`/默认语言/缺失行为/默认消息文件）存在且被组件引用。
- 业务 handler 只依赖 `pkg/i18n.Translator`，不依赖 `pkg/i18n.Config` 默认值。
- 沿用既有 module owner、pkg、Kernel 依赖方向与契约一致。

用正反 fixture 校验；禁止用别名/路径白名单/注释约定代替可执行门禁。

## 9. 验证矩阵

- 定向单元：i18n/logger/database/cache 集中默认、`./locales`、默认消息文件、TagPrefix 组件内声明。
- 协议/集成：composition/reload 集成测试（含 i18n 段）通过；Todo handler 错误呈现经统一 Translator。
- 完整门禁：`gofmt -l .`、`go generate ./...`、`go mod tidy -diff`、`go test ./...`、`go test -race`（受影响）、`go vet ./...`、`go build ./cmd/app`、`git diff --check`。

## 10. 单轨迁移顺序

1. 先落地 i18n 组件集中默认 + `./locales`，更新 `i18n_test.go` 与配置示例。
2. 对齐 logger/database/cache 默认值来源。
3. 增加架构/规范门禁与正反 fixture。
4. 同步权威文档（业务 i18n 接入、`pkg/README` 边界、配置说明、Kernel App 说明）。
5. 全量验证，审阅完整 diff 后提交（只提交 032 范围；不 push）。

不保留双默认、旧 `locales/...` 散落字符串或兼容 wrapper。

## 11. 重新确认触发器

出现以下任一事实时退回研究和待确认：

- 必须改变 `pkg/*` 通用能力契约、Kernel 生命周期/config 框架或 Translator 契约；
- 需要新增第三方依赖或引入外部 i18n 平台；
- `./locales` 语义或默认语言/缺失行为与应用实际需求冲突需要变更；
- 门禁无法在不破坏既有依赖方向的前提下表达该边界；
- 用户希望把某个 `pkg/*` 默认值（如 `redisstore.DefaultTagPrefix`）保留为组件直接引用，且构成例外。

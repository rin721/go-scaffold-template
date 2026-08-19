# R002 pkg 通用库与 kernel/app 组件配置职责边界审计

## 1. 研究问题

所有 `kernel/app/*` 组件在基于 `pkg/*` 通用库封装时，是否存在「直接复用通用库默认配置，或把应由应用层/使用者显式声明的动态值交给通用库默认值」的问题；哪些组件已正确自声明默认配置，可作为模板？

## 2. 方法与范围

- 只读取仓库内源码作为证据，不运行命令。
- 复核对象：`internal/kernel/app/{logger,database,cache,storage,observability,i18n}` 的默认配置来源。

## 3. 证据（事实）

### 3.1 logger（有问题）

`internal/kernel/app/logger/logger.go`：

- `defaults{}.Defaults(ctx)` 调 `values := pkglogger.DefaultConfig()`（第 106 行）生成 config Object。
- `defaultConfig()` 也调 `pkglogger.DefaultConfig()`（第 116 行）构造组件默认 `Config{Environment,Level,Encoding}`。

结论（事实）：logger 组件默认值直接复用 `pkg/logger.DefaultConfig()`。

### 3.2 database（有问题）

`internal/kernel/app/database/database.go`：

- `defaults{}.Defaults(ctx)` 调 `pkgdatabase.DefaultConfig()`（第 155 行）。
- `defaultConfig()` 调 `pkgdatabase.DefaultConfig()`（第 178 行）。

结论（事实）：database 组件默认值直接复用 `pkg/database.DefaultConfig()`。

### 3.3 cache（部分问题）

`internal/kernel/app/cache/cache.go`：

- 自声明 `defaultRedisAddress/DialTimeout/ReadTimeout/WriteTimeout/PingTimeout` 常量（第 34-38 行），`defaultConfig()` 内部自构造默认值。
- 但第 361 行 `TagPrefix: redisstore.DefaultTagPrefix`，即 Redis store 的 tag 前缀默认值直接复用 `pkg/cache/redisstore` 的默认常量。

结论（事实）：cache 大部分自声明，但 Redis `TagPrefix` 复用底层 store 默认常量。

### 3.4 storage（正确模板）

`internal/kernel/app/storage/storage.go`：

- 自声明 `defaultLocalBasePath = ".data/storage"`（第 30 行），`defaultConfig()` 自构造默认值（第 184 行），`defaults{}.Defaults` 调 `defaultConfig()`（第 277 行）。
- ObjectConfig 归一化（`normalizeObjectConfig`）在组件内完成（第 232 行起）。

结论（事实）：storage 已集中自声明默认行为，可作为正确模板。

### 3.5 observability（正确模板）

`internal/kernel/app/observability/config.go`：

- 自声明 `DefaultConfig()`（第 37 行）“……默认关闭外部 exporter 的安全配置”，内部集中声明；`defaults{}.Defaults` 与解码都以 `DefaultConfig()` 为基准（第 45、108 行）。

结论（事实）：observability 已集中自声明默认配置，作为正确模板。

### 3.6 小结

| 组件 | 复用 pkg 默认 | 备注 |
| --- | --- | --- |
| logger | 是（`pkg/logger.DefaultConfig()`） | 需改为应用组件自声明 |
| database | 是（`pkg/database.DefaultConfig()`） | 需改为应用组件自声明 |
| cache | 部分（`redisstore.DefaultTagPrefix`） | 需把 TagPrefix 默认值移到组件 |
| storage | 否（自声明） | 正确模板 |
| observability | 否（自声明 `DefaultConfig()`） | 正确模板 |
| i18n | 是（`pkg/i18n.DefaultConfig()`） | 本任务核心 |

## 4. 推断

1. 存在两类问题：(a) 直接复用 `pkg/*.DefaultConfig()`；(b) 复用 `pkg/*` 内部具体 store 的默认常量（如 `redisstore.DefaultTagPrefix`）。二者都让应用层配置隐式依赖通用库默认值。
2. 正确边界：`pkg/*` 提供通用能力与**基础**默认行为（作为库自己的最低可用默认）；`kernel/app/*` 应根据应用环境、装配与业务显式声明自己的默认配置与常量，并在需要时向使用者提供显式注入入口。
3. storage/observability 已示范正确形态：组件内集中 `defaults`/`DefaultConfig()`，`defaults{}.Defaults` 与 decode 都以组件内默认值为基准。
4. 门禁化：应在架构测试或规范性检查中加入「`kernel/app/*` 不得在默认配置路径处直接调用 `pkg/*.DefaultConfig()`/`pkg/*` 内部默认常量」的规则，避免再次引入。

## 5. 适用与不适用场景

- 适用：所有 `kernel/app/*` 组件默认值/常量职责边界；新增底层组件时的规范。
- 不适用：改变 `pkg/*` 通用能力契约；引入外部配置平台；重构 Kernel 生命周期框架。

## 6. 局限与剩余未知

- 未运行测试/生成；只读复核。
- `redisstore.DefaultTagPrefix` 的具体合理应用默认值需在设计阶段定案并在组件集中声明。
- 若某个 `pkg/*` 默认值确实代表「通用最低可用」且应用允许继承，需要明确例外与审批路径，避免误伤。

## 7. 对 032 的影响

- 计划需覆盖：i18n 组件集中声明默认配置 + `./locales`；logger/database/cache 对齐为应用组件自声明；storage/observability 保持并纳入文档；架构门禁防止回退。
- 权威文档（`pkg/README.md` 边界小节、Kernel App 组件开发说明、应用模块开发指南、配置说明）需同步「应用层不得隐式依赖通用库默认值」。
- 研究门禁通过。

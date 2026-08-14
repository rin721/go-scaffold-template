# R001：当前脚手架分发与消费边界

## 1. 研究问题

当前仓库是否已经具备作为外部 Go library、可复制 template 或 generator 产品被创建、定制和升级的真实路径？

## 2. 方法与范围

- 从 `go.mod -> cmd/app -> internal/composition -> internal/kernel/internal/module -> pkg` 追踪真实 composition root 和依赖方向。
- 检索固定 module path、应用名、环境变量前缀和 Todo 资产。
- 检查 generator/template manifest、Git tag、升级脚本和外部消费者测试是否存在。
- 使用 `go help gopath` 复核当前 Go tool 对 `internal` 的导入限制。

快照为 `af7fdadc`。本研究没有生成项目、修改模块或启动服务。

## 3. 可复核事实

### 3.1 完整 Runtime 不是外部公共库

- 根 module 固定为 `github.com/rin721/go-scaffold2`。
- `cmd/app/main.go` 直接导入 `internal/composition` 和 `internal/kernel/logging`；生命周期、配置协调、App Definition/Plan 和 Todo 组合均在 `internal/**`。
- Go tool 明确规定：`internal` 目录及其子目录只能被其父目录树内的代码导入。因此另一个 module 不能直接复用这条完整运行链。
- `pkg/boundary_test.go` 反向保证 `pkg/**` 不导入 `internal/**`。这使 `pkg` 具备独立能力库边界，但不等于公开了应用 Runtime 与 Composition。

### 3.2 当前仓库也不是可参数化模板

- 仓库跟踪文件中有 132 个文件包含固定 module path、`go-scaffold2` 或 `APP_` 身份；88 个 Go 文件包含当前 module import。
- `cmd/app` 固定声明应用名 `go-scaffold2`、配置路径 `config.yaml` 和环境变量前缀 `APP_`。
- Todo 业务模块、配置、migration、HTTP/CLI binding 与文档处于默认运行链，不存在“包含 Todo”与“空白服务”的生成选项。
- 跟踪文件中没有 generator、template manifest、变量 schema、golden output、生成文件标记或升级迁移工具。

### 3.3 当前没有可验证的版本与升级产品

- `git tag --list` 为空，没有可供外部 Go module 选择的 release tag。
- 没有外部 consumer module 测试，没有从版本 A 到版本 B 的升级样例，也没有定义 Runtime API 兼容规则。
- 没有区分“生成器拥有、可重新生成”和“应用拥有、绝不能覆盖”的文件清单。

## 4. 推断

1. **library-only 不能沿用当前目录直接发布。** 要成为 framework library，必须把最小 Runtime 契约设计成公共包；机械搬出整个 `internal/kernel` 会一次性制造过大的稳定 API 面。
2. **template-only 能最快创建副本，但不能自然升级。** 复制后代码由新项目拥有；若继续用上游 Git merge，会把脚手架历史和业务历史耦合，并且 GitHub template 创建的仓库与模板历史无关。
3. **generator-only 能解决身份参数化，不自动解决演进。** 生成后 Runtime 仍是复制代码；除非定义 schema migration 和所有权，否则重新生成会覆盖用户修改。
4. **组合模式值得优先验证但尚未成立。** 窄 Runtime 依赖可由 Go module 升级，应用装配和业务文件由消费者拥有；代价是必须严控公共 API，并分别治理 Runtime 版本和 template schema 版本。

## 5. 适用与不适用

适用于 020 的方案比较、隔离消费者验证和 ADR。它不证明任何候选已经可用，不授权移动 `internal`，也不代表当前 `pkg/**` 已经承诺 v1 兼容。

## 6. 局限与剩余未知

- 尚未在另一个 module 中执行创建、编译、定制和升级实验。
- 尚未量出实现最小公共 Runtime 所需的具体符号和迁移成本。
- 尚未决定是否使用自研 generator；Go 官方 `gonew` 仍明确是实验性工具。
- 真实 CI、release tag、Windows/Linux 生成一致性和离线行为均未验证。

## 7. 对 020 的影响

研究门禁足以进入“隔离验证后决策”的计划，但不足以直接采用组合模式。验证必须在 `_tmp`/`tmp` 消费者中完成，不允许为了试验先污染生产源码。

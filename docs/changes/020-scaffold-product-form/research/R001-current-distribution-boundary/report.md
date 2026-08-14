# R001：当前脚手架分发与消费边界

> 后续决策：用户于 2026-08-15 选择完整源码复制模型。GitHub canonical 仓库随后改名为 `go-scaffold-template`；本报告仍保留 `af7fdadc` 时 `go-scaffold2` module 的真实快照，当前 repository identity 见 [021-R001](../../../021-repository-identity-migration/research/R001-current-repository-identity/report.md)。

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

### 3.2 当前仓库尚不是受验证的复制基线

- 仓库跟踪文件中有 132 个文件包含固定 module path、`go-scaffold2` 或 `APP_` 身份；88 个 Go 文件包含当前 module import。
- `cmd/app` 固定声明应用名 `go-scaffold2`、配置路径 `config.yaml` 和环境变量前缀 `APP_`。
- Todo 业务模块、配置、migration、HTTP/CLI binding 与文档处于默认运行链，不存在“包含 Todo”与“空白服务”的生成选项。
- 跟踪文件中没有经过验证的复制包含/排除清单、一次性身份迁移清单、baseline 来源记录或升级迁移政策。

### 3.3 当前没有可验证的版本与升级产品

- `git tag --list` 为空，没有可供外部 Go module 选择的 release tag。
- 没有外部 consumer module 测试，没有从版本 A 到版本 B 的升级样例，也没有定义 Runtime API 兼容规则。
- 没有区分“生成器拥有、可重新生成”和“应用拥有、绝不能覆盖”的文件清单。

## 4. 推断

1. **完整复制与当前 `internal` 边界相容。** `pkg`、Kernel App 和 composition 一起进入目标 module 后，不需要暴露公共 Runtime API。
2. **复制后全部代码必须归新项目。** 不存在 generator-owned 文件，也不能把上游 merge 描述成可靠升级通道。
3. **身份迁移仍需严格验证。** 当前固定 module、应用名、`APP_` 前缀和 Todo 资产不能靠无边界全局替换处理。
4. **上游改进只能人工传播。** baseline tag、来源记录、release note、安全公告和逐版本迁移指南是该模型需要补齐的治理能力。

## 5. 适用与不适用

适用于 020 的产品边界依据、隔离复制验证和 ADR。它不证明复制流程已经可用，不授权移动 `internal`，也不代表复制后的项目与源仓库存在升级关系。

## 6. 局限与剩余未知

- 尚未在隔离目录的独立 module 中执行完整复制、身份迁移、编译测试和 Todo 移除。
- 真实 baseline tag、Windows/Linux 复制一致性和人工升级资料均未验证。

## 7. 对 020 的影响

研究事实足以支持用户选择的 copy-owned 方向，但复制基线是否可用仍需隔离验证。验证必须在 `tmp/` 副本中完成，不允许为了试验先修改生产源码。

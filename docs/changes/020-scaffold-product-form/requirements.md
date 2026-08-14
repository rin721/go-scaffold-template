# 需求：复制型脚手架产品形态

## 1. 已确认目标

用户已决定将 `go-scaffold2` 设计为完整源码脚手架：开发者复制一个版本化源码快照，在本地一次性迁移项目身份，然后由新项目独立拥有和演进全部代码。研究依据见 [R001](research/R001-current-distribution-boundary/report.md) 与 [R002](research/R002-go-distribution-versioning/report.md)。

这不是 library，也不是 generator。脚手架只承诺“复制时的基线质量和迁移说明”，不承诺复制后自动同步上游。

## 2. 使用场景

1. 开发者选择一个明确版本的 `go-scaffold2` 源码快照。
2. 将源码复制到目标目录，不携带源仓库 Git 身份、运行数据、凭据或临时文件。
3. 一次性修改 module path、应用名、描述、可执行名、配置文件名和环境变量前缀。
4. 选择保留 Todo 作为参考，或按受验证清单删除 Todo。
5. 完成编译、测试、配置初始化和身份残留检查后，建立新项目自己的版本历史。

## 3. 功能需求

| ID | 需求 |
| --- | --- |
| `FORM-001` | 唯一产品形态是完整源码复制；不得保留 generator、公共 Runtime 依赖或并行创建入口。 |
| `BASELINE-001` | 可复制输入必须对应明确 Git commit/tag/release，不允许用未标识工作树声称稳定基线。 |
| `COPY-001` | 复制范围必须显式包含源码、测试、配置示例、文档和必要 CI，同时排除 `.git`、运行数据、凭据、缓存、构建产物与任务临时档案。 |
| `OWNERSHIP-001` | 复制成功后全部文件归新项目；不存在脚手架保留所有权或后续自动覆盖的文件。 |
| `IDENTITY-001` | 一次性身份迁移至少覆盖 module path、Go imports、应用名、描述、二进制名、配置文件名、环境变量前缀、README 和架构测试常量。 |
| `IDENTITY-002` | 身份迁移必须有精确清单和残留扫描；不得依赖无边界的全局字符串替换。 |
| `ASSEMBLY-001` | `pkg -> internal/kernel/app -> internal/kernel/composition -> internal/composition -> cmd/app` 在目标 module 中保持同一依赖方向和生命周期语义。 |
| `EXAMPLE-001` | Todo 明确标记为示例；保留路径和完整移除路径都必须有文档与验证，不能留下失效配置、migration、路由或 CLI。 |
| `VERIFY-001` | 新项目必须完成 `go mod tidy -diff`、build、test、vet、配置初始化、身份残留和 Git 跟踪范围检查。 |
| `PROVENANCE-001` | 新项目必须记录来源 baseline 版本/commit 和复制日期，但该记录不建立运行期依赖。 |
| `UPGRADE-001` | 上游改进通过 release notes、安全公告和按版本编写的人工迁移指南传播；不得承诺自动 merge、重新生成或无冲突升级。 |
| `SECURITY-001` | 严重安全修复必须给出受影响 baseline、修复 commit/release、变更文件和验证命令，使已复制项目可以人工迁移。 |
| `PORTABLE-001` | 复制与身份迁移说明必须适用于 Windows 和 Linux；未验证的平台需明确标记。 |

## 4. 质量与治理约束

- 所有身份值必须有语义归属，不能用 `replace all go-scaffold2` 破坏历史文档、URL 或不相关文本。
- 复制产物必须是独立 Go module，不通过 `replace`、workspace 或相对路径依赖源脚手架。
- 新项目允许自由修改 `pkg` 和 `internal`；脚手架文档不得暗示这些文件以后仍由上游控制。
- 复制失败或身份迁移未完成时，不得把目录标记为可交付项目。
- 验证用副本只能位于 Git 忽略目录，不进入当前仓库提交。

## 5. 非目标

- 不实现项目生成器、模板变量 schema、生成文件 manifest 或 `scaffold create/new/init`。
- 不发布可导入的 Kernel Runtime，不移动 `internal/kernel/app` 到公共包。
- 不解决任意历史副本的自动升级、双向同步或三方 merge。
- 不在 020 中实施 019 的 OpenAPI、错误协议、认证、管理面或可观测性能力。
- 不在未确认前修改 Go 源码、配置、依赖、脚本、CI 或发布状态。

## 6. 验收标准

1. 从明确 commit 创建一个 Git 忽略的完整副本，排除项清单可复核。
2. 副本改为不同 module/app/env identity 后，不残留错误的源项目运行身份或 Go import。
3. 副本不引用源工作区路径，也不依赖源 module；完整 build/test/vet/config init 通过。
4. 底层装配链和资源生命周期测试在副本中保持通过。
5. Todo 保留路径通过；Todo 移除路径要么通过，要么准确形成后续最小改造任务，不用占位代码伪装成功。
6. 文档明确复制后全量归新项目以及无自动上游升级承诺。
7. ADR/设计记录确定 baseline、provenance、安全修复和人工迁移政策；正式复制指南与发布能力由后续独立任务实施。

# R018：CLI 与默认文件契约研究

## 1. 研究问题与本地缺口

本地事实不是“缺一个 CLI 框架”，而是：完整 command tree 冲突、mode/required capability/side effect、nil context、version/config exit path 和 Bootstrap-only composition 没有形成项目契约；默认文件虽有较强安全写入，但 runtime typed round-trip 与跨平台保证边界不完整。

因此研究只比较一手实现中与这些缺口直接相关的做法，不罗列框架排名。

## 2. Cobra v1.10.2

主源：

- [Cobra v1.10.2 User Guide](https://github.com/spf13/cobra/blob/v1.10.2/site/content/user_guide.md)
- [Cobra v1.10.2 command.go](https://github.com/spf13/cobra/blob/v1.10.2/command.go)

真实做法：

- parent 通过 `AddCommand` 显式建立 nested tree；`RunE` 将业务 error 返回给调用方，而不是要求 command 自行退出进程。
- positional args 通过 `Args` validator 显式校验；文档提供 Exact/Range/MatchAll 等组合，说明 nil validator 不能被项目误认为“无参数”。
- help group 需要 parent `AddGroup`，child 用 GroupID 关联，显示顺序按显式添加顺序。
- `SetIn/SetOut/SetErr` 与 inheritance 允许调用方管理标准流；`SilenceUsage/SilenceErrors` 允许项目在最外层统一输出。
- Cobra 解决命令树执行机制，但不表达项目的 Bootstrap/Service mode、最小资源集、文件副作用或业务退出类别。

适用判断：继续把 Cobra 留在 `pkg/cli` Adapter 内是合理的；项目应在构造 Cobra tree 前冻结自己的 command contract。不能以“Cobra 运行时也会报某些冲突”为由放弃静态全树治理。

## 3. Kubernetes cli-runtime v0.34.1

主源：[IOStreams source](https://github.com/kubernetes/cli-runtime/blob/v0.34.1/pkg/genericiooptions/io_options.go)

真实做法：`IOStreams` 用 `In io.Reader`、`Out io.Writer`、`ErrOut io.Writer` 统一标准流，源码明确其价值是 embedding 与 unit testing，并提供 test buffer helper。

适用判断：这支持本地 `runMain(stdin, stdout, stderr, args)` 和 `RunWithIO` 的方向。无需依赖 Kubernetes 包；保留项目自有小结构即可。关键是不允许 nil 被静默解释为不同环境下的 OS stream 或 discard。

## 4. Go `os` 文件与退出语义

主源：[Go os package](https://pkg.go.dev/os)

真实保证：

- `os.Exit` 立即终止，defer 不执行。因此清理必须在返回 exit code 的函数内完成，只有最外层 `main` 调用 Exit。
- `os.CreateTemp(dir, pattern)` 在指定目录创建唯一临时文件，初始权限 `0600`（umask 前），调用者负责清理。
- `os.Rename` 可以替换既有非目录目标，但官方明确非 Unix 平台即使同目录也不保证原子。

适用判断：本地薄 `main` 与同目录 temp/显式 cleanup 可保留。当前 Windows 使用专用 replace、Unix 使用 rename，比直接跨平台调用 Rename 更明确；仍必须把“原子/持久化”声明限定到实际实现、OS 与文件系统测试，不能从单次 `File.Sync` 推导任意 crash durability。

## 5. 可选方案

| 方案 | 收益 | 代价与风险 | 判断 |
|---|---|---|---|
| 依赖 Cobra 执行期校验全部契约 | 代码少 | mode、资源、副作用、退出和部分冲突不可表达 | 不足 |
| 更换 CLI 框架 | 可能获得另一组默认行为 | 没有证据能解决本地核心问题；迁移扩大范围 | 不推荐 |
| 每个命令接收完整 Application | 编写方便 | 隐藏依赖并可能启动无关资源 | 拒绝 |
| 项目 registry 冻结 + Cobra Adapter + mode-specific composition | 复用现有实现，契约可静态测试 | 需要拆分 Bootstrap 组装和补验证 | 推荐 |

## 6. 对当前架构的建议

- **保留**：`runMain` 返回 code 后最外层 Exit、显式 I/O、Cobra Adapter、typed CLI errors、ordered command contributions、同目录 temp/no-overwrite/force 显式发布。
- **补齐**：command path/group/alias/flag 冲突，明确 positional policy，Bootstrap/ApplicationCommand/Service mode，required narrow capabilities，side-effect class，version 和 ExitConfig 的真实路径。
- **优化**：nil context 直接失败；help/version/parse 先于完整 composition；default generation 只组装 config registration，并先做 strict typed round-trip。
- **不替换**：没有证据要求更换 Cobra 或默认文件 writer；问题主要位于项目契约与 composition ordering。

## 7. 验证方法与证据强度

证据强度：Cobra/Kubernetes 采用固定 tag 源码，Go `os` 为官方文档，足以支持行为判断；它们不证明本地目标已实现。

验证要求：完整树 conflict matrix、I/O 黑盒、0/1/2/3/130 exit matrix、Bootstrap no-resource spy、config init fault injection、并发 no-overwrite、force 平台测试、目标/temp 内容与权限测试，以及取消/cleanup error chain。

## 8. 未决问题

- `ApplicationCommand` 是否有真实需求，以及它需要哪些窄 capability；未确认前不冻结 API。
- force 并发是否支持，若支持采用何种 serialization/last-writer contract。
- 需要支持哪些 OS/filesystem，是否要求目录 fsync 级 crash durability。
- help group、completion 与 shell-specific behavior 是否进入首轮验收。

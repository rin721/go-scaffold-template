# CLI 与默认配置生成契约

## 1. 结论

实施保留了 CLI 的项目自有封装、显式 I/O、typed error 和安全文件写入，并补齐 Bootstrap composition、命令 mode/副作用/位置参数、完整树冲突冻结，以及默认配置与运行期同一 typed binding/validation contract 的回环校验。

本设计不指定更换或继续使用某个命令框架。Cobra 仍可作为内部 Adapter；项目契约由 `pkg/cli` 与 Application composition 拥有，框架行为不能代替项目治理。

状态：**已实施设计记录**。实施快照见 [R021](../research/R021-foundation-closure-implementation/report.md)；实施前现状见 [契约清单](../requirements/foundation-contract-catalog.md) 的 `BOOT-*`、`CLI-*`。

## 2. 实施前真实调用链

```text
main
  -> runMain(stdin, stdout, stderr, args)
     -> signal context + baseline Logger + logging Manager
     -> process.run
        -> Loader(FileSource, EnvSource)
        -> Kernel
        -> composition.Compose(Plan + Defaults + optional CLI)
        -> args 非空: CLI.RunWithIO             # 不 Kernel.Start
        -> args 为空: Host.Run(Kernel + Tasks)   # 长期运行
     -> execute(error -> stderr + exit code)
  -> os.Exit(code)
```

实施前已验证事实：

- `config init` 不启动 Database、Cache、Storage 或 HTTP；帮助、版本和解析错误也不启动 Kernel。
- CLI 分支仍创建 Loader、Kernel 并执行完整 `composition.Compose`，因此“没有启动资源”不等于“只构造所需能力”。
- stdin/stdout/stderr 由进程入口注入，CLI Adapter 不需要直接拥有 OS 全局流。
- `pkg/cli` 支持 version，但当前 `cmd/app` 没有填充 version。
- `ExitConfig=3` 已定义，但 `config init` 失败当前走 `CommandError`，实际返回 1。
- 顶层重名已校验；nested name/alias、group、flag name/shorthand 等冲突没有统一冻结门禁。
- nil `Context` 被静默替换为 `context.Background()`；nil positional validator 接受任意参数；`Get*` 缺失或类型错误返回零值。

## 3. 运行模式契约

### 3.1 模式

| 模式 | 用途 | 允许构造 | 禁止默认构造 | 完成语义 |
|---|---|---|---|---|
| `Bootstrap` | help、version、completion、默认配置生成、静态校验 | command registry、配置节 registration、Default manager、显式 I/O | Kernel runtime、DB、Cache、Storage、HTTP、Supervisor | one-shot，命令完成即退出 |
| `ApplicationCommand` | 未来真实业务命令 | 仅命令声明的窄 capability 与必要只读配置 | 未声明资源、全量 Capabilities、长期 listener | one-shot；存在真实需求后才确认 API |
| `Service` | 无参数默认服务或显式 serve | 完整已确认 Application graph、Kernel、Supervisor、HTTP/runner | 未注册的旁路资源 | 长期运行，signal/runtime failure 后排空停止 |

模式是项目专用类型，不是布尔值，也不从命令是否恰好带参数隐式推断。当前只确认 `Bootstrap` 和默认 `Service` 的问题；`ApplicationCommand` 仍为 **尚未确认**，不得预建业务命令 SDK。

### 3.2 命令声明的最小信息

每条命令在执行前必须能被静态读取并冻结：

```text
stable command path
group and aliases
positional argument policy
flags and shorthands
execution mode
required narrow capabilities
side-effect declaration
exit/error categories
```

这不要求定义一个万能 interface。简单命令仍可用 `CommandSpec` 值对象和函数；“required capabilities”必须是该命令真正使用的窄构造参数，不得是 `Capabilities` 大对象、Container 或运行时查询器。

### 3.3 注册、分组、发现和冲突

冻结前按规范化 command path 建唯一索引，并验证：

1. parent/child path 非空且可达；
2. 同一 parent 下 name 与 aliases 两两不冲突；
3. GroupID 已声明且组 ID 唯一；
4. local/persistent flag name 与 shorthand 在有效继承范围不冲突；
5. help、version 等保留入口不被覆盖；
6. 每条可执行命令有明确 positional policy、mode 和 side-effect class；
7. 同一注册源不得返回 nil command，注册集合顺序稳定。

框架在构树/执行时仍可做自己的校验，但启动副作用前必须先通过项目级完整校验。发现、help 排序和冲突结果只依赖冻结 registry，不依赖包扫描、`init` 自注册或 map 迭代顺序。

## 4. 执行、I/O、错误与退出码

### 4.1 执行前置条件

- `context.Context`、stdin、stdout、stderr 的 owner 是进程/Application；nil context 直接返回内部契约错误。
- 允许明确使用 `io.Discard`，但不得用 nil 隐式表示 discard。
- parse/help/version 在任何资源构造前完成；仅选中命令后才组装其 mode 和窄依赖。
- 参数和 flag 绑定必须区分“未提供”与“显式零值”；需要此语义时用 `IsFlagChanged` 或 typed option。

### 4.2 退出分类

| 类别 | 退出码 | stderr/stdout | 错误链 |
|---|---:|---|---|
| 成功/help/version | 0 | 正常结果与 help/version 写 stdout | 无 error |
| 命令执行/运行失败 | 1 | 简洁错误写 stderr | 保留 cause |
| 参数、命令或 flag 使用错误 | 2 | 使用错误写 stderr；是否附 usage 由统一策略决定 | typed UsageError |
| 配置 contract、绑定或校验错误 | 3 | 脱敏错误写 stderr | typed ConfigError，保留 source/section/stage/cause |
| 用户取消 | 130 | 不重复打印多层日志 | `errors.Is(context.Canceled)` 成立 |

进程退出码保持可移植范围。真正的资源清理发生在 `runMain` 返回前；最外层 `main` 才调用 `os.Exit`，避免 defer 被跳过。

### 4.3 副作用声明

副作用至少分类为：`None`、`FileCreate`、`FileReplace`、`ExternalWrite`、`LongRunningListener`。声明用于 composition 选择和验收，不授权副作用本身。

命令必须在执行结果中提供可验证信息，例如默认配置生成返回 resolved path、format、是否替换和参与的 section IDs。日志不是成功凭证。未来外部写入命令必须单独确认幂等、补偿和凭据策略；当前 012 不设计它们。

## 5. 默认配置生成契约

### 5.1 唯一声明与聚合

每个配置节由语义 owner 声明。一个注册单元把以下内容关联起来：

```text
SectionID + ConfigPath
  -> safe Defaults projection
  -> strict typed binding
  -> semantic validation
  -> sensitivity metadata
```

Default manager 只按已冻结 registration 顺序聚合，不扫描包，不允许重复 ID、重叠 path、nil contract 或 owner 不明。生成、实际加载和运行期绑定共享同一 registration；禁止再维护独立模板、独立 runtime defaults 和第三套文档 defaults。

### 5.2 生成流水线

```text
冻结 registrations
  -> 调用各 owner 的 Defaults(ctx)
  -> 校验 ID/path/object/value 和冲突
  -> 聚合成完整 canonical candidate
  -> 每个 section 执行 strict typed binding
  -> 执行字段与跨字段 semantic validation
  -> 检查 sensitivity/default safety policy
  -> 在内存编码为 YAML/JSON
  -> 同目录创建 0600 临时文件
  -> 完整写入 -> Sync -> Close
  -> no-force exclusive publish 或 force explicit replace
  -> 失败清理临时文件
  -> 返回结构化结果
```

资源探测不属于默认生成：DB ping、Cache connect、Storage remote access、HTTP bind 都不得执行。默认配置只证明结构和语义有效，不证明外部环境可达。

### 5.3 路径、格式与权限

- 格式由显式参数或目标扩展名唯一确定；不识别的扩展名在写入前失败。
- 目标路径解析为绝对路径后进入结果与诊断；相对路径相对于调用方明确的 working directory。
- 新建父目录权限为 `0700`，配置文件为 `0600`；既有目录权限不静默修改。
- 符号链接、reparse point、跨文件系统与特殊文件目标必须拒绝或在明确平台契约中处理，不能当普通文件覆盖。
- 当前同目录临时文件、Sync/Close、Unix rename、Windows replacement 的实现可以保留；“原子”只在已验证平台和文件系统语义内声明。Go 通用 `os.Rename` 并不保证非 Unix 原子替换，不能把实现宣传成无条件 crash-durable。

### 5.4 覆盖与并发

- 默认策略是 `NoOverwrite`：目标存在返回 typed `ErrTargetExists`，原文件字节和权限不变。
- 覆盖必须通过显式 `--force`/`ReplaceExisting` 表达；不得根据交互终端、环境变量或隐藏默认自动覆盖。
- no-force 并发生成同一目标，验收要求至多一个发布成功；其他调用得到 exists/conflict，不得部分覆盖。
- force 的并发竞争若未定义 last-writer policy，则明确标为不支持并在构造期/锁策略中拒绝，而不是依赖偶然调度。

### 5.5 失败、取消与清理

任一 Defaults、binding、validation、encode、mkdir、create temp、write、short write、sync、close、publish 或 cleanup 失败都必须：

1. 保留主错误和清理错误；
2. 不把目标报告为成功；
3. 在 publish 前保持既有目标不变；
4. 删除本次拥有的临时文件；
5. 让 `context.Canceled`/timeout 可识别；
6. 不在错误、日志或结果中输出 secret 值。

如果 publish 已完成而后续目录持久化/诊断步骤失败，错误必须明确标识“结果可能已发布”，不能谎称完全未写入。

### 5.6 敏感默认值

- Secret、Token、password、private key、完整 DSN 不得生成真实凭据。
- 可安全留空时使用显式空值，并由字段契约说明“未配置”；不可留空时使用明显不可运行的 placeholder，并在 typed validation 中区分“模板占位”与运行期有效凭据。
- 禁止生成看似可用的通用密码，禁止从当前环境读取凭据写入模板，禁止把隐藏代码默认伪装成配置缺失后的正常值。
- sensitivity metadata 同时驱动默认安全检查、Snapshot redaction 和 diagnostics；启发式 key 匹配只作为纵深保护。

## 6. 方案比较

| 方案 | 收益 | 代价/风险 | 结论 |
|---|---|---|---|
| 保持当前 `args` 分支和完整 Compose | 改动最少 | Bootstrap 仍依赖完整应用图，新增 command 易误启资源 | 不满足 |
| 让所有命令拿完整 Application/Capabilities | 接入方便 | 隐藏依赖、扩大副作用、难以静态验收 | 拒绝 |
| 完全换 CLI 框架 | 可能获得部分校验/生成能力 | 不能自动解决项目 mode、资源、退出和副作用契约，迁移无直接收益 | 不推荐 |
| 保留 Adapter，增加冻结 registry、显式 mode 和最小 composition | 与当前实现兼容，可针对已证实缺口测试 | 需拆分 Bootstrap composition，补 contract validation | 推荐 |

## 7. 验收门禁

- help、version、unknown command、invalid flag、`config init` 均不构造或启动 DB/Cache/Storage/HTTP。
- 完整命令树所有 name/alias/group/flag/shorthand 冲突在任何执行副作用前失败。
- nil context、nil/空/重复 command、未声明 positional policy、未知 mode/side-effect class 均在冻结期失败。
- stdin/stdout/stderr 可注入并有单元测试；help/version 只写 stdout，错误只在进程边界写 stderr 一次。
- 退出码 0/1/2/3/130 的真实调用路径均有黑盒测试。
- 默认配置对每个 owner 完成同一 strict binder/validator round-trip 后才写文件。
- no-force、force、取消、短写、Sync/Close、publish、cleanup、并发目标与敏感 placeholder 都有故障注入测试。
- 测试证明 Bootstrap 模式没有资源 listener、连接、goroutine 或外部网络副作用。

这些门禁通过只证明 CLI/默认配置底层契约闭合，不解锁业务 Command 设计；后者仍需要真实用例和窄依赖需求。

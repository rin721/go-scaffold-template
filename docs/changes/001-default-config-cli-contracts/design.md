# 开发设计：默认配置契约与可选 CLI 能力

## 1. 总体方案

默认配置属于能力定义的静态契约，不依赖运行中的实例。`kernel.Register` 在成功登记 Definition 时返回稳定 Access 和不可变默认配置 Binding；composition 显式收集 Binding，构造配置管理，并按 options 决定是否构造启动前 CLI。

```text
Database Definition
  -> kernel.Register
     -> Database Access
     -> Default Binding ----+
                            +-> config.DefaultManager
                                      |
                                      +-> kernel CLI 的 config init Contract
                                                  |
ComposeOptions.CLI != nil ------------------------+-> pkg/cli.App
```

配置生成不调用 `Decode`、`Build`、`InstanceHooks`、`Kernel.Start` 或稳定 Access。CLI 也不进入 Kernel 候选事务和 Supervisor 生命周期，因此首次没有配置文件时仍可运行 `config init`。

## 2. 默认配置文档模型

在 `internal/kernel/config/defaults.go` 新增项目自有、保持顺序的格式无关模型。核心类型固定为：

```go
type Control uint8

const (
	Continue Control = iota
	Abort
)

type DefaultContract interface {
	Defaults(context.Context) (Object, Control, error)
}

type Binding struct {
	CapabilityID string
	ConfigPath   string
	Contract     DefaultContract
}

type Object []Field

type Field struct {
	Name  string
	Value Value
}

type Value interface {
	isConfigValue()
}
```

`Value` 不允许能力包自行实现，统一通过 `config` 提供的构造函数创建：

- `String(string)`、`Bool(bool)`、`Duration(time.Duration)`、`Null()` 返回 Value；`Number(string) (Value, error)` 在创建时拒绝非法十进制数。
- `ObjectValue(Object)`、`List(...Value)`。
- `FieldOf(name, value)` 用于声明有序字段。

`Number` 接收十进制字符串并在构造时使用 `math/big` 校验；编码时作为 JSON/YAML 数字而不是字符串。`Duration` 在构造时保存 `time.Duration.String()` 结果。Object 使用切片而不是 map，编码器不得重新排序字段。

通用结构校验只处理：空字段名、同一 Object 的重复字段、nil Value、非法 Number、未知内部 Value 类型和递归构造错误。它不检查字段的业务含义。

## 3. Definition 与登记结果

`internal/kernel/definition.go` 将 Definition 和 Register 单轨改为：

```go
type Definition[C, T any] struct {
	ID         ID
	ConfigPath string
	Decode     Decoder[C]
	Defaults   config.DefaultContract
	Builder    Builder[C, T]
	Hooks      InstanceHooks[T]
}

type Registration[T any] struct {
	Access   *Handle[T]
	Defaults config.Binding
}

func Register[C, T any](
	runtime *Kernel,
	definition Definition[C, T],
) (Registration[T], error)
```

- `Register` 校验 Defaults 不是 nil，但不调用契约。
- 只有 `runtime.register` 成功后才返回 Binding，避免失败 Definition 进入生成清单。
- Binding 的 CapabilityID 和 ConfigPath 只能由 Definition 复制，契约不能覆盖。
- 任意失败返回零值 `Registration[T]` 和保留错误链的错误。
- 不保留旧 `Register` 返回值或兼容函数；全部内部调用方和测试同一任务迁移。

## 4. 配置管理

### 4.1 接口

`internal/kernel/config/default_manager.go` 定义：

```go
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
)

type GenerateRequest struct {
	Path  string
	Force bool
}

type GenerateResult struct {
	Path         string
	Format       Format
	Capabilities []string
}

type DefaultManager interface {
	Generate(context.Context, GenerateRequest) (GenerateResult, error)
}

func NewDefaultManager(bindings ...Binding) (DefaultManager, error)
```

构造函数复制 Binding 切片并按传入顺序保存。构造阶段校验 CapabilityID、ConfigPath、Contract 以及配置路径所有权：路径按点号分段，空段、完全重复、祖先与后代重叠都拒绝。例如 `database` 与 `database.read` 不能由两个 Binding 分别拥有。

### 4.2 生成事务

`Generate` 要求非 nil context，并按以下固定顺序执行：

1. 清理并转为绝对目标路径，但错误信息不得泄露不必要的环境信息。
2. 根据扩展名选择 Format；空路径和未知扩展名立即失败。
3. 依次调用全部 Binding 的 `Defaults(ctx)`，每次调用前检查 context。
4. 对返回值应用控制矩阵：

| Control | error | 处理 |
| --- | --- | --- |
| Continue | nil | 校验并暂存配置段。 |
| Continue | 非 nil | 包装为能力默认配置错误并终止。 |
| Abort | 非 nil | 返回 `AbortedError`，同时匹配 `ErrAborted` 和原始原因。 |
| Abort | nil | 返回无效契约结果错误。 |
| 未知值 | 任意 | 返回未知 Control 错误。 |

5. 把各配置段按 ConfigPath 组装为有序根 Object，只在内存中编码。
6. 全部成功后才创建父目录并进入文件写入阶段。

`AbortedError` 包含 CapabilityID 和 Cause，实现 `Error`、`Unwrap`、`Is`；`errors.Is(err, config.ErrAborted)` 与 `errors.Is(err, cause)` 都必须成立。

### 4.3 编码与文件替换

- `default_encode.go` 负责 YAML/JSON 编码。不得把 Object 转换成无序 map；使用 `yaml.Node` 和按顺序写入的 JSON encoder 保持字段顺序。
- YAML 缩进两个空格；JSON 缩进两个空格；编码结果统一补且只补一个结尾换行。
- `default_file.go` 负责创建 `0700` 父目录、`0600` 临时文件、完整写入、Sync、Close 和失败清理；主错误与清理错误使用 `errors.Join`。
- Force 为 false 时使用排他创建，已有目标返回可识别的 `ErrTargetExists`，不得先删除或截断目标。
- Force 为 true 时临时文件必须位于目标目录：
  - Unix 在 `default_replace_unix.go` 使用同文件系统 `os.Rename`。
  - Windows 在 `default_replace_windows.go` 使用 `windows.MoveFileEx`，flags 固定为 `MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH`。
- 替换失败时原目标保持不变；临时文件清理失败必须与替换错误一起返回。

## 5. Database 默认配置契约

`internal/kernel/capability/database/database.go` 让现有无状态 `capability` 同时实现 `config.DefaultContract`。`Definition()` 设置 `Defaults: implementation`。

Database 返回 ConfigPath 下的有序 Object：

```text
engine: ""
driver: ""
dsn: ""
pool:
  maxOpenConns: 25
  maxIdleConns: 5
  connMaxLifetime: 30m0s
  connMaxIdleTime: 5m0s
pingTimeout: 5s
```

具体数值必须从 `pkg/database.DefaultConfig()` 读取，不能在 capability 中复制数字。空 engine、driver 和 DSN 是 Database 自己的安全骨架策略；Kernel 不验证或推广该策略。

## 6. 可选 CLI Contract

### 6.1 Contract 与 App 构造

新增 `internal/kernel/cli/contracts.go`：

```go
type Contract interface {
	Commands() ([]pkgcli.CommandSpec, error)
}

type ContractFunc func() ([]pkgcli.CommandSpec, error)

func NewApp(cfg pkgcli.Config, contracts ...Contract) (pkgcli.App, error)
```

`NewApp` 先调用 `pkgcli.NewApp`，再按 Contract 和 CommandSpec 顺序调用 `AddCommand`。nil Contract、Contract 错误、空命令和重复顶层命令均返回错误，不返回部分 App。这里复用项目自有 CLI 类型，不导入 Cobra、Bubble Tea 或 Lip Gloss。

CLI Contract 是 composition 可选绑定。Kernel Definition 不直接导入 `pkg/cli`；各能力的 composition 文件可在成功登记后额外贡献 CLI Contract。首版 Database 不贡献 CLI Contract。

### 6.2 config init 命令

`internal/kernel/cli/config.go` 提供：

```go
func ConfigCommands(manager config.DefaultManager) Contract
```

它返回顶层 `config` 命令和子命令 `init`。`init` 使用已有 `FlagSpec`：

- `output`：string，shorthand `o`，默认 `config.yaml`。
- `force`：bool，shorthand `f`，默认 false。

ArgsValidator 要求位置参数数量为零。Run 调用 `manager.Generate(ctx, request)`；成功后使用 `fmt.Fprintf(ctx.Stdout, "created default configuration: %s\n", result.Path)`，写 stdout 失败也必须返回。命令不访问 Kernel、Database Access 或文件系统实现细节。

## 7. Composition API 与调用流程

`internal/kernel/composition` 单轨改为：

```go
type Options struct {
	CLI *CLIOptions
}

type CLIOptions struct {
	App pkgcli.Config
}

type Capabilities struct {
	Database      databasecapability.Access
	Configuration config.DefaultManager
	CLI           pkgcli.App
}

func Compose(runtime *kernel.Kernel, options Options) (Capabilities, error)
```

nil CLI 表示明确禁用。组合顺序固定为：

1. `database.go` 登记 Database，返回 Access、Defaults Binding 和空 CLI Contract 列表。
2. `configuration.go` 用所有 Defaults Binding 构造 DefaultManager，并创建 `ConfigCommands` Contract。
3. `cli.go` 仅在 options.CLI 非 nil 时聚合配置命令与各能力可选 CLI Contract，构造 App。
4. `composition.go` 汇总完整 Capabilities；任一步失败都返回零值 Capabilities。

调用方在 Kernel 启动前自行决定是否进入 CLI：

```go
runtime, err := kernel.New(loader, kernel.Options{})
capabilities, err := composition.Compose(runtime, composition.Options{
	CLI: &composition.CLIOptions{App: cli.Config{Name: "app"}},
})
if err != nil {
	return err
}
if runCLI {
	return capabilities.CLI.Run(ctx, args)
}
host, err := kernel.NewHost(runtime, kernel.HostOptions{}, server)
return host.Run(ctx)
```

不新增具体 `cmd` 二进制或自动 `os.Args` 分流。CLI 是否运行由最终应用入口显式决定。

## 8. 预期文件改动

- Kernel 配置与登记：新增默认文档、管理器、编码和跨平台文件替换文件；修改 `definition.go` 及对应测试。
- Database：在现有 capability 文件中实现默认契约并补充契约行为测试。
- Kernel CLI：新增 Contract、App 组合和 `config init` 命令及测试。
- Composition：保持 `composition.go`、`database.go`、`configuration.go`、`cli.go` 按能力拆分，迁移所有 Compose 调用方。
- 文档与边界：更新根说明、Kernel/Capability/CLI 使用说明；扩展 AST 边界测试，禁止 capability 自动登记、扫描和反射。

## 9. 测试设计

- 默认模型：有序嵌套对象、列表、数字、duration、null、重复字段和非法值。
- 配置管理：Binding 顺序、路径冲突、Continue/Abort 矩阵、context 取消、错误链和无部分输出。
- Golden 文件：Database YAML/JSON 完整字节、两个空格缩进、字段顺序和结尾换行。
- 文件系统：父目录创建、权限、已有文件拒绝、Force 替换、写入/Sync/Close/替换/清理错误聚合。
- 平台替换：Unix 和 Windows helper 分别使用 build-tag 测试或平台可执行测试，Windows 验证已有目标替换。
- CLI：禁用与启用、`config init` 默认值、`-o/-f`、位置参数拒绝、stdout 失败、命令错误链和重复命令。
- Composition：空 Kernel 不自动组合、Database Binding 稳定、重复组合失败、失败返回零值 Capabilities。
- 迁移：生产代码和文档中的旧 `Compose(runtime)`、绕过 Defaults 的 Definition、旧 Register 返回方式引用归零。

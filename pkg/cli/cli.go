package cli

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
)

// ProgramOption 定制内置交互式首页的运行行为。
type ProgramOption interface {
	applyProgramOptions(*programOptions)
}

type programOptions struct {
	AltScreen  bool
	teaOptions []tea.ProgramOption
}

type programOptionFunc func(*programOptions)

func (f programOptionFunc) applyProgramOptions(options *programOptions) {
	f(options)
}

// WithoutAltScreen 禁用交互式首页的全屏渲染模式，便于测试或嵌入式终端使用。
func WithoutAltScreen() ProgramOption {
	return programOptionFunc(func(options *programOptions) {
		options.AltScreen = false
	})
}

// Config 描述一个 CLI 应用。
type Config struct {
	// Name 是应用名称，用于 Cobra 根命令和 TUI 首页标题。
	Name string
	// Version 是应用版本号，用于 --version 和首页标题。
	Version string
	// Description 是应用描述，用于 help 和首页副标题。
	Description string

	// Stdin、Stdout、Stderr 允许调用方注入 I/O，便于测试和嵌入式运行。
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Theme 控制默认 TUI 首页样式；为空时使用 DefaultTheme。
	Theme *Theme
	// ProgramOptions 控制内置交互式首页行为，不向调用方暴露底层 TUI 类型。
	ProgramOptions []ProgramOption

	// DisableInteractiveHome 在空参数时禁用交互式首页，恢复为 Cobra help。
	DisableInteractiveHome bool

	// GlobalFlags 声明根命令持久 flag，所有子命令都能读取这些 flag。
	GlobalFlags []FlagSpec
	// CommandGroups 声明命令可引用的稳定分组。
	CommandGroups []CommandGroup
}

// CommandGroup 声明一个稳定命令分组。
type CommandGroup struct {
	ID    string
	Title string
}

// App 是公开 CLI 边界，具体实现隐藏 Cobra 和 Bubble Tea。
type App interface {
	Name() string
	Version() string
	Description() string
	AddCommand(CommandSpec) error
	Run(context.Context, []string) error
	RunWithIO(context.Context, []string, io.Reader, io.Writer, io.Writer) error
}

// CommandFunc 处理已完成参数解析的命令调用。
type CommandFunc func(*Context) error

// ArgsValidator 在 flag 解析完成后校验位置参数。
type ArgsValidator func(*Context) error

// CommandSpec 声明一个命令，同时避免向调用方暴露 Cobra 细节。
type CommandSpec struct {
	// Name 是命令的唯一注册名。
	Name string
	// Use 覆盖 Cobra 的 usage 行；为空时使用 Name。
	Use string
	// Aliases 是命令别名。
	Aliases []string
	// Description 是命令短描述，用于列表和 help。
	Description string
	// Long 是命令长描述；为空时回退到 Description。
	Long string
	// Example 是命令示例文本。
	Example string
	// HomeLabel 覆盖交互首页中的显示文案；为空时使用 Name。
	HomeLabel string
	// HomeOrder 控制交互首页排序，数值越小越靠前。
	HomeOrder int
	// HomeHidden 表示该命令不出现在交互首页中，但仍可直接调用。
	HomeHidden bool
	// GroupID 引用 Config.CommandGroups 中的稳定分组。
	GroupID string
	// Mode 声明命令执行所需的装配模式。
	Mode CommandMode
	// SideEffect 声明命令允许产生的最大副作用类别。
	SideEffect SideEffect
	// Positional 声明位置参数策略。
	Positional PositionalPolicy

	// Flags 声明命令支持的 flag。
	Flags []FlagSpec
	// Args 校验位置参数；为空时不做额外校验。
	Args ArgsValidator
	// Run 是命令执行函数；为空时只输出该命令 help。
	Run CommandFunc
	// Commands 声明子命令。
	Commands []CommandSpec
}

// CommandMode 声明命令的装配边界。
type CommandMode uint8

const (
	CommandModeUnspecified CommandMode = iota
	CommandModeBootstrap
	CommandModeApplication
)

// SideEffect 声明命令执行可能产生的副作用类别。
type SideEffect uint8

const (
	SideEffectUnspecified SideEffect = iota
	SideEffectNone
	SideEffectFileCreate
	SideEffectFileReplace
	SideEffectExternalWrite
)

// PositionalPolicy 声明位置参数的接受策略。
type PositionalPolicy uint8

const (
	PositionalUnspecified PositionalPolicy = iota
	PositionalNone
	PositionalAny
	PositionalCustom
)

// FlagType 标识支持的 flag 值类型。
type FlagType int

const (
	FlagTypeString FlagType = iota
	FlagTypeInt
	FlagTypeBool
	FlagTypeStringSlice
)

// FlagSpec 声明一个命令行 flag。
type FlagSpec struct {
	// Name 是长 flag 名称。
	Name string
	// ShortName 是短 flag 名称。
	ShortName string
	// Shorthand 是 ShortName 的兼容别名，优先级更高。
	Shorthand string
	// Type 指定 flag 值类型。
	Type FlagType
	// Required 表示该 flag 是否必填。
	Required bool
	// Default 是 flag 默认值。
	Default interface{}
	// Description 是 help 中展示的 flag 描述。
	Description string
	// EnvVar 指定环境变量回退来源。
	EnvVar string
}

// Context 是传递给命令处理函数的执行上下文，包含参数、flag 和 I/O。
type Context struct {
	context.Context

	// CommandName 是当前执行的命令名。
	CommandName string
	// CommandPath 是包含父命令的完整命令路径。
	CommandPath string
	// Args 是解析 flag 后剩余的位置参数。
	Args []string
	// Flags 是解析后的 flag 值集合。
	Flags map[string]interface{}
	// ChangedFlags 标记本次调用中显式传入的 flag。
	ChangedFlags map[string]bool

	// Stdin、Stdout、Stderr 是本次命令调用使用的 I/O。
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// UI 提供通用交互式输入输出能力，业务层不需要直接读取 stdin。
	UI PromptUI
}

// GetString 返回字符串 flag 值；不存在或类型不匹配时返回空字符串。
func (c *Context) GetString(name string) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Flags[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt 返回整数 flag 值；不存在或类型不匹配时返回零。
func (c *Context) GetInt(name string) int {
	if c == nil {
		return 0
	}
	if v, ok := c.Flags[name]; ok {
		if i, ok := v.(int); ok {
			return i
		}
	}
	return 0
}

// GetBool 返回布尔 flag 值；不存在或类型不匹配时返回 false。
func (c *Context) GetBool(name string) bool {
	if c == nil {
		return false
	}
	if v, ok := c.Flags[name]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetStringSlice 返回字符串切片 flag 值；不存在或类型不匹配时返回 nil。
func (c *Context) GetStringSlice(name string) []string {
	if c == nil {
		return nil
	}
	if v, ok := c.Flags[name]; ok {
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}

// IsFlagChanged 返回指定 flag 是否由本次命令行显式传入。
func (c *Context) IsFlagChanged(name string) bool {
	if c == nil || c.ChangedFlags == nil {
		return false
	}
	return c.ChangedFlags[name]
}

// PromptUI 是业务命令可使用的通用交互抽象。
type PromptUI interface {
	Select(context.Context, string, []SelectOption) (string, error)
	Confirm(context.Context, string, bool) (bool, error)
	Input(context.Context, string, string) (string, error)
	Password(context.Context, string) (string, error)
	Info(string) error
}

// SelectOption 声明一个通用选择项。
type SelectOption struct {
	Value       string
	Label       string
	Description string
}

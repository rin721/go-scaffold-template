package cli

const (
	// ExitSuccess 表示成功退出。
	ExitSuccess = 0
	// ExitError 表示通用执行错误。
	ExitError = 1
	// ExitUsage 表示命令行参数或调用方式错误。
	ExitUsage = 2
	// ExitConfig 表示配置错误。
	ExitConfig = 3
	// ExitInterrupted 表示用户中断或取消。
	ExitInterrupted = 130
)

const (
	// ErrMsgCommandNotFound 是命令未找到的标准错误信息。
	ErrMsgCommandNotFound = "command not found"
	// ErrMsgInvalidArgs 是无效参数的标准错误信息。
	ErrMsgInvalidArgs = "invalid arguments"
	// ErrMsgMissingRequired 是缺少必填 flag 的标准错误信息。
	ErrMsgMissingRequired = "missing required flag"
	// ErrMsgDuplicateCommand 是重复注册命令名的标准错误信息。
	ErrMsgDuplicateCommand = "duplicate command name"
	// ErrMsgCancelled 是用户取消操作的标准错误信息。
	ErrMsgCancelled = "operation cancelled"
	// ErrMsgInvalidFlagValue 是 flag 默认值或输入值非法的标准错误信息。
	ErrMsgInvalidFlagValue = "invalid flag value"
)

package logger

// Environment 表示 logger 运行环境，用于选择适合开发或生产的默认格式。
type Environment string

const (
	// EnvironmentDevelopment 表示开发环境，默认使用 console 编码。
	EnvironmentDevelopment Environment = environmentDevelopmentValue
	// EnvironmentProduction 表示生产环境，默认使用 json 编码。
	EnvironmentProduction Environment = environmentProductionValue
)

// Encoding 表示日志输出编码格式。
type Encoding string

const (
	// EncodingConsole 表示面向本地阅读的 console 输出。
	EncodingConsole Encoding = encodingConsoleValue
	// EncodingJSON 表示面向采集系统的 JSON 输出。
	EncodingJSON Encoding = encodingJSONValue
)

// Config 定义 logger 构造参数。
//
// AddCaller 和 AddStacktrace 使用指针区分“未配置”和“显式配置为 false”：
// nil 会采用当前 Environment 的默认值，非 nil 会按调用方指定值覆盖。
type Config struct {
	Environment      Environment
	Level            Level
	Encoding         Encoding
	OutputPaths      []string
	ErrorOutputPaths []string
	AddCaller        *bool
	AddStacktrace    *bool
}

type resolvedConfig struct {
	Environment      Environment
	Level            Level
	Encoding         Encoding
	OutputPaths      []string
	ErrorOutputPaths []string
	AddCaller        bool
	AddStacktrace    bool
}

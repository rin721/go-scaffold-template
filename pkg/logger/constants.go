package logger

const (
	environmentDevelopmentValue = "development"
	environmentProductionValue  = "production"

	encodingConsoleValue = "console"
	encodingJSONValue    = "json"

	outputPathStdout = "stdout"
	outputPathStderr = "stderr"
)

const (
	encoderTimeKey       = "time"
	encoderLevelKey      = "level"
	encoderLoggerNameKey = "logger"
	encoderCallerKey     = "caller"
	encoderMessageKey    = "msg"
	encoderStacktraceKey = "stacktrace"
	encoderErrorKey      = "error"
)

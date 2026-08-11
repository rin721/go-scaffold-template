package logger

import "os"

const (
	environmentDevelopmentValue = "development"
	environmentProductionValue  = "production"

	encodingConsoleValue = "console"
	encodingJSONValue    = "json"

	outputPathStdout = "stdout"
	outputPathStderr = "stderr"

	defaultLogFileMode os.FileMode = 0o666
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

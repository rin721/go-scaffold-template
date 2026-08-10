package health

// Kind 表示健康检查用途。
type Kind string

const (
	KindLiveness  Kind = "liveness"
	KindReadiness Kind = "readiness"
	KindStartup   Kind = "startup"
)

// Degraded 创建降级但可运行的结果。
func Degraded(message string) Result {
	return Result{Status: StatusWarn, Message: message}
}

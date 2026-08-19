// 执行记录的全链路追踪上下文：经 context 传递低敏 trace/span 标识，供记录关联定位。
package execution

import "context"

// traceContextKey 是传递全链路追踪标识的 context 键（私有，仅本包访问）。
type traceContextKey struct{}

// WithTrace 把给定的低敏全链路追踪标识（如 trace/span ID）写入 context，随执行记录落盘。
func WithTrace(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, traceContextKey{}, traceID)
}

// TraceFrom 从 context 读取全链路追踪标识；未设置时返回空字符串。
func TraceFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(traceContextKey{}).(string); ok {
		return value
	}
	return ""
}

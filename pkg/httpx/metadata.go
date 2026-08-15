package httpx

import "context"

type requestLanguageContextKey struct{}

// WithRequestLanguage 把已解析的请求语言写入当前请求 context。
func WithRequestLanguage(ctx context.Context, language string) context.Context {
	return context.WithValue(ctx, requestLanguageContextKey{}, language)
}

// RequestLanguageFromContext 读取协议边界写入的请求语言。
func RequestLanguageFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	language, _ := ctx.Value(requestLanguageContextKey{}).(string)
	return language
}

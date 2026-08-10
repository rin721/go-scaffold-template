package httpx

const (
	headerContentType = "Content-Type"
	headerAccept      = "Accept"

	contentTypeJSON = "application/json"
	contentTypeText = "text/plain; charset=utf-8"

	defaultServerAddr = ":8080"
)

const (
	errorCodeInternalServer = "internal_server_error"
	errorCodeInvalidJSON    = "invalid_json"
	errorCodeHTTPStatus     = "http_status_error"
)

// Method 表示 HTTP 请求方法。
type Method string

const (
	MethodGet     Method = "GET"
	MethodPost    Method = "POST"
	MethodPut     Method = "PUT"
	MethodPatch   Method = "PATCH"
	MethodDelete  Method = "DELETE"
	MethodHead    Method = "HEAD"
	MethodOptions Method = "OPTIONS"
)

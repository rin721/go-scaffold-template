package httpx

// Request 定义一次 HTTP 客户端请求。
type Request struct {
	Method       Method
	URL          string
	Headers      map[string]string
	Query        map[string]string
	Body         any
	RawBody      []byte
	ContentType  string
	AcceptStatus []int
}

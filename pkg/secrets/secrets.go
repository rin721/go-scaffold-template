package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
)

const redactedValue = "[REDACTED]"

// Secret 保存敏感字符串，避免默认格式化泄露原文。
type Secret struct {
	value string
}

// New 用原始字符串创建敏感值。
func New(value string) Secret {
	return Secret{value: value}
}

// Value 返回敏感值原文；只应在连接外部系统的边界使用。
func (s Secret) Value() string {
	return s.value
}

// Redacted 返回脱敏后的展示值。
func (s Secret) Redacted() string {
	if s.value == "" {
		return ""
	}
	return redactedValue
}

func (s Secret) String() string {
	return s.Redacted()
}

// IsSensitiveKey 判断配置键是否具有敏感语义。
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, part := range []string{"password", "passwd", "secret", "token", "key", "credential", "dsn"} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

// RedactMap 深拷贝配置并按 key 语义脱敏。
func RedactMap(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		if IsSensitiveKey(key) {
			out[key] = redactedValue
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = RedactMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}

// Token 生成 URL 安全的随机 token。
func Token(byteLength int) (Secret, error) {
	if byteLength <= 0 {
		byteLength = 32
	}
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return Secret{}, err
	}
	return New(base64.RawURLEncoding.EncodeToString(buf)), nil
}

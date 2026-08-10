package secrets

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Source 是 secret 来源抽象。
type Source interface {
	Secret(name string) (Secret, error)
}

// MapSource 从内存 map 提供 secret，适合测试和本地装配。
type MapSource map[string]Secret

func (s MapSource) Secret(name string) (Secret, error) {
	value, ok := s[name]
	if !ok {
		return Secret{}, fmt.Errorf("secret %s not found", name)
	}
	return value, nil
}

// HMACSHA256 计算消息认证码。
func HMACSHA256(secret Secret, message []byte) string {
	mac := hmac.New(sha256.New, []byte(secret.Value()))
	mac.Write(message)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// DeriveKey 使用 PBKDF2 派生密钥。
func DeriveKey(secret Secret, salt []byte, iterations int, keyLength int) (Secret, error) {
	if iterations <= 0 {
		iterations = 100_000
	}
	if keyLength <= 0 {
		keyLength = 32
	}
	key, err := pbkdf2.Key(sha256.New, secret.Value(), salt, iterations, keyLength)
	if err != nil {
		return Secret{}, err
	}
	return New(base64.RawURLEncoding.EncodeToString(key)), nil
}

# secrets

`pkg/secrets` 提供敏感值类型、配置脱敏和随机 token 生成。

`Secret.String()` 默认返回脱敏文本，只有连接外部系统的边界才应显式调用 `Value()`。

## 推荐入口

- `New(value)`：把原始字符串包装为敏感值。
- `Secret.Redacted()` / `Secret.String()`：返回脱敏展示文本。
- `Secret.Value()`：返回原文，只允许在连接外部系统、签名或加密边界使用。
- `Token(byteLength)`：生成 URL 安全随机 token，`byteLength <= 0` 时使用 32 字节。
- `RedactMap(values)`：按 key 语义深拷贝并脱敏配置。
- `MapSource`：测试或本地装配使用的 secret 来源。
- `HMACSHA256`、`DeriveKey`：基于敏感值执行 HMAC 和 PBKDF2 派生。

## 基础使用示例

```go
package datasource

import "github.com/rin721/go-scaffold2/pkg/secrets"

type Config struct {
	DSN secrets.Secret
}

func SafeFields(cfg Config) map[string]any {
	return map[string]any{
		"dsn": cfg.DSN.Redacted(),
	}
}

func connect(cfg Config) error {
	dsn := cfg.DSN.Value()
	_ = dsn
	return nil
}
```

## secret source 示例

```go
source := secrets.MapSource{
	"database.password": secrets.New("local-password"),
}
password, err := source.Secret("database.password")
if err != nil {
	return err
}
_ = password
```

## 脱敏配置示例

```go
safe := secrets.RedactMap(map[string]any{
	"database": map[string]any{
		"password": "plain-text",
	},
	"server": map[string]any{
		"addr": ":8080",
	},
})
```

## 安全边界

- 日志、错误、测试失败信息和调试输出只能使用脱敏值，不得输出 `Value()`。
- `IsSensitiveKey` 基于 password、secret、token、key、credential、dsn 等 key 名语义做保守匹配，不能替代权限控制或密钥管理系统。
- `MapSource` 适合测试和本地装配，不代表生产 secret backend。
- `DeriveKey` 默认使用 100000 次 PBKDF2 和 32 字节输出；调用方如需调整参数，应在自己的安全边界中集中配置。

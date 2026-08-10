# codec

`pkg/codec` 统一 JSON、YAML 和 msgpack 编解码、内容类型和输入大小限制。

config、cache、storage、httpx 可以逐步迁移到这里复用错误语义。

## 推荐入口

- `JSON()`、`YAML()`、`Msgpack()`：创建项目自有 `Codec`，调用方只依赖 `Codec` 接口和 `ContentType`。
- `DecodeLimited(reader, maxBytes, codec, out)`：从 `io.Reader` 读取并限制最大字节数，避免边界入口无限读入。
- `EncodeReader(codec, value)`：编码后返回 `*bytes.Reader`，便于 HTTP request body 或对象存储上传复用。

## 基础使用示例

```go
package payload

import (
	"io"
	"strings"

	"github.com/rin721/go-scaffold2/pkg/codec"
)

type Profile struct {
	ID   string `json:"id" yaml:"id" msgpack:"id"`
	Name string `json:"name" yaml:"name" msgpack:"name"`
}

func DecodeProfile(body string) (Profile, error) {
	var profile Profile
	err := codec.DecodeLimited(strings.NewReader(body), 1<<20, codec.JSON(), &profile)
	return profile, err
}

func EncodeProfile(profile Profile) (string, codec.ContentType, error) {
	reader, err := codec.EncodeReader(codec.JSON(), profile)
	if err != nil {
		return "", "", err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", "", err
	}
	return string(data), codec.ContentTypeJSON, nil
}
```

## 错误和大小限制

- `DecodeLimited` 要求 `maxBytes > 0`，超过限制会返回错误，不会继续解码。
- 编解码错误保留底层库原始原因；调用方应向上返回或转换为所在边界的项目错误，不依赖错误字符串。
- 本包不负责根据 HTTP Header 自动选择格式；格式协商应由 HTTP 或业务边界显式选择 `Codec`。

## 在业务代码中的推荐使用方式

推荐在边界层选择具体格式，再把 `codec.Codec` 作为依赖传给需要序列化的组件。业务组件不要直接依赖 `yaml.v3`、`msgpack` 或标准库 JSON 细节，避免同一 payload 在不同入口出现不一致的大小限制和错误处理。

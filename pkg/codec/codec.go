package codec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

// ContentType 表示项目支持的编解码格式。
type ContentType string

const (
	ContentTypeJSON    ContentType = "application/json"
	ContentTypeYAML    ContentType = "application/yaml"
	ContentTypeMsgpack ContentType = "application/x-msgpack"
)

// Codec 是项目自有编解码契约。
type Codec interface {
	ContentType() ContentType
	Encode(any) ([]byte, error)
	Decode([]byte, any) error
}

type codecFunc struct {
	contentType ContentType
	encode      func(any) ([]byte, error)
	decode      func([]byte, any) error
}

func (c codecFunc) ContentType() ContentType { return c.contentType }
func (c codecFunc) Encode(value any) ([]byte, error) {
	return c.encode(value)
}
func (c codecFunc) Decode(data []byte, out any) error {
	return c.decode(data, out)
}

func JSON() Codec {
	return codecFunc{contentType: ContentTypeJSON, encode: json.Marshal, decode: json.Unmarshal}
}

func YAML() Codec {
	return codecFunc{contentType: ContentTypeYAML, encode: yaml.Marshal, decode: yaml.Unmarshal}
}

func Msgpack() Codec {
	return codecFunc{contentType: ContentTypeMsgpack, encode: msgpack.Marshal, decode: msgpack.Unmarshal}
}

// DecodeLimited 限制最大输入字节数后解码。
func DecodeLimited(reader io.Reader, maxBytes int64, codec Codec, out any) error {
	if maxBytes <= 0 {
		return fmt.Errorf("max bytes must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("payload exceeds %d bytes", maxBytes)
	}
	return codec.Decode(data, out)
}

// EncodeReader 编码后返回 reader，供 HTTP/storage 共享。
func EncodeReader(codec Codec, value any) (*bytes.Reader, error) {
	data, err := codec.Encode(value)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

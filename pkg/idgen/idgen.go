package idgen

import (
	"fmt"

	"github.com/google/uuid"
)

// Generator 生成跨组件复用的稳定 ID。
type Generator interface {
	New() (string, error)
}

type uuidGenerator struct{}

// UUID 返回基于 github.com/google/uuid 的随机 ID 生成器。
func UUID() Generator {
	return uuidGenerator{}
}

func (uuidGenerator) New() (string, error) {
	value, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

// Validate 校验项目 ID 是否为规范 UUID 字符串。
func Validate(value string) error {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed.String() != value {
		return fmt.Errorf("id is not a canonical UUID")
	}
	return nil
}

// MustNew 在启动构造等不可恢复路径生成 ID。
func MustNew(generator Generator) string {
	value, err := generator.New()
	if err != nil {
		panic(err)
	}
	return value
}

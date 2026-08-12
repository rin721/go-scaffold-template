# idgen

`pkg/idgen` 提供项目自有 ID 生成契约。当前默认实现使用 UUID，业务代码依赖 `Generator`，便于测试替换和后续技术切换。

## 推荐入口

- `UUID()`：创建基于 `github.com/google/uuid` 的随机 ID 生成器。
- `Generator.New()`：生成 ID 并返回错误。
- `MustNew(generator)`：只用于启动构造等不可恢复路径，生成失败时 panic。

## 基础使用示例

```go
package requestid

import "github.com/rin721/go-scaffold2/pkg/idgen"

type Factory struct {
	generator idgen.Generator
}

func NewFactory(generator idgen.Generator) *Factory {
	if generator == nil {
		generator = idgen.UUID()
	}
	return &Factory{generator: generator}
}

func (f *Factory) NewRequestID() (string, error) {
	return f.generator.New()
}
```

## 测试替换示例

```go
type fixedGenerator struct {
	value string
}

func (g fixedGenerator) New() (string, error) {
	return g.value, nil
}
```

## 边界说明

- `Generator` 只承诺生成字符串 ID，不承诺排序性、时间编码或分布式节点信息。
- 业务组件应通过构造函数接收 `idgen.Generator`，不要直接依赖 `uuid` 第三方类型。
- 普通请求路径优先使用 `New()` 并处理错误；`MustNew` 只适合失败即不可恢复的初始化路径。

当前进程统一装配时，`internal/kernel/app/idgen.UUID()` 通过 `app.Value` 输出普通 `idgen.Generator`，composition 将它放入 `Capabilities.IDGenerator`。该组件没有配置、生命周期或 `Access.Use`；调用方只依赖本包接口。

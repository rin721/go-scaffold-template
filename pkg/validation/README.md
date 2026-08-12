# validation

`pkg/validation` 封装结构体校验能力。底层使用 `go-playground/validator/v10`，但调用方只接收项目自有的 `validation.Error` 和 `FieldError`。

该包用于配置、HTTP 入参、CLI 参数等边界校验，不承载业务规则推导。

## 推荐入口

- `New()`：创建默认校验器，启用 required struct 语义。
- `Validator.Struct(value)`：校验结构体并返回项目自有错误。
- `Struct(value)`：使用默认校验器执行一次性校验。
- `Error.Fields`：获取字段、规则、原始值和底层错误消息。

## 基础使用示例

```go
package input

import (
	"errors"

	"github.com/rin721/go-scaffold2/pkg/validation"
)

type CreateUser struct {
	Name string `validate:"required"`
	Age  int    `validate:"gte=0,lte=130"`
}

func Validate(req CreateUser) ([]validation.FieldError, error) {
	if err := validation.Struct(req); err != nil {
		var fieldErr *validation.Error
		if errors.As(err, &fieldErr) {
			return fieldErr.Fields, nil
		}
		return nil, err
	}
	return nil, nil
}
```

## 错误语义

- `nil` 输入会返回 `*validation.Error`，字段为 `$`，规则为 `required`。
- 结构体字段校验失败会聚合为 `*validation.Error`，调用方可用 `errors.As` 提取。
- 无效校验目标会返回带原始原因的普通错误，调用方应向上返回或转换为所在边界错误。

## 边界说明

- 本包只负责输入形状和边界约束校验，不推导业务规则，不访问数据库，也不执行跨资源一致性检查。
- 业务入口可以把 `validation.Validator` 注入 handler、command 或 config loader；业务组件不要直接依赖 `validator/v10` 的具体错误类型。

当前进程统一装配时，`internal/kernel/app/validation.Default()` 通过 `app.Value` 输出普通 `validation.Validator`，composition 将它放入 `Capabilities.Validator`。该组件没有配置、生命周期或 `Access.Use`；只有出现真实规则集或 locale 配置需求时才应增加配置契约。

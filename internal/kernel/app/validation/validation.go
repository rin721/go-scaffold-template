// Package validation 把项目 Validator 作为 Fixed Direct App 组件声明。
package validation

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgvalidation "github.com/rin721/go-scaffold-template/pkg/validation"
)

const ID app.ID = "validation"

// Default 返回当前进程选择的默认 Validator 组件。
func Default() app.Definition[pkgvalidation.Validator] {
	definition, err := app.Value(ID, pkgvalidation.New())
	if err != nil {
		panic(fmt.Sprintf("define default validator: %v", err))
	}
	return definition
}

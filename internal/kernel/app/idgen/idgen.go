// Package idgen 把项目 ID Generator 作为 Fixed Direct App 组件声明。
package idgen

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkgidgen "github.com/rin721/go-scaffold2/pkg/idgen"
)

const ID app.ID = "idgen"

// UUID 返回当前进程选择的 UUID Generator 组件。
func UUID() app.Definition[pkgidgen.Generator] {
	definition, err := app.Value(ID, pkgidgen.UUID())
	if err != nil {
		panic(fmt.Sprintf("define UUID generator: %v", err))
	}
	return definition
}

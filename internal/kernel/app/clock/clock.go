// Package clock 把项目 Clock 作为 Fixed Direct App 组件声明。
package clock

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
)

const ID app.ID = "clock"

// System 返回当前进程选择的系统时钟组件。
func System() app.Definition[pkgclock.Clock] {
	definition, err := app.Value(ID, pkgclock.System())
	if err != nil {
		panic(fmt.Sprintf("define system clock: %v", err))
	}
	return definition
}

// Package composition 显式选择并登记由 Kernel 托管的能力定义。
package composition

import (
	"github.com/rin721/go-scaffold2/internal/kernel"
	databasecapability "github.com/rin721/go-scaffold2/internal/kernel/capability/database"
)

// Capabilities 保存已完成组合的稳定能力入口。
type Capabilities struct {
	Database databasecapability.Access
}

// Compose 按固定清单把能力定义显式登记到尚未启动的 Kernel。
//
// Compose 当前只登记 Database。调用方必须主动调用本函数；Kernel.New 不会自动
// 发现、选择或登记任何能力。
func Compose(runtime *kernel.Kernel) (Capabilities, error) {
	databaseAccess, err := composeDatabase(runtime)
	if err != nil {
		return Capabilities{}, err
	}
	return Capabilities{Database: databaseAccess}, nil
}

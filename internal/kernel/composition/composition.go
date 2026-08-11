// Package composition 显式选择并登记由 Kernel 托管的能力定义。
package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	databasecapability "github.com/rin721/go-scaffold2/internal/kernel/capability/database"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
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
	databaseAccess, err := registerDatabase(runtime, databasecapability.Definition())
	if err != nil {
		return Capabilities{}, fmt.Errorf("compose database capability: %w", err)
	}
	return Capabilities{Database: databaseAccess}, nil
}

func registerDatabase(
	runtime *kernel.Kernel,
	definition kernel.Definition[databasecapability.Config, pkgdatabase.Client],
) (databasecapability.Access, error) {
	handle, err := kernel.Register(runtime, definition)
	if err != nil {
		return nil, err
	}
	return handle, nil
}

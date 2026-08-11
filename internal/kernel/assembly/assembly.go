// Package assembly 显式选择并登记由 Kernel 托管的底层能力。
package assembly

import (
	"fmt"

	databaseadapter "github.com/rin721/go-scaffold2/internal/adapter/database"
	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

// Capabilities 保存已显式注入 Kernel 的稳定能力入口。
type Capabilities struct {
	Database databaseadapter.Access
}

// Inject 按固定清单把底层能力显式登记到尚未启动的 Kernel。
//
// Inject 当前只登记 Database。调用方必须主动调用本函数；Kernel.New 不会自动
// 扫描或注入任何 Adapter。
func Inject(runtime *kernel.Kernel) (Capabilities, error) {
	databaseAccess, err := injectDatabase(runtime, databaseadapter.New())
	if err != nil {
		return Capabilities{}, fmt.Errorf("inject database capability: %w", err)
	}
	return Capabilities{Database: databaseAccess}, nil
}

type databaseCapability interface {
	kernel.Builder[databaseadapter.Config, pkgdatabase.Client]
	kernel.Lifecycle[pkgdatabase.Client]
	Decode(config.Snapshot) (databaseadapter.Config, error)
}

func injectDatabase(runtime *kernel.Kernel, capability databaseCapability) (databaseadapter.Access, error) {
	handle, err := kernel.Register(runtime, kernel.Definition[databaseadapter.Config, pkgdatabase.Client]{
		ID:         databaseadapter.ID,
		ConfigPath: databaseadapter.ConfigPath,
		Decode:     capability.Decode,
		Builder:    capability,
		Lifecycle:  capability,
	})
	if err != nil {
		return nil, err
	}
	return handle, nil
}

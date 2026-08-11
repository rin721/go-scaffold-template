package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	databasecapability "github.com/rin721/go-scaffold2/internal/kernel/capability/database"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

func composeDatabase(runtime *kernel.Kernel) (databasecapability.Access, error) {
	return registerDatabase(runtime, databasecapability.Definition())
}

func registerDatabase(
	runtime *kernel.Kernel,
	definition kernel.Definition[databasecapability.Config, pkgdatabase.Client],
) (databasecapability.Access, error) {
	handle, err := kernel.Register(runtime, definition)
	if err != nil {
		return nil, fmt.Errorf("compose database capability: %w", err)
	}
	return handle, nil
}

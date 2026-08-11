package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	loggercapability "github.com/rin721/go-scaffold2/internal/kernel/capability/logger"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type loggerComposition struct {
	access   loggercapability.Access
	defaults config.Binding
}

func composeLogger(runtime *kernel.Kernel) (loggerComposition, error) {
	if runtime == nil {
		return loggerComposition{}, fmt.Errorf("compose logger runtime is nil")
	}
	return registerLogger(runtime, loggercapability.Definition(runtime.LoggingManager()))
}

func registerLogger(
	runtime *kernel.Kernel,
	definition kernel.Definition[loggercapability.Config, *loggercapability.Instance],
) (loggerComposition, error) {
	registration, err := kernel.Register(runtime, definition)
	if err != nil {
		return loggerComposition{}, fmt.Errorf("compose logger capability: %w", err)
	}
	access, err := loggercapability.NewAccess(registration.Access)
	if err != nil {
		return loggerComposition{}, fmt.Errorf("compose logger access: %w", err)
	}
	return loggerComposition{access: access, defaults: registration.Defaults}, nil
}

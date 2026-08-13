package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	loggerapp "github.com/rin721/go-scaffold2/internal/kernel/app/logger"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
)

func composeBuiltinLogger(plan *app.Plan, target kernellogging.Target) (app.Added[kernellogging.Target], error) {
	definition, err := app.Value[kernellogging.Target](kernel.BuiltinLoggerID, target)
	if err != nil {
		return app.Added[kernellogging.Target]{}, fmt.Errorf("define kernel builtin logger: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[kernellogging.Target]{}, fmt.Errorf("compose kernel builtin logger: %w", err)
	}
	return added, nil
}

func composeLoggerReplacement(
	plan *app.Plan,
	target app.Binding[kernellogging.Target],
) error {
	replacement, err := loggerapp.Replacement()
	if err != nil {
		return fmt.Errorf("define logger replacement app: %w", err)
	}
	if err := app.Replace(plan, target, replacement); err != nil {
		return fmt.Errorf("compose logger replacement app: %w", err)
	}
	return nil
}

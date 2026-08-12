package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	loggerapp "github.com/rin721/go-scaffold2/internal/kernel/app/logger"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
)

func composeLogger(plan *app.Plan, manager *kernellogging.Manager) (app.Added[loggerapp.Access], error) {
	definition, err := loggerapp.Definition(manager)
	if err != nil {
		return app.Added[loggerapp.Access]{}, fmt.Errorf("define logger app: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[loggerapp.Access]{}, fmt.Errorf("compose logger app: %w", err)
	}
	return added, nil
}

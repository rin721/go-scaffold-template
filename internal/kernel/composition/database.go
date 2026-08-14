package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
)

func composeDatabase(plan *app.Plan) (app.Added[databaseapp.Access], error) {
	definition, err := databaseapp.Definition()
	if err != nil {
		return app.Added[databaseapp.Access]{}, fmt.Errorf("define database app: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[databaseapp.Access]{}, fmt.Errorf("compose database app: %w", err)
	}
	return added, nil
}

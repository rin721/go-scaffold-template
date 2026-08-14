package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	cacheapp "github.com/rin721/go-scaffold-template/internal/kernel/app/cache"
)

func composeCache(plan *app.Plan) (app.Added[cacheapp.Access], error) {
	definition, err := cacheapp.Definition()
	if err != nil {
		return app.Added[cacheapp.Access]{}, fmt.Errorf("define cache app: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[cacheapp.Access]{}, fmt.Errorf("compose cache app: %w", err)
	}
	return added, nil
}

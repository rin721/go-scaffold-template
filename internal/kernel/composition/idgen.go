package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	idgenapp "github.com/rin721/go-scaffold2/internal/kernel/app/idgen"
	pkgidgen "github.com/rin721/go-scaffold2/pkg/idgen"
)

func composeIDGenerator(plan *app.Plan) (app.Added[pkgidgen.Generator], error) {
	added, err := app.Add(plan, idgenapp.UUID())
	if err != nil {
		return app.Added[pkgidgen.Generator]{}, fmt.Errorf("compose ID generator app: %w", err)
	}
	return added, nil
}

package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	validationapp "github.com/rin721/go-scaffold-template/internal/kernel/app/validation"
	pkgvalidation "github.com/rin721/go-scaffold-template/pkg/validation"
)

func composeValidator(plan *app.Plan) (app.Added[pkgvalidation.Validator], error) {
	added, err := app.Add(plan, validationapp.Default())
	if err != nil {
		return app.Added[pkgvalidation.Validator]{}, fmt.Errorf("compose validation app: %w", err)
	}
	return added, nil
}

package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	i18napp "github.com/rin721/go-scaffold-template/internal/kernel/app/i18n"
	pkgi18n "github.com/rin721/go-scaffold-template/pkg/i18n"
)

func composeI18n(plan *app.Plan) (app.Added[pkgi18n.Translator], error) {
	definition, err := i18napp.Definition()
	if err != nil {
		return app.Added[pkgi18n.Translator]{}, fmt.Errorf("define i18n app: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[pkgi18n.Translator]{}, fmt.Errorf("compose i18n app: %w", err)
	}
	return added, nil
}

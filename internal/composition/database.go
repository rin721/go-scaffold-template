package composition

import (
	"context"
	"fmt"

	databaseapp "github.com/rin721/go-scaffold2/internal/kernel/app/database"
	"github.com/rin721/go-scaffold2/internal/module/todo/repo"
	"github.com/rin721/go-scaffold2/pkg/database"
)

// databaseAccessAdapter 把 Kernel-owned Access 转成 Todo Adapter 使用方拥有的窄契约。
type databaseAccessAdapter struct{ access databaseapp.Access }

func adaptDatabaseAccess(access databaseapp.Access) (repo.Access, error) {
	if access == nil {
		return nil, fmt.Errorf("application database access is nil")
	}
	return databaseAccessAdapter{access: access}, nil
}

func (a databaseAccessAdapter) Use(ctx context.Context, use func(database.Client) error) error {
	if use == nil {
		return database.ErrNilClientFunc
	}
	return a.access.Use(ctx, func(client databaseapp.Client) error { return use(client) })
}

func (a databaseAccessAdapter) WithinTx(
	ctx context.Context,
	use func(context.Context, database.Client, database.Tx) error,
) error {
	if use == nil {
		return database.ErrNilTransactionFunc
	}
	return a.access.WithinTx(ctx, func(txCtx context.Context, client databaseapp.Client, tx database.Tx) error {
		return use(txCtx, client, tx)
	})
}

var _ repo.Access = databaseAccessAdapter{}

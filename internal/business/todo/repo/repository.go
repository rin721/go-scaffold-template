// Package repo 实现 Todo 的数据库 Repository port。
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold2/internal/business/todo/model"
	"github.com/rin721/go-scaffold2/internal/business/todo/service"
	"github.com/rin721/go-scaffold2/pkg/database"
	"github.com/rin721/go-scaffold2/pkg/fault"
)

// Record 是 Todo 的持久化模型；它不向 Service 或协议边界传播。
type Record struct {
	ID          string
	Title       string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
	Version     uint64
}

// Access 是 Todo Database Adapter 使用方拥有的稳定租约契约。
type Access interface {
	Use(context.Context, func(database.Client) error) error
	WithinTx(context.Context, func(context.Context, database.Client, database.Tx) error) error
}

// Repository 使用稳定 Database Access 实现 Todo 持久化。
type Repository struct {
	access Access
	schema database.Schema
}

// New 创建不获取数据库租约的 Repository。
func New(access Access, schema database.Schema) (*Repository, error) {
	if access == nil {
		return nil, fmt.Errorf("todo database access is nil")
	}
	return &Repository{access: access, schema: schema}, nil
}

// Create 创建 Todo。
func (r *Repository) Create(ctx context.Context, todo model.Todo) (model.Todo, error) {
	record := recordFromModel(todo)
	err := r.access.Use(ctx, func(client database.Client) error {
		repository, err := database.NewRepository[Record](client, r.schema)
		if err != nil {
			return err
		}
		return repository.Create(ctx, &record)
	})
	if err != nil {
		return model.Todo{}, translate(err, "todo.repo.create")
	}
	return modelFromRecord(record)
}

// Get 按 ID 查询 Todo。
func (r *Repository) Get(ctx context.Context, id string) (model.Todo, error) {
	var record Record
	err := r.access.Use(ctx, func(client database.Client) error {
		repository, err := database.NewRepository[Record](client, r.schema)
		if err != nil {
			return err
		}
		record, err = repository.First(ctx, database.Query{Filters: []database.Filter{{
			Field: "ID", Operator: database.OpEqual, Value: id,
		}}})
		return err
	})
	if err != nil {
		return model.Todo{}, translate(err, "todo.repo.get")
	}
	return modelFromRecord(record)
}

// List 在一个事务快照内返回分页数据和总数。
func (r *Repository) List(ctx context.Context, filter service.ListFilter) ([]model.Todo, int64, error) {
	var records []Record
	var total int64
	err := r.access.WithinTx(ctx, func(txCtx context.Context, client database.Client, tx database.Tx) error {
		base, err := database.NewRepository[Record](client, r.schema)
		if err != nil {
			return err
		}
		repository, err := base.WithTx(tx)
		if err != nil {
			return err
		}
		filters := make([]database.Filter, 0, 1)
		if filter.Status != nil {
			filters = append(filters, database.Filter{Field: "Status", Operator: database.OpEqual, Value: string(*filter.Status)})
		}
		total, err = repository.Count(txCtx, database.Query{Filters: filters})
		if err != nil {
			return err
		}
		records, err = repository.Find(txCtx, database.Query{
			Filters: filters,
			Orders: []database.Order{
				{Field: "CreatedAt", Direction: database.OrderDescending},
				{Field: "ID", Direction: database.OrderAscending},
			},
			Page: &database.Page{Offset: filter.Offset, Limit: filter.Limit},
		})
		return err
	})
	if err != nil {
		return nil, 0, translate(err, "todo.repo.list")
	}
	items := make([]model.Todo, len(records))
	for index, record := range records {
		converted, err := modelFromRecord(record)
		if err != nil {
			return nil, 0, err
		}
		items[index] = converted
	}
	return items, total, nil
}

// Save 使用 ID 与 Version 原子保存 Todo 状态。
func (r *Repository) Save(ctx context.Context, todo model.Todo) (model.Todo, error) {
	var record Record
	err := r.access.Use(ctx, func(client database.Client) error {
		repository, err := database.NewRepository[Record](client, r.schema)
		if err != nil {
			return err
		}
		filters := []database.Filter{
			{Field: "ID", Operator: database.OpEqual, Value: todo.ID},
			{Field: "Version", Operator: database.OpEqual, Value: todo.Version},
		}
		if _, err := repository.Update(ctx, database.Query{Filters: filters}, database.Changes{
			"Status": string(todo.Status), "UpdatedAt": todo.UpdatedAt, "CompletedAt": todo.CompletedAt,
		}); err != nil {
			return err
		}
		record, err = repository.First(ctx, database.Query{Filters: []database.Filter{{
			Field: "ID", Operator: database.OpEqual, Value: todo.ID,
		}}})
		return err
	})
	if err != nil {
		return model.Todo{}, translate(err, "todo.repo.save")
	}
	return modelFromRecord(record)
}

func recordFromModel(todo model.Todo) Record {
	return Record{
		ID: todo.ID, Title: todo.Title, Status: string(todo.Status), CreatedAt: todo.CreatedAt,
		UpdatedAt: todo.UpdatedAt, CompletedAt: todo.CompletedAt, Version: todo.Version,
	}
}

func modelFromRecord(record Record) (model.Todo, error) {
	status, err := model.ParseStatus(record.Status)
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.repo.record.status", false)
	}
	result, err := model.Restore(model.Todo{
		ID: record.ID, Title: record.Title, Status: status, CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt, CompletedAt: record.CompletedAt, Version: record.Version,
	})
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.repo.record", false)
	}
	return result, nil
}

func translate(err error, operation string) error {
	code := fault.CodeInternal
	retryable := false
	switch {
	case errors.Is(err, database.ErrNotFound):
		code = fault.CodeNotFound
	case errors.Is(err, database.ErrDuplicateKey), errors.Is(err, database.ErrOptimisticConflict):
		code = fault.CodeConflict
	case errors.Is(err, database.ErrClientUnavailable), errors.Is(err, database.ErrOperationFailed):
		code, retryable = fault.CodeUnavailable, true
	case errors.Is(err, context.Canceled):
		code = fault.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		code = fault.CodeTimeout
	}
	return fault.Wrap(err, code, operation, retryable)
}

var _ service.Repository = (*Repository)(nil)

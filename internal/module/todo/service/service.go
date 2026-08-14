// Package service 实现 Todo 用例并定义调用方拥有的持久化 port。
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/rin721/go-scaffold2/internal/module/todo/model"
	"github.com/rin721/go-scaffold2/pkg/clock"
	"github.com/rin721/go-scaffold2/pkg/fault"
	"github.com/rin721/go-scaffold2/pkg/idgen"
)

// Policy 是已由配置边界校验的 Todo 业务策略。
type Policy struct {
	TitleMaxRunes    int
	DefaultListLimit int
	MaxListLimit     int
}

// CreateCommand 是创建 Todo 的用例输入。
type CreateCommand struct{ Title string }

// GetQuery 是按 ID 查询 Todo 的用例输入。
type GetQuery struct{ ID string }

// ListQuery 是 Todo 列表的筛选和分页输入。
type ListQuery struct {
	Status string
	Offset int
	Limit  int
}

// CompleteCommand 是完成 Todo 的用例输入。
type CompleteCommand struct{ ID string }

// ListResult 是协议无关的 Todo 列表结果。
type ListResult struct {
	Items  []model.Todo
	Offset int
	Limit  int
	Total  int64
}

// ListFilter 是 Service 交给 Repository 的稳定查询条件。
type ListFilter struct {
	Status *model.Status
	Offset int
	Limit  int
}

// Repository 是 Todo Service 使用方定义的最小持久化契约。
type Repository interface {
	Create(context.Context, model.Todo) (model.Todo, error)
	Get(context.Context, string) (model.Todo, error)
	List(context.Context, ListFilter) ([]model.Todo, int64, error)
	Save(context.Context, model.Todo) (model.Todo, error)
}

// UseCases 是 HTTP 与 CLI 共用的 Todo 用例入口。
type UseCases interface {
	Create(context.Context, CreateCommand) (model.Todo, error)
	Get(context.Context, GetQuery) (model.Todo, error)
	List(context.Context, ListQuery) (ListResult, error)
	Complete(context.Context, CompleteCommand) (model.Todo, error)
}

// Service 实现 Todo 用例。
type Service struct {
	repository Repository
	clock      clock.Clock
	ids        idgen.Generator
	policy     Policy
}

// New 创建无 I/O 副作用的 Todo Service。
func New(repository Repository, currentClock clock.Clock, ids idgen.Generator, policy Policy) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("todo repository is nil")
	}
	if currentClock == nil {
		return nil, fmt.Errorf("todo clock is nil")
	}
	if ids == nil {
		return nil, fmt.Errorf("todo id generator is nil")
	}
	if policy.TitleMaxRunes <= 0 || policy.DefaultListLimit <= 0 || policy.MaxListLimit <= 0 ||
		policy.DefaultListLimit > policy.MaxListLimit {
		return nil, fmt.Errorf("todo policy is invalid")
	}
	return &Service{repository: repository, clock: currentClock, ids: ids, policy: policy}, nil
}

// Create 创建一个待完成 Todo。
func (s *Service) Create(ctx context.Context, command CreateCommand) (model.Todo, error) {
	if err := validateContext(ctx); err != nil {
		return model.Todo{}, err
	}
	title, err := model.NormalizeTitle(command.Title, s.policy.TitleMaxRunes)
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInvalidArgument, "todo.create.title", false)
	}
	id, err := s.ids.New()
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.create.id", false)
	}
	if err := idgen.Validate(id); err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.create.id", false)
	}
	todo, err := model.New(id, title, s.clock.Now())
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.create.model", false)
	}
	created, err := s.repository.Create(ctx, todo)
	if err != nil {
		return model.Todo{}, preserveRepositoryError(err, "todo.create.repository")
	}
	return created, nil
}

// Get 按 ID 查询 Todo。
func (s *Service) Get(ctx context.Context, query GetQuery) (model.Todo, error) {
	if err := validateContext(ctx); err != nil {
		return model.Todo{}, err
	}
	id, err := validateID(query.ID)
	if err != nil {
		return model.Todo{}, err
	}
	todo, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Todo{}, preserveRepositoryError(err, "todo.get.repository")
	}
	return todo, nil
}

// List 查询稳定分页的 Todo 列表。
func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if err := validateContext(ctx); err != nil {
		return ListResult{}, err
	}
	if query.Offset < 0 {
		return ListResult{}, fault.New(fault.CodeInvalidArgument, "offset must be non-negative")
	}
	limit := query.Limit
	if limit == 0 {
		limit = s.policy.DefaultListLimit
	}
	if limit < 1 || limit > s.policy.MaxListLimit {
		return ListResult{}, fault.New(fault.CodeInvalidArgument, "limit is outside the allowed range")
	}
	filter := ListFilter{Offset: query.Offset, Limit: limit}
	if query.Status != "" {
		status, err := model.ParseStatus(query.Status)
		if err != nil {
			return ListResult{}, fault.Wrap(err, fault.CodeInvalidArgument, "todo.list.status", false)
		}
		filter.Status = &status
	}
	items, total, err := s.repository.List(ctx, filter)
	if err != nil {
		return ListResult{}, preserveRepositoryError(err, "todo.list.repository")
	}
	if items == nil {
		items = []model.Todo{}
	}
	return ListResult{Items: items, Offset: query.Offset, Limit: limit, Total: total}, nil
}

// Complete 完成 Todo；已经完成的对象保持幂等。
func (s *Service) Complete(ctx context.Context, command CompleteCommand) (model.Todo, error) {
	if err := validateContext(ctx); err != nil {
		return model.Todo{}, err
	}
	id, err := validateID(command.ID)
	if err != nil {
		return model.Todo{}, err
	}
	todo, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Todo{}, preserveRepositoryError(err, "todo.complete.get")
	}
	changed, err := todo.Complete(s.clock.Now())
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.complete.model", false)
	}
	if !changed {
		return todo, nil
	}
	saved, err := s.repository.Save(ctx, todo)
	if err != nil {
		return model.Todo{}, preserveRepositoryError(err, "todo.complete.save")
	}
	return saved, nil
}

func validateID(value string) (string, error) {
	id := strings.TrimSpace(value)
	if err := idgen.Validate(id); err != nil {
		return "", fault.Wrap(err, fault.CodeInvalidArgument, "todo.id", false)
	}
	return id, nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return fault.New(fault.CodeInvalidArgument, "context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func preserveRepositoryError(err error, operation string) error {
	code := fault.CodeOf(err)
	retryable := code == fault.CodeUnavailable
	return fault.Wrap(err, code, operation, retryable)
}

var _ UseCases = (*Service)(nil)

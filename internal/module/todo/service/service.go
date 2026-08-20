// Package service 实现 Todo 用例并定义调用方拥有的持久化 port。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/idgen"
)

var ErrPermissionDenied = errors.New("todo actor is not authorized")

// completeExecKeyPrefix 是 Todo「完成」用例幂等键的前缀（业务语义稳定值）。
const completeExecKeyPrefix = "todo:complete:"

// Actor 是 HTTP/CLI 边界显式传入的项目主体事实。
type Actor struct {
	Subject string
	Kind    string
	Scopes  []string
}

// Action 是 Todo module 拥有的业务授权动作。
type Action string

const (
	ActionCreate   Action = "todo.create"
	ActionList     Action = "todo.list"
	ActionRead     Action = "todo.read"
	ActionComplete Action = "todo.complete"
)

// ResourceFacts 是完成对象级授权所需的真实持久化事实。
type ResourceFacts struct {
	ID           string
	OwnerSubject string
}

// Authorizer 是 Todo Service 使用方定义的对象授权与审计窄 port。
type Authorizer interface {
	Enforce(context.Context, Actor, Action, ResourceFacts) error
}

// Policy 是已由配置边界校验的 Todo 业务策略。
type Policy struct {
	TitleMaxRunes    int
	DefaultListLimit int
	MaxListLimit     int
}

// CreateCommand 是创建 Todo 的用例输入。
type CreateCommand struct {
	Actor Actor
	Title string
}

// GetQuery 是按 ID 查询 Todo 的用例输入。
type GetQuery struct {
	Actor Actor
	ID    string
}

// ListQuery 是 Todo 列表的筛选和分页输入。
type ListQuery struct {
	Actor  Actor
	Status string
	Offset int
	Limit  int
}

// CompleteCommand 是完成 Todo 的用例输入。
type CompleteCommand struct {
	Actor Actor
	ID    string
}

// ListResult 是协议无关的 Todo 列表结果。
type ListResult struct {
	Items  []model.Todo
	Offset int
	Limit  int
	Total  int64
}

// ListFilter 是 Service 交给 Repository 的稳定查询条件。
type ListFilter struct {
	Status       *model.Status
	OwnerSubject string
	Offset       int
	Limit        int
}

// Repository 是 Todo Service 使用方定义的最小持久化契约。
type Repository interface {
	Create(context.Context, model.Todo) (model.Todo, error)
	Get(context.Context, string) (model.Todo, error)
	List(context.Context, ListFilter) ([]model.Todo, int64, error)
	Save(context.Context, model.Todo) (model.Todo, error)
}

// Executor 是 Todo「完成」用例经 execution 能力治理关键写操作（幂等 / 重试 / 执行记录）时使用的窄 port。
// key 为幂等键；operation 为真实写操作，成功返回已写入实体；duplicate=true 表示同幂等键已完成、operation 未被重跑。
type Executor func(ctx context.Context, key string, operation func(context.Context) (model.Todo, error)) (saved model.Todo, duplicate bool, err error)

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
	authorizer Authorizer
	executor   Executor
}

// New 创建无 I/O 副作用的 Todo Service。
func New(repository Repository, currentClock clock.Clock, ids idgen.Generator, policy Policy, authorizer Authorizer, executor Executor) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("todo repository is nil")
	}
	if currentClock == nil {
		return nil, fmt.Errorf("todo clock is nil")
	}
	if ids == nil {
		return nil, fmt.Errorf("todo id generator is nil")
	}
	if authorizer == nil {
		return nil, fmt.Errorf("todo authorizer is nil")
	}
	if executor == nil {
		return nil, fmt.Errorf("todo executor is nil")
	}
	if policy.TitleMaxRunes <= 0 || policy.DefaultListLimit <= 0 || policy.MaxListLimit <= 0 ||
		policy.DefaultListLimit > policy.MaxListLimit {
		return nil, fmt.Errorf("todo policy is invalid")
	}
	return &Service{repository: repository, clock: currentClock, ids: ids, policy: policy, authorizer: authorizer, executor: executor}, nil
}

// Create 创建一个待完成 Todo。
func (s *Service) Create(ctx context.Context, command CreateCommand) (model.Todo, error) {
	if err := validateContext(ctx); err != nil {
		return model.Todo{}, err
	}
	actor, err := validateActor(command.Actor)
	if err != nil {
		return model.Todo{}, err
	}
	if err := s.enforce(ctx, actor, ActionCreate, ResourceFacts{OwnerSubject: actor.Subject}, false); err != nil {
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
	todo, err := model.New(id, title, actor.Subject, s.clock.Now())
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
	actor, err := validateActor(query.Actor)
	if err != nil {
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
	if err := s.enforce(ctx, actor, ActionRead, ResourceFacts{ID: todo.ID, OwnerSubject: todo.OwnerSubject}, true); err != nil {
		return model.Todo{}, err
	}
	return todo, nil
}

// List 查询稳定分页的 Todo 列表。
func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if err := validateContext(ctx); err != nil {
		return ListResult{}, err
	}
	actor, err := validateActor(query.Actor)
	if err != nil {
		return ListResult{}, err
	}
	if err := s.enforce(ctx, actor, ActionList, ResourceFacts{OwnerSubject: actor.Subject}, false); err != nil {
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
	filter := ListFilter{OwnerSubject: actor.Subject, Offset: query.Offset, Limit: limit}
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
	actor, err := validateActor(command.Actor)
	if err != nil {
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
	if err := s.enforce(ctx, actor, ActionComplete, ResourceFacts{ID: todo.ID, OwnerSubject: todo.OwnerSubject}, true); err != nil {
		return model.Todo{}, err
	}
	changed, err := todo.Complete(s.clock.Now())
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.complete.model", false)
	}
	if !changed {
		return todo, nil
	}
	// 把关键写操作经执行治理（幂等 / 重试 / 执行记录）执行；幂等键取自业务 ID。
	saved, duplicate, err := s.executor(ctx, completeExecKeyPrefix+id, func(operationCtx context.Context) (model.Todo, error) {
		return s.repository.Save(operationCtx, todo)
	})
	if err != nil {
		return model.Todo{}, fault.Wrap(err, fault.CodeInternal, "todo.complete.execution", false)
	}
	if duplicate {
		// 并发下同幂等键刚被本次完成：本地 model 即持久化结果，直接返回，不重跑写操作。
		return todo, nil
	}
	return saved, nil
}

func validateActor(actor Actor) (Actor, error) {
	actor.Subject = strings.TrimSpace(actor.Subject)
	if actor.Subject == "" || strings.TrimSpace(actor.Kind) == "" || len(actor.Scopes) == 0 {
		return Actor{}, fault.New(fault.CodePermissionDenied, "Todo actor is not authorized")
	}
	return actor, nil
}

func (s *Service) enforce(ctx context.Context, actor Actor, action Action, resource ResourceFacts, hideExistence bool) error {
	if err := s.authorizer.Enforce(ctx, actor, action, resource); err != nil {
		if hideExistence && errors.Is(err, ErrPermissionDenied) {
			return fault.Wrap(err, fault.CodeNotFound, "todo.authorize.hidden", false)
		}
		code := fault.CodeInternal
		if errors.Is(err, ErrPermissionDenied) {
			code = fault.CodePermissionDenied
		}
		return fault.Wrap(err, code, "todo.authorize", false)
	}
	return nil
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

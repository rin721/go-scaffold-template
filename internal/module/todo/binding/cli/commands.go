// Package clibinding 绑定 Todo Application CLI 命令。
package clibinding

import (
	"context"
	"encoding/json"
	"fmt"

	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/module/todo/model"
	"github.com/rin721/go-scaffold2/internal/module/todo/service"
	"github.com/rin721/go-scaffold2/pkg/cli"
	"github.com/rin721/go-scaffold2/pkg/clock"
	"github.com/rin721/go-scaffold2/pkg/fault"
)

// Executor 在命令解析完成后进入受管 application operation。
type Executor interface {
	Create(context.Context, service.CreateCommand) (model.Todo, error)
	Get(context.Context, service.GetQuery) (model.Todo, error)
	List(context.Context, service.ListQuery) (service.ListResult, error)
	Complete(context.Context, service.CompleteCommand) (model.Todo, error)
}

// Contract 提供 Todo 命令树，不在构造时创建业务资源。
type Contract struct{ executor Executor }

// New 创建 Todo CLI command binding。
func New(executor Executor) (kernelcli.Contract, error) {
	if executor == nil {
		return nil, fmt.Errorf("todo CLI executor is nil")
	}
	return &Contract{executor: executor}, nil
}

// Commands 返回真实 Application command 规格。
func (c *Contract) Commands() ([]cli.CommandSpec, error) {
	return []cli.CommandSpec{{
		Name: "todo", Description: "管理 Todo", Mode: cli.CommandModeApplication,
		SideEffect: cli.SideEffectExternalWrite, Positional: cli.PositionalNone,
		Commands: []cli.CommandSpec{
			c.createCommand(), c.getCommand(), c.listCommand(), c.completeCommand(),
		},
	}}, nil
}

func (c *Contract) createCommand() cli.CommandSpec {
	return cli.CommandSpec{
		Name: "create", Description: "创建 Todo", Mode: cli.CommandModeApplication,
		SideEffect: cli.SideEffectExternalWrite, Positional: cli.PositionalNone,
		Flags: []cli.FlagSpec{{Name: "title", Shorthand: "t", Type: cli.FlagTypeString, Required: true, Description: "Todo 标题"}},
		Run: func(ctx *cli.Context) error {
			created, err := c.executor.Create(ctx.Context, service.CreateCommand{Title: ctx.GetString("title")})
			return writeTodo(ctx, created, err)
		},
	}
}

func (c *Contract) getCommand() cli.CommandSpec {
	return cli.CommandSpec{
		Name: "get", Description: "查询 Todo", Mode: cli.CommandModeApplication,
		SideEffect: cli.SideEffectExternalWrite, Positional: cli.PositionalNone,
		Flags: []cli.FlagSpec{{Name: "id", Type: cli.FlagTypeString, Required: true, Description: "Todo ID"}},
		Run: func(ctx *cli.Context) error {
			found, err := c.executor.Get(ctx.Context, service.GetQuery{ID: ctx.GetString("id")})
			return writeTodo(ctx, found, err)
		},
	}
}

func (c *Contract) listCommand() cli.CommandSpec {
	return cli.CommandSpec{
		Name: "list", Description: "列出 Todo", Mode: cli.CommandModeApplication,
		SideEffect: cli.SideEffectExternalWrite, Positional: cli.PositionalNone,
		Flags: []cli.FlagSpec{
			{Name: "status", Type: cli.FlagTypeString, Description: "pending 或 completed"},
			{Name: "offset", Type: cli.FlagTypeInt, Default: 0, Description: "分页偏移"},
			{Name: "limit", Type: cli.FlagTypeInt, Default: 0, Description: "分页数量；0 使用配置默认值"},
		},
		Run: func(ctx *cli.Context) error {
			result, err := c.executor.List(ctx.Context, service.ListQuery{
				Status: ctx.GetString("status"), Offset: ctx.GetInt("offset"), Limit: ctx.GetInt("limit"),
			})
			if err != nil {
				return commandError(ctx, err)
			}
			return encode(ctx, listViewOf(result))
		},
	}
}

func (c *Contract) completeCommand() cli.CommandSpec {
	return cli.CommandSpec{
		Name: "complete", Description: "完成 Todo", Mode: cli.CommandModeApplication,
		SideEffect: cli.SideEffectExternalWrite, Positional: cli.PositionalNone,
		Flags: []cli.FlagSpec{{Name: "id", Type: cli.FlagTypeString, Required: true, Description: "Todo ID"}},
		Run: func(ctx *cli.Context) error {
			completed, err := c.executor.Complete(ctx.Context, service.CompleteCommand{ID: ctx.GetString("id")})
			return writeTodo(ctx, completed, err)
		},
	}
}

type todoView struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	CompletedAt *string `json:"completedAt"`
}

type listView struct {
	Items  []todoView `json:"items"`
	Offset int        `json:"offset"`
	Limit  int        `json:"limit"`
	Total  int64      `json:"total"`
}

func writeTodo(ctx *cli.Context, todo model.Todo, err error) error {
	if err != nil {
		return commandError(ctx, err)
	}
	return encode(ctx, todoViewOf(todo))
}

func commandError(ctx *cli.Context, err error) error {
	if fault.CodeOf(err) == fault.CodeInvalidArgument {
		return &cli.UsageError{Command: ctx.CommandPath, Message: err.Error()}
	}
	return err
}

func encode(ctx *cli.Context, value any) error {
	encoder := json.NewEncoder(ctx.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode todo CLI output: %w", err)
	}
	return nil
}

func todoViewOf(todo model.Todo) todoView {
	view := todoView{
		ID: todo.ID, Title: todo.Title, Status: string(todo.Status),
		CreatedAt: clock.RFC3339Millis(todo.CreatedAt), UpdatedAt: clock.RFC3339Millis(todo.UpdatedAt),
	}
	if todo.CompletedAt != nil {
		completed := clock.RFC3339Millis(*todo.CompletedAt)
		view.CompletedAt = &completed
	}
	return view
}

func listViewOf(result service.ListResult) listView {
	items := make([]todoView, len(result.Items))
	for index, todo := range result.Items {
		items[index] = todoViewOf(todo)
	}
	return listView{Items: items, Offset: result.Offset, Limit: result.Limit, Total: result.Total}
}

var _ kernelcli.Contract = (*Contract)(nil)

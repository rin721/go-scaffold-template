package clibinding

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/module/todo/model"
	"github.com/rin721/go-scaffold2/internal/module/todo/service"
	"github.com/rin721/go-scaffold2/pkg/cli"
	"github.com/rin721/go-scaffold2/pkg/fault"
)

func TestCommandsUseApplicationModeAndExternalWrite(t *testing.T) {
	contract, err := New(&stubExecutor{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	commands, err := contract.Commands()
	if err != nil || len(commands) != 1 || len(commands[0].Commands) != 4 {
		t.Fatalf("Commands() = %#v, %v", commands, err)
	}
	for _, command := range append([]cli.CommandSpec{commands[0]}, commands[0].Commands...) {
		if command.Mode != cli.CommandModeApplication || command.SideEffect != cli.SideEffectExternalWrite {
			t.Fatalf("command %s mode/effect = %v/%v", command.Name, command.Mode, command.SideEffect)
		}
	}
}

func TestCommandsWriteJSONAndMapInvalidInputToUsage(t *testing.T) {
	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	executor := &stubExecutor{todo: model.Todo{
		ID: "11111111-1111-4111-8111-111111111111", Title: "学习 Go", Status: model.StatusPending,
		CreatedAt: now, UpdatedAt: now,
	}}
	contract, _ := New(executor)
	var stdout bytes.Buffer
	app, err := kernelcli.NewApp(cli.Config{Name: "test", Stdout: &stdout, Stderr: &bytes.Buffer{}}, contract)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if err := app.Run(t.Context(), []string{"todo", "create", "--title", "学习 Go"}); err != nil {
		t.Fatalf("Run(create) error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"id":"11111111-1111-4111-8111-111111111111"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for _, args := range [][]string{
		{"todo", "get", "--id", executor.todo.ID},
		{"todo", "list", "--status", "pending", "--offset", "0", "--limit", "20"},
		{"todo", "complete", "--id", executor.todo.ID},
	} {
		stdout.Reset()
		commandApp, err := kernelcli.NewApp(cli.Config{Name: "test", Stdout: &stdout, Stderr: &bytes.Buffer{}}, contract)
		if err != nil {
			t.Fatalf("NewApp(%v) error = %v", args, err)
		}
		if err := commandApp.Run(t.Context(), args); err != nil {
			t.Fatalf("Run(%v) error = %v", args, err)
		}
		if !strings.Contains(stdout.String(), `"id":"11111111-1111-4111-8111-111111111111"`) {
			t.Fatalf("Run(%v) stdout = %q", args, stdout.String())
		}
	}

	invalidContract, _ := New(&stubExecutor{err: fault.New(fault.CodeInvalidArgument, "bad limit")})
	invalidApp, err := kernelcli.NewApp(cli.Config{Name: "test"}, invalidContract)
	if err != nil {
		t.Fatalf("NewApp(invalid) error = %v", err)
	}
	err = invalidApp.Run(t.Context(), []string{"todo", "list", "--limit", "999"})
	if cli.GetExitCode(err) != cli.ExitUsage {
		t.Fatalf("Run(invalid) error = %v, exit = %d", err, cli.GetExitCode(err))
	}
}

type stubExecutor struct {
	todo model.Todo
	err  error
}

func (s *stubExecutor) Create(context.Context, service.CreateCommand) (model.Todo, error) {
	return s.todo, s.err
}
func (s *stubExecutor) Get(context.Context, service.GetQuery) (model.Todo, error) {
	return s.todo, s.err
}
func (s *stubExecutor) List(context.Context, service.ListQuery) (service.ListResult, error) {
	return service.ListResult{Items: []model.Todo{s.todo}, Limit: 20, Total: 1}, s.err
}
func (s *stubExecutor) Complete(context.Context, service.CompleteCommand) (model.Todo, error) {
	return s.todo, s.err
}

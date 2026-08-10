package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRunRoutesCobraCommandAndParsesFlags(t *testing.T) {
	t.Setenv("CLI_TEST_OUTPUT", "from-env")

	app, err := NewApp(Config{Name: "tool", Version: "1.2.3", Description: "test cli"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	var got *Context
	err = app.AddCommand(CommandSpec{
		Name:        "run",
		Description: "run command",
		Flags: []FlagSpec{
			{Name: "name", Type: FlagTypeString, Required: true},
			{Name: "count", Type: FlagTypeInt, Default: 1},
			{Name: "verbose", Type: FlagTypeBool},
			{Name: "tags", Type: FlagTypeStringSlice},
			{Name: "output", Type: FlagTypeString, EnvVar: "CLI_TEST_OUTPUT"},
		},
		Run: func(ctx *Context) error {
			got = ctx
			return nil
		},
	})
	if err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = app.RunWithIO(context.Background(), []string{
		"run",
		"--name", "alice",
		"--count", "3",
		"--verbose",
		"--tags", "alpha,beta",
		"positional",
	}, strings.NewReader("input"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	if got == nil {
		t.Fatal("command was not executed")
	}
	if got.CommandName != "run" {
		t.Fatalf("CommandName = %q, want run", got.CommandName)
	}
	if got.GetString("name") != "alice" {
		t.Fatalf("name = %q, want alice", got.GetString("name"))
	}
	if got.GetInt("count") != 3 {
		t.Fatalf("count = %d, want 3", got.GetInt("count"))
	}
	if !got.GetBool("verbose") {
		t.Fatal("verbose = false, want true")
	}
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(got.GetStringSlice("tags"), want) {
		t.Fatalf("tags = %#v, want %#v", got.GetStringSlice("tags"), want)
	}
	if got.GetString("output") != "from-env" {
		t.Fatalf("output = %q, want from-env", got.GetString("output"))
	}
	if want := []string{"positional"}; !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("Args = %#v, want %#v", got.Args, want)
	}
}

func TestRunMapsUsageAndExecutionErrors(t *testing.T) {
	app, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	cause := errors.New("boom")
	if err := app.AddCommand(CommandSpec{
		Name:  "run",
		Flags: []FlagSpec{{Name: "name", Type: FlagTypeString, Required: true}},
		Run: func(*Context) error {
			return cause
		},
	}); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	err = app.RunWithIO(context.Background(), []string{"run"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("missing required error = %T, want *UsageError", err)
	}
	if got := GetExitCode(err); got != ExitUsage {
		t.Fatalf("usage exit = %d, want %d", got, ExitUsage)
	}

	err = app.RunWithIO(context.Background(), []string{"run", "--name", "alice"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("execution error = %T, want *CommandError", err)
	}
	if !errors.Is(err, cause) {
		t.Fatal("execution error does not wrap cause")
	}
	if got := GetExitCode(err); got != ExitError {
		t.Fatalf("command exit = %d, want %d", got, ExitError)
	}
}

func TestRunMapsInvalidEnvDefaultToUsageError(t *testing.T) {
	t.Setenv("CLI_TEST_COUNT", "not-an-int")

	app, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if err := app.AddCommand(CommandSpec{
		Name:  "run",
		Flags: []FlagSpec{{Name: "count", Type: FlagTypeInt, EnvVar: "CLI_TEST_COUNT"}},
		Run:   func(*Context) error { return nil },
	}); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	err = app.RunWithIO(context.Background(), []string{"run"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("invalid env error = %T, want *UsageError", err)
	}
	if got := GetExitCode(err); got != ExitUsage {
		t.Fatalf("exit = %d, want %d", got, ExitUsage)
	}
}

func TestAddCommandRejectsDuplicateNames(t *testing.T) {
	app, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if err := app.AddCommand(CommandSpec{Name: "run"}); err != nil {
		t.Fatalf("first AddCommand() error = %v", err)
	}

	err = app.AddCommand(CommandSpec{Name: "run"})
	if err == nil {
		t.Fatal("second AddCommand() error = nil, want duplicate error")
	}
	if !strings.Contains(err.Error(), ErrMsgDuplicateCommand) {
		t.Fatalf("duplicate error = %q, want duplicate message", err.Error())
	}
}

func TestRunHelpAndVersionUseCobra(t *testing.T) {
	app, err := NewApp(Config{Name: "tool", Version: "1.2.3", Description: "test cli"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if err := app.AddCommand(CommandSpec{Name: "run", Description: "run command"}); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	var help bytes.Buffer
	if err := app.RunWithIO(context.Background(), []string{"--help"}, nil, &help, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunWithIO(--help) error = %v", err)
	}
	helpText := help.String()
	for _, want := range []string{"test cli", "run command", "Usage:"} {
		if !strings.Contains(helpText, want) {
			t.Fatalf("help output %q does not contain %q", helpText, want)
		}
	}

	var version bytes.Buffer
	if err := app.RunWithIO(context.Background(), []string{"--version"}, nil, &version, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunWithIO(--version) error = %v", err)
	}
	if got := strings.TrimSpace(version.String()); got != "tool version 1.2.3" {
		t.Fatalf("version output = %q, want %q", got, "tool version 1.2.3")
	}
}

func TestRunWithoutArgsStartsInteractiveHome(t *testing.T) {
	impl, err := NewApp(Config{Name: "tool", Version: "1.2.3", Description: "test cli"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	appImpl := impl.(*app)

	var called bool
	appImpl.runShell = func(_ context.Context, _ *app, model homeModel, _ streams, _ programOptions) (homeResult, error) {
		called = true
		shell := newShellModel(context.Background(), func() {}, appImpl, model, streams{}, make(chan tea.Msg, 1))
		view := shell.View()
		if !view.AltScreen {
			t.Fatal("shell view AltScreen = false, want true")
		}
		if !strings.Contains(view.Content, "tool v1.2.3") {
			t.Fatalf("shell content %q does not contain title", view.Content)
		}
		return homeResult{}, &CancelledError{}
	}

	err = appImpl.RunWithIO(context.Background(), nil, nil, &bytes.Buffer{}, &bytes.Buffer{})
	var cancelled *CancelledError
	if !errors.As(err, &cancelled) {
		t.Fatalf("RunWithIO(nil) error = %T, want *CancelledError", err)
	}
	if !called {
		t.Fatal("interactive home runner was not called")
	}
}

func TestHomeModelNavigationHelpAndQuit(t *testing.T) {
	model := newHomeModel(homeConfig{
		Name:        "tool",
		Version:     "1.2.3",
		Description: "test cli",
		Theme:       DefaultTheme(),
		Commands: []homeCommand{
			{Name: "server", Label: "启动 / server", Description: "run server", Help: "server help"},
			{Name: "db", Label: "数据库 / db", Description: "database tools", Help: "db help"},
		},
	})

	if view := model.View(); !strings.Contains(view.Content, "启动 / server") || !view.AltScreen {
		t.Fatalf("initial view = %#v", view)
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = updated.(homeModel)
	if model.width != 100 || model.height != 40 {
		t.Fatalf("size = %dx%d, want 100x40", model.width, model.height)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(homeModel)
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	model = updated.(homeModel)
	if !model.showingHelp || !strings.Contains(model.View().Content, "db help") {
		t.Fatalf("help state = %v, content = %q", model.showingHelp, model.View().Content)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(homeModel)
	if model.showingHelp {
		t.Fatal("showingHelp = true after esc, want false")
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(homeModel)
	if model.selectedCmd != "db" {
		t.Fatalf("selected command = %q, want db", model.selectedCmd)
	}
	if cmd == nil {
		t.Fatal("enter command = nil")
	}

	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	model = updated.(homeModel)
	if !model.exited || model.cancelled {
		t.Fatalf("quit state exited=%v cancelled=%v, want exited normal", model.exited, model.cancelled)
	}
	if cmd == nil {
		t.Fatal("quit command = nil")
	}

	model = newHomeModel(homeConfig{Name: "tool", Theme: DefaultTheme()})
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = updated.(homeModel)
	if !model.cancelled || model.exited {
		t.Fatalf("ctrl+c state exited=%v cancelled=%v, want cancelled", model.exited, model.cancelled)
	}
	if cmd == nil {
		t.Fatal("ctrl+c command = nil")
	}
}

func TestDefaultHomeRunnerMapsQToNormalExit(t *testing.T) {
	model := newHomeModel(homeConfig{
		Name:  "tool",
		Theme: DefaultTheme(),
	})

	var stdout, stderr bytes.Buffer
	result, err := defaultHomeRunner(
		context.Background(),
		model,
		streams{
			stdin:  strings.NewReader("q"),
			stdout: &stdout,
			stderr: &stderr,
		},
		programOptions{AltScreen: true, teaOptions: []tea.ProgramOption{tea.WithoutRenderer(), tea.WithoutSignals()}},
	)
	if err != nil {
		t.Fatalf("defaultHomeRunner() error = %v", err)
	}
	if !result.exited {
		t.Fatal("defaultHomeRunner() exited = false, want true")
	}
}

func TestHomeModelExitReturnsNormalResult(t *testing.T) {
	model := newHomeModel(homeConfig{
		Name:  "tool",
		Theme: DefaultTheme(),
		Commands: []homeCommand{
			{Name: "exit", Label: "退出 / exit", Builtin: homeBuiltinExit},
		},
	})
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(homeModel)
	if !model.result().exited {
		t.Fatal("exit result = false, want true")
	}
	if cmd == nil {
		t.Fatal("exit command = nil")
	}
}

func TestHomeModelFiltersAndOrdersCommands(t *testing.T) {
	impl, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	appImpl := impl.(*app)
	for _, spec := range []CommandSpec{
		{Name: "server", Description: "hidden server", HomeHidden: true},
		{Name: "service", Description: "service center", HomeLabel: "服务 / service", HomeOrder: 20},
		{Name: "run", Description: "start center", HomeLabel: "启动 / run", HomeOrder: 10},
	} {
		if err := appImpl.AddCommand(spec); err != nil {
			t.Fatalf("AddCommand(%s) error = %v", spec.Name, err)
		}
	}
	model, err := appImpl.homeModel(context.Background(), streams{stdin: strings.NewReader(""), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("homeModel() error = %v", err)
	}
	content := model.View().Content
	if strings.Contains(content, "server") {
		t.Fatalf("hidden command rendered in home:\n%s", content)
	}
	runIndex := strings.Index(content, "启动 / run")
	serviceIndex := strings.Index(content, "服务 / service")
	if runIndex < 0 || serviceIndex < 0 || runIndex > serviceIndex {
		t.Fatalf("home order/content unexpected:\n%s", content)
	}
	if !strings.Contains(content, "帮助 / help") || !strings.Contains(content, "退出 / exit") {
		t.Fatalf("home builtins missing:\n%s", content)
	}
}

func TestHomeSelectionExecutesCommand(t *testing.T) {
	impl, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	appImpl := impl.(*app)
	var ran bool
	if err := appImpl.AddCommand(CommandSpec{
		Name: "run",
		Run: func(*Context) error {
			ran = true
			return nil
		},
	}); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}
	appImpl.runShell = func(context.Context, *app, homeModel, streams, programOptions) (homeResult, error) {
		return homeResult{command: "run"}, nil
	}
	if err := appImpl.RunWithIO(context.Background(), nil, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunWithIO(nil) error = %v", err)
	}
	if !ran {
		t.Fatal("selected command was not executed")
	}
}

func TestPromptUIUsesInjectedIO(t *testing.T) {
	var out bytes.Buffer
	ui := NewPromptUI(strings.NewReader("2\n\ncustom\nsecret\n"), &out)
	selected, err := ui.Select(context.Background(), "choose", []SelectOption{
		{Value: "one", Label: "One"},
		{Value: "two", Label: "Two"},
	})
	if err != nil || selected != "two" {
		t.Fatalf("Select() = %q, %v; want two, nil", selected, err)
	}
	confirmed, err := ui.Confirm(context.Background(), "confirm", true)
	if err != nil || !confirmed {
		t.Fatalf("Confirm() = %v, %v; want true, nil", confirmed, err)
	}
	input, err := ui.Input(context.Background(), "input", "default")
	if err != nil || input != "custom" {
		t.Fatalf("Input() = %q, %v; want custom, nil", input, err)
	}
	password, err := ui.Password(context.Background(), "password")
	if err != nil || password != "secret" {
		t.Fatalf("Password() = %q, %v; want secret, nil", password, err)
	}
	if err := ui.Info("done"); err != nil {
		t.Fatalf("Info() error = %v", err)
	}
}

func TestPromptUIReturnsWriteErrors(t *testing.T) {
	writeErr := errors.New("stdout unavailable")
	tests := []struct {
		name string
		run  func(PromptUI) error
	}{
		{
			name: "Select",
			run: func(ui PromptUI) error {
				_, err := ui.Select(context.Background(), "choose", []SelectOption{{Value: "one", Label: "One"}})
				return err
			},
		},
		{
			name: "Confirm",
			run: func(ui PromptUI) error {
				_, err := ui.Confirm(context.Background(), "confirm", true)
				return err
			},
		},
		{
			name: "Input",
			run: func(ui PromptUI) error {
				_, err := ui.Input(context.Background(), "input", "default")
				return err
			},
		},
		{
			name: "Password",
			run: func(ui PromptUI) error {
				_, err := ui.Password(context.Background(), "password")
				return err
			},
		},
		{
			name: "Info",
			run: func(ui PromptUI) error {
				return ui.Info("done")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := NewPromptUI(strings.NewReader(""), promptWriteErrorWriter{err: writeErr})
			err := tt.run(ui)
			if !errors.Is(err, writeErr) {
				t.Fatalf("%s() error = %v, want %v", tt.name, err, writeErr)
			}
			if !strings.Contains(err.Error(), "write prompt output") {
				t.Fatalf("%s() error missing prompt output context: %v", tt.name, err)
			}
		})
	}
}

func TestChainArgsAutofillKeyedPrompts(t *testing.T) {
	app, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	var gotName string
	var gotMode string
	var gotConfirm bool
	var gotInput string
	var gotPassword string
	var changed map[string]bool
	if err := app.AddCommand(CommandSpec{
		Name:  "ask",
		Flags: []FlagSpec{{Name: "name", Type: FlagTypeString}},
		Run: func(ctx *Context) error {
			gotName = ctx.GetString("name")
			changed = ctx.ChangedFlags
			var err error
			gotMode, err = SelectKey(ctx.Context, ctx.UI, "mode", "mode", []SelectOption{
				{Value: "one", Label: "One"},
				{Value: "two", Label: "Two"},
			})
			if err != nil {
				return err
			}
			gotConfirm, err = ConfirmKey(ctx.Context, ctx.UI, "confirm", "confirm", false)
			if err != nil {
				return err
			}
			gotInput, err = InputKey(ctx.Context, ctx.UI, "value", "value", "default")
			if err != nil {
				return err
			}
			gotPassword, err = PasswordKey(ctx.Context, ctx.UI, "secret", "secret")
			return err
		},
	}); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = app.RunWithIO(context.Background(), []string{
		"ask",
		"--name", "alice",
		"--chain.mode=two",
		"--chain.confirm", "true",
		"--chain.value=custom",
		"--chain.secret", "s3cr3t",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v\nstderr:\n%s", err, stderr.String())
	}
	if gotName != "alice" || gotMode != "two" || !gotConfirm || gotInput != "custom" || gotPassword != "s3cr3t" {
		t.Fatalf("chain answers = name %q mode %q confirm %v input %q password %q", gotName, gotMode, gotConfirm, gotInput, gotPassword)
	}
	if !changed["name"] {
		t.Fatalf("name flag should be changed: %#v", changed)
	}
	for _, key := range []string{"mode", "confirm", "value", "secret"} {
		if changed[key] {
			t.Fatalf("chain key %q polluted ChangedFlags: %#v", key, changed)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("keyed prompts should not write fallback UI output:\n%s", stdout.String())
	}
}

type promptWriteErrorWriter struct {
	err error
}

func (w promptWriteErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestChainArgsDoNotSwallowUnknownFlags(t *testing.T) {
	app, err := NewApp(Config{Name: "tool"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if err := app.AddCommand(CommandSpec{Name: "run", Run: func(*Context) error { return nil }}); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	err = app.RunWithIO(context.Background(), []string{"run", "--unknown=value"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("RunWithIO(unknown flag) error = %T %v, want *UsageError", err, err)
	}
}

func TestChainWildcardAnswersDynamicPromptKeys(t *testing.T) {
	ui := WithPromptAnswers(NewPromptUI(strings.NewReader(""), io.Discard), map[string]string{
		"privacy.*.value":                "fallback",
		"privacy.auth.*.value":           "generate",
		"privacy.auth.signing_key.*":     "from-suffix",
		"privacy.auth.signing_key.value": "exact",
	})

	got, err := InputKey(context.Background(), ui, "privacy.auth.signing_key.value", "secret", "")
	if err != nil {
		t.Fatalf("InputKey(exact) error = %v", err)
	}
	if got != "exact" {
		t.Fatalf("InputKey(exact) = %q, want exact", got)
	}

	got, err = InputKey(context.Background(), ui, "privacy.auth.refresh_token_pepper.value", "secret", "")
	if err != nil {
		t.Fatalf("InputKey(specific wildcard) error = %v", err)
	}
	if got != "generate" {
		t.Fatalf("InputKey(specific wildcard) = %q, want generate", got)
	}

	got, err = InputKey(context.Background(), ui, "privacy.database.mysql.password.value", "secret", "")
	if err != nil {
		t.Fatalf("InputKey(fallback wildcard) error = %v", err)
	}
	if got != "fallback" {
		t.Fatalf("InputKey(fallback wildcard) = %q, want fallback", got)
	}
}

func TestChainSelectRejectsInvalidAnswer(t *testing.T) {
	ui := WithPromptAnswers(NewPromptUI(strings.NewReader(""), io.Discard), map[string]string{"mode": "missing"})
	_, err := SelectKey(context.Background(), ui, "mode", "mode", []SelectOption{
		{Value: "one", Label: "One"},
		{Value: "two", Label: "Two"},
	})
	if err == nil || !strings.Contains(err.Error(), "chain.mode") {
		t.Fatalf("SelectKey() error = %v, want chain.mode usage detail", err)
	}
}

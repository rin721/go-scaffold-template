package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type streams struct {
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	ui           PromptUI
	chainAnswers map[string]string
}

type app struct {
	cfg      Config
	theme    Theme
	commands []CommandSpec
	names    map[string]struct{}
	mu       sync.RWMutex

	runHome  func(context.Context, homeModel, streams, programOptions) (homeResult, error)
	runShell func(context.Context, *app, homeModel, streams, programOptions) (homeResult, error)
}

// NewApp 创建一个由 Cobra 和 Bubble Tea 驱动的 CLI 应用。
func NewApp(cfg Config) (App, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, fmt.Errorf("cli app name cannot be empty")
	}

	theme := DefaultTheme()
	if cfg.Theme != nil {
		theme = *cfg.Theme
	}

	return &app{
		cfg:      cfg,
		theme:    theme,
		names:    make(map[string]struct{}),
		runHome:  defaultHomeRunner,
		runShell: defaultShellRunner,
	}, nil
}

func (a *app) Name() string {
	return a.cfg.Name
}

func (a *app) Version() string {
	return a.cfg.Version
}

func (a *app) Description() string {
	return a.cfg.Description
}

func (a *app) AddCommand(spec CommandSpec) error {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.names[name]; exists {
		return fmt.Errorf("%s: %s", ErrMsgDuplicateCommand, name)
	}
	a.names[name] = struct{}{}
	a.commands = append(a.commands, spec)
	return nil
}

func (a *app) Run(ctx context.Context, args []string) error {
	s := streams{
		stdin:  firstReader(a.cfg.Stdin, os.Stdin),
		stdout: firstWriter(a.cfg.Stdout, os.Stdout),
		stderr: firstWriter(a.cfg.Stderr, os.Stderr),
	}
	return a.run(ctx, args, s)
}

func (a *app) RunWithIO(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	s := streams{
		stdin:  firstReader(stdin, os.Stdin),
		stdout: firstWriter(stdout, io.Discard),
		stderr: firstWriter(stderr, io.Discard),
	}
	return a.run(ctx, args, s)
}

func (a *app) run(ctx context.Context, args []string, s streams) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cleanedArgs, chainAnswers, err := extractChainArgs(args)
	if err != nil {
		return err
	}
	args = cleanedArgs
	s.chainAnswers = mergePromptAnswers(s.chainAnswers, chainAnswers)

	if len(args) == 0 && !a.cfg.DisableInteractiveHome {
		model, err := a.homeModel(ctx, s)
		if err != nil {
			return err
		}
		result, err := a.runShell(ctx, a, model, s, resolveProgramOptions(a.cfg.ProgramOptions))
		if err != nil {
			return err
		}
		if result.exited {
			return nil
		}
		if result.command != "" {
			return a.run(ctx, []string{result.command}, s)
		}
		return nil
	}

	root, err := a.rootCommand(ctx, s)
	if err != nil {
		return err
	}
	root.SetArgs(args)

	_, err = root.ExecuteC()
	return normalizeExecuteError(err)
}

func normalizeExecuteError(err error) error {
	if err == nil {
		return nil
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return usageErr
	}
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		return commandErr
	}
	var cancelledErr *CancelledError
	if errors.As(err, &cancelledErr) {
		return cancelledErr
	}
	return &UsageError{Message: err.Error()}
}

func (a *app) runCommandWithUI(ctx context.Context, args []string, s streams, ui PromptUI) error {
	commandStreams := s
	commandStreams.ui = ui
	root, err := a.rootCommand(ctx, commandStreams)
	if err != nil {
		return err
	}
	root.SetArgs(args)
	_, err = root.ExecuteC()
	return normalizeExecuteError(err)
}

func (a *app) rootCommand(ctx context.Context, s streams) (*cobra.Command, error) {
	root := &cobra.Command{
		Use:           a.cfg.Name,
		Short:         a.cfg.Description,
		Long:          a.cfg.Description,
		Version:       a.cfg.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetIn(s.stdin)
	root.SetOut(s.stdout)
	root.SetErr(s.stderr)
	if err := registerPersistentFlags(root, a.cfg.GlobalFlags); err != nil {
		return nil, err
	}

	a.mu.RLock()
	specs := append([]CommandSpec(nil), a.commands...)
	a.mu.RUnlock()

	for _, spec := range specs {
		cmd, err := a.command(ctx, spec, s)
		if err != nil {
			return nil, err
		}
		root.AddCommand(cmd)
	}

	return root, nil
}

func (a *app) command(ctx context.Context, spec CommandSpec, s streams) (*cobra.Command, error) {
	use := spec.Use
	if use == "" {
		use = spec.Name
	}

	cmd := &cobra.Command{
		Use:           use,
		Aliases:       append([]string(nil), spec.Aliases...),
		Short:         firstString(spec.Description, spec.Long),
		Long:          firstString(spec.Long, spec.Description),
		Example:       spec.Example,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			commandCtx, err := a.commandContext(ctx, cmd, args, mergeFlagSpecs(a.cfg.GlobalFlags, spec.Flags), s)
			if err != nil {
				return err
			}
			if err := validateRequiredFlags(commandCtx, spec.Flags); err != nil {
				return err
			}
			if spec.Args != nil {
				if err := spec.Args(commandCtx); err != nil {
					var usageErr *UsageError
					if errors.As(err, &usageErr) {
						return usageErr
					}
					return &UsageError{Command: cmd.CommandPath(), Message: err.Error()}
				}
			}
			if spec.Run == nil {
				return cmd.Help()
			}
			if err := spec.Run(commandCtx); err != nil {
				var usageErr *UsageError
				if errors.As(err, &usageErr) {
					return usageErr
				}
				var commandErr *CommandError
				if errors.As(err, &commandErr) {
					return commandErr
				}
				var cancelledErr *CancelledError
				if errors.As(err, &cancelledErr) {
					return cancelledErr
				}
				return &CommandError{
					Command: cmd.CommandPath(),
					Message: "execution failed",
					Cause:   err,
				}
			}
			return nil
		},
	}
	cmd.SetIn(s.stdin)
	cmd.SetOut(s.stdout)
	cmd.SetErr(s.stderr)

	if err := registerFlags(cmd, spec.Flags); err != nil {
		return nil, err
	}

	for _, child := range spec.Commands {
		childCmd, err := a.command(ctx, child, s)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(childCmd)
	}

	return cmd, nil
}

func registerFlags(cmd *cobra.Command, specs []FlagSpec) error {
	return registerFlagSet(cmd, cmd.Flags(), specs)
}

func registerPersistentFlags(cmd *cobra.Command, specs []FlagSpec) error {
	return registerFlagSet(cmd, cmd.PersistentFlags(), specs)
}

func registerFlagSet(cmd *cobra.Command, flags *pflag.FlagSet, specs []FlagSpec) error {
	for _, spec := range specs {
		if strings.TrimSpace(spec.Name) == "" {
			return fmt.Errorf("flag name cannot be empty")
		}

		short := spec.Shorthand
		if short == "" {
			short = spec.ShortName
		}

		description := spec.Description
		if spec.Required {
			description = strings.TrimSpace(description + " (required)")
		}
		if spec.EnvVar != "" {
			description = strings.TrimSpace(description + " (env: " + spec.EnvVar + ")")
		}

		switch spec.Type {
		case FlagTypeString:
			def, err := defaultString(spec)
			if err != nil {
				return flagDefaultError(cmd, spec, err)
			}
			flags.StringP(spec.Name, short, def, description)
		case FlagTypeInt:
			def, err := defaultInt(spec)
			if err != nil {
				return flagDefaultError(cmd, spec, err)
			}
			flags.IntP(spec.Name, short, def, description)
		case FlagTypeBool:
			def, err := defaultBool(spec)
			if err != nil {
				return flagDefaultError(cmd, spec, err)
			}
			flags.BoolP(spec.Name, short, def, description)
		case FlagTypeStringSlice:
			def, err := defaultStringSlice(spec)
			if err != nil {
				return flagDefaultError(cmd, spec, err)
			}
			flags.StringSliceP(spec.Name, short, def, description)
		default:
			return fmt.Errorf("unsupported flag type for --%s", spec.Name)
		}
	}
	return nil
}

func mergeFlagSpecs(global []FlagSpec, local []FlagSpec) []FlagSpec {
	if len(global) == 0 {
		return local
	}
	specs := make([]FlagSpec, 0, len(global)+len(local))
	specs = append(specs, global...)
	specs = append(specs, local...)
	return specs
}

func flagDefaultError(cmd *cobra.Command, spec FlagSpec, err error) error {
	return &UsageError{
		Command: cmd.CommandPath(),
		Message: fmt.Sprintf("%s for --%s: %v", ErrMsgInvalidFlagValue, spec.Name, err),
	}
}

func (a *app) commandContext(ctx context.Context, cmd *cobra.Command, args []string, specs []FlagSpec, s streams) (*Context, error) {
	values := make(map[string]interface{}, len(specs))
	changed := make(map[string]bool, len(specs))
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		changed[flag.Name] = true
	})
	for _, spec := range specs {
		switch spec.Type {
		case FlagTypeString:
			value, err := cmd.Flags().GetString(spec.Name)
			if err != nil {
				return nil, &UsageError{Command: cmd.CommandPath(), Message: err.Error()}
			}
			values[spec.Name] = value
		case FlagTypeInt:
			value, err := cmd.Flags().GetInt(spec.Name)
			if err != nil {
				return nil, &UsageError{Command: cmd.CommandPath(), Message: err.Error()}
			}
			values[spec.Name] = value
		case FlagTypeBool:
			value, err := cmd.Flags().GetBool(spec.Name)
			if err != nil {
				return nil, &UsageError{Command: cmd.CommandPath(), Message: err.Error()}
			}
			values[spec.Name] = value
		case FlagTypeStringSlice:
			value, err := cmd.Flags().GetStringSlice(spec.Name)
			if err != nil {
				return nil, &UsageError{Command: cmd.CommandPath(), Message: err.Error()}
			}
			values[spec.Name] = value
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ui := s.ui
	if ui == nil {
		ui = newPromptUI(s)
	}
	ui = WithPromptAnswers(ui, s.chainAnswers)
	return &Context{
		Context:      ctx,
		CommandName:  cmd.Name(),
		CommandPath:  cmd.CommandPath(),
		Args:         append([]string(nil), args...),
		Flags:        values,
		ChangedFlags: changed,
		Stdin:        s.stdin,
		Stdout:       s.stdout,
		Stderr:       s.stderr,
		UI:           ui,
	}, nil
}

func validateRequiredFlags(ctx *Context, specs []FlagSpec) error {
	for _, spec := range specs {
		if !spec.Required {
			continue
		}
		value, ok := ctx.Flags[spec.Name]
		if !ok {
			return &UsageError{Command: ctx.CommandPath, Message: fmt.Sprintf("%s --%s", ErrMsgMissingRequired, spec.Name)}
		}
		switch spec.Type {
		case FlagTypeString:
			if value == "" {
				return &UsageError{Command: ctx.CommandPath, Message: fmt.Sprintf("required flag --%s cannot be empty", spec.Name)}
			}
		case FlagTypeStringSlice:
			slice, ok := value.([]string)
			if !ok || len(slice) == 0 {
				return &UsageError{Command: ctx.CommandPath, Message: fmt.Sprintf("required flag --%s cannot be empty", spec.Name)}
			}
		}
	}
	return nil
}

func defaultString(spec FlagSpec) (string, error) {
	value := defaultSource(spec)
	if value == nil {
		return "", nil
	}
	return fmt.Sprint(value), nil
}

func defaultInt(spec FlagSpec) (int, error) {
	value := defaultSource(spec)
	if value == nil {
		return 0, nil
	}
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return strconv.Atoi(fmt.Sprint(v))
	}
}

func defaultBool(spec FlagSpec) (bool, error) {
	value := defaultSource(spec)
	if value == nil {
		return false, nil
	}
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return strconv.ParseBool(fmt.Sprint(v))
	}
}

func defaultStringSlice(spec FlagSpec) ([]string, error) {
	value := defaultSource(spec)
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...), nil
	case string:
		if v == "" {
			return nil, nil
		}
		return strings.Split(v, ","), nil
	default:
		if fmt.Sprint(v) == "" {
			return nil, nil
		}
		return strings.Split(fmt.Sprint(v), ","), nil
	}
}

func defaultSource(spec FlagSpec) interface{} {
	if spec.EnvVar != "" {
		if value := os.Getenv(spec.EnvVar); value != "" {
			return value
		}
	}
	return spec.Default
}

func (a *app) homeModel(ctx context.Context, s streams) (homeModel, error) {
	a.mu.RLock()
	specs := append([]CommandSpec(nil), a.commands...)
	a.mu.RUnlock()

	root, err := a.rootCommand(ctx, s)
	if err != nil {
		return homeModel{}, err
	}

	commands := make([]homeCommand, 0, len(specs))
	for index, spec := range specs {
		if spec.HomeHidden {
			continue
		}
		help := commandHelp(root, spec.Name)
		commands = append(commands, homeCommand{
			Name:        spec.Name,
			Label:       firstString(spec.HomeLabel, spec.Name),
			Description: spec.Description,
			Help:        help,
			Order:       spec.HomeOrder,
			index:       index,
		})
	}
	sort.SliceStable(commands, func(i, j int) bool {
		if commands[i].Order == commands[j].Order {
			return commands[i].index < commands[j].index
		}
		return commands[i].Order < commands[j].Order
	})
	commands = append(commands,
		homeCommand{
			Name:        "help",
			Label:       "帮助 / help",
			Description: "查看命令和配置说明",
			Help:        rootHelp(root),
			Builtin:     homeBuiltinHelp,
			Order:       9000,
		},
		homeCommand{
			Name:        "exit",
			Label:       "退出 / exit",
			Description: "退出 CLI 首页，不影响后台服务",
			Builtin:     homeBuiltinExit,
			Order:       10000,
		},
	)

	return newHomeModel(homeConfig{
		Name:        a.cfg.Name,
		Version:     a.cfg.Version,
		Description: a.cfg.Description,
		Commands:    commands,
		Theme:       a.theme,
	}), nil
}

func commandHelp(root *cobra.Command, name string) string {
	cmd, _, err := root.Find([]string{name})
	if err != nil || cmd == nil {
		return ""
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	_ = cmd.Help()
	return out.String()
}

func rootHelp(root *cobra.Command) string {
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	_ = root.Help()
	return out.String()
}

func defaultHomeRunner(ctx context.Context, model homeModel, s streams, opts programOptions) (homeResult, error) {
	model.altScreen = opts.AltScreen
	options := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(s.stdin),
		tea.WithOutput(s.stdout),
	}
	options = append(options, opts.teaOptions...)

	program := tea.NewProgram(model, options...)
	finalModel, err := program.Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) {
			return homeResult{}, &CancelledError{}
		}
		return homeResult{}, err
	}
	if finalHome, ok := finalModel.(homeModel); ok && finalHome.cancelled {
		return homeResult{}, &CancelledError{}
	} else if ok {
		return finalHome.result(), nil
	}
	return homeResult{}, nil
}

func resolveProgramOptions(options []ProgramOption) programOptions {
	resolved := programOptions{AltScreen: true}
	for _, option := range options {
		if option != nil {
			option.applyProgramOptions(&resolved)
		}
	}
	return resolved
}

func firstReader(value io.Reader, fallback io.Reader) io.Reader {
	if value != nil {
		return value
	}
	return fallback
}

func firstWriter(value io.Writer, fallback io.Writer) io.Writer {
	if value != nil {
		return value
	}
	return fallback
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

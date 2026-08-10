package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

type shellScene string

const (
	shellSceneHome    shellScene = "home"
	shellSceneMenu    shellScene = "menu"
	shellSceneForm    shellScene = "form"
	shellSceneConfirm shellScene = "confirm"
	shellSceneTask    shellScene = "task"
	shellSceneLogs    shellScene = "logs"
	shellSceneResult  shellScene = "result"
	shellSceneHelp    shellScene = "help"
)

type shellPromptKind int

const (
	shellPromptSelect shellPromptKind = iota
	shellPromptConfirm
	shellPromptInput
	shellPromptPassword
)

type shellPromptRequest struct {
	kind        shellPromptKind
	prompt      string
	options     []SelectOption
	defaultText string
	defaultBool bool
	response    chan shellPromptResponse
}

type shellPromptResponse struct {
	text string
	ok   bool
	err  error
}

type shellPromptMsg struct {
	request *shellPromptRequest
}

type shellOutputMsg struct {
	text string
}

type shellFlowDoneMsg struct {
	err error
}

type shellSpinnerMsg time.Time
type shellTransitionMsg time.Time

type shellModel struct {
	ctx    context.Context
	cancel context.CancelFunc
	app    *app

	streams streams
	events  chan tea.Msg

	name        string
	version     string
	description string
	commands    []homeCommand
	theme       Theme

	scene         shellScene
	previousScene shellScene
	selected      int
	width         int
	height        int
	focus         int
	spinner       int
	transition    int

	activeCommand string
	flowCancel    context.CancelFunc
	flowErr       error
	output        string

	prompt         *shellPromptRequest
	promptSelected int
	input          string

	paletteOpen     bool
	paletteFilter   string
	paletteSelected int

	cancelled bool
	exited    bool
	altScreen bool
}

func defaultShellRunner(ctx context.Context, appImpl *app, home homeModel, s streams, opts programOptions) (homeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	shellCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := make(chan tea.Msg, 256)
	model := newShellModel(shellCtx, cancel, appImpl, home, s, events)
	model.altScreen = opts.AltScreen
	options := []tea.ProgramOption{
		tea.WithContext(shellCtx),
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
	if finalShell, ok := finalModel.(shellModel); ok {
		if finalShell.cancelled {
			return homeResult{}, &CancelledError{}
		}
		return homeResult{exited: finalShell.exited}, nil
	}
	return homeResult{}, nil
}

func newShellModel(ctx context.Context, cancel context.CancelFunc, appImpl *app, home homeModel, s streams, events chan tea.Msg) shellModel {
	return shellModel{
		ctx:         ctx,
		cancel:      cancel,
		app:         appImpl,
		streams:     s,
		events:      events,
		name:        home.name,
		version:     home.version,
		description: home.description,
		commands:    append([]homeCommand(nil), home.commands...),
		theme:       home.theme,
		scene:       shellSceneHome,
		transition:  3,
		altScreen:   true,
	}
}

func (m shellModel) Init() tea.Cmd {
	return tea.Batch(shellWaitEvent(m.events), shellSpinnerCmd(), shellTransitionCmd())
}

func (m shellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case shellPromptMsg:
		m.prompt = msg.request
		m.input = ""
		m.promptSelected = 0
		if msg.request != nil && msg.request.kind == shellPromptConfirm && !msg.request.defaultBool {
			m.promptSelected = 1
		}
		m.scene = m.sceneForPrompt(msg.request)
		m.transition = 3
		return m, tea.Batch(shellWaitEvent(m.events), shellTransitionCmd())
	case shellOutputMsg:
		m.appendOutput(msg.text)
		if m.scene == shellSceneTask && looksLikeLogOutput(m.output) {
			m.scene = shellSceneLogs
			m.transition = 2
			return m, tea.Batch(shellWaitEvent(m.events), shellTransitionCmd())
		}
		return m, shellWaitEvent(m.events)
	case shellFlowDoneMsg:
		m.flowCancel = nil
		m.prompt = nil
		m.flowErr = msg.err
		m.scene = shellSceneResult
		m.transition = 3
		return m, shellTransitionCmd()
	case shellSpinnerMsg:
		m.spinner = (m.spinner + 1) % len(shellSpinnerFrames)
		return m, shellSpinnerCmd()
	case shellTransitionMsg:
		if m.transition > 0 {
			m.transition--
			if m.transition > 0 {
				return m, shellTransitionCmd()
			}
		}
	case tea.KeyPressMsg:
		if m.paletteOpen {
			return m.handlePaletteKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m shellModel) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = m.altScreen
	return view
}

func (m shellModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := keyName(msg)
	switch key {
	case "ctrl+c":
		return m.quit(true)
	case "/":
		m.paletteOpen = true
		m.paletteFilter = ""
		m.paletteSelected = 0
		return m, nil
	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "?":
		m.previousScene = m.scene
		m.scene = shellSceneHelp
		m.transition = 2
		return m, shellTransitionCmd()
	}

	switch m.scene {
	case shellSceneHome:
		return m.handleHomeKey(key)
	case shellSceneMenu:
		return m.handleMenuKey(key)
	case shellSceneConfirm:
		return m.handleConfirmKey(key)
	case shellSceneForm:
		return m.handleFormKey(msg)
	case shellSceneTask, shellSceneLogs:
		return m.handleTaskKey(key)
	case shellSceneResult:
		return m.handleResultKey(key)
	case shellSceneHelp:
		return m.handleHelpKey(key)
	default:
		return m, nil
	}
}

func (m shellModel) handleHomeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if len(m.commands) > 0 {
			m.selected = (m.selected - 1 + len(m.commands)) % len(m.commands)
		}
	case "down", "j":
		if len(m.commands) > 0 {
			m.selected = (m.selected + 1) % len(m.commands)
		}
	case "enter":
		if len(m.commands) == 0 {
			return m, nil
		}
		return m.activateCommand(m.commands[m.selected])
	case "q", "esc":
		return m.quit(false)
	}
	return m, nil
}

func (m shellModel) handleMenuKey(key string) (tea.Model, tea.Cmd) {
	if m.prompt == nil {
		m.scene = shellSceneTask
		return m, nil
	}
	switch key {
	case "up", "k":
		if len(m.prompt.options) > 0 {
			m.promptSelected = (m.promptSelected - 1 + len(m.prompt.options)) % len(m.prompt.options)
		}
	case "down", "j":
		if len(m.prompt.options) > 0 {
			m.promptSelected = (m.promptSelected + 1) % len(m.prompt.options)
		}
	case "enter":
		if len(m.prompt.options) > 0 {
			value := m.prompt.options[m.promptSelected].Value
			m.respond(shellPromptResponse{text: value})
			m.prompt = nil
			m.scene = shellSceneTask
			m.transition = 2
			return m, shellTransitionCmd()
		}
	case "esc", "q":
		m.cancelPrompt()
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			index := int(key[0] - '1')
			if index >= 0 && index < len(m.prompt.options) {
				m.promptSelected = index
				value := m.prompt.options[m.promptSelected].Value
				m.respond(shellPromptResponse{text: value})
				m.prompt = nil
				m.scene = shellSceneTask
				m.transition = 2
				return m, shellTransitionCmd()
			}
		}
	}
	return m, nil
}

func (m shellModel) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "left", "right", "up", "down", "h", "j", "k", "l":
		if m.promptSelected == 0 {
			m.promptSelected = 1
		} else {
			m.promptSelected = 0
		}
	case "y":
		m.promptSelected = 0
		m.submitConfirm()
	case "n":
		m.promptSelected = 1
		m.submitConfirm()
	case "enter":
		m.submitConfirm()
	case "esc", "q":
		m.cancelPrompt()
	}
	if m.prompt == nil {
		m.scene = shellSceneTask
		m.transition = 2
		return m, shellTransitionCmd()
	}
	return m, nil
}

func (m shellModel) handleFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := keyName(msg)
	switch key {
	case "enter":
		value := m.input
		if value == "" && m.prompt != nil {
			value = m.prompt.defaultText
		}
		m.respond(shellPromptResponse{text: value})
		m.prompt = nil
		m.scene = shellSceneTask
		m.transition = 2
		return m, shellTransitionCmd()
	case "backspace", "ctrl+h":
		m.input = dropLastRune(m.input)
	case "esc":
		m.cancelPrompt()
		return m, nil
	default:
		if text := printableKeyText(msg); text != "" {
			m.input += text
		}
	}
	return m, nil
}

func (m shellModel) handleTaskKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		if m.flowCancel != nil {
			m.flowCancel()
			m.appendOutput("\n[cancel requested]\n")
			return m, nil
		}
		m.scene = shellSceneHome
	case "q":
		return m.quit(true)
	}
	return m, nil
}

func (m shellModel) handleResultKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		m.scene = shellSceneHome
		m.activeCommand = ""
		m.flowErr = nil
		m.output = ""
		m.transition = 3
		return m, shellTransitionCmd()
	case "q":
		return m.quit(false)
	}
	return m, nil
}

func (m shellModel) handleHelpKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "enter", "?":
		if m.previousScene == "" {
			m.previousScene = shellSceneHome
		}
		m.scene = m.previousScene
		m.transition = 2
		return m, shellTransitionCmd()
	case "q":
		return m.quit(true)
	}
	return m, nil
}

func (m shellModel) handlePaletteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := keyName(msg)
	filtered := m.filteredCommands()
	switch key {
	case "esc":
		m.paletteOpen = false
	case "up", "k":
		if len(filtered) > 0 {
			m.paletteSelected = (m.paletteSelected - 1 + len(filtered)) % len(filtered)
		}
	case "down", "j":
		if len(filtered) > 0 {
			m.paletteSelected = (m.paletteSelected + 1) % len(filtered)
		}
	case "enter":
		if len(filtered) > 0 {
			m.paletteOpen = false
			m.paletteFilter = ""
			return m.activateCommand(filtered[m.paletteSelected])
		}
	case "backspace", "ctrl+h":
		m.paletteFilter = dropLastRune(m.paletteFilter)
		m.paletteSelected = 0
	default:
		if text := printableKeyText(msg); text != "" && text != "/" {
			m.paletteFilter += text
			m.paletteSelected = 0
		}
	}
	return m, nil
}

func (m shellModel) activateCommand(command homeCommand) (tea.Model, tea.Cmd) {
	switch command.Builtin {
	case homeBuiltinHelp:
		m.previousScene = m.scene
		m.scene = shellSceneHelp
		m.transition = 2
		return m, shellTransitionCmd()
	case homeBuiltinExit:
		return m.quit(false)
	}
	if strings.TrimSpace(command.Name) == "" {
		return m, nil
	}
	flowCtx, cancel := context.WithCancel(m.ctx)
	m.flowCancel = cancel
	m.activeCommand = command.Name
	m.flowErr = nil
	m.output = ""
	m.prompt = nil
	m.input = ""
	m.scene = shellSceneTask
	m.transition = 3
	return m, tea.Batch(m.runCommand(flowCtx, command.Name), shellTransitionCmd())
}

func (m shellModel) runCommand(ctx context.Context, command string) tea.Cmd {
	appImpl := m.app
	baseStreams := m.streams
	events := m.events
	return func() tea.Msg {
		writer := &shellOutputWriter{events: events}
		errWriter := &shellOutputWriter{events: events}
		ui := &shellPromptUI{events: events}
		commandStreams := streams{
			stdin:        baseStreams.stdin,
			stdout:       writer,
			stderr:       errWriter,
			chainAnswers: baseStreams.chainAnswers,
		}
		err := appImpl.runCommandWithUI(ctx, []string{command}, commandStreams, ui)
		return shellFlowDoneMsg{err: err}
	}
}

func (m shellModel) quit(cancelled bool) (tea.Model, tea.Cmd) {
	if m.flowCancel != nil {
		m.flowCancel()
	}
	m.cancelPrompt()
	if m.cancel != nil {
		m.cancel()
	}
	m.cancelled = cancelled
	m.exited = !cancelled
	return m, tea.Quit
}

func (m *shellModel) appendOutput(text string) {
	if text == "" {
		return
	}
	m.output += text
	const maxOutputBytes = 32768
	if len(m.output) > maxOutputBytes {
		m.output = m.output[len(m.output)-maxOutputBytes:]
		if index := strings.IndexByte(m.output, '\n'); index >= 0 && index+1 < len(m.output) {
			m.output = m.output[index+1:]
		}
	}
}

func (m shellModel) cancelPrompt() {
	if m.prompt == nil {
		return
	}
	m.respond(shellPromptResponse{err: &CancelledError{}})
}

func (m shellModel) respond(response shellPromptResponse) {
	if m.prompt == nil {
		return
	}
	select {
	case m.prompt.response <- response:
	default:
	}
}

func (m *shellModel) submitConfirm() {
	if m.prompt == nil {
		return
	}
	m.respond(shellPromptResponse{ok: m.promptSelected == 0})
	m.prompt = nil
}

func (m shellModel) sceneForPrompt(req *shellPromptRequest) shellScene {
	if req == nil {
		return shellSceneTask
	}
	switch req.kind {
	case shellPromptSelect:
		return shellSceneMenu
	case shellPromptConfirm:
		return shellSceneConfirm
	default:
		return shellSceneForm
	}
}

func (m shellModel) render() string {
	var parts []string
	parts = append(parts, m.renderHeader())
	parts = append(parts, m.renderBody())
	parts = append(parts, m.renderStatus())
	if m.paletteOpen {
		parts = append(parts, m.renderPalette())
	}
	return m.theme.ShellFrame.Render(strings.Join(parts, "\n\n"))
}

func (m shellModel) renderHeader() string {
	title := m.theme.ShellHeader.Render(m.title())
	if m.activeCommand != "" {
		title += " " + m.theme.ShellAccent.Render("["+m.activeCommand+"]")
	}
	if m.transition > 0 {
		title += " " + m.theme.ShellMuted.Render(strings.Repeat(">", m.transition))
	}
	lines := []string{title}
	if m.description != "" {
		lines = append(lines, m.theme.ShellMuted.Render(m.description))
	}
	lines = append(lines, m.theme.ShellBreadcrumb.Render(m.breadcrumb()))
	return strings.Join(lines, "\n")
}

func (m shellModel) renderBody() string {
	switch m.scene {
	case shellSceneHome:
		return m.panel("Command Center", m.renderHome())
	case shellSceneMenu:
		return m.panel("Choose", m.renderSelectPrompt())
	case shellSceneConfirm:
		return m.panel("Confirm", m.renderConfirmPrompt())
	case shellSceneForm:
		return m.panel("Input", m.renderFormPrompt())
	case shellSceneTask:
		return m.panel("Running", m.renderTask())
	case shellSceneLogs:
		return m.panel("Logs", m.renderLogs())
	case shellSceneResult:
		return m.panel("Result", m.renderResult())
	case shellSceneHelp:
		return m.panel("Help", m.renderHelp())
	default:
		return m.panel("CLI", "")
	}
}

func (m shellModel) renderHome() string {
	if len(m.commands) == 0 {
		return m.theme.Empty.Render("No commands registered.")
	}
	var lines []string
	for i, command := range m.commands {
		line := firstString(command.Label, command.Name)
		if command.Description != "" {
			line = fmt.Sprintf("%s  %s", line, m.theme.ShellMuted.Render(command.Description))
		}
		if i == m.selected {
			lines = append(lines, m.theme.Selected.Render("> "+line))
		} else {
			lines = append(lines, m.theme.ListItem.Render("  "+line))
		}
	}
	if selected := m.currentCommand(); selected.Help != "" {
		lines = append(lines, "", m.theme.ShellPanelTitle.Render("Preview"))
		lines = append(lines, m.theme.ShellMuted.Render(firstHelpLines(selected.Help, 4)))
	}
	return strings.Join(lines, "\n")
}

func (m shellModel) renderSelectPrompt() string {
	if m.prompt == nil {
		return m.theme.ShellMuted.Render("Waiting for prompt.")
	}
	lines := []string{m.theme.ShellPanelTitle.Render(m.prompt.prompt)}
	for i, option := range m.prompt.options {
		label := firstString(option.Label, option.Value)
		if option.Description != "" {
			label = fmt.Sprintf("%s  %s", label, m.theme.ShellMuted.Render(option.Description))
		}
		if i == m.promptSelected {
			lines = append(lines, m.theme.Selected.Render("> "+label))
		} else {
			lines = append(lines, m.theme.ListItem.Render("  "+label))
		}
	}
	return strings.Join(lines, "\n")
}

func (m shellModel) renderConfirmPrompt() string {
	if m.prompt == nil {
		return m.theme.ShellMuted.Render("Waiting for prompt.")
	}
	yes := "Yes"
	no := "No"
	if m.promptSelected == 0 {
		yes = m.theme.Selected.Render("> " + yes)
		no = m.theme.ListItem.Render("  " + no)
	} else {
		yes = m.theme.ListItem.Render("  " + yes)
		no = m.theme.Selected.Render("> " + no)
	}
	return strings.Join([]string{
		m.theme.ShellPanelTitle.Render(m.prompt.prompt),
		yes,
		no,
	}, "\n")
}

func (m shellModel) renderFormPrompt() string {
	if m.prompt == nil {
		return m.theme.ShellMuted.Render("Waiting for prompt.")
	}
	value := m.input
	if m.prompt.kind == shellPromptPassword {
		value = strings.Repeat("*", utf8.RuneCountInString(value))
	}
	if value == "" && m.prompt.defaultText != "" {
		value = m.theme.ShellMuted.Render(m.prompt.defaultText)
	}
	input := m.theme.ShellInput.Width(m.contentWidth() - 8).Render(value + m.theme.ShellAccent.Render("_"))
	return strings.Join([]string{
		m.theme.ShellPanelTitle.Render(m.prompt.prompt),
		input,
	}, "\n")
}

func (m shellModel) renderTask() string {
	lines := []string{
		m.theme.ShellAccent.Render(shellSpinnerFrames[m.spinner]) + " " + firstString(m.activeCommand, "command"),
	}
	output := strings.TrimSpace(tailText(m.output, m.outputLines()))
	if output == "" {
		output = m.theme.ShellMuted.Render("Waiting for the next prompt or command output.")
	} else {
		output = m.theme.ShellLog.Render(output)
	}
	lines = append(lines, "", output)
	return strings.Join(lines, "\n")
}

func (m shellModel) renderLogs() string {
	output := strings.TrimSpace(tailText(m.output, m.outputLines()+4))
	if output == "" {
		output = "No log output yet."
	}
	return m.theme.ShellLog.Render(output)
}

func (m shellModel) renderResult() string {
	var status string
	if m.flowErr == nil {
		status = m.theme.ShellSuccess.Render("Completed")
	} else {
		var cancelled *CancelledError
		if errors.As(m.flowErr, &cancelled) {
			status = m.theme.ShellWarning.Render("Cancelled")
		} else {
			status = m.theme.ShellDanger.Render("Failed: " + m.flowErr.Error())
		}
	}
	output := strings.TrimSpace(tailText(m.output, m.outputLines()))
	if output == "" {
		output = m.theme.ShellMuted.Render("No output.")
	} else {
		output = m.theme.ShellLog.Render(output)
	}
	return strings.Join([]string{status, "", output}, "\n")
}

func (m shellModel) renderHelp() string {
	var text string
	if m.scene == shellSceneHelp && m.previousScene == shellSceneHome {
		text = m.currentCommand().Help
	}
	if strings.TrimSpace(text) == "" && m.selected >= 0 && m.selected < len(m.commands) {
		text = m.commands[m.selected].Help
	}
	if strings.TrimSpace(text) == "" {
		text = strings.Join([]string{
			"j/k or up/down: move selection",
			"enter: confirm",
			"esc: back or cancel current prompt",
			"/: command palette",
			"tab: switch focus label",
			"q or ctrl+c: quit",
		}, "\n")
	}
	return m.theme.Help.Render(strings.TrimSpace(text))
}

func (m shellModel) renderStatus() string {
	focus := []string{"main", "details", "logs"}[m.focus%3]
	var state string
	switch {
	case m.scene == shellSceneTask || m.scene == shellSceneLogs:
		state = shellSpinnerFrames[m.spinner] + " running"
	case m.scene == shellSceneResult && m.flowErr == nil:
		state = "ready"
	case m.scene == shellSceneResult && m.flowErr != nil:
		state = "attention"
	default:
		state = string(m.scene)
	}
	hints := "j/k move  enter confirm  ? help  / commands  tab focus  q quit"
	return m.theme.ShellStatus.Render(fmt.Sprintf("%s | focus:%s | %s", state, focus, hints))
}

func (m shellModel) renderPalette() string {
	filtered := m.filteredCommands()
	var lines []string
	lines = append(lines, m.theme.ShellPanelTitle.Render("Command Palette"))
	lines = append(lines, m.theme.ShellInput.Width(m.contentWidth()-8).Render("/"+m.paletteFilter+m.theme.ShellAccent.Render("_")))
	if len(filtered) == 0 {
		lines = append(lines, m.theme.ShellMuted.Render("No matching commands."))
	} else {
		for i, command := range filtered {
			label := firstString(command.Label, command.Name)
			if i == m.paletteSelected {
				lines = append(lines, m.theme.Selected.Render("> "+label))
			} else {
				lines = append(lines, m.theme.ListItem.Render("  "+label))
			}
		}
	}
	return m.panel("Palette", strings.Join(lines, "\n"))
}

func (m shellModel) panel(title string, body string) string {
	content := m.theme.ShellPanelTitle.Render(title)
	if strings.TrimSpace(body) != "" {
		content += "\n\n" + body
	}
	return m.theme.ShellPanel.Width(m.contentWidth()).Render(content)
}

func (m shellModel) contentWidth() int {
	width := m.width - 8
	if width <= 0 {
		width = 88
	}
	if width < 52 {
		return 52
	}
	if width > 112 {
		return 112
	}
	return width
}

func (m shellModel) outputLines() int {
	if m.height <= 0 {
		return 12
	}
	lines := m.height - 14
	if lines < 6 {
		return 6
	}
	if lines > 24 {
		return 24
	}
	return lines
}

func (m shellModel) title() string {
	if m.version == "" {
		return m.name
	}
	return fmt.Sprintf("%s v%s", m.name, m.version)
}

func (m shellModel) breadcrumb() string {
	items := []string{"home"}
	if m.activeCommand != "" && m.scene != shellSceneHome {
		items = append(items, m.activeCommand)
	}
	if m.scene != shellSceneHome {
		items = append(items, string(m.scene))
	}
	return strings.Join(items, " > ")
}

func (m shellModel) currentCommand() homeCommand {
	if len(m.commands) == 0 || m.selected < 0 || m.selected >= len(m.commands) {
		return homeCommand{}
	}
	return m.commands[m.selected]
}

func (m shellModel) filteredCommands() []homeCommand {
	filter := strings.ToLower(strings.TrimSpace(m.paletteFilter))
	if filter == "" {
		return append([]homeCommand(nil), m.commands...)
	}
	var filtered []homeCommand
	for _, command := range m.commands {
		haystack := strings.ToLower(strings.Join([]string{command.Name, command.Label, command.Description}, " "))
		if strings.Contains(haystack, filter) {
			filtered = append(filtered, command)
		}
	}
	return filtered
}

type shellPromptUI struct {
	events chan tea.Msg
}

func (ui *shellPromptUI) Select(ctx context.Context, prompt string, options []SelectOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("select prompt requires at least one option")
	}
	req := &shellPromptRequest{
		kind:     shellPromptSelect,
		prompt:   prompt,
		options:  append([]SelectOption(nil), options...),
		response: make(chan shellPromptResponse, 1),
	}
	resp, err := ui.request(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.text, resp.err
}

func (ui *shellPromptUI) Confirm(ctx context.Context, prompt string, defaultValue bool) (bool, error) {
	req := &shellPromptRequest{
		kind:        shellPromptConfirm,
		prompt:      prompt,
		defaultBool: defaultValue,
		response:    make(chan shellPromptResponse, 1),
	}
	resp, err := ui.request(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.ok, resp.err
}

func (ui *shellPromptUI) Input(ctx context.Context, prompt string, defaultValue string) (string, error) {
	req := &shellPromptRequest{
		kind:        shellPromptInput,
		prompt:      prompt,
		defaultText: defaultValue,
		response:    make(chan shellPromptResponse, 1),
	}
	resp, err := ui.request(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.text, resp.err
}

func (ui *shellPromptUI) Password(ctx context.Context, prompt string) (string, error) {
	req := &shellPromptRequest{
		kind:     shellPromptPassword,
		prompt:   prompt,
		response: make(chan shellPromptResponse, 1),
	}
	resp, err := ui.request(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.text, resp.err
}

func (ui *shellPromptUI) Info(message string) error {
	select {
	case ui.events <- shellOutputMsg{text: message + "\n"}:
	default:
	}
	return nil
}

func (ui *shellPromptUI) request(ctx context.Context, req *shellPromptRequest) (shellPromptResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case ui.events <- shellPromptMsg{request: req}:
	case <-ctx.Done():
		return shellPromptResponse{}, ctx.Err()
	}
	select {
	case resp := <-req.response:
		if resp.err != nil {
			return resp, resp.err
		}
		return resp, nil
	case <-ctx.Done():
		return shellPromptResponse{}, ctx.Err()
	}
}

type shellOutputWriter struct {
	events chan tea.Msg
}

func (w *shellOutputWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	text := string(append([]byte(nil), p...))
	w.events <- shellOutputMsg{text: text}
	return len(p), nil
}

var _ io.Writer = (*shellOutputWriter)(nil)

var shellSpinnerFrames = []string{"|", "/", "-", "\\"}

func shellWaitEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

func shellSpinnerCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return shellSpinnerMsg(t)
	})
}

func shellTransitionCmd() tea.Cmd {
	return tea.Tick(35*time.Millisecond, func(t time.Time) tea.Msg {
		return shellTransitionMsg(t)
	})
}

func dropLastRune(value string) string {
	if value == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(value)
	if size <= 0 {
		return ""
	}
	return value[:len(value)-size]
}

func printableKeyText(msg tea.KeyPressMsg) string {
	if text := msg.Key().Text; text != "" {
		return text
	}
	key := keyName(msg)
	if len(key) == 1 {
		return key
	}
	return ""
}

func tailText(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= limit {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-limit:], "\n")
}

func firstHelpLines(help string, limit int) string {
	help = strings.TrimSpace(help)
	if help == "" {
		return ""
	}
	lines := strings.Split(help, "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func looksLikeLogOutput(text string) bool {
	return strings.Contains(text, "--- stdout") || strings.Contains(text, "following logs")
}

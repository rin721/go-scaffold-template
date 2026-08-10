package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type homeConfig struct {
	Name        string
	Version     string
	Description string
	Commands    []homeCommand
	Theme       Theme
}

type homeCommand struct {
	Name        string
	Label       string
	Description string
	Help        string
	Builtin     string
	Order       int
	index       int
}

type homeModel struct {
	name        string
	version     string
	description string
	commands    []homeCommand
	theme       Theme

	selected int
	width    int
	height   int

	showingHelp bool
	help        string
	cancelled   bool
	exited      bool
	selectedCmd string
	altScreen   bool
}

type homeResult struct {
	command string
	exited  bool
}

const (
	homeBuiltinHelp = "help"
	homeBuiltinExit = "exit"
)

func newHomeModel(cfg homeConfig) homeModel {
	return homeModel{
		name:        cfg.Name,
		version:     cfg.Version,
		description: cfg.Description,
		commands:    append([]homeCommand(nil), cfg.Commands...),
		theme:       cfg.Theme,
		altScreen:   true,
	}
}

func (m homeModel) Init() tea.Cmd {
	return nil
}

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch keyName(msg) {
		case "up", "k":
			if len(m.commands) > 0 && !m.showingHelp {
				m.selected = (m.selected - 1 + len(m.commands)) % len(m.commands)
			}
		case "down", "j":
			if len(m.commands) > 0 && !m.showingHelp {
				m.selected = (m.selected + 1) % len(m.commands)
			}
		case "enter":
			if len(m.commands) > 0 && !m.showingHelp {
				current := m.commands[m.selected]
				switch current.Builtin {
				case homeBuiltinHelp:
					m.showingHelp = true
					m.help = current.Help
				case homeBuiltinExit:
					m.exited = true
					return m, tea.Quit
				default:
					m.selectedCmd = current.Name
					return m, tea.Quit
				}
			}
		case "?", "h":
			if len(m.commands) > 0 && !m.showingHelp {
				m.showingHelp = true
				m.help = m.commands[m.selected].Help
			}
		case "esc":
			if m.showingHelp {
				m.showingHelp = false
				m.help = ""
				return m, nil
			}
			m.exited = true
			return m, tea.Quit
		case "q":
			m.exited = true
			return m, tea.Quit
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m homeModel) View() tea.View {
	content := m.render()
	view := tea.NewView(content)
	view.AltScreen = m.altScreen
	return view
}

func (m homeModel) render() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(m.title()))

	if m.description != "" {
		b.WriteString("\n")
		b.WriteString(m.theme.Subtitle.Render(m.description))
	}

	b.WriteString("\n")
	if m.showingHelp {
		help := strings.TrimSpace(m.help)
		if help == "" {
			help = "该命令暂无帮助信息。"
		}
		b.WriteString(m.theme.Help.Render(help))
	} else if len(m.commands) == 0 {
		b.WriteString(m.theme.Empty.Render("暂无已注册命令。"))
	} else {
		for i, command := range m.commands {
			line := firstString(command.Label, command.Name)
			if command.Description != "" {
				line = fmt.Sprintf("%s  %s", line, command.Description)
			}
			if i == m.selected {
				b.WriteString(m.theme.Selected.Render("> " + line))
			} else {
				b.WriteString(m.theme.ListItem.Render("  " + line))
			}
			if i < len(m.commands)-1 {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Hint.Render("使用 up/down 或 j/k 移动，enter 执行，?/h 查看帮助，q/esc/ctrl+c 退出"))
	return b.String()
}

func (m homeModel) title() string {
	if m.version == "" {
		return m.name
	}
	return fmt.Sprintf("%s v%s", m.name, m.version)
}

func keyName(msg tea.KeyPressMsg) string {
	if msg.Keystroke() != "" {
		return msg.Keystroke()
	}
	return msg.String()
}

func (m homeModel) result() homeResult {
	return homeResult{
		command: m.selectedCmd,
		exited:  m.exited,
	}
}

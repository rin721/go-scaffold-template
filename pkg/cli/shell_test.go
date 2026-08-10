package cli

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestShellModelPromptSelectionAndCancel(t *testing.T) {
	model := newTestShellModel(t)
	req := &shellPromptRequest{
		kind:   shellPromptSelect,
		prompt: "choose",
		options: []SelectOption{
			{Value: "one", Label: "One"},
			{Value: "two", Label: "Two"},
		},
		response: make(chan shellPromptResponse, 1),
	}

	updated, _ := model.Update(shellPromptMsg{request: req})
	model = requireShellModel(t, updated)
	if model.scene != shellSceneMenu {
		t.Fatalf("scene = %s, want menu", model.scene)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "2", Code: '2'}))
	model = requireShellModel(t, updated)
	resp := <-req.response
	if resp.text != "two" {
		t.Fatalf("selection = %q, want two", resp.text)
	}
	if model.scene != shellSceneTask {
		t.Fatalf("scene after response = %s, want task", model.scene)
	}

	cancelReq := &shellPromptRequest{
		kind:     shellPromptInput,
		prompt:   "input",
		response: make(chan shellPromptResponse, 1),
	}
	updated, _ = model.Update(shellPromptMsg{request: cancelReq})
	model = requireShellModel(t, updated)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	_ = requireShellModel(t, updated)
	cancelResp := <-cancelReq.response
	if cancelResp.err == nil {
		t.Fatal("cancel response error = nil, want error")
	}
}

func TestShellModelFormInputAndHelp(t *testing.T) {
	model := newTestShellModel(t)
	req := &shellPromptRequest{
		kind:        shellPromptInput,
		prompt:      "name",
		defaultText: "default",
		response:    make(chan shellPromptResponse, 1),
	}
	updated, _ := model.Update(shellPromptMsg{request: req})
	model = requireShellModel(t, updated)

	for _, key := range []tea.Key{
		{Text: "a", Code: 'a'},
		{Text: "b", Code: 'b'},
		{Code: tea.KeyBackspace},
		{Code: tea.KeyEnter},
	} {
		updated, _ = model.Update(tea.KeyPressMsg(key))
		model = requireShellModel(t, updated)
	}
	resp := <-req.response
	if resp.text != "a" {
		t.Fatalf("input = %q, want a", resp.text)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	model = requireShellModel(t, updated)
	if model.scene != shellSceneHelp {
		t.Fatalf("scene = %s, want help", model.scene)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = requireShellModel(t, updated)
	if model.scene != shellSceneTask {
		t.Fatalf("scene after help esc = %s, want task", model.scene)
	}
}

func TestShellModelPaletteAndCommandActivation(t *testing.T) {
	model := newTestShellModel(t)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model = requireShellModel(t, updated)
	if !model.paletteOpen {
		t.Fatal("paletteOpen = false, want true")
	}
	model.paletteFilter = "serv"
	filtered := model.filteredCommands()
	if len(filtered) != 1 || filtered[0].Name != "service" {
		t.Fatalf("filtered commands = %#v, want service", filtered)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = requireShellModel(t, updated)
	if model.scene != shellSceneTask || model.activeCommand != "service" {
		t.Fatalf("activated scene/command = %s/%s, want task/service", model.scene, model.activeCommand)
	}
	if cmd == nil {
		t.Fatal("activation command = nil")
	}
}

func TestShellHomeQuitReturnsNormalExit(t *testing.T) {
	model := newTestShellModel(t)
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	model = requireShellModel(t, updated)
	if !model.exited || model.cancelled {
		t.Fatalf("quit state exited=%v cancelled=%v, want normal exit", model.exited, model.cancelled)
	}
	if cmd == nil {
		t.Fatal("quit command = nil")
	}
}

func TestShellPromptUIUsesEventChannel(t *testing.T) {
	events := make(chan tea.Msg, 1)
	ui := &shellPromptUI{events: events}
	done := make(chan string, 1)
	go func() {
		value, err := ui.Select(context.Background(), "choose", []SelectOption{{Value: "one"}})
		if err != nil {
			done <- "error:" + err.Error()
			return
		}
		done <- value
	}()

	msg, ok := (<-events).(shellPromptMsg)
	if !ok {
		t.Fatal("event is not shellPromptMsg")
	}
	msg.request.response <- shellPromptResponse{text: "one"}
	if got := <-done; got != "one" {
		t.Fatalf("Select() = %q, want one", got)
	}
}

func newTestShellModel(t *testing.T) shellModel {
	t.Helper()
	impl, err := NewApp(Config{Name: "tool", Version: "1.0.0", Description: "test cli"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	appImpl, ok := impl.(*app)
	if !ok {
		t.Fatal("NewApp did not return *app")
	}
	home := newHomeModel(homeConfig{
		Name:        "tool",
		Version:     "1.0.0",
		Description: "test cli",
		Theme:       DefaultTheme(),
		Commands: []homeCommand{
			{Name: "run", Label: "Run", Description: "start", Help: "run help"},
			{Name: "service", Label: "Service", Description: "inspect", Help: "service help"},
		},
	})
	return newShellModel(context.Background(), func() {}, appImpl, home, streams{}, make(chan tea.Msg, 8))
}

func requireShellModel(t *testing.T, model tea.Model) shellModel {
	t.Helper()
	shell, ok := model.(shellModel)
	if !ok {
		t.Fatalf("model = %T, want shellModel", model)
	}
	return shell
}

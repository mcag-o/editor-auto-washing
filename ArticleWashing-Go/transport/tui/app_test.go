package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewApp(t *testing.T) {
	api := NewAPIClient("http://localhost:8080")
	app := NewApp(api)

	if app == nil {
		t.Fatal("expected app to be created")
	}
	if app.tab != TabDashboard {
		t.Errorf("expected initial tab to be TabDashboard, got %v", app.tab)
	}
	if app.api != api {
		t.Error("expected api to be set")
	}
}

func TestTabNavigation(t *testing.T) {
	api := NewAPIClient("http://localhost:8080")
	app := NewApp(api)

	tests := []struct {
		key     string
		wantTab Tab
	}{
		{"1", TabDashboard},
		{"2", TabArticles},
		{"3", TabConfig},
		{"1", TabDashboard},
	}

	for _, tt := range tests {
		model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
		app = model.(*AppModel)
		if app.tab != tt.wantTab {
			t.Errorf("after pressing %q: expected tab %v, got %v", tt.key, tt.wantTab, app.tab)
		}
	}
}

func TestTabCycle(t *testing.T) {
	api := NewAPIClient("http://localhost:8080")
	app := NewApp(api)

	expected := []Tab{TabArticles, TabConfig, TabDashboard}
	for _, want := range expected {
		model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
		app = model.(*AppModel)
		if app.tab != want {
			t.Errorf("expected tab %v after tab key, got %v", want, app.tab)
		}
	}
}

func TestQuit(t *testing.T) {
	api := NewAPIClient("http://localhost:8080")
	app := NewApp(api)

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	app = model.(*AppModel)

	if !app.quit {
		t.Error("expected quit flag to be set")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestViewQuit(t *testing.T) {
	api := NewAPIClient("http://localhost:8080")
	app := NewApp(api)
	app.quit = true

	view := app.View()
	if view != "Shutting down...\n" {
		t.Errorf("unexpected view: %q", view)
	}
}

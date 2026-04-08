package tui

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Tab int

const (
	TabDashboard Tab = iota
	TabArticles
	TabConfig
)

type AppModel struct {
	api        *APIClient
	tab        Tab
	width      int
	height     int
	quit       bool
	msg        string
	styles     Styles
	dashboard  *DashboardModel
	config     *ConfigModel
	health     string
	healthErr  error
	articles   []map[string]any
	articleErr error
	configData map[string]any
	configErr  error
}

type Styles struct {
	Base        lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	Title       lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
}

func MakeStyles() Styles {
	return Styles{
		Base:        lipgloss.NewStyle().Padding(1),
		TabActive:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("7")).Background(lipgloss.Color("63")).Padding(0, 1),
		TabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1),
		Title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		Success:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	}
}

type DashboardMsg struct {
	Health string
	Err    error
}

type ArticlesMsg struct {
	Articles []map[string]any
	Err      error
}

type ConfigMsg struct {
	Config map[string]any
	Err    error
}

func fetchHealth(api *APIClient) tea.Msg {
	var result map[string]string
	if err := api.Get("/health", &result); err != nil {
		return DashboardMsg{Err: err}
	}
	return DashboardMsg{Health: result["status"]}
}

func fetchArticles(api *APIClient) tea.Msg {
	var result struct {
		Articles []map[string]any `json:"articles"`
	}
	if err := api.Get("/api/articles", &result); err != nil {
		return ArticlesMsg{Err: err}
	}
	return ArticlesMsg{Articles: result.Articles}
}

func fetchConfig(api *APIClient) tea.Msg {
	var result map[string]any
	if err := api.Get("/config", &result); err != nil {
		return ConfigMsg{Err: err}
	}
	return ConfigMsg{Config: result}
}

func NewApp(api *APIClient) *AppModel {
	return &AppModel{
		api:    api,
		tab:    TabDashboard,
		styles: MakeStyles(),
	}
}

func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("Content Hub TUI"),
		func() tea.Msg { return fetchHealth(m.api) },
	)
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "1":
			m.tab = TabDashboard
			return m, func() tea.Msg { return fetchHealth(m.api) }
		case "2":
			m.tab = TabArticles
			return m, func() tea.Msg { return fetchArticles(m.api) }
		case "3":
			m.tab = TabConfig
			return m, func() tea.Msg { return fetchConfig(m.api) }
		case "tab":
			m.tab = (m.tab + 1) % 3
			switch m.tab {
			case TabDashboard:
				return m, func() tea.Msg { return fetchHealth(m.api) }
			case TabArticles:
				return m, func() tea.Msg { return fetchArticles(m.api) }
			case TabConfig:
				return m, func() tea.Msg { return fetchConfig(m.api) }
			}
		case "r":
			switch m.tab {
			case TabDashboard:
				return m, func() tea.Msg { return fetchHealth(m.api) }
			case TabArticles:
				return m, func() tea.Msg { return fetchArticles(m.api) }
			case TabConfig:
				return m, func() tea.Msg { return fetchConfig(m.api) }
			}
		}
	case DashboardMsg:
		m.health = msg.Health
		m.healthErr = msg.Err
	case ArticlesMsg:
		m.articles = msg.Articles
		m.articleErr = msg.Err
	case ConfigMsg:
		m.configData = msg.Config
		m.configErr = msg.Err
	}
	return m, nil
}

func (m *AppModel) View() string {
	if m.quit {
		return "Shutting down...\n"
	}

	var body string
	switch m.tab {
	case TabDashboard:
		body = m.viewDashboard()
	case TabArticles:
		body = m.viewArticles()
	case TabConfig:
		body = m.viewConfig()
	}

	tabBar := m.renderTabBar()
	contentHeight := m.height - 6
	if contentHeight < 5 {
		contentHeight = 5
	}
	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.width - 4).
		Height(contentHeight).
		Padding(1).
		Render(body)

	return lipgloss.JoinVertical(lipgloss.Top, tabBar, content, m.renderFooter())
}

func (m *AppModel) renderTabBar() string {
	tabs := []string{"[1] Dashboard", "[2] Articles", "[3] Config"}
	parts := make([]string, 3)
	for i, t := range tabs {
		if i == int(m.tab) {
			parts[i] = m.styles.TabActive.Render(t)
		} else {
			parts[i] = m.styles.TabInactive.Render(t)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func (m *AppModel) renderFooter() string {
	return "\n  [q] Quit  [tab] Next tab  [1-3] Jump to tab  [r] Refresh"
}

func (m *AppModel) viewDashboard() string {
	s := m.styles.Title.Render("  System Status") + "\n\n"

	if m.healthErr != nil {
		s += m.styles.Error.Render("  Server connection failed:\n    " + m.healthErr.Error())
	} else if m.health != "" {
		s += fmt.Sprintf("  Status: %s\n", m.styles.Success.Render("● "+m.health))
	} else {
		s += "  Status: " + m.styles.Success.Render("● connected") + "\n"
	}

	s += fmt.Sprintf("  API: %s\n\n", m.api.baseURL)

	s += m.styles.Title.Render("  Quick Stats") + "\n"
	if m.articles != nil {
		s += fmt.Sprintf("  Articles loaded: %d\n", len(m.articles))
	} else {
		s += "  Articles: press 2 to load\n"
	}

	return s
}

func (m *AppModel) viewArticles() string {
	s := m.styles.Title.Render("  Articles") + "\n\n"

	if m.articleErr != nil {
		s += m.styles.Error.Render("  Failed to load articles:\n    " + m.articleErr.Error())
	} else if m.articles == nil {
		s += "  Loading articles..."
	} else if len(m.articles) == 0 {
		s += "  No articles found."
	} else {
		for i, art := range m.articles {
			title := ""
			if t, ok := art["title"].(string); ok {
				title = t
			}
			id := ""
			if i, ok := art["id"].(string); ok {
				id = i
			}
			prefix := fmt.Sprintf("  %d. ", i+1)
			if id != "" {
				prefix += fmt.Sprintf("[%s] ", id[:8])
			}
			s += prefix + title + "\n"
		}
	}

	return s
}

func (m *AppModel) viewConfig() string {
	s := m.styles.Title.Render("  Configuration") + "\n\n"

	if m.configErr != nil {
		s += m.styles.Error.Render("  Failed to load config:\n    " + m.configErr.Error())
	} else if m.configData == nil {
		s += "  Loading config..."
	} else {
		data, _ := json.MarshalIndent(m.configData, "  ", "  ")
		s += "  " + string(data)
	}

	return s
}

type DashboardModel struct{}
type ConfigModel struct{}

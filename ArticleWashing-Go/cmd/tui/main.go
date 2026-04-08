package main

import (
	"flag"
	"fmt"
	"os"

	"content-hub/transport/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8080", "API base URL")
	flag.Parse()

	client := tui.NewAPIClient(*apiURL)
	app := tui.NewApp(client)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

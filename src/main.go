package main

import (
	"arc/internal/app"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(app.NewModel(newRuntimeServices()), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}

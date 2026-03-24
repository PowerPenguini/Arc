package main

import (
	"fmt"
	"io"
	"os"

	"arc/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runTUI(stdout, stderr io.Writer) int {
	p := tea.NewProgram(app.NewModel(newRuntimeServices()), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

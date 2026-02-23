package app

import tea "github.com/charmbracelet/bubbletea"

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case localSudoCheckMsg:
		return m.handleLocalSudoCheck(msg)
	case connectReleaseMsg:
		return m.handleConnectRelease()
	case spinnerTickMsg:
		return m.handleSpinnerTick()
	case setupStepDoneMsg:
		return m.handleSetupStepDone(msg)
	case tea.MouseMsg:
		return m.handleMouseMsg(tea.MouseEvent(msg))
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	default:
		return m, nil
	}
}

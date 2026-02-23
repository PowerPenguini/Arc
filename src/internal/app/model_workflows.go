package app

import (
	"arc/components"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func spinnerCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func (m model) runSetupStepCmd(index int) tea.Cmd {
	return func() tea.Msg {
		msg := setupStepDoneMsg{index: index}
		res, err := m.svc.RunSetupStep(SetupStepRequest{
			BootstrapUser: m.bootstrapUser,
			Host:          m.host,
			Addr:          m.addr,
			Password:      m.password,
			UseSudo:       m.useSudo,
			PubKeyLine:    m.pubKeyLine,
			WG:            m.wg,
			StepID:        m.steps[index].ID,
		})
		if err != nil {
			msg.err = err
			return msg
		}
		msg.useSudo = res.UseSudo
		msg.pubKeyLine = res.PubKeyLine
		msg.readyAs = res.ReadyAs
		msg.wg = res.WG
		return msg
	}
}

func (m *model) startSetupWorkflow() tea.Cmd {
	m.phase = phaseLog
	m.spinnerTick = 0
	m.logScroll = 0
	m.working = true
	m.submitted = false
	m.err = ""
	m.steps = m.svc.SetupDefinition()
	if len(m.steps) == 0 {
		m.working = false
		m.submitted = true
		return nil
	}
	m.steps[0].State = stepRunning
	return tea.Batch(
		m.runSetupStepCmd(0),
		spinnerCmd(),
	)
}

func (m model) viewPhase() components.Phase {
	switch m.phase {
	case phaseLog:
		return components.PhaseLog
	default:
		return components.PhaseRemote
	}
}

func (m model) inputRects() (components.Rect, components.Rect, bool) {
	cardR, ok := m.credCardRect()
	if !ok {
		return components.Rect{}, components.Rect{}, false
	}
	return components.InputRectsFromCard(cardR)
}

func (m model) credCardRect() (components.Rect, bool) {
	layout, ok := components.ComputeLayout(m.w, m.h, m.viewPhase())
	if !ok {
		return components.Rect{}, false
	}
	return layout.Card, true
}

func (m model) connectRect() (components.Rect, bool) {
	cardR, ok := m.credCardRect()
	if !ok {
		return components.Rect{}, false
	}
	return components.ButtonRect(cardR, components.ConnectLabel)
}

func (m model) nextRect() (components.Rect, bool) {
	layout, ok := components.ComputeLayout(m.w, m.h, m.viewPhase())
	if !ok {
		return components.Rect{}, false
	}
	if !m.submitted || m.err != "" || m.working || m.phase != phaseLog {
		return components.Rect{}, false
	}
	return components.ButtonRect(layout.Card, components.FinishLabel)
}

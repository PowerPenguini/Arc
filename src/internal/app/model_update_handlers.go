package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.w, m.h = msg.Width, msg.Height
	m.clampLogScroll()
	return m, nil
}

func (m model) handleLocalSudoCheck(msg localSudoCheckMsg) (tea.Model, tea.Cmd) {
	m.localSudoChecked = true
	m.localSudoOK = msg.ok
	m.localSudoErr = msg.err
	return m, nil
}

func (m model) handleConnectRelease() (tea.Model, tea.Cmd) {
	wasDown := m.btnDown
	m.btnDown = false
	if m.phase == phaseRemote && wasDown && !m.submitted && !m.working && m.focus == 2 {
		return m, m.attemptSubmit()
	}
	return m, nil
}

func (m model) handleSpinnerTick() (tea.Model, tea.Cmd) {
	if m.phase == phaseLog && m.working {
		m.spinnerTick = (m.spinnerTick + 1) % len(spinnerFrames)
		return m, spinnerCmd()
	}
	return m, nil
}

func (m model) handleSetupStepDone(msg setupStepDoneMsg) (tea.Model, tea.Cmd) {
	if m.phase != phaseLog || msg.index < 0 || msg.index >= len(m.steps) {
		return m, nil
	}
	if msg.err != nil {
		m.steps[msg.index].State = stepFailed
		m.steps[msg.index].Err = msg.err.Error()
		m.err = fmt.Sprintf("Step %d failed: %v", msg.index+1, msg.err)
		m.working = false
		m.submitted = false
		m.clampLogScroll()
		return m, nil
	}

	if msg.useSudo != nil {
		m.useSudo = *msg.useSudo
	}
	if msg.pubKeyLine != "" {
		m.pubKeyLine = msg.pubKeyLine
	}
	if msg.readyAs != "" {
		m.readyAs = msg.readyAs
	}
	if msg.wg != nil {
		m.wg = *msg.wg
	}

	m.steps[msg.index].State = stepDone
	m.clampLogScroll()
	next := msg.index + 1
	if next >= len(m.steps) {
		m.working = false
		m.submitted = true
		return m, nil
	}
	m.steps[next].State = stepRunning
	return m, m.runSetupStepCmd(next)
}

func (m model) handleMouseMsg(me tea.MouseEvent) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseRemote:
		return m.handleMouseRemote(me)
	case phaseLog:
		return m.handleMouseLog(me)
	default:
		return m, nil
	}
}

func (m model) handleMouseRemote(me tea.MouseEvent) (tea.Model, tea.Cmd) {
	if !m.localSudoChecked || !m.localSudoOK {
		return m, nil
	}
	if !m.submitted && !m.working {
		hover := false
		if r, ok := m.connectRect(); ok {
			hover = r.Contains(me.X, me.Y)
		}
		m.btnHover = hover
	}

	if !m.submitted && !m.working && me.Action == tea.MouseActionMotion {
		return m, nil
	}

	if me.Button == tea.MouseButtonLeft && !m.submitted && !m.working {
		if r, ok := m.connectRect(); ok {
			switch me.Action {
			case tea.MouseActionPress:
				if r.Contains(me.X, me.Y) {
					if !m.localSudoChecked {
						m.err = "Checking local sudo..."
						return m, nil
					}
					if !m.localSudoOK {
						m.err = "Local sudo is required. Run: sudo -v  (then retry)"
						return m, nil
					}
					m.setFocus(2)
					m.btnDown = true
					m.btnHover = true
					return m, nil
				}
				m.btnDown = false
			case tea.MouseActionRelease:
				wasDown := m.btnDown
				m.btnDown = false
				if wasDown && r.Contains(me.X, me.Y) {
					return m, m.attemptSubmit()
				}
				return m, nil
			}
		}

		if me.Action == tea.MouseActionPress {
			ipR, passR, ok := m.inputRects()
			if ok {
				switch {
				case ipR.Contains(me.X, me.Y):
					m.setFocus(0)
					return m, nil
				case passR.Contains(me.X, me.Y):
					m.setFocus(1)
					return m, nil
				}
			}
		}
	}
	return m, nil
}

func (m model) handleMouseLog(me tea.MouseEvent) (tea.Model, tea.Cmd) {
	if me.Button == tea.MouseButtonWheelUp {
		m.logScroll += 2
		m.clampLogScroll()
		return m, nil
	}
	if me.Button == tea.MouseButtonWheelDown {
		m.logScroll -= 2
		m.clampLogScroll()
		return m, nil
	}
	if m.submitted && m.err == "" && !m.working {
		if r, ok := m.nextRect(); ok {
			m.btnHover = r.Contains(me.X, me.Y)
			if me.Action == tea.MouseActionMotion {
				return m, nil
			}
			if me.Button == tea.MouseButtonLeft {
				switch me.Action {
				case tea.MouseActionPress:
					if r.Contains(me.X, me.Y) {
						m.btnDown = true
						m.btnHover = true
						return m, nil
					}
					m.btnDown = false
				case tea.MouseActionRelease:
					wasDown := m.btnDown
					m.btnDown = false
					if wasDown && r.Contains(me.X, me.Y) {
						return m, tea.Quit
					}
					return m, nil
				}
			}
		}
	}
	return m, nil
}

func (m model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.phase == phaseRemote && (!m.localSudoChecked || !m.localSudoOK) {
		if k == "q" {
			return m, tea.Quit
		}
		return m, nil
	}
	if m.phase == phaseLog {
		return m.handleLogKey(k)
	}
	if m.working {
		return m, nil
	}
	if m.submitted {
		switch k {
		case "esc":
			m.submitted = false
			m.err = ""
			m.readyAs = ""
			m.setFocus(0)
			return m, nil
		case "enter", "q":
			return m, tea.Quit
		default:
			return m, nil
		}
	}

	switch k {
	case "tab", "down":
		m.setFocus((m.focus + 1) % 3)
		return m, nil
	case "shift+tab", "up":
		m.setFocus((m.focus + 3 - 1) % 3)
		return m, nil
	case "enter":
		if m.focus == 0 {
			m.setFocus(1)
			return m, nil
		}
		if m.focus == 1 {
			m.setFocus(2)
			return m, nil
		}
		if m.focus == 2 {
			if !m.localSudoChecked {
				m.err = "Checking local sudo..."
				return m, nil
			}
			if !m.localSudoOK {
				m.err = "Local sudo is required. Run: sudo -v  (then retry)"
				return m, nil
			}
			if !m.btnDown {
				m.btnDown = true
				return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return connectReleaseMsg{} })
			}
			return m, nil
		}
	}

	if m.focus == 0 {
		m.ip.HandleKey(msg)
	} else if m.focus == 1 {
		m.pass.HandleKey(msg)
	}
	return m, nil
}

func (m model) handleLogKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "up", "k":
		m.logScroll++
		m.clampLogScroll()
		return m, nil
	case "down", "j":
		m.logScroll--
		m.clampLogScroll()
		return m, nil
	case "pgup":
		m.logScroll += 6
		m.clampLogScroll()
		return m, nil
	case "pgdown":
		m.logScroll -= 6
		m.clampLogScroll()
		return m, nil
	case "end":
		m.logScroll = 0
		return m, nil
	case "esc":
		if m.working {
			return m, nil
		}
		if m.submitted {
			return m, nil
		}
		m.resetRemotePhaseState()
		return m, nil
	case "enter", "q":
		if m.working {
			return m, nil
		}
		if k == "q" && m.submitted && m.err == "" {
			return m, tea.Quit
		}
		if k == "enter" && m.submitted && m.err == "" {
			return m, tea.Quit
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) resetRemotePhaseState() {
	m.phase = phaseRemote
	m.submitted = false
	m.err = ""
	m.readyAs = ""
	m.steps = nil
	m.bootstrapUser = ""
	m.host = ""
	m.addr = ""
	m.password = ""
	m.useSudo = false
	m.pubKeyLine = ""
	m.setFocus(0)
}

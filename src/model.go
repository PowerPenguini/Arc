package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type setupPhase int

const (
	phaseRemote setupPhase = iota
	phaseLog
	phaseLocal
	phaseInfra
)

type stepState int

const (
	stepPending stepState = iota
	stepRunning
	stepDone
	stepFailed
)

type setupStep struct {
	label string
	state stepState
	err   string
}

type setupStepDoneMsg struct {
	index int
	err   error

	useSudo    *bool
	pubKeyLine string
	readyAs    string
}

type spinnerTickMsg struct{}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type model struct {
	w int
	h int

	focus     int // 0=ip, 1=pass, 2=connect
	phase     setupPhase
	working   bool
	submitted bool
	readyAs   string
	err       string
	btnDown   bool
	btnHover  bool

	localSudoChecked bool
	localSudoOK      bool
	localSudoErr     string

	localWorking bool
	localDone    bool
	localErr     string
	localSteps   []setupStep

	infraWorking bool
	infraDone    bool
	infraErr     string
	infraSteps   []setupStep
	wg           wgConfig

	ip   field
	pass field

	bootstrapUser string
	host          string
	addr          string
	password      string
	useSudo       bool
	pubKeyLine    string

	steps       []setupStep
	spinnerTick int
}

func newModel() model {
	m := model{}
	m.ip = field{placeholder: "user@192.168.1.10"}
	m.pass = field{placeholder: "password", mask: true}
	m.phase = phaseRemote
	return m
}

type localSudoCheckMsg struct {
	ok  bool
	err string
}

func checkLocalSudoCmd() tea.Cmd {
	return func() tea.Msg {
		_, err := execLocal("sudo", "-n", "true")
		if err != nil {
			return localSudoCheckMsg{ok: false, err: err.Error()}
		}
		return localSudoCheckMsg{ok: true}
	}
}

func (m model) Init() tea.Cmd { return checkLocalSudoCmd() }

func (m *model) resetButtonState() {
	m.btnDown = false
	m.btnHover = false
}

func (m *model) setFocus(f int) {
	m.focus = f
	m.resetButtonState()
}

func (m *model) attemptSubmit() tea.Cmd {
	m.err = ""
	m.phase = phaseRemote

	// Hard gate: we require local sudo to be available non-interactively up front.
	// This avoids getting deep into the flow only to fail later (e.g. INFRA/local installs).
	if !m.localSudoChecked {
		m.err = "Checking local sudo..."
		return nil
	}
	if !m.localSudoOK {
		m.err = "Local sudo is required. Run: sudo -v  (then retry)"
		return nil
	}

	target := strings.TrimSpace(m.ip.valueString())
	if target == "" || !strings.Contains(target, "@") {
		m.err = "User@IP is required"
		m.setFocus(0)
		return nil
	}
	parts := strings.SplitN(target, "@", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		m.err = "Use format: user@host"
		m.setFocus(0)
		return nil
	}

	password := m.pass.valueString()
	if strings.TrimSpace(password) == "" {
		m.err = "Password is required"
		m.setFocus(1)
		return nil
	}

	m.readyAs = ""
	m.bootstrapUser = ""
	m.host = ""
	m.addr = ""
	m.password = ""
	m.useSudo = false
	m.pubKeyLine = ""
	m.steps = nil
	m.spinnerTick = 0
	m.btnDown = false
	m.btnHover = false

	u, host, addr, err := parseSSHConnectTarget(target)
	if err != nil {
		m.err = err.Error()
		m.setFocus(0)
		return nil
	}

	m.bootstrapUser = u
	m.host = host
	m.addr = addr
	m.password = password

	return m.startSetupWorkflow()
}

func spinnerCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func runSetupStepCmd(bootstrapUser, host, addr, password string, useSudo bool, pubKeyLine string, index int) tea.Cmd {
	return func() tea.Msg {
		msg := setupStepDoneMsg{index: index}

		switch index {
		case 0:
			client, err := dialWithPassword(bootstrapUser, addr, password)
			if err != nil {
				msg.err = err
				return msg
			}
			sudoOK, err := canRunPrivileged(bootstrapUser, client, password)
			if err != nil {
				_ = client.Close()
				msg.err = err
				return msg
			}
			msg.useSudo = &sudoOK
			_ = client.Close()
			return msg
		case 1:
			if err := ensureLocalArcHostsAliases(host); err != nil {
				msg.err = err
			}
			return msg
		case 2:
			if err := ensureLocalSSHKeyPair(); err != nil {
				msg.err = err
				return msg
			}
			pubPath := filepath.Join(userSSHDir(), "id_ed25519.pub")
			pubKeyLine, err := readPublicKeyLine(pubPath)
			if err != nil {
				msg.err = err
				return msg
			}
			msg.pubKeyLine = pubKeyLine
			return msg
		case 3:
			client, err := dialWithPassword(bootstrapUser, addr, password)
			if err != nil {
				msg.err = err
				return msg
			}
			err = ensureArcUser(client, useSudo, password)
			_ = client.Close()
			if err != nil {
				msg.err = err
			}
			return msg
		case 4:
			if strings.TrimSpace(pubKeyLine) == "" {
				msg.err = fmt.Errorf("missing public key line")
				return msg
			}
			client, err := dialWithPassword(bootstrapUser, addr, password)
			if err != nil {
				msg.err = err
				return msg
			}
			err = ensureArcAuthorizedKey(client, useSudo, password, pubKeyLine)
			_ = client.Close()
			if err != nil {
				msg.err = err
			}
			return msg
		case 5:
			client, err := dialWithPassword(bootstrapUser, addr, password)
			if err != nil {
				msg.err = err
				return msg
			}
			err = ensureArcSudoers(client, useSudo, password)
			_ = client.Close()
			if err != nil {
				msg.err = err
			}
			return msg
		case 6:
			client, err := dialWithPassword(bootstrapUser, addr, password)
			if err != nil {
				msg.err = err
				return msg
			}
			err = ensureArcHushLogin(client, useSudo, password)
			_ = client.Close()
			if err != nil {
				msg.err = err
			}
			return msg
		case 7:
			if err := verifyArcKeyLogin(host, addr); err != nil {
				msg.err = err
				return msg
			}
			msg.readyAs = arcUser + "@" + host
			return msg
		case 8:
			if err := ensureArcBashPrompt(addr); err != nil {
				msg.err = err
			}
			return msg
		default:
			msg.err = fmt.Errorf("unknown step index: %d", index)
			return msg
		}
	}
}

func defaultSetupSteps() []setupStep {
	return []setupStep{
		{label: "Connected (password)"},
		{label: "Add remotehost to /etc/hosts (local)"},
		{label: "Ensure local SSH key"},
		{label: "Create arc user"},
		{label: "Add arc authorized_keys"},
		{label: "Add arc to sudoers"},
		{label: "Create ~/.hushlogin for arc"},
		{label: "Verify arc login (key + sudo)"},
		{label: "Install arc bash prompt"},
	}
}

func (m *model) startSetupWorkflow() tea.Cmd {
	m.phase = phaseLog
	m.spinnerTick = 0
	m.working = true
	m.submitted = false
	m.err = ""
	m.steps = defaultSetupSteps()
	if len(m.steps) == 0 {
		m.working = false
		m.submitted = true
		return nil
	}
	m.steps[0].state = stepRunning
	return tea.Batch(
		runSetupStepCmd(m.bootstrapUser, m.host, m.addr, m.password, m.useSudo, m.pubKeyLine, 0),
		spinnerCmd(),
	)
}

func (m model) inputRects() (rect, rect, bool) {
	cardR, ok := m.credCardRect()
	if !ok {
		return rect{}, rect{}, false
	}
	return inputRectsFromCard(cardR)
}

func (m model) credCardRect() (rect, bool) {
	layout, ok := m.computeLayout()
	if !ok {
		return rect{}, false
	}
	return layout.card, true
}

func (m model) connectRect() (rect, bool) {
	cardR, ok := m.credCardRect()
	if !ok {
		return rect{}, false
	}
	return buttonRect(cardR, connectLabel)
}

func (m model) nextRect() (rect, bool) {
	layout, ok := m.computeLayout()
	if !ok {
		return rect{}, false
	}
	if !m.submitted || m.err != "" || m.working || m.phase != phaseLog {
		return rect{}, false
	}
	return buttonRect(layout.card, nextLabel)
}

func (m model) localButtonRect() (rect, bool) {
	layout, ok := m.computeLayout()
	if !ok {
		return rect{}, false
	}
	if m.phase != phaseLocal {
		return rect{}, false
	}
	label := installLabel
	if m.localDone {
		label = nextLabel
	}
	return buttonRect(layout.card, label)
}

type localSetupDoneMsg struct{ err error }

type localStepDoneMsg struct {
	index int
	err   error
}

func runLocalStepCmd(index int) tea.Cmd {
	return func() tea.Msg {
		var err error
		home, herr := os.UserHomeDir()
		if herr != nil || home == "" {
			err = fmt.Errorf("cannot resolve home dir")
		} else {
			switch index {
			case 0:
				err = ensureArcPromptInBashrc(filepath.Join(home, ".bashrc"), arcPromptBlockLocal)
			case 1:
				err = ensureProfileSourcesBashrc(filepath.Join(home, ".bash_profile"))
			default:
				err = fmt.Errorf("unknown local step index: %d", index)
			}
		}
		return localStepDoneMsg{index: index, err: err}
	}
}

func defaultLocalSteps() []setupStep {
	return []setupStep{
		{label: "Update ~/.bashrc (ARC prompt block)"},
		{label: "Update ~/.bash_profile (source ~/.bashrc)"},
	}
}

func defaultInfraSteps() []setupStep {
	return []setupStep{
		{label: "Detect remote OS"},
		{label: "Install WireGuard (remote)"},
		{label: "Write /etc/wireguard/wg0.conf (remote)"},
		{label: "Open firewall (ufw if active)"},
		{label: "Enable wg-quick@wg0 (remote)"},
		{label: "Detect local OS"},
		{label: "Install WireGuard (local)"},
		{label: "Write /etc/wireguard/wg0.conf (local)"},
		{label: "Enable wg-quick@wg0 (local)"},
		{label: "Verify tunnel (ping server)"},
	}
}

type infraStepDoneMsg struct {
	index int
	err   error
}

func runInfraStepCmd(m model, index int) tea.Cmd {
	return func() tea.Msg {
		err := runInfraStep(m, index)
		return infraStepDoneMsg{index: index, err: err}
	}
}

func (m *model) startInfraWorkflow() tea.Cmd {
	m.phase = phaseInfra
	m.spinnerTick = 0
	m.infraWorking = true
	m.infraDone = false
	m.infraErr = ""
	m.infraSteps = defaultInfraSteps()
	if len(m.infraSteps) == 0 {
		m.infraWorking = false
		m.infraDone = true
		return nil
	}

	wg, err := buildWGConfig(m.host)
	if err != nil {
		m.infraWorking = false
		m.infraErr = err.Error()
		return nil
	}
	m.wg = wg

	m.infraSteps[0].state = stepRunning
	return tea.Batch(runInfraStepCmd(*m, 0), spinnerCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case localSudoCheckMsg:
		m.localSudoChecked = true
		m.localSudoOK = msg.ok
		m.localSudoErr = msg.err
		return m, nil
	case connectReleaseMsg:
		wasDown := m.btnDown
		m.btnDown = false
		if m.phase == phaseRemote && wasDown && !m.submitted && !m.working && m.focus == 2 {
			return m, m.attemptSubmit()
		}
		return m, nil
	case spinnerTickMsg:
		if (m.phase == phaseLog && m.working) || (m.phase == phaseLocal && m.localWorking) || (m.phase == phaseInfra && m.infraWorking) {
			m.spinnerTick = (m.spinnerTick + 1) % len(spinnerFrames)
			return m, spinnerCmd()
		}
		return m, nil
	case setupStepDoneMsg:
		if m.phase != phaseLog || msg.index < 0 || msg.index >= len(m.steps) {
			return m, nil
		}
		if msg.err != nil {
			m.steps[msg.index].state = stepFailed
			m.steps[msg.index].err = msg.err.Error()
			m.err = fmt.Sprintf("Step %d failed: %v", msg.index+1, msg.err)
			m.working = false
			m.submitted = false
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

		m.steps[msg.index].state = stepDone
		next := msg.index + 1
		if next >= len(m.steps) {
			m.working = false
			m.submitted = true
			return m, nil
		}
		m.steps[next].state = stepRunning
		return m, runSetupStepCmd(m.bootstrapUser, m.host, m.addr, m.password, m.useSudo, m.pubKeyLine, next)
	case localStepDoneMsg:
		if m.phase != phaseLocal || !m.localWorking || msg.index < 0 || msg.index >= len(m.localSteps) {
			return m, nil
		}

		if msg.err != nil {
			m.localSteps[msg.index].state = stepFailed
			m.localSteps[msg.index].err = msg.err.Error()
			m.localErr = fmt.Sprintf("Step %d failed: %v", msg.index+1, msg.err)
			m.localWorking = false
			m.localDone = false
			return m, nil
		}

		m.localSteps[msg.index].state = stepDone
		next := msg.index + 1
		if next >= len(m.localSteps) {
			m.localWorking = false
			m.localDone = true
			m.localErr = ""
			return m, nil
		}

		m.localSteps[next].state = stepRunning
		return m, runLocalStepCmd(next)
	case infraStepDoneMsg:
		if m.phase != phaseInfra || !m.infraWorking || msg.index < 0 || msg.index >= len(m.infraSteps) {
			return m, nil
		}
		if msg.err != nil {
			m.infraSteps[msg.index].state = stepFailed
			m.infraSteps[msg.index].err = msg.err.Error()
			m.infraErr = fmt.Sprintf("Step %d failed: %v", msg.index+1, msg.err)
			m.infraWorking = false
			m.infraDone = false
			return m, nil
		}
		m.infraSteps[msg.index].state = stepDone
		next := msg.index + 1
		if next >= len(m.infraSteps) {
			m.infraWorking = false
			m.infraDone = true
			m.infraErr = ""
			return m, nil
		}
		m.infraSteps[next].state = stepRunning
		return m, runInfraStepCmd(m, next)
	case tea.MouseMsg:
		me := tea.MouseEvent(msg)
		switch m.phase {
		case phaseRemote:
			if !m.localSudoChecked || !m.localSudoOK {
				return m, nil
			}
			if !m.submitted && !m.working {
				hover := false
				if r, ok := m.connectRect(); ok {
					hover = r.contains(me.X, me.Y)
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
						if r.contains(me.X, me.Y) {
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
						if wasDown && r.contains(me.X, me.Y) {
							return m, m.attemptSubmit()
						}
						return m, nil
					}
				}

				if me.Action == tea.MouseActionPress {
					ipR, passR, ok := m.inputRects()
					if ok {
						switch {
						case ipR.contains(me.X, me.Y):
							m.setFocus(0)
							return m, nil
						case passR.contains(me.X, me.Y):
							m.setFocus(1)
							return m, nil
						}
					}
				}
			}
			return m, nil
		case phaseLog:
			if m.submitted && m.err == "" && !m.working {
				if r, ok := m.nextRect(); ok {
					m.btnHover = r.contains(me.X, me.Y)
					if me.Action == tea.MouseActionMotion {
						return m, nil
					}
					if me.Button == tea.MouseButtonLeft {
						switch me.Action {
						case tea.MouseActionPress:
							if r.contains(me.X, me.Y) {
								m.btnDown = true
								m.btnHover = true
								return m, nil
							}
							m.btnDown = false
						case tea.MouseActionRelease:
							wasDown := m.btnDown
							m.btnDown = false
							if wasDown && r.contains(me.X, me.Y) {
								m.phase = phaseLocal
								m.resetButtonState()
								m.localWorking = false
								m.localDone = false
								m.localErr = ""
								m.localSteps = nil
								m.setFocus(0)
								return m, nil
							}
							return m, nil
						}
					}
				}
			}
			return m, nil
		case phaseLocal:
			if m.localWorking {
				return m, nil
			}
			if r, ok := m.localButtonRect(); ok {
				m.btnHover = r.contains(me.X, me.Y)
				if me.Action == tea.MouseActionMotion {
					return m, nil
				}
				if me.Button == tea.MouseButtonLeft {
					switch me.Action {
					case tea.MouseActionPress:
						if r.contains(me.X, me.Y) {
							m.btnDown = true
							m.btnHover = true
							return m, nil
						}
						m.btnDown = false
					case tea.MouseActionRelease:
						wasDown := m.btnDown
						m.btnDown = false
						if wasDown && r.contains(me.X, me.Y) {
							if m.localDone {
								m.resetButtonState()
								m.localWorking = false
								m.localErr = ""
								m.localSteps = nil
								return m, m.startInfraWorkflow()
							}
							m.localWorking = true
							m.localErr = ""
							m.localDone = false
							m.localSteps = defaultLocalSteps()
							if len(m.localSteps) == 0 {
								m.localWorking = false
								m.localDone = true
								return m, nil
							}
							m.localSteps[0].state = stepRunning
							return m, tea.Batch(runLocalStepCmd(0), spinnerCmd())
						}
						return m, nil
					}
				}
			}
			return m, nil
		case phaseInfra:
			if m.infraWorking {
				return m, nil
			}
			if m.infraDone && m.infraErr == "" {
				layout, ok := m.computeLayout()
				if ok {
					if r, ok := buttonRect(layout.card, finishLabel); ok {
						m.btnHover = r.contains(me.X, me.Y)
						if me.Action == tea.MouseActionMotion {
							return m, nil
						}
						if me.Button == tea.MouseButtonLeft {
							switch me.Action {
							case tea.MouseActionPress:
								if r.contains(me.X, me.Y) {
									m.btnDown = true
									m.btnHover = true
									return m, nil
								}
								m.btnDown = false
							case tea.MouseActionRelease:
								wasDown := m.btnDown
								m.btnDown = false
								if wasDown && r.contains(me.X, me.Y) {
									return m, tea.Quit
								}
								return m, nil
							}
						}
					}
				}
			}
			return m, nil
		default:
			return m, nil
		}
	case tea.KeyMsg:
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
			if m.working {
				return m, nil
			}
			switch k {
			case "esc":
				// Don't allow going back after successful setup.
				if m.submitted {
					return m, nil
				}
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
				return m, nil
			case "enter", "q":
				if k == "q" && m.submitted && m.err == "" {
					return m, tea.Quit
				}
				if k == "enter" && m.submitted && m.err == "" {
					m.phase = phaseLocal
					m.resetButtonState()
					m.localWorking = false
					m.localDone = false
					m.localErr = ""
					m.setFocus(0)
					return m, nil
				}
				return m, nil
			default:
				return m, nil
			}
		}
		if m.phase == phaseLocal {
			if m.localWorking {
				return m, nil
			}
			switch k {
			case "enter":
				if m.localDone {
					m.resetButtonState()
					m.localWorking = false
					m.localErr = ""
					m.localSteps = nil
					return m, m.startInfraWorkflow()
				}
				m.localWorking = true
				m.localErr = ""
				m.localDone = false
				m.localSteps = defaultLocalSteps()
				if len(m.localSteps) == 0 {
					m.localWorking = false
					m.localDone = true
					return m, nil
				}
				m.localSteps[0].state = stepRunning
				return m, tea.Batch(runLocalStepCmd(0), spinnerCmd())
			case "q":
				return m, tea.Quit
			default:
				return m, nil
			}
		}
		if m.phase == phaseInfra {
			if m.infraWorking {
				return m, nil
			}
			switch k {
			case "enter":
				if m.infraDone && m.infraErr == "" {
					return m, tea.Quit
				}
				return m, nil
			case "q":
				return m, tea.Quit
			default:
				return m, nil
			}
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
			m.ip.handleKey(msg)
		} else if m.focus == 1 {
			m.pass.handleKey(msg)
		}
		return m, nil
	}
	return m, nil
}

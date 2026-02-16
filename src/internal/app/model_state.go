package app

import (
	"arc/components"
	"arc/internal/workflow"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type setupPhase = workflow.Phase
type stepState = workflow.StepState
type setupStep = workflow.Step

const (
	phaseRemote = workflow.PhaseRemote
	phaseLog    = workflow.PhaseLog
)

const (
	stepPending = workflow.StepPending
	stepRunning = workflow.StepRunning
	stepDone    = workflow.StepDone
	stepFailed  = workflow.StepFailed
)

type setupStepDoneMsg struct {
	index int
	err   error

	useSudo    *bool
	pubKeyLine string
	readyAs    string
	wg         *WGConfig
}

type spinnerTickMsg struct{}
type connectReleaseMsg struct{}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type model struct {
	svc Services

	w int
	h int

	focus     int // 0=ip, 1=pass, 2=connect
	phase     setupPhase
	logScroll int
	working   bool
	submitted bool
	readyAs   string
	err       string
	btnDown   bool
	btnHover  bool

	localSudoChecked bool
	localSudoOK      bool
	localSudoErr     string
	wg               WGConfig

	ip   components.Field
	pass components.Field

	bootstrapUser string
	host          string
	addr          string
	password      string
	useSudo       bool
	pubKeyLine    string

	steps       []setupStep
	spinnerTick int
}

func NewModel(svc Services) tea.Model {
	m := model{svc: svc}
	m.ip = components.Field{Placeholder: "user@192.168.1.10"}
	m.pass = components.Field{Placeholder: "password", Mask: true}
	m.phase = phaseRemote
	return m
}

type localSudoCheckMsg struct {
	ok  bool
	err string
}

func (m model) checkLocalSudoCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.svc.CheckLocalSudo()
		if err != nil {
			return localSudoCheckMsg{ok: false, err: err.Error()}
		}
		return localSudoCheckMsg{ok: true}
	}
}

func (m model) Init() tea.Cmd { return m.checkLocalSudoCmd() }

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
	// This avoids getting deep into the flow only to fail later during setup.
	if !m.localSudoChecked {
		m.err = "Checking local sudo..."
		return nil
	}
	if !m.localSudoOK {
		m.err = "Local sudo is required. Run: sudo -v  (then retry)"
		return nil
	}

	target := strings.TrimSpace(m.ip.ValueString())
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

	password := m.pass.ValueString()
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
	m.logScroll = 0
	m.btnDown = false
	m.btnHover = false

	u, host, addr, err := m.svc.ParseSSHConnectTarget(target)
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

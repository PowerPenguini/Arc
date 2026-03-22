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

	useSudo *bool
	readyAs string
	wg      *WGConfig
}

type spinnerTickMsg struct{}
type connectReleaseMsg struct{}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type model struct {
	svc Services

	w int
	h int

	focus     int // 0=ssh-target, 1=password, 2=onboard
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

	target components.Field
	pass   components.Field

	bootstrapUser string
	host          string
	addr          string
	password      string
	useSudo       bool

	steps       []setupStep
	spinnerTick int

	mobilePayload string
	mobileQR      []string
	mobileQRErr   string
}

func NewModel(svc Services) tea.Model {
	m := model{svc: svc}
	m.target = components.Field{Placeholder: "ssh://root@192.168.1.10:22"}
	m.pass = components.Field{Placeholder: "optional password", Mask: true}
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

	target := strings.TrimSpace(m.target.ValueString())
	if target == "" {
		m.err = "SSH target is required"
		m.setFocus(0)
		return nil
	}
	if !strings.Contains(target, "@") {
		m.err = "Use ssh://user@host[:port] or user@host[:port]"
		m.setFocus(0)
		return nil
	}

	password := strings.TrimSpace(m.pass.ValueString())

	m.readyAs = ""
	m.bootstrapUser = ""
	m.host = ""
	m.addr = ""
	m.password = ""
	m.useSudo = false
	m.steps = nil
	m.spinnerTick = 0
	m.logScroll = 0
	m.btnDown = false
	m.btnHover = false

	u, host, addr, err := m.svc.ParseSSHDeviceTarget(target)
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

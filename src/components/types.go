package components

const (
	ConnectLabel = "Connect"
	FinishLabel  = "Finish"
)

type Phase int

const (
	PhaseRemote Phase = iota
	PhaseLog
)

type StepState int

const (
	StepPending StepState = iota
	StepRunning
	StepDone
	StepFailed
)

type Step struct {
	Label string
	State StepState
	Err   string
}

type Rect struct {
	X int
	Y int
	W int
	H int
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

type Layout struct {
	LogoX int
	LogoY int
	Card  Rect
}

type ViewState struct {
	W int
	H int

	Focus     int
	Phase     Phase
	LogScroll int
	Working   bool
	Submitted bool
	ReadyAs   string
	Host      string
	Err       string
	BtnDown   bool
	BtnHover  bool

	LocalSudoChecked bool
	LocalSudoOK      bool

	Steps       []Step
	SpinnerRune rune

	IP   Field
	Pass Field
}

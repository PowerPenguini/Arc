package workflow

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

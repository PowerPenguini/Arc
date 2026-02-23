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

type StepID string

type StepDef struct {
	ID    StepID
	Label string
}

type Step struct {
	ID    StepID
	Label string
	State StepState
	Err   string
}

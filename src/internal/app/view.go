package app

import "arc/components"

func mapStepState(s stepState) components.StepState {
	switch s {
	case stepRunning:
		return components.StepRunning
	case stepDone:
		return components.StepDone
	case stepFailed:
		return components.StepFailed
	default:
		return components.StepPending
	}
}

func toComponentSteps(steps []setupStep) []components.Step {
	if len(steps) == 0 {
		return nil
	}
	out := make([]components.Step, 0, len(steps))
	for _, s := range steps {
		out = append(out, components.Step{
			Label: s.Label,
			State: mapStepState(s.State),
			Err:   s.Err,
		})
	}
	return out
}

func (m model) spinnerRune() rune {
	if len(spinnerFrames) == 0 {
		return '*'
	}
	idx := m.spinnerTick % len(spinnerFrames)
	if idx < 0 {
		idx += len(spinnerFrames)
	}
	return spinnerFrames[idx]
}

func (m model) toViewState() components.ViewState {
	return components.ViewState{
		W: m.w,
		H: m.h,

		Focus:     m.focus,
		Phase:     m.viewPhase(),
		LogScroll: m.logScroll,
		Working:   m.working,
		Submitted: m.submitted,
		ReadyAs:   m.readyAs,
		Host:      m.host,
		Err:       m.err,
		BtnDown:   m.btnDown,
		BtnHover:  m.btnHover,

		LocalSudoChecked: m.localSudoChecked,
		LocalSudoOK:      m.localSudoOK,

		Steps:       toComponentSteps(m.steps),
		SpinnerRune: m.spinnerRune(),

		IP:   m.ip,
		Pass: m.pass,
	}
}

func (m model) View() string {
	return components.Render(m.toViewState())
}

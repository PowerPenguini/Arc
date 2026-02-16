package app

import "arc/components"

func (m model) maxLogScroll() int {
	lines := 0
	hasRunning := false
	for _, step := range m.steps {
		if step.State == stepPending {
			continue
		}
		lines++
		if step.State == stepRunning {
			hasRunning = true
		}
	}
	if lines <= 0 {
		return 0
	}

	layout, ok := components.ComputeLayout(m.w, m.h, m.viewPhase())
	if !ok {
		// Before first layout pass, keep a conservative bound.
		return lines
	}

	y := layout.Card.Y
	h := layout.Card.H
	baseY := y + 5
	if hasRunning {
		baseY = y + 6
	}

	contentBottom := y + h - 3
	if btnR, hasButton := components.ButtonRect(layout.Card, components.FinishLabel); hasButton {
		contentBottom = btnR.Y - 3
	}
	if contentBottom < baseY {
		contentBottom = baseY
	}

	avail := contentBottom - baseY + 1
	if avail < 1 {
		avail = 1
	}

	max := lines - avail
	if max < 0 {
		return 0
	}
	return max
}

func (m *model) clampLogScroll() {
	if m.logScroll < 0 {
		m.logScroll = 0
		return
	}
	max := m.maxLogScroll()
	if m.logScroll > max {
		m.logScroll = max
	}
}

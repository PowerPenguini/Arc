package components

func drawLogCard(state ViewState, b [][]cell, x, y, w, h int) {
	border := cGrid2
	drawBox(b, x, y, w, h, border)

	hdrY := y + 1
	barX := x + 1
	barW := w - 2
	if barW > 0 {
		fillRect(b, barX, hdrY, barW, 1, cBG, cLime, ' ')
		drawText(b, barX+1, hdrY, cBG, cLime, " 2 - SETUP LOG")
	}

	if w > 2 && h > 3 {
		fillRect(b, x+1, y+2, w-2, h-3, cText, cBG, ' ')
	}

	target := state.ReadyAs
	if target == "" && state.Host != "" {
		target = "arc@" + state.Host
	}
	drawText(b, x+2, y+3, cText, cBG, "Target: "+target)

	baseY := y + 5
	for _, step := range state.Steps {
		if step.State == StepRunning {
			drawText(b, x+2, y+4, cSub, cBG, "Current: "+step.Label)
			baseY = y + 6
			break
		}
	}

	cardR := Rect{X: x, Y: y, W: w, H: h}
	btnR, hasButton := ButtonRect(cardR, FinishLabel)

	contentBottom := y + h - 3
	if hasButton {
		contentBottom = btnR.Y - 3
	}
	if contentBottom < baseY {
		contentBottom = baseY
	}

	type line struct {
		step Step
	}
	var lines []line
	for _, step := range state.Steps {
		if step.State == StepPending {
			continue
		}
		lines = append(lines, line{step: step})
	}

	avail := contentBottom - baseY + 1
	if avail < 1 {
		avail = 1
	}

	start := 0
	if len(lines) > avail {
		start = len(lines) - avail
	}
	if state.LogScroll > 0 {
		start -= state.LogScroll
		if start < 0 {
			start = 0
		}
	}
	if len(lines) > avail && start > len(lines)-avail {
		start = len(lines) - avail
	}

	visible := 0
	for i := start; i < len(lines) && visible < avail; i++ {
		step := lines[i].step
		rowY := baseY + visible
		visible++

		label := step.Label
		prefix := "[ ]"
		prefixFG := cSub
		lineFG := cText

		switch step.State {
		case StepRunning:
			spin := state.SpinnerRune
			if spin == 0 {
				spin = '*'
			}
			prefix = "[" + string(spin) + "]"
			prefixFG = cLime
		case StepDone:
			prefix = "[✓]"
			prefixFG = cLime
		case StepFailed:
			prefix = "[✗]"
			prefixFG = cErr
			lineFG = cErr
			if step.Err != "" {
				label = step.Label + " (failed)"
			}
		}

		drawText(b, x+2, rowY, prefixFG, cBG, prefix)
		drawText(b, x+2+len([]rune(prefix))+1, rowY, lineFG, cBG, label)
	}

	if len(lines) > avail {
		if start > 0 {
			drawText(b, x+w-14, y+3, cSub, cBG, "↑ older")
		}
		if start+avail < len(lines) {
			drawText(b, x+w-14, y+4, cSub, cBG, "↓ newer")
		}
	}

	footerY := y + h - 2
	if footerY > baseY {
		switch {
		case state.Err != "":
			lines := wrapText(state.Err, w-4)
			startY := footerY - (len(lines) - 1)
			if startY < baseY {
				startY = baseY
			}
			for i, ln := range lines {
				yy := startY + i
				if yy >= y+h-1 {
					break
				}
				drawText(b, x+2, yy, cErr, cBG, ln)
			}
		}
	}

	if state.Submitted && state.Err == "" {
		completeY := contentBottom + 1
		if hasButton {
			completeY = btnR.Y - 1
		}
		if completeY >= y+2 && completeY < y+h-1 {
			fx := x + 2
			drawText(b, fx, completeY, cLime, cBG, "----")
			drawText(b, fx+4, completeY, cText, cBG, " SETUP COMPLETE ")
			drawText(b, fx+4+len(" SETUP COMPLETE "), completeY, cLime, cBG, "----")
		}

		if hasButton {
			fg := cLime
			bgMain := cGrid
			if state.BtnDown {
				fg = cBG
				bgMain = cLime
			} else if state.BtnHover {
				bgMain = cGrid2
			}

			midY := btnR.Y + btnR.H/2
			if btnR.H >= 3 {
				fillRect(b, btnR.X, btnR.Y, btnR.W, 1, bgMain, cBG, '▄')
				fillRect(b, btnR.X, midY, btnR.W, 1, cText, bgMain, ' ')
				fillRect(b, btnR.X, btnR.Y+btnR.H-1, btnR.W, 1, bgMain, cBG, '▀')
			} else {
				fillRect(b, btnR.X, btnR.Y, btnR.W, btnR.H, cText, bgMain, ' ')
			}

			labelX := btnR.X + (btnR.W-len(FinishLabel))/2
			if labelX < btnR.X {
				labelX = btnR.X
			}
			drawText(b, labelX, midY, fg, bgMain, FinishLabel)
		}
	}
}

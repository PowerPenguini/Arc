package components

func drawLogCard(state ViewState, b [][]cell, x, y, w, h int) {
	border := cGrid2
	drawBox(b, x, y, w, h, border)

	hdrY := y + 1
	barX := x + 1
	barW := w - 2
	if barW > 0 {
		fillRect(b, barX, hdrY, barW, 1, cBG, cLime, ' ')
		drawText(b, barX+1, hdrY, cBG, cLime, " 2 - ONBOARD LOG")
	}

	if w > 2 && h > 3 {
		fillRect(b, x+1, y+2, w-2, h-3, cText, cBG, ' ')
	}

	target := state.ReadyAs
	if target == "" && state.Host != "" {
		target = "arc@" + state.Host
	}
	drawText(b, x+2, y+3, cText, cBG, "ARC target: "+target)

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
	if state.Submitted && state.Err == "" {
		hasButton = false
	}

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

	if !(state.Submitted && state.Err == "" && (len(state.MobileQR) > 0 || state.MobileQRErr != "")) {
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
		if len(state.MobileQR) > 0 {
			drawSubmittedQRCode(state, b, x, y, w, h, hasButton, btnR)
		} else if state.MobileQRErr != "" {
			drawSubmittedQRError(state, b, x, y, w, h, hasButton, btnR)
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
		} else {
			hint := "Press Enter to finish"
			hintX := x + w - 2 - len([]rune(hint))
			if hintX < x+2 {
				hintX = x + 2
			}
			drawText(b, hintX, y+h-2, cSub, cBG, hint)
		}
	}
}

func drawSubmittedQRCode(state ViewState, b [][]cell, x, y, w, h int, hasButton bool, btnR Rect) {
	headerY := y + 5
	drawText(b, x+2, headerY, cLime, cBG, "Scan with ARC mobile")

	contentTop := headerY + 2
	contentBottom := y + h - 3
	if hasButton {
		contentBottom = btnR.Y - 2
	}
	if contentBottom < contentTop {
		return
	}

	qrW := 0
	for _, row := range state.MobileQR {
		width := len([]rune(row))
		if width > qrW {
			qrW = width
		}
	}
	if qrW == 0 {
		return
	}

	availH := contentBottom - contentTop + 1
	qrH := len(state.MobileQR)
	if qrW > w-4 || qrH > availH {
		drawText(b, x+2, contentTop, cErr, cBG, "Terminal too small for phone QR.")
		drawText(b, x+2, contentTop+2, cSub, cBG, "Make the window larger and rerun ARC.")
		return
	}
	startY := contentTop
	if qrH < availH {
		startY = contentTop + (availH-qrH)/2
	}
	startX := x + 2
	if qrW < w-4 {
		startX = x + 2 + ((w-4)-qrW)/2
	}

	maxRows := qrH
	if maxRows > availH {
		maxRows = availH
	}
	for i := 0; i < maxRows; i++ {
		drawText(b, startX, startY+i, cBG, cQRBg, state.MobileQR[i])
	}
}

func drawSubmittedQRError(state ViewState, b [][]cell, x, y, w, h int, hasButton bool, btnR Rect) {
	headerY := y + 5
	drawText(b, x+2, headerY, cLime, cBG, "Phone QR unavailable")

	contentBottom := y + h - 3
	if hasButton {
		contentBottom = btnR.Y - 2
	}
	for i, ln := range wrapText(state.MobileQRErr, w-4) {
		yy := headerY + 2 + i
		if yy > contentBottom {
			break
		}
		drawText(b, x+2, yy, cErr, cBG, ln)
	}
}

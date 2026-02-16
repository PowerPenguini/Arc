package main

func (m model) View() string {
	if m.w <= 0 || m.h <= 0 {
		return ""
	}

	b := newBuf(m.w, m.h)
	drawGrid(b, 6, 3)

	layout, ok := m.computeLayout()
	if !ok {
		return renderBuf(b)
	}

	boltW := maxLineLen(logoBolt)
	drawLinesSkipSpaces(b, layout.logoX, layout.logoY, cLime, cBG, logoBolt)
	drawLinesSkipSpaces(b, layout.logoX+boltW+2, layout.logoY, cText, cBG, logoArc)
	drawTextSkipSpaces(b, layout.logoX+boltW+2, layout.logoY+len(logoArc), cDim, cBG, logoTag)

	if m.phase == phaseLog {
		m.drawLogCard(b, layout.card.x, layout.card.y, layout.card.w, layout.card.h)
	} else if m.phase == phaseLocal {
		m.drawLocalCard(b, layout.card.x, layout.card.y, layout.card.w, layout.card.h)
	} else if m.phase == phaseInfra {
		m.drawInfraCard(b, layout.card.x, layout.card.y, layout.card.w, layout.card.h)
	} else {
		m.drawCredCard(b, layout.card.x, layout.card.y, layout.card.w, layout.card.h)
	}

	return renderBuf(b)
}

func (m model) drawCredCard(b [][]cell, x, y, w, h int) {
	border := cGrid2
	drawBox(b, x, y, w, h, border)

	hdrY := y + 1
	barX := x + 1
	barW := w - 2
	if barW > 0 {
		fillRect(b, barX, hdrY, barW, 1, cBG, cLime, ' ')
		drawText(b, barX+1, hdrY, cBG, cLime, cardHeaderText)
	}

	if w > 2 && h > 3 {
		fillRect(b, x+1, y+2, w-2, h-3, cText, cBG, ' ')
	}

	// Hard gate: require local passwordless sudo before showing any credential inputs.
	if !m.localSudoChecked || (m.localSudoChecked && !m.localSudoOK) {
		msgX := x + 2
		msgY := y + 4
		if !m.localSudoChecked {
			drawText(b, msgX, msgY, cText, cBG, "Checking local sudo...")
			drawText(b, msgX, msgY+2, cSub, cBG, "This is required to run local setup + infra steps.")
			return
		}

		drawText(b, msgX, msgY, cErr, cBG, "Local sudo is required.")
		drawText(b, msgX, msgY+2, cText, cBG, "Fix it by running in another terminal:")
		drawText(b, msgX, msgY+3, cLime, cBG, "sudo -v")
		drawText(b, msgX, msgY+5, cSub, cBG, "Then restart this program.")
		return
	}

	ipY := y + 3
	passY := ipY + cardInputBoxH + 1

	boxX := x + 2 + cardInputLabelW
	boxW := w - 4 - cardInputLabelW
	if boxW < 16 {
		boxW = 16
	}
	if boxX+boxW > x+w-2 {
		boxW = (x + w - 2) - boxX
	}

	ipFocused := m.focus == 0 && !m.submitted && !m.working
	passFocused := m.focus == 1 && !m.submitted && !m.working
	connectFocused := (m.focus == 2 || m.btnHover) && !m.submitted && !m.working

	drawText(b, x+2, ipY+1, cDim, cBG, "User@IP")
	drawText(b, x+2, passY+1, cDim, cBG, "Password")

	ipBorder := cGrid2
	if ipFocused {
		ipBorder = cLime
	}
	passBorder := cGrid2
	if passFocused {
		passBorder = cLime
	}

	if boxW >= 4 {
		drawBox(b, boxX, ipY, boxW, cardInputBoxH, ipBorder)
		fillRect(b, boxX+1, ipY+1, boxW-2, 1, cText, cBG, ' ')
		m.ip.drawInto(b, boxX+1, ipY+1, boxW-2, ipFocused)
	}

	if boxW >= 4 {
		drawBox(b, boxX, passY, boxW, cardInputBoxH, passBorder)
		fillRect(b, boxX+1, passY+1, boxW-2, 1, cText, cBG, ' ')
		m.pass.drawInto(b, boxX+1, passY+1, boxW-2, passFocused)
	}

	cardR := rect{x: x, y: y, w: w, h: h}
	if btnR, ok := buttonRect(cardR, connectLabel); ok {
		// Inline status line (we no longer render messages below the card).
		msgY := btnR.y - 1
		if msgY > passY+1 && msgY < btnR.y {
			if m.err != "" {
				for i, ln := range wrapText(m.err, w-4) {
					yy := msgY + i
					if yy >= btnR.y {
						break
					}
					drawText(b, x+2, yy, cErr, cBG, ln)
				}
			}
		}

		fg := cBG
		bgMain := cLime
		if m.submitted || m.working || !m.localSudoChecked || (m.localSudoChecked && !m.localSudoOK) {
			fg = cSub
			bgMain = cBG
		} else if !m.btnDown {
			fg = cLime
			bgMain = cGrid
			if connectFocused {
				bgMain = cGrid2
			}
		}

		midY := btnR.y + btnR.h/2
		if bgMain == cBG {
			fillRect(b, btnR.x, btnR.y, btnR.w, btnR.h, cText, cBG, ' ')
		} else if btnR.h >= 3 {
			fillRect(b, btnR.x, btnR.y, btnR.w, 1, bgMain, cBG, '▄')
			fillRect(b, btnR.x, midY, btnR.w, 1, cText, bgMain, ' ')
			fillRect(b, btnR.x, btnR.y+btnR.h-1, btnR.w, 1, bgMain, cBG, '▀')
		} else {
			fillRect(b, btnR.x, btnR.y, btnR.w, btnR.h, cText, bgMain, ' ')
		}

		labelX := btnR.x + (btnR.w-len(connectLabel))/2
		if labelX < btnR.x {
			labelX = btnR.x
		}
		drawText(b, labelX, midY, fg, bgMain, connectLabel)
	}
}

func (m model) drawLogCard(b [][]cell, x, y, w, h int) {
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

	target := m.readyAs
	if target == "" {
		target = arcUser + "@" + m.host
	}
	drawText(b, x+2, y+3, cText, cBG, "Target: "+target)

	baseY := y + 5
	row := 0
	for _, step := range m.steps {
		if step.state == stepPending {
			continue
		}
		rowY := baseY + row
		if rowY >= y+h-1 {
			break
		}
		row++

		line := step.label
		prefix := "[ ]"
		prefixFG := cSub
		lineFG := cText

		switch step.state {
		case stepRunning:
			prefix = "[" + string(spinnerFrames[m.spinnerTick%len(spinnerFrames)]) + "]"
			prefixFG = cLime
		case stepDone:
			prefix = "[✓]"
			prefixFG = cLime
		case stepFailed:
			prefix = "[✗]"
			prefixFG = cErr
			if step.err != "" {
				line = step.label + " (failed)"
			}
		}

		drawText(b, x+2, rowY, prefixFG, cBG, prefix)
		drawText(b, x+2+len([]rune(prefix))+1, rowY, lineFG, cBG, line)
	}

	footerY := y + h - 2
	if footerY > baseY {
		switch {
		case m.err != "":
			lines := wrapText(m.err, w-4)
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
		case !m.submitted:
			drawText(b, x+2, footerY, cText, cBG, "esc: back")
		}
	}

	// "SETUP COMPLETE" should appear at the end of the log, separated by one blank line.
	if m.submitted && m.err == "" {
		blankY := baseY + row
		if blankY >= y+h-1 {
			return
		}
		// One empty line.
		drawText(b, x+2, blankY, cText, cBG, "")
		completeY := blankY + 1
		if completeY >= y+h-1 {
			return
		}
		fx := x + 2
		drawText(b, fx, completeY, cLime, cBG, "----")
		drawText(b, fx+4, completeY, cText, cBG, " SETUP COMPLETE ")
		drawText(b, fx+4+len(" SETUP COMPLETE "), completeY, cLime, cBG, "----")

		// Next button (go to local setup).
		cardR := rect{x: x, y: y, w: w, h: h}
		if btnR, ok := buttonRect(cardR, nextLabel); ok {
			fg := cLime
			bgMain := cGrid
			if m.btnDown {
				fg = cBG
				bgMain = cLime
			} else if m.btnHover {
				bgMain = cGrid2
			}

			midY := btnR.y + btnR.h/2
			if btnR.h >= 3 {
				fillRect(b, btnR.x, btnR.y, btnR.w, 1, bgMain, cBG, '▄')
				fillRect(b, btnR.x, midY, btnR.w, 1, cText, bgMain, ' ')
				fillRect(b, btnR.x, btnR.y+btnR.h-1, btnR.w, 1, bgMain, cBG, '▀')
			} else {
				fillRect(b, btnR.x, btnR.y, btnR.w, btnR.h, cText, bgMain, ' ')
			}

			labelX := btnR.x + (btnR.w-len(nextLabel))/2
			if labelX < btnR.x {
				labelX = btnR.x
			}
			drawText(b, labelX, midY, fg, bgMain, nextLabel)
		}
	}
}

func (m model) drawLocalCard(b [][]cell, x, y, w, h int) {
	border := cGrid2
	drawBox(b, x, y, w, h, border)

	hdrY := y + 1
	barX := x + 1
	barW := w - 2
	if barW > 0 {
		fillRect(b, barX, hdrY, barW, 1, cBG, cLime, ' ')
		drawText(b, barX+1, hdrY, cBG, cLime, localHeaderText)
	}

	if w > 2 && h > 3 {
		fillRect(b, x+1, y+2, w-2, h-3, cText, cBG, ' ')
	}

	baseY := y + 3
	if !m.localWorking && len(m.localSteps) == 0 && !m.localDone && m.localErr == "" {
		lines := []string{
			"Local install writes the ARC prompt to ~/.bashrc",
			"and ensures ~/.bash_profile sources ~/.bashrc.",
			"",
			"Icons require a Nerd Font in your terminal.",
			"Install one (e.g. MesloLGS NF, FiraCode Nerd Font).",
		}
		for i, ln := range lines {
			yy := baseY + i
			if yy >= y+h-3 {
				break
			}
			drawText(b, x+2, yy, cText, cBG, ln)
		}
	} else {
		row := 0
		for _, step := range m.localSteps {
			if step.state == stepPending {
				continue
			}
			rowY := baseY + row
			if rowY >= y+h-4 {
				break
			}
			row++

			line := step.label
			prefix := "[ ]"
			prefixFG := cSub
			lineFG := cText
			switch step.state {
			case stepRunning:
				prefix = "[" + string(spinnerFrames[m.spinnerTick%len(spinnerFrames)]) + "]"
				prefixFG = cLime
			case stepDone:
				prefix = "[✓]"
				prefixFG = cLime
			case stepFailed:
				prefix = "[✗]"
				prefixFG = cErr
				if step.err != "" {
					line = step.label + " (failed)"
				}
			}

			drawText(b, x+2, rowY, prefixFG, cBG, prefix)
			drawText(b, x+2+len([]rune(prefix))+1, rowY, lineFG, cBG, line)
		}

		if m.localDone && m.localErr == "" {
			blankY := baseY + row
			if blankY < y+h-4 {
				drawText(b, x+2, blankY, cText, cBG, "")
				completeY := blankY + 1
				if completeY < y+h-4 {
					fx := x + 2
					drawText(b, fx, completeY, cLime, cBG, "----")
					drawText(b, fx+4, completeY, cText, cBG, " SETUP COMPLETE ")
					drawText(b, fx+4+len(" SETUP COMPLETE "), completeY, cLime, cBG, "----")
				}
			}
		}
	}

	cardR := rect{x: x, y: y, w: w, h: h}
	label := installLabel
	disabled := m.localWorking
	if m.localDone {
		label = nextLabel
	}
	if btnR, ok := buttonRect(cardR, label); ok {
		// Inline error/status line above the button.
		if m.localErr != "" {
			msgY := btnR.y - 1
			if msgY >= y+3 && msgY < btnR.y {
				for i, ln := range wrapText(m.localErr, w-4) {
					yy := msgY + i
					if yy >= btnR.y {
						break
					}
					drawText(b, x+2, yy, cErr, cBG, ln)
				}
			}
		}

		fg := cLime
		bgMain := cGrid
		if disabled {
			fg = cSub
			bgMain = cBG
		} else if m.btnDown {
			fg = cBG
			bgMain = cLime
		} else if m.btnHover {
			bgMain = cGrid2
		}

		midY := btnR.y + btnR.h/2
		if bgMain == cBG {
			fillRect(b, btnR.x, btnR.y, btnR.w, btnR.h, cText, cBG, ' ')
		} else if btnR.h >= 3 {
			fillRect(b, btnR.x, btnR.y, btnR.w, 1, bgMain, cBG, '▄')
			fillRect(b, btnR.x, midY, btnR.w, 1, cText, bgMain, ' ')
			fillRect(b, btnR.x, btnR.y+btnR.h-1, btnR.w, 1, bgMain, cBG, '▀')
		} else {
			fillRect(b, btnR.x, btnR.y, btnR.w, btnR.h, cText, bgMain, ' ')
		}

		labelX := btnR.x + (btnR.w-len(label))/2
		if labelX < btnR.x {
			labelX = btnR.x
		}
		drawText(b, labelX, midY, fg, bgMain, label)
	}
}

func (m model) drawInfraCard(b [][]cell, x, y, w, h int) {
	border := cGrid2
	drawBox(b, x, y, w, h, border)

	hdrY := y + 1
	barX := x + 1
	barW := w - 2
	if barW > 0 {
		fillRect(b, barX, hdrY, barW, 1, cBG, cLime, ' ')
		drawText(b, barX+1, hdrY, cBG, cLime, infraHeaderText)
	}

	if w > 2 && h > 3 {
		fillRect(b, x+1, y+2, w-2, h-3, cText, cBG, ' ')
	}

	baseY := y + 3
	row := 0
	for _, step := range m.infraSteps {
		if step.state == stepPending {
			continue
		}
		rowY := baseY + row
		if rowY >= y+h-4 {
			break
		}
		row++

		line := step.label
		prefix := "[ ]"
		prefixFG := cSub
		lineFG := cText

		switch step.state {
		case stepRunning:
			prefix = "[" + string(spinnerFrames[m.spinnerTick%len(spinnerFrames)]) + "]"
			prefixFG = cLime
		case stepDone:
			prefix = "[✓]"
			prefixFG = cLime
		case stepFailed:
			prefix = "[✗]"
			prefixFG = cErr
			if step.err != "" {
				line = step.label + " (failed)"
			}
		}

		drawText(b, x+2, rowY, prefixFG, cBG, prefix)
		drawText(b, x+2+len([]rune(prefix))+1, rowY, lineFG, cBG, line)
	}

	footerY := y + h - 2
	if footerY > baseY {
		switch {
		case m.infraErr != "":
			lines := wrapText(m.infraErr, w-4)
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
		case !m.infraDone:
			drawText(b, x+2, footerY, cText, cBG, "")
		}
	}

	if m.infraDone && m.infraErr == "" {
		blankY := baseY + row
		if blankY < y+h-4 {
			drawText(b, x+2, blankY, cText, cBG, "")
			completeY := blankY + 1
			if completeY < y+h-4 {
				fx := x + 2
				drawText(b, fx, completeY, cLime, cBG, "----")
				drawText(b, fx+4, completeY, cText, cBG, " SETUP COMPLETE ")
				drawText(b, fx+4+len(" SETUP COMPLETE "), completeY, cLime, cBG, "----")
			}
		}

		cardR := rect{x: x, y: y, w: w, h: h}
		if btnR, ok := buttonRect(cardR, finishLabel); ok {
			fg := cLime
			bgMain := cGrid
			if m.btnDown {
				fg = cBG
				bgMain = cLime
			} else if m.btnHover {
				bgMain = cGrid2
			}

			midY := btnR.y + btnR.h/2
			if btnR.h >= 3 {
				fillRect(b, btnR.x, btnR.y, btnR.w, 1, bgMain, cBG, '▄')
				fillRect(b, btnR.x, midY, btnR.w, 1, cText, bgMain, ' ')
				fillRect(b, btnR.x, btnR.y+btnR.h-1, btnR.w, 1, bgMain, cBG, '▀')
			} else {
				fillRect(b, btnR.x, btnR.y, btnR.w, btnR.h, cText, bgMain, ' ')
			}

			labelX := btnR.x + (btnR.w-len(finishLabel))/2
			if labelX < btnR.x {
				labelX = btnR.x
			}
			drawText(b, labelX, midY, fg, bgMain, finishLabel)
		}
	}
}

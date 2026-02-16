package components

func drawCredCard(state ViewState, b [][]cell, x, y, w, h int) {
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

	if !state.LocalSudoChecked || (state.LocalSudoChecked && !state.LocalSudoOK) {
		msgX := x + 2
		msgY := y + 4
		if !state.LocalSudoChecked {
			drawText(b, msgX, msgY, cText, cBG, "Checking local sudo...")
			drawText(b, msgX, msgY+2, cSub, cBG, "This is required to run setup steps.")
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

	ipFocused := state.Focus == 0 && !state.Submitted && !state.Working
	passFocused := state.Focus == 1 && !state.Submitted && !state.Working
	connectFocused := (state.Focus == 2 || state.BtnHover) && !state.Submitted && !state.Working

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
		ip := state.IP
		ip.drawInto(b, boxX+1, ipY+1, boxW-2, ipFocused)
	}

	if boxW >= 4 {
		drawBox(b, boxX, passY, boxW, cardInputBoxH, passBorder)
		fillRect(b, boxX+1, passY+1, boxW-2, 1, cText, cBG, ' ')
		pass := state.Pass
		pass.drawInto(b, boxX+1, passY+1, boxW-2, passFocused)
	}

	cardR := Rect{X: x, Y: y, W: w, H: h}
	if btnR, ok := ButtonRect(cardR, ConnectLabel); ok {
		msgY := btnR.Y - 1
		if msgY > passY+1 && msgY < btnR.Y {
			if state.Err != "" {
				for i, ln := range wrapText(state.Err, w-4) {
					yy := msgY + i
					if yy >= btnR.Y {
						break
					}
					drawText(b, x+2, yy, cErr, cBG, ln)
				}
			}
		}

		fg := cBG
		bgMain := cLime
		if state.Submitted || state.Working || !state.LocalSudoChecked || (state.LocalSudoChecked && !state.LocalSudoOK) {
			fg = cSub
			bgMain = cBG
		} else if !state.BtnDown {
			fg = cLime
			bgMain = cGrid
			if connectFocused {
				bgMain = cGrid2
			}
		}

		midY := btnR.Y + btnR.H/2
		if bgMain == cBG {
			fillRect(b, btnR.X, btnR.Y, btnR.W, btnR.H, cText, cBG, ' ')
		} else if btnR.H >= 3 {
			fillRect(b, btnR.X, btnR.Y, btnR.W, 1, bgMain, cBG, '▄')
			fillRect(b, btnR.X, midY, btnR.W, 1, cText, bgMain, ' ')
			fillRect(b, btnR.X, btnR.Y+btnR.H-1, btnR.W, 1, bgMain, cBG, '▀')
		} else {
			fillRect(b, btnR.X, btnR.Y, btnR.W, btnR.H, cText, bgMain, ' ')
		}

		labelX := btnR.X + (btnR.W-len(ConnectLabel))/2
		if labelX < btnR.X {
			labelX = btnR.X
		}
		drawText(b, labelX, midY, fg, bgMain, ConnectLabel)
	}
}

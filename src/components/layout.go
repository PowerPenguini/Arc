package components

func ButtonRect(cardR Rect, label string) (Rect, bool) {
	btnW := len(label) + connectPadX*2
	btnH := 3
	if cardR.W < btnW+2 || cardR.H < btnH+2 {
		return Rect{}, false
	}

	btnY := cardR.Y + cardR.H - 1 - btnH
	btnX := cardR.X + (cardR.W-btnW)/2

	minX := cardR.X + 1
	maxX := cardR.X + cardR.W - 1 - btnW
	if btnX < minX {
		btnX = minX
	}
	if btnX > maxX {
		btnX = maxX
	}
	if btnY < cardR.Y+1 || btnY+btnH > cardR.Y+cardR.H-1 {
		return Rect{}, false
	}

	return Rect{X: btnX, Y: btnY, W: btnW, H: btnH}, true
}

func InputRectsFromCard(cardR Rect) (Rect, Rect, bool) {
	ipY := cardR.Y + 3
	passY := ipY + cardInputBoxH + 1
	boxX := cardR.X + 2 + cardInputLabelW
	boxW := cardR.W - 4 - cardInputLabelW
	if boxW < 16 {
		boxW = 16
	}
	if boxX+boxW > cardR.X+cardR.W-2 {
		boxW = (cardR.X + cardR.W - 2) - boxX
	}
	if boxW < 4 {
		return Rect{}, Rect{}, false
	}

	ipRect := Rect{X: boxX + 1, Y: ipY + 1, W: boxW - 2, H: 1}
	passRect := Rect{X: boxX + 1, Y: passY + 1, W: boxW - 2, H: 1}
	return ipRect, passRect, true
}

func logoSize() (int, int) {
	boltW := maxLineLen(logoBolt)
	arcW := maxLineLen(logoArc)
	logoW := boltW + 2 + arcW
	logoH := maxInt(len(logoBolt), len(logoArc)+1)
	return logoW, logoH
}

func ComputeLayout(w, h int, phase Phase) (Layout, bool) {
	if w <= 0 || h <= 0 {
		return Layout{}, false
	}

	logoW, logoH := logoSize()

	containerW := containerWFixed
	if containerW > w-4 {
		containerW = w - 4
	}
	if containerW < logoW+colGap+cardMinW {
		containerW = minInt(w-4, logoW+colGap+cardMinW)
	}
	cx0 := (w - containerW) / 2
	if cx0 < 0 {
		cx0 = 0
	}

	leftW := containerW - colGap - cardWFixed
	if leftW < logoW {
		leftW = logoW
	}
	containerW = leftW + colGap + cardWFixed
	if containerW > w-4 {
		containerW = w - 4
		if containerW-colGap-leftW < cardMinW {
			leftW = maxInt(logoW, containerW-colGap-cardMinW)
		}
	}
	cx0 = (w - containerW) / 2
	if cx0 < 0 {
		cx0 = 0
	}

	cardH := 14
	maxH := h - 8
	if phase == PhaseLog {
		cardH = 22
		maxH = h - 6
	}
	if cardH > maxH {
		cardH = maxH
	}
	if cardH < 7 {
		return Layout{}, false
	}

	cardBlockH := cardH + 2
	blockH := maxInt(logoH, cardBlockH)
	baseY := (h - blockH) / 2
	if baseY < 1 {
		baseY = 1
	}
	logoY := baseY + (blockH-logoH)/2
	cardY := baseY + (blockH-cardBlockH)/2

	cardX := cx0 + leftW + colGap
	cardW := containerW - leftW - colGap
	if cardW < cardMinW {
		return Layout{}, false
	}

	return Layout{
		LogoX: cx0 + 2,
		LogoY: logoY,
		Card:  Rect{X: cardX, Y: cardY, W: cardW, H: cardH},
	}, true
}

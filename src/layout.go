package main

type appLayout struct {
	logoX int
	logoY int
	card  rect
}

func buttonRect(cardR rect, label string) (rect, bool) {
	btnW := len(label) + connectPadX*2
	btnH := 3
	if cardR.w < btnW+2 || cardR.h < btnH+2 {
		return rect{}, false
	}

	btnY := cardR.y + cardR.h - 1 - btnH
	btnX := cardR.x + (cardR.w-btnW)/2

	minX := cardR.x + 1
	maxX := cardR.x + cardR.w - 1 - btnW
	if btnX < minX {
		btnX = minX
	}
	if btnX > maxX {
		btnX = maxX
	}
	if btnY < cardR.y+1 || btnY+btnH > cardR.y+cardR.h-1 {
		return rect{}, false
	}

	return rect{x: btnX, y: btnY, w: btnW, h: btnH}, true
}

func inputRectsFromCard(cardR rect) (rect, rect, bool) {
	ipY := cardR.y + 3
	passY := ipY + cardInputBoxH + 1
	boxX := cardR.x + 2 + cardInputLabelW
	boxW := cardR.w - 4 - cardInputLabelW
	if boxW < 16 {
		boxW = 16
	}
	if boxX+boxW > cardR.x+cardR.w-2 {
		boxW = (cardR.x + cardR.w - 2) - boxX
	}
	if boxW < 4 {
		return rect{}, rect{}, false
	}

	ipRect := rect{x: boxX + 1, y: ipY + 1, w: boxW - 2, h: 1}
	passRect := rect{x: boxX + 1, y: passY + 1, w: boxW - 2, h: 1}
	return ipRect, passRect, true
}

func logoSize() (int, int) {
	boltW := maxLineLen(logoBolt)
	arcW := maxLineLen(logoArc)
	logoW := boltW + 2 + arcW
	logoH := maxInt(len(logoBolt), len(logoArc)+1)
	return logoW, logoH
}

func (m model) computeLayout() (appLayout, bool) {
	if m.w <= 0 || m.h <= 0 {
		return appLayout{}, false
	}

	logoW, logoH := logoSize()

	containerW := containerWFixed
	if containerW > m.w-4 {
		containerW = m.w - 4
	}
	if containerW < logoW+colGap+cardMinW {
		containerW = minInt(m.w-4, logoW+colGap+cardMinW)
	}
	cx0 := (m.w - containerW) / 2
	if cx0 < 0 {
		cx0 = 0
	}

	leftW := containerW - colGap - cardWFixed
	if leftW < logoW {
		leftW = logoW
	}
	containerW = leftW + colGap + cardWFixed
	if containerW > m.w-4 {
		containerW = m.w - 4
		if containerW-colGap-leftW < cardMinW {
			leftW = maxInt(logoW, containerW-colGap-cardMinW)
		}
	}
	cx0 = (m.w - containerW) / 2
	if cx0 < 0 {
		cx0 = 0
	}

	// Make the "SETUP LOG" card taller so it can show more steps.
	cardH := 14
	maxH := m.h - 8
	if m.phase == phaseLog {
		cardH = 22
		maxH = m.h - 6
	}
	if cardH > maxH {
		cardH = maxH
	}
	if cardH < 7 {
		return appLayout{}, false
	}

	cardBlockH := cardH + 2
	blockH := maxInt(logoH, cardBlockH)
	baseY := (m.h - blockH) / 2
	if baseY < 1 {
		baseY = 1
	}
	logoY := baseY + (blockH-logoH)/2
	cardY := baseY + (blockH-cardBlockH)/2

	cardX := cx0 + leftW + colGap
	cardW := containerW - leftW - colGap
	if cardW < cardMinW {
		return appLayout{}, false
	}

	return appLayout{
		logoX: cx0 + 2,
		logoY: logoY,
		card:  rect{x: cardX, y: cardY, w: cardW, h: cardH},
	}, true
}

package components

func useSubmittedQRFullscreen(state ViewState) bool {
	return state.Phase == PhaseLog &&
		state.Submitted &&
		state.Err == "" &&
		(len(state.MobileQR) > 0 || state.MobileQRErr != "")
}

func Render(state ViewState) string {
	if state.W <= 0 || state.H <= 0 {
		return ""
	}

	b := newBuf(state.W, state.H)
	drawGrid(b, 6, 3)

	if useSubmittedQRFullscreen(state) {
		card := Rect{X: 2, Y: 1, W: state.W - 4, H: state.H - 2}
		if card.W <= 0 || card.H <= 0 {
			return renderBuf(b)
		}
		drawLogCard(state, b, card.X, card.Y, card.W, card.H)
		return renderBuf(b)
	}

	layout, ok := ComputeLayout(state.W, state.H, state.Phase)
	if !ok {
		return renderBuf(b)
	}

	boltW := maxLineLen(logoBolt)
	drawLinesSkipSpaces(b, layout.LogoX, layout.LogoY, cLime, cBG, logoBolt)
	drawLinesSkipSpaces(b, layout.LogoX+boltW+2, layout.LogoY, cText, cBG, logoArc)
	drawTextSkipSpaces(b, layout.LogoX+boltW+2, layout.LogoY+len(logoArc), cDim, cBG, logoTag)

	switch state.Phase {
	case PhaseLog:
		drawLogCard(state, b, layout.Card.X, layout.Card.Y, layout.Card.W, layout.Card.H)
	default:
		drawCredCard(state, b, layout.Card.X, layout.Card.Y, layout.Card.W, layout.Card.H)
	}

	return renderBuf(b)
}

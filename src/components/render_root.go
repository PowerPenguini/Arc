package components

func Render(state ViewState) string {
	if state.W <= 0 || state.H <= 0 {
		return ""
	}

	b := newBuf(state.W, state.H)
	drawGrid(b, 6, 3)

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

package components

import (
	"strings"
	"unicode/utf8"
)

func newBuf(w, h int) [][]cell {
	b := make([][]cell, h)
	for y := 0; y < h; y++ {
		row := make([]cell, w)
		for x := 0; x < w; x++ {
			row[x] = cell{ch: ' ', fg: cText, bg: cBG}
		}
		b[y] = row
	}
	return b
}

func drawText(b [][]cell, x, y int, fg, bg rgb, s string) {
	if y < 0 || y >= len(b) {
		return
	}
	row := b[y]
	xi := x
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			break
		}
		s = s[size:]
		if r == '\n' {
			return
		}
		if xi >= 0 && xi < len(row) {
			row[xi] = cell{ch: r, fg: fg, bg: bg}
		}
		xi++
		if xi >= len(row) {
			return
		}
	}
}

func drawTextSkipSpaces(b [][]cell, x, y int, fg, bg rgb, s string) {
	if y < 0 || y >= len(b) {
		return
	}
	row := b[y]
	xi := x
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			break
		}
		s = s[size:]
		if r == '\n' {
			return
		}
		if r != ' ' {
			if xi >= 0 && xi < len(row) {
				row[xi] = cell{ch: r, fg: fg, bg: bg}
			}
		}
		xi++
		if xi >= len(row) {
			return
		}
	}
}

func drawHLine(b [][]cell, x, y, w int, fg, bg rgb, ch rune) {
	if y < 0 || y >= len(b) || w <= 0 {
		return
	}
	row := b[y]
	for i := 0; i < w; i++ {
		xi := x + i
		if xi >= 0 && xi < len(row) {
			row[xi] = cell{ch: ch, fg: fg, bg: bg}
		}
	}
}

func drawVLine(b [][]cell, x, y, h int, fg, bg rgb, ch rune) {
	if h <= 0 {
		return
	}
	for i := 0; i < h; i++ {
		yi := y + i
		if yi < 0 || yi >= len(b) {
			continue
		}
		row := b[yi]
		if x >= 0 && x < len(row) {
			row[x] = cell{ch: ch, fg: fg, bg: bg}
		}
	}
}

func drawBox(b [][]cell, x, y, w, h int, border rgb) {
	if w < 2 || h < 2 {
		return
	}

	tl, tr, bl, br := '╭', '╮', '╰', '╯'
	hz, vt := '─', '│'

	drawHLine(b, x+1, y, w-2, border, cBG, hz)
	drawHLine(b, x+1, y+h-1, w-2, border, cBG, hz)
	drawVLine(b, x, y+1, h-2, border, cBG, vt)
	drawVLine(b, x+w-1, y+1, h-2, border, cBG, vt)

	if y >= 0 && y < len(b) && x >= 0 && x < len(b[y]) {
		b[y][x] = cell{ch: tl, fg: border, bg: cBG}
	}
	if y >= 0 && y < len(b) && x+w-1 >= 0 && x+w-1 < len(b[y]) {
		b[y][x+w-1] = cell{ch: tr, fg: border, bg: cBG}
	}
	if y+h-1 >= 0 && y+h-1 < len(b) && x >= 0 && x < len(b[y+h-1]) {
		b[y+h-1][x] = cell{ch: bl, fg: border, bg: cBG}
	}
	if y+h-1 >= 0 && y+h-1 < len(b) && x+w-1 >= 0 && x+w-1 < len(b[y+h-1]) {
		b[y+h-1][x+w-1] = cell{ch: br, fg: border, bg: cBG}
	}
}

func drawGrid(b [][]cell, stepX, stepY int) {
	h := len(b)
	if h == 0 {
		return
	}
	w := len(b[0])
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			b[y][x].bg = cBG
			vx := stepX > 0 && x%stepX == 0
			hy := stepY > 0 && y%stepY == 0
			switch {
			case vx && hy:
				b[y][x] = cell{ch: '┼', fg: cGrid2, bg: cBG}
			case vx:
				b[y][x] = cell{ch: '│', fg: cGrid, bg: cBG}
			case hy:
				b[y][x] = cell{ch: '─', fg: cGrid, bg: cBG}
			}
		}
	}
}

func fillRect(b [][]cell, x, y, w, h int, fg, bg rgb, ch rune) {
	if w <= 0 || h <= 0 {
		return
	}
	for yi := 0; yi < h; yi++ {
		yy := y + yi
		if yy < 0 || yy >= len(b) {
			continue
		}
		row := b[yy]
		for xi := 0; xi < w; xi++ {
			xx := x + xi
			if xx < 0 || xx >= len(row) {
				continue
			}
			row[xx] = cell{ch: ch, fg: fg, bg: bg}
		}
	}
}

func maxLineLen(lines []string) int {
	m := 0
	for _, ln := range lines {
		if len(ln) > m {
			m = len(ln)
		}
	}
	return m
}

func drawLinesSkipSpaces(b [][]cell, x, y int, fg, bg rgb, lines []string) {
	for i, ln := range lines {
		drawTextSkipSpaces(b, x, y+i, fg, bg, ln)
	}
}

func renderBuf(b [][]cell) string {
	var sb strings.Builder
	curFG := rgb{-1, -1, -1}
	curBG := rgb{-1, -1, -1}

	set := func(fg, bg rgb) {
		if fg != curFG {
			sb.WriteString(ansiFG(fg))
			curFG = fg
		}
		if bg != curBG {
			sb.WriteString(ansiBG(bg))
			curBG = bg
		}
	}

	for y := 0; y < len(b); y++ {
		row := b[y]
		for x := 0; x < len(row); x++ {
			c := row[x]
			set(c.fg, c.bg)
			sb.WriteRune(c.ch)
		}
		sb.WriteString(ansiReset)
		curFG, curBG = rgb{-1, -1, -1}, rgb{-1, -1, -1}
		if y != len(b)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

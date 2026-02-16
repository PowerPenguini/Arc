package main

import tea "github.com/charmbracelet/bubbletea"

type field struct {
	placeholder string
	mask        bool

	value  []rune
	cursor int
}

func (f *field) valueString() string { return string(f.value) }

func (f *field) handleKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyLeft:
		f.cursor = clamp(f.cursor-1, 0, len(f.value))
	case tea.KeyRight:
		f.cursor = clamp(f.cursor+1, 0, len(f.value))
	case tea.KeyHome:
		f.cursor = 0
	case tea.KeyEnd:
		f.cursor = len(f.value)
	case tea.KeyBackspace:
		if f.cursor > 0 && len(f.value) > 0 {
			f.value = append(f.value[:f.cursor-1], f.value[f.cursor:]...)
			f.cursor--
		}
	case tea.KeyDelete:
		if f.cursor < len(f.value) {
			f.value = append(f.value[:f.cursor], f.value[f.cursor+1:]...)
		}
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return
		}
		ins := msg.Runes
		buf := make([]rune, 0, len(f.value)+len(ins))
		buf = append(buf, f.value[:f.cursor]...)
		buf = append(buf, ins...)
		buf = append(buf, f.value[f.cursor:]...)
		f.value = buf
		f.cursor += len(ins)
	}
	f.cursor = clamp(f.cursor, 0, len(f.value))
}

func (f *field) drawInto(b [][]cell, x, y, w int, focused bool) {
	if w < 1 {
		return
	}
	drawHLine(b, x, y, w, cText, cBG, ' ')

	val := []rune(f.valueString())
	valFG := cText
	if len(val) == 0 && !focused {
		val = []rune(f.placeholder)
		valFG = cSub
	}

	if f.mask && !(len(f.value) == 0 && !focused) {
		for i := range val {
			val[i] = '*'
		}
	}

	cur := clamp(f.cursor, 0, len([]rune(f.valueString())))
	if len(f.value) == 0 && !focused {
		cur = 0
	}

	start := 0
	if focused && cur >= w {
		start = cur - (w - 1)
	}
	if start < 0 {
		start = 0
	}

	for i := 0; i < w; i++ {
		idx := start + i
		r := ' '
		if idx < len(val) {
			r = val[idx]
		}
		fg := valFG
		bg := cBG
		if focused {
			curInWin := cur - start
			if curInWin < 0 {
				curInWin = 0
			}
			if curInWin >= w {
				curInWin = w - 1
			}
			if i == curInWin {
				fg = cBG
				bg = cLime
			}
		}
		if y >= 0 && y < len(b) {
			row := b[y]
			xi := x + i
			if xi >= 0 && xi < len(row) {
				row[xi] = cell{ch: r, fg: fg, bg: bg}
			}
		}
	}
}

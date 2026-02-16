package components

import tea "github.com/charmbracelet/bubbletea"

type Field struct {
	Placeholder string
	Mask        bool

	Value  []rune
	Cursor int
}

func (f *Field) ValueString() string { return string(f.Value) }

func (f *Field) HandleKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyLeft:
		f.Cursor = clamp(f.Cursor-1, 0, len(f.Value))
	case tea.KeyRight:
		f.Cursor = clamp(f.Cursor+1, 0, len(f.Value))
	case tea.KeyHome:
		f.Cursor = 0
	case tea.KeyEnd:
		f.Cursor = len(f.Value)
	case tea.KeyBackspace:
		if f.Cursor > 0 && len(f.Value) > 0 {
			f.Value = append(f.Value[:f.Cursor-1], f.Value[f.Cursor:]...)
			f.Cursor--
		}
	case tea.KeyDelete:
		if f.Cursor < len(f.Value) {
			f.Value = append(f.Value[:f.Cursor], f.Value[f.Cursor+1:]...)
		}
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return
		}
		ins := msg.Runes
		buf := make([]rune, 0, len(f.Value)+len(ins))
		buf = append(buf, f.Value[:f.Cursor]...)
		buf = append(buf, ins...)
		buf = append(buf, f.Value[f.Cursor:]...)
		f.Value = buf
		f.Cursor += len(ins)
	}
	f.Cursor = clamp(f.Cursor, 0, len(f.Value))
}

func (f *Field) drawInto(b [][]cell, x, y, w int, focused bool) {
	if w < 1 {
		return
	}
	drawHLine(b, x, y, w, cText, cBG, ' ')

	val := []rune(f.ValueString())
	valFG := cText
	if len(val) == 0 && !focused {
		val = []rune(f.Placeholder)
		valFG = cSub
	}

	if f.Mask && !(len(f.Value) == 0 && !focused) {
		for i := range val {
			val[i] = '*'
		}
	}

	cur := clamp(f.Cursor, 0, len([]rune(f.ValueString())))
	if len(f.Value) == 0 && !focused {
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

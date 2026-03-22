package app

import (
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const qrQuietZone = 4

func (m *model) buildMobileQRCode() {
	m.mobilePayload = ""
	m.mobileQR = nil
	m.mobileQRErr = ""

	payload, err := m.svc.BuildMobilePayload(m.host, m.wg)
	if err != nil {
		m.mobileQRErr = err.Error()
		return
	}

	qr, err := qrcode.New(payload, qrcode.Low)
	if err != nil {
		m.mobileQRErr = err.Error()
		return
	}

	m.mobilePayload = payload
	m.mobileQR = renderQRCodeRows(qr.Bitmap())
}

func renderQRCodeRows(bitmap [][]bool) []string {
	if len(bitmap) == 0 {
		return nil
	}

	size := len(bitmap) + qrQuietZone*2
	padded := make([][]bool, size)
	for y := range padded {
		padded[y] = make([]bool, size)
	}
	for y := range bitmap {
		for x := range bitmap[y] {
			padded[y+qrQuietZone][x+qrQuietZone] = bitmap[y][x]
		}
	}

	rows := make([]string, 0, (size+1)/2)
	for y := 0; y < size; y += 2 {
		var b strings.Builder
		for x := 0; x < size; x++ {
			b.WriteRune(qrHalfBlockRune(
				qrAt(padded, x, y),
				qrAt(padded, x, y+1),
			))
		}
		rows = append(rows, b.String())
	}
	return rows
}

func qrAt(bitmap [][]bool, x, y int) bool {
	if y < 0 || y >= len(bitmap) {
		return false
	}
	if x < 0 || x >= len(bitmap[y]) {
		return false
	}
	return bitmap[y][x]
}

func qrHalfBlockRune(top, bottom bool) rune {
	switch {
	case top && bottom:
		return '█'
	case top:
		return '▀'
	case bottom:
		return '▄'
	default:
		return ' '
	}
}

package app

import "testing"

func TestRenderQRCodeRows_CompressesTwoByTwoModules(t *testing.T) {
	bitmap := [][]bool{
		{true, false, true},
		{false, true, false},
		{true, true, false},
	}

	rows := renderQRCodeRows(bitmap)
	if len(rows) != 6 {
		t.Fatalf("unexpected row count: got %d want 6", len(rows))
	}

	for i, row := range rows {
		if got := len([]rune(row)); got != 11 {
			t.Fatalf("row %d width mismatch: got %d want 11", i, got)
		}
	}
}

func TestQrHalfBlockRune_CoversKnownPatterns(t *testing.T) {
	cases := []struct {
		name        string
		top, bottom bool
		want        rune
	}{
		{name: "empty", want: ' '},
		{name: "top", top: true, want: '▀'},
		{name: "bottom", bottom: true, want: '▄'},
		{name: "full", top: true, bottom: true, want: '█'},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := qrHalfBlockRune(tc.top, tc.bottom)
			if got != tc.want {
				t.Fatalf("unexpected rune: got %q want %q", got, tc.want)
			}
		})
	}
}

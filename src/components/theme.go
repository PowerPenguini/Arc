package components

import "fmt"

type rgb struct{ r, g, b int }

type cell struct {
	ch rune
	fg rgb
	bg rgb
}

var (
	cBG    = rgb{0, 0, 0}
	cLime  = rgb{0xD7, 0xFF, 0x00}
	cDim   = rgb{0x9F, 0xB8, 0x00}
	cText  = rgb{0xEE, 0xEE, 0xEE}
	cSub   = rgb{0x88, 0x88, 0x88}
	cGrid  = rgb{0x16, 0x16, 0x16}
	cGrid2 = rgb{0x20, 0x20, 0x20}
	cErr   = rgb{0xFF, 0x4D, 0x4D}
)

func ansiFG(c rgb) string { return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.r, c.g, c.b) }
func ansiBG(c rgb) string { return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.r, c.g, c.b) }

const ansiReset = "\x1b[0m"

const (
	connectPadX = 2
	logoTag     = "Unified workspace"

	containerWFixed = 116
	cardWFixed      = 64
	colGap          = 6
	cardMinW        = 44

	cardHeaderText  = " 1 - SETUP REMOTE"
	cardInputLabelW = 10
	cardInputBoxH   = 3
)

var (
	logoBolt = []string{
		"     /",
		"   //",
		" ///",
		"////////",
		"    ///",
		"   //",
		"  /",
	}
	logoArc = []string{
		"    ___    ____  ______",
		"   /   |  / __ \\/ ____/",
		"  / /| | / /_/ / /     ",
		" / ___ |/ _, _/ /___   ",
		"/_/  |_/_/ |_|\\____/   ",
	}
)

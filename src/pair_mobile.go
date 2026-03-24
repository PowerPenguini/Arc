package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	arcPairingConfigDir      = ".config/arc"
	arcPairingPayloadPath    = ".config/arc/mobile-pairing.json"
	arcPairingPayloadPerm    = 0o600
	arcPairingBinaryPath     = ".local/bin/arc"
	arcPairingBinaryPerm     = 0o700
	arcPairingQRQuietZone    = 4
	arcPairingPayloadBegin   = "--- BEGIN ARC MOBILE PAYLOAD ---"
	arcPairingPayloadEnd     = "--- END ARC MOBILE PAYLOAD ---"
)

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runTUI(stdout, stderr)
	}

	switch args[0] {
	case "pair-mobile":
		if err := runPairMobile(stdout); err != nil {
			fmt.Fprintf(stderr, "arc pair-mobile: %v\n", err)
			return 1
		}
		return 0
	case "help", "--help", "-h":
		printArcUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "arc: unknown command %q\n\n", args[0])
		printArcUsage(stderr)
		return 2
	}
}

func printArcUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  arc pair-mobile")
}

func runPairMobile(w io.Writer) error {
	payload, err := readRemotePairingPayload()
	if err != nil {
		return err
	}

	rows, err := renderPairingQRCodeRows(payload)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "ARC mobile pairing")
	fmt.Fprintln(w)
	for _, row := range rows {
		fmt.Fprintln(w, row)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Fallback: in the mobile app use \"Paste QR payload\" and paste the JSON below.")
	fmt.Fprintln(w, arcPairingPayloadBegin)
	fmt.Fprintln(w, prettyPairingPayload(payload))
	fmt.Fprintln(w, arcPairingPayloadEnd)
	return nil
}

func readRemotePairingPayload() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot resolve home directory")
	}

	raw, err := os.ReadFile(filepath.Join(home, arcPairingPayloadPath))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("pairing payload is unavailable; run ARC setup first")
		}
		return "", fmt.Errorf("read pairing payload: %w", err)
	}

	payload := strings.TrimSpace(string(raw))
	if payload == "" {
		return "", fmt.Errorf("pairing payload is empty")
	}
	return payload, nil
}

func prettyPairingPayload(payload string) string {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, []byte(payload), "", "  "); err == nil {
		return formatted.String()
	}
	return payload
}

func renderPairingQRCodeRows(payload string) ([]string, error) {
	qr, err := qrcode.New(payload, qrcode.Low)
	if err != nil {
		return nil, fmt.Errorf("build pairing QR: %w", err)
	}
	return renderQRCodeRows(qr.Bitmap()), nil
}

func renderQRCodeRows(bitmap [][]bool) []string {
	if len(bitmap) == 0 {
		return nil
	}

	size := len(bitmap) + arcPairingQRQuietZone*2
	padded := make([][]bool, size)
	for y := range padded {
		padded[y] = make([]bool, size)
	}
	for y := range bitmap {
		for x := range bitmap[y] {
			padded[y+arcPairingQRQuietZone][x+arcPairingQRQuietZone] = bitmap[y][x]
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

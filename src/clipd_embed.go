//go:build arc_clipd_embed

package main

import (
	"embed"
	"fmt"
)

//go:embed embedded/arc-clipd
var embeddedClipdFS embed.FS

func embeddedArcClipdBinary() ([]byte, error) {
	binary, err := embeddedClipdFS.ReadFile("embedded/arc-clipd")
	if err != nil {
		return nil, fmt.Errorf("read embedded arc-clipd: %w", err)
	}
	if len(binary) == 0 {
		return nil, fmt.Errorf("embedded arc-clipd is empty")
	}
	return binary, nil
}

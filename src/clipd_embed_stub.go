//go:build !arc_clipd_embed

package main

import "fmt"

func embeddedArcClipdBinary() ([]byte, error) {
	return nil, fmt.Errorf("arc-clipd is not embedded in this build; rebuild via make build or make arc")
}

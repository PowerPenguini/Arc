package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPairMobile_PrintsQRCodeAndFallbackPayload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	payload := `{"host":"example.com","port":22,"username":"arc","privateKeyPem":"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----"}`
	payloadPath := filepath.Join(os.Getenv("HOME"), arcPairingPayloadPath)
	if err := os.MkdirAll(filepath.Dir(payloadPath), 0o700); err != nil {
		t.Fatalf("mkdir payload dir: %v", err)
	}
	if err := os.WriteFile(payloadPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	var out bytes.Buffer
	if err := runPairMobile(&out); err != nil {
		t.Fatalf("runPairMobile returned error: %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "ARC mobile pairing") {
		t.Fatalf("expected header in output, got %q", rendered)
	}
	if !strings.Contains(rendered, arcPairingPayloadBegin) || !strings.Contains(rendered, arcPairingPayloadEnd) {
		t.Fatalf("expected payload markers in output, got %q", rendered)
	}
	if !strings.Contains(rendered, `"host": "example.com"`) {
		t.Fatalf("expected pretty payload in output, got %q", rendered)
	}
}

func TestReadRemotePairingPayload_MissingState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := readRemotePairingPayload()
	if err == nil {
		t.Fatalf("expected error for missing payload")
	}
	if !strings.Contains(err.Error(), "pairing payload is unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMapRemoteUnameToGoArch(t *testing.T) {
	cases := map[string]string{
		"x86_64":  "amd64",
		"aarch64": "arm64",
		"armv7l":  "arm",
		"i686":    "386",
	}

	for raw, want := range cases {
		got, err := mapRemoteUnameToGoArch(raw)
		if err != nil {
			t.Fatalf("mapRemoteUnameToGoArch(%q) returned error: %v", raw, err)
		}
		if got != want {
			t.Fatalf("mapRemoteUnameToGoArch(%q) = %q, want %q", raw, got, want)
		}
	}
}

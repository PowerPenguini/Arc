package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

func syncRemoteArcHelper(addr, host string, wg wgConfig) error {
	payload, err := buildMobilePayload(host, wg)
	if err != nil {
		return err
	}

	client, err := dialArcWithKey(addr)
	if err != nil {
		return fmt.Errorf("cannot connect as %s: %w", arcUser, err)
	}
	defer client.Close()

	binary, err := currentArcHelperBinaryForRemote(client)
	if err != nil {
		return err
	}

	if err := uploadRemoteFile(client, arcPairingBinaryPath, binary, arcPairingBinaryPerm); err != nil {
		return fmt.Errorf("install remote arc helper: %w", err)
	}
	if err := uploadRemoteFile(client, arcPairingPayloadPath, append([]byte(payload), '\n'), arcPairingPayloadPerm); err != nil {
		return fmt.Errorf("install remote pairing payload: %w", err)
	}
	return nil
}

func currentArcHelperBinaryForRemote(client *ssh.Client) ([]byte, error) {
	goos, goarch, err := detectRemoteGoTarget(client)
	if err != nil {
		return nil, err
	}
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		return nil, fmt.Errorf(
			"remote helper install requires matching architecture; local=%s/%s remote=%s/%s",
			runtime.GOOS,
			runtime.GOARCH,
			goos,
			goarch,
		)
	}

	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve current executable: %w", err)
	}
	binary, err := os.ReadFile(execPath)
	if err != nil {
		return nil, fmt.Errorf("read current executable %s: %w", execPath, err)
	}
	return binary, nil
}

func detectRemoteGoTarget(client *ssh.Client) (goos, goarch string, err error) {
	out, err := runRemoteCommand(client, "printf '%s %s' \"$(uname -s)\" \"$(uname -m)\"", false, "")
	if err != nil {
		return "", "", fmt.Errorf("detect remote platform: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) != 2 {
		return "", "", fmt.Errorf("unexpected remote platform output %q", out)
	}

	goos, err = mapRemoteUnameToGoOS(fields[0])
	if err != nil {
		return "", "", err
	}
	goarch, err = mapRemoteUnameToGoArch(fields[1])
	if err != nil {
		return "", "", err
	}
	return goos, goarch, nil
}

func mapRemoteUnameToGoOS(uname string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(uname)) {
	case "linux":
		return "linux", nil
	default:
		return "", fmt.Errorf("unsupported remote OS %q", uname)
	}
}

func mapRemoteUnameToGoArch(uname string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(uname)) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	case "armv7l", "armv6l", "arm":
		return "arm", nil
	case "i386", "i686", "386":
		return "386", nil
	default:
		return "", fmt.Errorf("unsupported remote architecture %q", uname)
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

func withArcClient(addr string, fn func(*ssh.Client) error) error {
	client, err := dialArcWithKey(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	return fn(client)
}

func readRemoteOSID(client *ssh.Client) (string, error) {
	out, err := runRemoteCommand(client, "cat /etc/os-release", false, "")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(parseOSRelease(out)["ID"]), nil
}

func installRemotePackages(client *ssh.Client, osID string, debianPkgs, archPkgs []string) error {
	switch osID {
	case "ubuntu", "debian":
		if _, err := runRemoteCommand(client, "sudo -n apt-get update", false, ""); err != nil {
			return err
		}
		if len(debianPkgs) == 0 {
			return nil
		}
		_, err := runRemoteCommand(client, "sudo -n apt-get install -y "+strings.Join(debianPkgs, " "), false, "")
		return err
	case "arch", "manjaro":
		if len(archPkgs) == 0 {
			return nil
		}
		_, err := runRemoteCommand(client, "sudo -n pacman -Sy --noconfirm "+strings.Join(archPkgs, " "), false, "")
		return err
	default:
		return fmt.Errorf("unsupported remote OS ID=%q (supported: ubuntu, debian, arch, manjaro)", osID)
	}
}

func installLocalPackages(osID string, debianPkgs, archPkgs []string) error {
	switch osID {
	case "ubuntu", "debian":
		if _, err := execLocal("sudo", "-n", "apt-get", "update"); err != nil {
			return err
		}
		if len(debianPkgs) == 0 {
			return nil
		}
		args := append([]string{"-n", "apt-get", "install", "-y"}, debianPkgs...)
		_, err := execLocal("sudo", args...)
		return err
	case "arch", "manjaro":
		if len(archPkgs) == 0 {
			return nil
		}
		args := append([]string{"-n", "pacman", "-Sy", "--noconfirm"}, archPkgs...)
		_, err := execLocal("sudo", args...)
		return err
	default:
		return fmt.Errorf("unsupported local OS ID=%q (supported: ubuntu, debian, arch, manjaro)", osID)
	}
}

func arcConfigPaths() (configDir, systemdDir, localBinDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", "", "", fmt.Errorf("cannot resolve home dir")
	}
	return filepath.Join(home, ".config", "arc"),
		filepath.Join(home, ".config", "systemd", "user"),
		filepath.Join(home, ".local", "bin"),
		nil
}

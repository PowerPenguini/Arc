package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	aptLockRetryAttempts = 30
	aptLockRetryDelay    = 2 * time.Second
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
		if _, err := runRemoteAPTCommand(client, "sudo -n apt-get update"); err != nil {
			return err
		}
		if len(debianPkgs) == 0 {
			return nil
		}
		_, err := runRemoteAPTCommand(client, "sudo -n apt-get install -y "+strings.Join(debianPkgs, " "))
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
		if _, err := runLocalAPTCommand("sudo", "-n", "apt-get", "update"); err != nil {
			return err
		}
		if len(debianPkgs) == 0 {
			return nil
		}
		args := append([]string{"-n", "apt-get", "install", "-y"}, debianPkgs...)
		_, err := runLocalAPTCommand("sudo", args...)
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

func isAPTLockError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, needle := range []string{
		"Could not get lock /var/lib/dpkg/lock-frontend",
		"Could not get lock /var/lib/dpkg/lock",
		"Could not get lock /var/lib/apt/lists/lock",
		"Unable to acquire the dpkg frontend lock",
		"Unable to lock directory /var/lib/apt/lists/",
		"unattended-upgr",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func runWithAPTRetry(run func() (string, error)) (string, error) {
	var out string
	var err error
	for attempt := 1; attempt <= aptLockRetryAttempts; attempt++ {
		out, err = run()
		if err == nil || !isAPTLockError(err) || attempt == aptLockRetryAttempts {
			return out, err
		}
		time.Sleep(aptLockRetryDelay)
	}
	return out, err
}

func runRemoteAPTCommand(client *ssh.Client, cmd string) (string, error) {
	return runWithAPTRetry(func() (string, error) {
		return runRemoteCommand(client, cmd, false, "")
	})
}

func runLocalAPTCommand(name string, args ...string) (string, error) {
	return runWithAPTRetry(func() (string, error) {
		return execLocal(name, args...)
	})
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

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type knownHostTarget struct {
	Host string
	Port string
}

func syncLocalKnownHostsForBootstrap(host, addr string) error {
	targets := knownHostTargetsForAddr(addr, host)
	return syncLocalKnownHosts(execLocal, targets...)
}

func syncLocalKnownHostsForArcRemote() error {
	targets := knownHostTargetsForAddr(net.JoinHostPort(wgServerIP, "22"), "remotehost", "rh")
	return syncLocalKnownHosts(execLocal, targets...)
}

func syncLocalKnownHosts(execFn localExecFunc, targets ...knownHostTarget) error {
	knownHostsPath, err := ensureLocalKnownHostsFile()
	if err != nil {
		return err
	}

	for _, target := range targets {
		for _, stale := range knownHostRemovalKeys(target) {
			_, _ = execFn("ssh-keygen", "-R", stale, "-f", knownHostsPath)
		}
	}

	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts for append: %w", err)
	}
	defer f.Close()

	for _, target := range targets {
		args := []string{"-T", "5"}
		if target.Port != "" && target.Port != "22" {
			args = append(args, "-p", target.Port)
		}
		args = append(args, target.Host)

		out, err := execFn("ssh-keyscan", args...)
		if err != nil {
			return fmt.Errorf("scan SSH host key for %s: %w", target.Host, err)
		}

		out = strings.TrimSpace(out)
		if out == "" {
			return fmt.Errorf("scan SSH host key for %s returned no keys", target.Host)
		}
		if _, err := f.WriteString(out + "\n"); err != nil {
			return fmt.Errorf("append known_hosts entry for %s: %w", target.Host, err)
		}
	}

	return nil
}

func ensureLocalKnownHostsFile() (string, error) {
	sshDir := userSSHDir()
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("create ssh dir: %w", err)
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("chmod ssh dir: %w", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	f, err := os.OpenFile(knownHostsPath, os.O_CREATE, 0o600)
	if err != nil {
		return "", fmt.Errorf("create known_hosts: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close known_hosts: %w", err)
	}
	if err := os.Chmod(knownHostsPath, 0o600); err != nil {
		return "", fmt.Errorf("chmod known_hosts: %w", err)
	}
	return knownHostsPath, nil
}

func knownHostTargetsForAddr(addr string, aliases ...string) []knownHostTarget {
	host, port := splitKnownHostAddr(addr)
	seen := map[string]struct{}{}
	targets := make([]knownHostTarget, 0, len(aliases)+1)

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := name + "|" + port
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		targets = append(targets, knownHostTarget{Host: name, Port: port})
	}

	add(host)
	for _, alias := range aliases {
		add(alias)
	}
	return targets
}

func splitKnownHostAddr(addr string) (host, port string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", "22"
	}

	if h, p, err := net.SplitHostPort(addr); err == nil {
		h = strings.Trim(strings.TrimSpace(h), "[]")
		if h == "" {
			return "", "22"
		}
		if p == "" {
			p = "22"
		}
		return h, p
	}

	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = strings.Trim(addr, "[]")
	}
	return addr, "22"
}

func knownHostRemovalKeys(target knownHostTarget) []string {
	host := strings.TrimSpace(target.Host)
	port := strings.TrimSpace(target.Port)
	if host == "" {
		return nil
	}
	if port == "" {
		port = "22"
	}

	keys := []string{host}
	bracketed := "[" + host + "]:" + port
	if bracketed != host {
		keys = append(keys, bracketed)
	}
	return keys
}

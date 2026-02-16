package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

func parseOSRelease(raw string) map[string]string {
	out := map[string]string{}
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"`)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func localOSID() (string, error) {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}
	m := parseOSRelease(string(b))
	id := strings.TrimSpace(m["ID"])
	if id == "" {
		return "", fmt.Errorf("missing ID in /etc/os-release")
	}
	return id, nil
}

func execLocal(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		if out == "" {
			return "", err
		}
		return out, fmt.Errorf("%w (%s)", err, out)
	}
	return out, nil
}

func kernelRel() (string, error) {
	return execLocal("uname", "-r")
}

func kernelPkgBase(krel string) string {
	if strings.TrimSpace(krel) == "" {
		return ""
	}
	p := filepath.Join("/usr/lib/modules", krel, "pkgbase")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func manjaroHeadersPkg(kernel string) string {
	// Example: 6.6.11-1-MANJARO -> linux66-headers
	// Example: 6.10.1-3-MANJARO -> linux610-headers
	parts := strings.SplitN(kernel, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	maj, err1 := strconv.Atoi(strings.TrimLeftFunc(parts[0], func(r rune) bool { return r < '0' || r > '9' }))
	minStr := ""
	for _, ch := range parts[1] {
		if ch < '0' || ch > '9' {
			break
		}
		minStr += string(ch)
	}
	min, err2 := strconv.Atoi(minStr)
	if err1 != nil || err2 != nil {
		return ""
	}
	return fmt.Sprintf("linux%d%d-headers", maj, min)
}

func ensureWireGuardKernelLocal(osid string) error {
	// First try loading the module (or no-op if built-in).
	if _, err := execLocal("sudo", "-n", "modprobe", "wireguard"); err == nil {
		return nil
	}

	krel, _ := kernelRel()
	pkgbase := kernelPkgBase(krel)
	var installLog []string

	try := func(name string, args ...string) {
		out, err := execLocal(name, args...)
		if err != nil {
			installLog = append(installLog, fmt.Sprintf("$ %s %s\n%s\nERR: %v", name, strings.Join(args, " "), out, err))
			return
		}
		if out != "" {
			installLog = append(installLog, fmt.Sprintf("$ %s %s\n%s", name, strings.Join(args, " "), out))
		} else {
			installLog = append(installLog, fmt.Sprintf("$ %s %s\n(ok)", name, strings.Join(args, " ")))
		}
	}

	switch osid {
	case "ubuntu":
		// On Ubuntu, WireGuard is typically built-in, but linux-modules-extra may be required on some flavors.
		try("sudo", "-n", "apt-get", "install", "-y", "linux-modules-extra-"+krel)
		try("sudo", "-n", "apt-get", "install", "-y", "wireguard-dkms", "linux-headers-"+krel)
	case "debian":
		try("sudo", "-n", "apt-get", "install", "-y", "wireguard-dkms", "linux-headers-"+krel)
	case "arch":
		// Most Arch kernels include WireGuard, but allow DKMS fallback for custom kernels.
		try("sudo", "-n", "pacman", "-Sy", "--noconfirm", "dkms", "wireguard-dkms")
		var hdrCandidates []string
		if pkgbase != "" {
			hdrCandidates = append(hdrCandidates, pkgbase+"-headers")
		}
		if h := manjaroHeadersPkg(krel); h != "" {
			hdrCandidates = append(hdrCandidates, h)
		}
		hdrCandidates = append(hdrCandidates, "linux-headers")
		for _, h := range hdrCandidates {
			out, err := execLocal("sudo", "-n", "pacman", "-Sy", "--noconfirm", h)
			if err != nil {
				installLog = append(installLog, fmt.Sprintf("$ sudo -n pacman -Sy --noconfirm %s\n%s\nERR: %v", h, out, err))
				continue
			}
			if out != "" {
				installLog = append(installLog, fmt.Sprintf("$ sudo -n pacman -Sy --noconfirm %s\n%s", h, out))
			} else {
				installLog = append(installLog, fmt.Sprintf("$ sudo -n pacman -Sy --noconfirm %s\n(ok)", h))
			}
			break
		}
	case "manjaro":
		try("sudo", "-n", "pacman", "-Sy", "--noconfirm", "dkms", "wireguard-dkms")
		var hdrCandidates []string
		if pkgbase != "" {
			hdrCandidates = append(hdrCandidates, pkgbase+"-headers")
		}
		if h := manjaroHeadersPkg(krel); h != "" {
			hdrCandidates = append(hdrCandidates, h)
		}
		// Explicitly try linuxXX-headers style even if pkgbase probing fails.
		hdrCandidates = append(hdrCandidates, "linux-headers")
		for _, h := range hdrCandidates {
			out, err := execLocal("sudo", "-n", "pacman", "-Sy", "--noconfirm", h)
			if err != nil {
				installLog = append(installLog, fmt.Sprintf("$ sudo -n pacman -Sy --noconfirm %s\n%s\nERR: %v", h, out, err))
				continue
			}
			if out != "" {
				installLog = append(installLog, fmt.Sprintf("$ sudo -n pacman -Sy --noconfirm %s\n%s", h, out))
			} else {
				installLog = append(installLog, fmt.Sprintf("$ sudo -n pacman -Sy --noconfirm %s\n(ok)", h))
			}
			break
		}
	default:
		// Unsupported OS handled elsewhere.
	}

	if _, err := execLocal("sudo", "-n", "depmod", "-a"); err != nil {
		installLog = append(installLog, fmt.Sprintf("$ sudo -n depmod -a\nERR: %v", err))
	}

	if _, err := execLocal("sudo", "-n", "modprobe", "wireguard"); err != nil {
		details := ""
		if len(installLog) > 0 {
			details = "\n\ninstall log:\n" + strings.Join(installLog, "\n\n")
		}
		return fmt.Errorf("wireguard kernel support missing (modprobe wireguard failed): %v%s", err, details)
	}
	return nil
}

func ensureWireGuardKernelRemote(client *ssh.Client, osid string) error {
	// Remote: best-effort modprobe + dkms/headers fallback.
	if _, err := runRemoteCommand(client, "sudo -n modprobe wireguard", false, ""); err == nil {
		return nil
	}

	script := "set -eu\nkrel=\"$(uname -r)\"\n"
	switch osid {
	case "ubuntu":
		script += "sudo -n apt-get install -y \"linux-modules-extra-${krel}\" || true\n"
		script += "sudo -n apt-get install -y wireguard-dkms \"linux-headers-${krel}\" || true\n"
	case "debian":
		script += "sudo -n apt-get install -y wireguard-dkms \"linux-headers-${krel}\" || true\n"
	}
	script += "sudo -n modprobe wireguard\n"
	if _, err := runRemoteCommand(client, script, false, ""); err != nil {
		return fmt.Errorf("wireguard kernel support missing on remote (modprobe wireguard failed): %v", err)
	}
	return nil
}

func ensureDir0700(path string) error {
	return os.MkdirAll(path, 0o700)
}

func writeFile0600(path string, data []byte) error {
	if err := ensureDir0700(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

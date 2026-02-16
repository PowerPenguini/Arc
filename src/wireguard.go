package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ssh"
)

const (
	wgInterface = "wg0"
	wgPort      = 51820

	wgServerCIDR = "10.0.0.1/32"
	wgClientCIDR = "10.0.0.2/32"
	wgServerIP   = "10.0.0.1"
)

type wgConfig struct {
	ServerPriv string
	ServerPub  string
	ClientPriv string
	ClientPub  string

	ServerConf string
	ClientConf string
	Endpoint   string
}

func buildWGConfig(endpointHost string) (wgConfig, error) {
	host := strings.TrimSpace(endpointHost)
	if host == "" {
		return wgConfig{}, fmt.Errorf("missing host for WireGuard endpoint")
	}

	sPriv, sPub, err := genWGKeyPair()
	if err != nil {
		return wgConfig{}, err
	}
	cPriv, cPub, err := genWGKeyPair()
	if err != nil {
		return wgConfig{}, err
	}

	endpoint := fmt.Sprintf("%s:%d", host, wgPort)
	serverConf := strings.Join([]string{
		"[Interface]",
		"Address = " + wgServerCIDR,
		fmt.Sprintf("ListenPort = %d", wgPort),
		"PrivateKey = " + sPriv,
		"",
		"[Peer]",
		"PublicKey = " + cPub,
		"AllowedIPs = " + strings.Split(wgClientCIDR, "/")[0] + "/32",
		"",
	}, "\n")

	clientConf := strings.Join([]string{
		"[Interface]",
		"Address = " + wgClientCIDR,
		"PrivateKey = " + cPriv,
		"",
		"[Peer]",
		"PublicKey = " + sPub,
		"Endpoint = " + endpoint,
		"AllowedIPs = " + wgServerIP + "/32",
		"PersistentKeepalive = 25",
		"",
	}, "\n")

	return wgConfig{
		ServerPriv: sPriv,
		ServerPub:  sPub,
		ClientPriv: cPriv,
		ClientPub:  cPub,
		ServerConf: serverConf,
		ClientConf: clientConf,
		Endpoint:   endpoint,
	}, nil
}

func genWGKeyPair() (privB64, pubB64 string, err error) {
	var priv [32]byte
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return "", "", err
	}
	// Clamp (per X25519).
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(priv[:]), base64.StdEncoding.EncodeToString(pub), nil
}

func wgPublicKeyFromPrivateKeyB64(privB64 string) (string, error) {
	privRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privB64))
	if err != nil {
		return "", fmt.Errorf("decode wg private key: %w", err)
	}
	if len(privRaw) != 32 {
		return "", fmt.Errorf("wg private key must be 32 bytes, got %d", len(privRaw))
	}

	// Clamp (per X25519). Some inputs are already clamped, but do it defensively.
	var priv [32]byte
	copy(priv[:], privRaw)
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive wg public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

func parseWGPrivateKeyFromConf(conf string) (string, error) {
	section := ""
	for _, ln := range strings.Split(conf, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, ";") {
			continue
		}
		if strings.HasPrefix(ln, "[") && strings.HasSuffix(ln, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(ln, "["), "]")
			section = strings.TrimSpace(section)
			continue
		}
		if !strings.EqualFold(section, "Interface") {
			continue
		}
		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if strings.EqualFold(k, "PrivateKey") && v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("wg conf missing [Interface] PrivateKey")
}

func patchWGPeerInConf(conf string, matchAllowedIPs string, setPeerPublicKey string, ensureEndpoint string, ensureKeepalive string) (string, bool, error) {
	lines := strings.Split(conf, "\n")

	allowedContains := func(allowedRaw, want string) bool {
		want = strings.TrimSpace(want)
		if want == "" {
			return false
		}
		// AllowedIPs may be a comma-separated list.
		for _, part := range strings.Split(allowedRaw, ",") {
			if strings.TrimSpace(part) == want {
				return true
			}
		}
		return false
	}

	type peerBlock struct {
		startIdx int
		endIdx   int // exclusive
		pubIdx   int
		allowIdx int
		endpIdx  int
		keepIdx  int
		allowed  string
	}

	section := ""
	var blocks []peerBlock
	var cur *peerBlock

	flushCur := func(end int) {
		if cur == nil {
			return
		}
		cur.endIdx = end
		blocks = append(blocks, *cur)
		cur = nil
	}

	for i := 0; i <= len(lines); i++ {
		ln := ""
		if i < len(lines) {
			ln = strings.TrimSpace(lines[i])
		}

		if i == len(lines) {
			flushCur(i)
			break
		}

		if strings.HasPrefix(ln, "[") && strings.HasSuffix(ln, "]") {
			flushCur(i)
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(ln, "["), "]"))
			if strings.EqualFold(section, "Peer") {
				cur = &peerBlock{
					startIdx: i,
					endIdx:   -1,
					pubIdx:   -1,
					allowIdx: -1,
					endpIdx:  -1,
					keepIdx:  -1,
				}
			}
			continue
		}

		if cur == nil || !strings.EqualFold(section, "Peer") {
			continue
		}

		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch {
		case strings.EqualFold(k, "PublicKey"):
			cur.pubIdx = i
		case strings.EqualFold(k, "AllowedIPs"):
			cur.allowIdx = i
			cur.allowed = v
		case strings.EqualFold(k, "Endpoint"):
			cur.endpIdx = i
		case strings.EqualFold(k, "PersistentKeepalive"):
			cur.keepIdx = i
		}
	}

	if len(blocks) == 0 {
		return conf, false, fmt.Errorf("wg conf missing [Peer] section")
	}

	// Pick the peer that routes the address we care about.
	targetIdx := -1
	want := strings.TrimSpace(matchAllowedIPs)
	for i, b := range blocks {
		if allowedContains(b.allowed, want) {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		// Fallback: patch the first peer (common for arc-generated configs).
		targetIdx = 0
	}
	b := blocks[targetIdx]

	changed := false
	insertAt := b.startIdx + 1

	// Ensure PublicKey
	pubLine := "PublicKey = " + strings.TrimSpace(setPeerPublicKey)
	if b.pubIdx >= 0 {
		if strings.TrimSpace(lines[b.pubIdx]) != pubLine {
			lines[b.pubIdx] = pubLine
			changed = true
		}
	} else {
		lines = append(lines[:insertAt], append([]string{pubLine}, lines[insertAt:]...)...)
		changed = true
		// Adjust indices after insertion.
		if b.allowIdx >= insertAt {
			b.allowIdx++
		}
		if b.endpIdx >= insertAt {
			b.endpIdx++
		}
		if b.keepIdx >= insertAt {
			b.keepIdx++
		}
	}

	// Ensure AllowedIPs
	if want != "" {
		allowLine := "AllowedIPs = " + want
		if b.allowIdx >= 0 {
			if strings.TrimSpace(lines[b.allowIdx]) != allowLine {
				lines[b.allowIdx] = allowLine
				changed = true
			}
		} else {
			lines = append(lines[:insertAt], append([]string{allowLine}, lines[insertAt:]...)...)
			changed = true
			if b.endpIdx >= insertAt {
				b.endpIdx++
			}
			if b.keepIdx >= insertAt {
				b.keepIdx++
			}
		}
	}

	// Ensure Endpoint (client)
	if strings.TrimSpace(ensureEndpoint) != "" {
		endpLine := "Endpoint = " + strings.TrimSpace(ensureEndpoint)
		if b.endpIdx >= 0 {
			if strings.TrimSpace(lines[b.endpIdx]) != endpLine {
				lines[b.endpIdx] = endpLine
				changed = true
			}
		} else {
			lines = append(lines[:insertAt], append([]string{endpLine}, lines[insertAt:]...)...)
			changed = true
			if b.keepIdx >= insertAt {
				b.keepIdx++
			}
		}
	}

	// Ensure PersistentKeepalive (client)
	if strings.TrimSpace(ensureKeepalive) != "" {
		keepLine := "PersistentKeepalive = " + strings.TrimSpace(ensureKeepalive)
		if b.keepIdx >= 0 {
			if strings.TrimSpace(lines[b.keepIdx]) != keepLine {
				lines[b.keepIdx] = keepLine
				changed = true
			}
		} else {
			lines = append(lines[:insertAt], append([]string{keepLine}, lines[insertAt:]...)...)
			changed = true
		}
	}

	// Keep trailing newline behavior stable (arc-generated configs end with blank line).
	out := strings.Join(lines, "\n")
	return out, changed, nil
}

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

func runInfraStep(m model, index int) error {
	switch index {
	case 0:
		client, err := dialArcWithKey(m.addr)
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := runRemoteCommand(client, "cat /etc/os-release", false, "")
		if err != nil {
			return err
		}
		id := strings.TrimSpace(parseOSRelease(out)["ID"])
		if id != "ubuntu" && id != "debian" {
			return fmt.Errorf("unsupported remote OS ID=%q (supported: ubuntu, debian)", id)
		}
		return nil
	case 1:
		client, err := dialArcWithKey(m.addr)
		if err != nil {
			return err
		}
		defer client.Close()
		_, err = runRemoteCommand(client, "sudo -n apt-get update", false, "")
		if err != nil {
			return err
		}
		if _, err := runRemoteCommand(client, "sudo -n apt-get install -y wireguard wireguard-tools", false, ""); err != nil {
			return err
		}
		out, err := runRemoteCommand(client, "cat /etc/os-release", false, "")
		if err != nil {
			return err
		}
		id := strings.TrimSpace(parseOSRelease(out)["ID"])
		return ensureWireGuardKernelRemote(client, id)
	case 2:
		client, err := dialArcWithKey(m.addr)
		if err != nil {
			return err
		}
		defer client.Close()

		// Ensure we don't keep a previously-running wg0 with stale keys/peers.
		_, _ = runRemoteCommand(client, "sudo -n systemctl stop wg-quick@"+wgInterface+" || true", false, "")

		// Save a user copy too.
		userCopy := fmt.Sprintf(
			"set -eu\ninstall -d -m 0700 ~/.arc/wireguard\ncat > ~/.arc/wireguard/server-%s.conf <<'EOF'\n%sEOF\nchmod 600 ~/.arc/wireguard/server-%s.conf\n",
			wgInterface, m.wg.ServerConf, wgInterface,
		)
		if _, err := runRemoteCommand(client, userCopy, false, ""); err != nil {
			return err
		}

			script := fmt.Sprintf(
				"umask 077\ninstall -d -m 0700 /etc/wireguard\nrm -f /etc/wireguard/%s.conf\ncat > /etc/wireguard/%s.conf <<'EOF'\n%sEOF\nchmod 600 /etc/wireguard/%s.conf\n",
				wgInterface, wgInterface, m.wg.ServerConf, wgInterface,
			)
		cmd := "sudo -n sh -lc " + shSingleQuote(script)
		_, err = runRemoteCommand(client, cmd, false, "")
		return err
	case 3:
		client, err := dialArcWithKey(m.addr)
		if err != nil {
			return err
		}
		defer client.Close()
		script := fmt.Sprintf(`set -eu
if command -v ufw >/dev/null 2>&1; then
	if sudo -n ufw status 2>/dev/null | grep -q 'Status: active'; then
		sudo -n ufw allow %d/udp >/dev/null
	fi
fi
`, wgPort)
		_, err = runRemoteCommand(client, script, false, "")
		return err
	case 4:
		client, err := dialArcWithKey(m.addr)
		if err != nil {
			return err
		}
		defer client.Close()
		// Always restart so we definitely apply the freshly-written config.
		cmd := fmt.Sprintf("sudo -n systemctl enable wg-quick@%s && sudo -n systemctl restart wg-quick@%s && sudo -n systemctl is-active --quiet wg-quick@%s", wgInterface, wgInterface, wgInterface)
		_, err = runRemoteCommand(client, cmd, false, "")
		return err
	case 5:
		id, err := localOSID()
		if err != nil {
			return err
		}
		if id != "ubuntu" && id != "debian" && id != "arch" && id != "manjaro" {
			return fmt.Errorf("unsupported local OS ID=%q (supported: ubuntu, debian, arch, manjaro)", id)
		}
		return nil
	case 6:
		id, err := localOSID()
		if err != nil {
			return err
		}
		switch id {
		case "ubuntu", "debian":
			if _, err := execLocal("sudo", "-n", "apt-get", "update"); err != nil {
				return err
			}
			_, err = execLocal("sudo", "-n", "apt-get", "install", "-y", "wireguard", "wireguard-tools")
			if err != nil {
				return err
			}
			return ensureWireGuardKernelLocal(id)
		case "arch", "manjaro":
			_, err := execLocal("sudo", "-n", "pacman", "-Sy", "--noconfirm", "wireguard-tools")
			if err != nil {
				return err
			}
			return ensureWireGuardKernelLocal(id)
		default:
			return fmt.Errorf("unsupported local OS ID=%q", id)
		}
	case 7:
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return fmt.Errorf("cannot resolve home dir")
		}
		dir := filepath.Join(home, ".arc", "wireguard")
		if err := ensureDir0700(dir); err != nil {
			return err
		}

		clientCopyPath := filepath.Join(dir, "client-"+wgInterface+".conf")
		serverCopyPath := filepath.Join(dir, "server-"+wgInterface+".conf")
		if err := writeFile0600(clientCopyPath, []byte(m.wg.ClientConf)); err != nil {
			return err
		}
		if err := writeFile0600(serverCopyPath, []byte(m.wg.ServerConf)); err != nil {
			return err
		}

		// Ensure we don't keep a previously-running wg0 with stale keys/peers.
		_, _ = execLocal("sudo", "-n", "systemctl", "stop", "wg-quick@"+wgInterface)

		// Install system config via passwordless sudo.
		tmp := filepath.Join(dir, "."+wgInterface+".conf.tmp")
		if err := writeFile0600(tmp, []byte(m.wg.ClientConf)); err != nil {
			return err
		}
		if _, err := execLocal("sudo", "-n", "install", "-d", "-m", "0700", "/etc/wireguard"); err != nil {
			return fmt.Errorf("sudo required to install system config; config saved to %s", clientCopyPath)
		}
		_, _ = execLocal("sudo", "-n", "rm", "-f", "/etc/wireguard/"+wgInterface+".conf")
		if _, err := execLocal("sudo", "-n", "install", "-m", "0600", tmp, "/etc/wireguard/"+wgInterface+".conf"); err != nil {
			return fmt.Errorf("sudo required to install system config; config saved to %s", clientCopyPath)
		}
		_ = os.Remove(tmp)
		return nil
	case 8:
		if _, err := execLocal("sudo", "-n", "systemctl", "enable", "wg-quick@"+wgInterface); err != nil {
			return err
		}
		// Always restart so we definitely apply the freshly-written config.
		if _, err := execLocal("sudo", "-n", "systemctl", "restart", "wg-quick@"+wgInterface); err != nil {
			status, _ := execLocal("sudo", "-n", "systemctl", "status", "--no-pager", "-l", "wg-quick@"+wgInterface)
			journal, _ := execLocal("sudo", "-n", "journalctl", "-u", "wg-quick@"+wgInterface, "-b", "--no-pager", "-n", "120")
			if status != "" {
				if journal != "" {
					return fmt.Errorf("%v; status:\n%s\n\njournal:\n%s", err, status, journal)
				}
				return fmt.Errorf("%v; status:\n%s", err, status)
			}
			return err
		}
		if _, err := execLocal("sudo", "-n", "systemctl", "is-active", "--quiet", "wg-quick@"+wgInterface); err != nil {
			status, _ := execLocal("sudo", "-n", "systemctl", "status", "--no-pager", "-l", "wg-quick@"+wgInterface)
			journal, _ := execLocal("sudo", "-n", "journalctl", "-u", "wg-quick@"+wgInterface, "-b", "--no-pager", "-n", "120")
			if status != "" {
				if journal != "" {
					return fmt.Errorf("%v; status:\n%s\n\njournal:\n%s", err, status, journal)
				}
				return fmt.Errorf("%v; status:\n%s", err, status)
			}
			return err
		}
		return nil
	case 9:
		_, err := execLocal("ping", "-c", "1", "-W", "2", wgServerIP)
		if err == nil {
			return nil
		}

		// Auto-repair: if the tunnel is up but keys are mismatched, fix peer keys on both ends and retry.
		changed, syncErr := autoSyncWireGuardPeerKeys(m)
		if changed {
			if _, rerr := execLocal("ping", "-c", "1", "-W", "2", wgServerIP); rerr == nil {
				return nil
			}
		}

		localDiag, _ := wgDiagLocal()
		remoteDiag, _ := wgDiagRemote(m)
		if syncErr != nil {
			return fmt.Errorf("tunnel verification failed (ping %s): %v\n\nauto-sync error: %v\n\nlocal wg diag:\n%s\n\nremote wg diag:\n%s", wgServerIP, err, syncErr, localDiag, remoteDiag)
		}
		return fmt.Errorf("tunnel verification failed (ping %s): %v\n\nlocal wg diag:\n%s\n\nremote wg diag:\n%s", wgServerIP, err, localDiag, remoteDiag)
	default:
		return fmt.Errorf("unknown infra step index: %d", index)
	}
}

func wgDiagLocal() (string, error) {
	var parts []string
	add := func(label string, cmd ...string) {
		out, err := execLocal(cmd[0], cmd[1:]...)
		if err != nil && out == "" {
			parts = append(parts, fmt.Sprintf("%s: (error: %v)", label, err))
			return
		}
		if out == "" {
			out = "(empty)"
		}
		parts = append(parts, fmt.Sprintf("%s:\n%s", label, out))
	}
	add("wg show", "sudo", "-n", "wg", "show", wgInterface)
	add("latest-handshakes", "sudo", "-n", "wg", "show", wgInterface, "latest-handshakes")
	add("endpoints", "sudo", "-n", "wg", "show", wgInterface, "endpoints")
	add("transfer", "sudo", "-n", "wg", "show", wgInterface, "transfer")
	return strings.Join(parts, "\n\n"), nil
}

func wgDiagRemote(m model) (string, error) {
	client, err := dialArcWithKey(m.addr)
	if err != nil {
		return "", err
	}
	defer client.Close()

	var parts []string
	add := func(label, cmd string) {
		out, err := runRemoteCommand(client, "sudo -n "+cmd, false, "")
		if err != nil && strings.TrimSpace(out) == "" {
			parts = append(parts, fmt.Sprintf("%s: (error: %v)", label, err))
			return
		}
		out = strings.TrimSpace(out)
		if out == "" {
			out = "(empty)"
		}
		parts = append(parts, fmt.Sprintf("%s:\n%s", label, out))
	}
	add("wg show", "wg show "+wgInterface)
	add("latest-handshakes", "wg show "+wgInterface+" latest-handshakes")
	add("endpoints", "wg show "+wgInterface+" endpoints")
	add("transfer", "wg show "+wgInterface+" transfer")
	return strings.Join(parts, "\n\n"), nil
}

func autoSyncWireGuardPeerKeys(m model) (bool, error) {
	// Read local+remote wg0.conf, derive interface public keys from PrivateKey, and ensure each side's
	// peer PublicKey references the other side. This only touches the relevant [Peer] stanza.

	localConf, err := execLocal("sudo", "-n", "cat", "/etc/wireguard/"+wgInterface+".conf")
	if err != nil {
		// Fallback: pull live config.
		localConf, err = execLocal("sudo", "-n", "wg", "showconf", wgInterface)
		if err != nil {
			return false, fmt.Errorf("read local wg config: %v", err)
		}
	}

	client, err := dialArcWithKey(m.addr)
	if err != nil {
		return false, fmt.Errorf("dial remote for wg sync: %w", err)
	}
	defer client.Close()

	remoteConf, err := runRemoteCommand(client, "sudo -n cat /etc/wireguard/"+wgInterface+".conf", false, "")
	if err != nil {
		remoteConf, err = runRemoteCommand(client, "sudo -n wg showconf "+wgInterface, false, "")
		if err != nil {
			return false, fmt.Errorf("read remote wg config: %v", err)
		}
	}

	localPriv, err := parseWGPrivateKeyFromConf(localConf)
	if err != nil {
		return false, fmt.Errorf("parse local wg private key: %w", err)
	}
	remotePriv, err := parseWGPrivateKeyFromConf(remoteConf)
	if err != nil {
		return false, fmt.Errorf("parse remote wg private key: %w", err)
	}

	localPub, err := wgPublicKeyFromPrivateKeyB64(localPriv)
	if err != nil {
		return false, fmt.Errorf("derive local wg public key: %w", err)
	}
	remotePub, err := wgPublicKeyFromPrivateKeyB64(remotePriv)
	if err != nil {
		return false, fmt.Errorf("derive remote wg public key: %w", err)
	}

	// Patch local peer (routes to server IP) to use remote's pubkey.
	localPatched, localChanged, lerr := patchWGPeerInConf(localConf, wgServerIP+"/32", remotePub, m.wg.Endpoint, "25")
	if lerr != nil {
		return false, fmt.Errorf("patch local wg peer: %w", lerr)
	}
	// Patch remote peer (routes to client IP) to use local's pubkey.
	remotePatched, remoteChanged, rerr := patchWGPeerInConf(remoteConf, strings.Split(wgClientCIDR, "/")[0]+"/32", localPub, "", "")
	if rerr != nil {
		return false, fmt.Errorf("patch remote wg peer: %w", rerr)
	}

	if !localChanged && !remoteChanged {
		return false, nil
	}

	home, herr := os.UserHomeDir()
	if herr != nil || home == "" {
		return false, errors.New("cannot resolve home dir")
	}
	dir := filepath.Join(home, ".arc", "wireguard")
	if err := ensureDir0700(dir); err != nil {
		return false, err
	}

	// Local: install updated config.
	tmp := filepath.Join(dir, "."+wgInterface+".conf.sync.tmp")
	if err := writeFile0600(tmp, []byte(localPatched)); err != nil {
		return false, err
	}
	if _, err := execLocal("sudo", "-n", "install", "-m", "0600", tmp, "/etc/wireguard/"+wgInterface+".conf"); err != nil {
		return false, fmt.Errorf("install local wg conf: %w", err)
	}
	_ = os.Remove(tmp)

	// Remote: install updated config.
	script := fmt.Sprintf(
		"umask 077\ninstall -d -m 0700 /etc/wireguard\ncat > /etc/wireguard/%s.conf <<'EOF'\n%sEOF\nchmod 600 /etc/wireguard/%s.conf\n",
		wgInterface, remotePatched, wgInterface,
	)
	cmd := "sudo -n sh -lc " + shSingleQuote(script)
	if _, err := runRemoteCommand(client, cmd, false, ""); err != nil {
		return false, fmt.Errorf("install remote wg conf: %w", err)
	}

	// Restart both ends to apply.
	if _, err := execLocal("sudo", "-n", "systemctl", "restart", "wg-quick@"+wgInterface); err != nil {
		return false, fmt.Errorf("restart local wg: %w", err)
	}
	if _, err := execLocal("sudo", "-n", "systemctl", "is-active", "--quiet", "wg-quick@"+wgInterface); err != nil {
		return false, fmt.Errorf("local wg not active after restart: %w", err)
	}

	if _, err := runRemoteCommand(client, "sudo -n systemctl restart wg-quick@"+wgInterface+" && sudo -n systemctl is-active --quiet wg-quick@"+wgInterface, false, ""); err != nil {
		return false, fmt.Errorf("restart remote wg: %w", err)
	}

	return true, nil
}

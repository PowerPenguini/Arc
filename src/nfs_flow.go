package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	nfsMountTarget = "/home/arc"
	nfsExportsFile = "/etc/exports.d/arc.exports"
)

func nfsClientCIDR() string {
	return strings.Split(wgClientCIDR, "/")[0] + "/32"
}

func nfsClientIP() string {
	return strings.Split(wgClientCIDR, "/")[0]
}

func nfsServerExportSource() string {
	return wgServerIP + ":" + nfsMountTarget
}

func renderArcExports(anonUID, anonGID string) string {
	return fmt.Sprintf("%s %s(rw,sync,all_squash,no_subtree_check,anonuid=%s,anongid=%s,sec=sys)\n", nfsMountTarget, nfsClientCIDR(), strings.TrimSpace(anonUID), strings.TrimSpace(anonGID))
}

func renderArcFstabLine() string {
	opts := []string{
		"rw",
		"soft",
		"noauto",
		"x-systemd.automount",
		"x-systemd.idle-timeout=300",
		"x-systemd.mount-timeout=8s",
		"_netdev",
		"nofail",
		"nfsvers=4.2",
		"proto=tcp",
		"timeo=10",
		"retrans=1",
	}
	return fmt.Sprintf("%s %s nfs4 %s 0 0", nfsServerExportSource(), nfsMountTarget, strings.Join(opts, ","))
}

func upsertFstabEntry(content, mountTarget, entry string) (string, bool, error) {
	lines := strings.Split(content, "\n")
	changed := false
	found := false

	for i, raw := range lines {
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) < 2 {
			continue
		}
		if fields[1] != mountTarget {
			continue
		}
		found = true
		if strings.TrimSpace(raw) != entry {
			lines[i] = entry
			changed = true
		}
	}

	if !found {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, entry)
		changed = true
	}

	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, changed, nil
}

func remoteArcUIDGID(ctx infraRunContext) (string, string, error) {
	client, err := dialArcWithKey(ctx.Addr)
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	remoteUID, err := runRemoteCommand(client, "id -u arc", false, "")
	if err != nil {
		return "", "", fmt.Errorf("resolve remote arc UID: %w", err)
	}
	remoteGID, err := runRemoteCommand(client, "id -g arc", false, "")
	if err != nil {
		return "", "", fmt.Errorf("resolve remote arc GID: %w", err)
	}
	uid := strings.TrimSpace(remoteUID)
	gid := strings.TrimSpace(remoteGID)
	if uid == "" || gid == "" {
		return "", "", fmt.Errorf("resolved empty arc UID/GID on remote")
	}
	return uid, gid, nil
}

func verifyRemoteArcIdentity(ctx infraRunContext) error {
	_, _, err := remoteArcUIDGID(ctx)
	return err
}

func installRemoteNFS(ctx infraRunContext) error {
	client, err := dialArcWithKey(ctx.Addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if _, err := runRemoteCommand(client, "sudo -n apt-get update", false, ""); err != nil {
		return err
	}
	if _, err := runRemoteCommand(client, "sudo -n apt-get install -y nfs-kernel-server", false, ""); err != nil {
		return err
	}
	return nil
}

func configureRemoteArcNFS(ctx infraRunContext) error {
	client, err := dialArcWithKey(ctx.Addr)
	if err != nil {
		return err
	}
	defer client.Close()

	arcUID, arcGID, err := remoteArcUIDGID(ctx)
	if err != nil {
		return err
	}
	exports := renderArcExports(arcUID, arcGID)
	script := fmt.Sprintf(`set -eu
umask 022
sudo -n install -d -m 0755 /etc/exports.d
sudo -n sh -lc 'cat > %s <<"EOF"
%sEOF'
sudo -n exportfs -ra
if sudo -n systemctl list-unit-files | grep -q '^nfs-server\\.service'; then
	sudo -n systemctl enable --now nfs-server
else
	sudo -n systemctl enable --now nfs-kernel-server
fi
if command -v ufw >/dev/null 2>&1; then
	if sudo -n ufw status 2>/dev/null | grep -q 'Status: active'; then
		sudo -n ufw allow in on wg0 proto tcp from %s to any port 2049 >/dev/null
	fi
fi
`, nfsExportsFile, exports, nfsClientIP())

	if _, err := runRemoteCommand(client, script, false, ""); err != nil {
		return fmt.Errorf("configure remote NFS export: %w", err)
	}
	return nil
}

func installLocalNFSClient() error {
	id, err := localOSID()
	if err != nil {
		return err
	}
	switch id {
	case "ubuntu", "debian":
		if _, err := execLocal("sudo", "-n", "apt-get", "update"); err != nil {
			return err
		}
		if _, err := execLocal("sudo", "-n", "apt-get", "install", "-y", "nfs-common"); err != nil {
			return err
		}
		return nil
	case "arch", "manjaro":
		if _, err := execLocal("sudo", "-n", "pacman", "-Sy", "--noconfirm", "nfs-utils"); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported local OS ID=%q (supported: ubuntu, debian, arch, manjaro)", id)
	}
}

func ensureLocalArcMountTarget() error {
	if out, err := execLocal("findmnt", "-n", "-o", "SOURCE,FSTYPE", "-T", nfsMountTarget); err == nil {
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) < 2 {
			return fmt.Errorf("unexpected findmnt output for %s: %q", nfsMountTarget, out)
		}
		// For systemd automount, target can be shown as autofs (systemd-1) until first real access.
		if fields[1] == "autofs" && strings.HasPrefix(fields[0], "systemd-") {
			return nil
		}
		if fields[0] != nfsServerExportSource() || fields[1] != "nfs4" {
			return fmt.Errorf("%s is already mounted as %s (%s), expected %s (nfs4)", nfsMountTarget, fields[0], fields[1], nfsServerExportSource())
		}
		return nil
	}

	if _, err := execLocal("sudo", "-n", "test", "-e", nfsMountTarget); err != nil {
		if _, mkErr := execLocal("sudo", "-n", "install", "-d", "-m", "0755", nfsMountTarget); mkErr != nil {
			return fmt.Errorf("create %s: %w", nfsMountTarget, mkErr)
		}
		return nil
	}

	if _, err := execLocal("sudo", "-n", "test", "-d", nfsMountTarget); err != nil {
		return fmt.Errorf("%s exists but is not a directory", nfsMountTarget)
	}

	out, err := execLocal("sudo", "-n", "sh", "-lc", "if [ -z \"$(ls -A /home/arc 2>/dev/null)\" ]; then echo empty; else echo nonempty; fi")
	if err != nil {
		return fmt.Errorf("inspect %s contents: %w", nfsMountTarget, err)
	}
	if strings.TrimSpace(out) == "nonempty" {
		return fmt.Errorf("%s exists and is not empty; move existing data first, then retry", nfsMountTarget)
	}
	return nil
}

func configureLocalArcAutomount() error {
	if err := ensureLocalArcMountTarget(); err != nil {
		return err
	}

	fstabRaw, err := execLocal("sudo", "-n", "cat", "/etc/fstab")
	if err != nil {
		return fmt.Errorf("read /etc/fstab: %w", err)
	}

	updated, changed, err := upsertFstabEntry(fstabRaw, nfsMountTarget, renderArcFstabLine())
	if err != nil {
		return err
	}
	if changed {
		tmp, err := os.CreateTemp("/tmp", "arc-fstab-*.tmp")
		if err != nil {
			return fmt.Errorf("create temp fstab: %w", err)
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()
		defer os.Remove(tmpPath)

		if err := os.WriteFile(tmpPath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("write temp fstab: %w", err)
		}
		if _, err := execLocal("sudo", "-n", "install", "-m", "0644", tmpPath, "/etc/fstab"); err != nil {
			return fmt.Errorf("update /etc/fstab: %w", err)
		}
	}

	if _, err := execLocal("sudo", "-n", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := execLocal("sudo", "-n", "systemctl", "restart", "home-arc.automount"); err != nil {
		if _, startErr := execLocal("sudo", "-n", "systemctl", "start", "home-arc.automount"); startErr != nil {
			return fmt.Errorf("restart home-arc.automount: %v; start fallback failed: %w", err, startErr)
		}
	}
	return nil
}

func verifyLocalArcNFSMount() error {
	const attempts = 5
	var lastErr error

	verifyOnce := func() error {
		if _, err := execLocal("ls", "-la", nfsMountTarget); err != nil {
			return fmt.Errorf("trigger automount for %s: %w", nfsMountTarget, err)
		}

		// Validate the real NFS mount (not the autofs trigger layer).
		out, err := execLocal("findmnt", "-n", "-t", "nfs4", "-o", "SOURCE,TARGET", "-T", nfsMountTarget)
		if err != nil {
			diag, _ := execLocal("findmnt", "-n", "-o", "SOURCE,FSTYPE,TARGET", "-T", nfsMountTarget)
			if strings.TrimSpace(diag) != "" {
				return fmt.Errorf("nfs4 mount not active for %s (%v); current mount view: %s", nfsMountTarget, err, diag)
			}
			return fmt.Errorf("nfs4 mount not active for %s: %w", nfsMountTarget, err)
		}
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) < 2 {
			return fmt.Errorf("unexpected findmnt output for %s: %q", nfsMountTarget, out)
		}
		if fields[0] != nfsServerExportSource() {
			return fmt.Errorf("unexpected NFS source for %s: got %s want %s", nfsMountTarget, fields[0], nfsServerExportSource())
		}
		if fields[1] != nfsMountTarget {
			return fmt.Errorf("unexpected mount target: got %s want %s", fields[1], nfsMountTarget)
		}
		return nil
	}

	for i := 1; i <= attempts; i++ {
		if err := verifyOnce(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < attempts {
			time.Sleep(time.Duration(1<<(i-1)) * time.Second)
		}
	}

	return fmt.Errorf("verify %s failed after %d attempts with backoff: %w", nfsMountTarget, attempts, lastErr)
}

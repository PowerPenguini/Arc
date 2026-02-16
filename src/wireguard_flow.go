package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type infraRunContext struct {
	Addr string
	Host string
	WG   wgConfig
}

func runInfraStep(ctx infraRunContext, index int) error {
	switch index {
	case 0:
		client, err := dialArcWithKey(ctx.Addr)
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
		client, err := dialArcWithKey(ctx.Addr)
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
		client, err := dialArcWithKey(ctx.Addr)
		if err != nil {
			return err
		}
		defer client.Close()

		// Ensure we don't keep a previously-running wg0 with stale keys/peers.
		_, _ = runRemoteCommand(client, "sudo -n systemctl stop wg-quick@"+wgInterface+" || true", false, "")

		// Save a user copy too.
		userCopy := fmt.Sprintf(
			"set -eu\ninstall -d -m 0700 ~/.arc/wireguard\ncat > ~/.arc/wireguard/server-%s.conf <<'EOF'\n%sEOF\nchmod 600 ~/.arc/wireguard/server-%s.conf\n",
			wgInterface, ctx.WG.ServerConf, wgInterface,
		)
		if _, err := runRemoteCommand(client, userCopy, false, ""); err != nil {
			return err
		}

		script := fmt.Sprintf(
			"umask 077\ninstall -d -m 0700 /etc/wireguard\nrm -f /etc/wireguard/%s.conf\ncat > /etc/wireguard/%s.conf <<'EOF'\n%sEOF\nchmod 600 /etc/wireguard/%s.conf\n",
			wgInterface, wgInterface, ctx.WG.ServerConf, wgInterface,
		)
		cmd := "sudo -n sh -lc " + shSingleQuote(script)
		_, err = runRemoteCommand(client, cmd, false, "")
		return err
	case 3:
		client, err := dialArcWithKey(ctx.Addr)
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
		client, err := dialArcWithKey(ctx.Addr)
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
		if err := writeFile0600(clientCopyPath, []byte(ctx.WG.ClientConf)); err != nil {
			return err
		}
		if err := writeFile0600(serverCopyPath, []byte(ctx.WG.ServerConf)); err != nil {
			return err
		}

		// Ensure we don't keep a previously-running wg0 with stale keys/peers.
		_, _ = execLocal("sudo", "-n", "systemctl", "stop", "wg-quick@"+wgInterface)

		// Install system config via passwordless sudo.
		tmp := filepath.Join(dir, "."+wgInterface+".conf.tmp")
		if err := writeFile0600(tmp, []byte(ctx.WG.ClientConf)); err != nil {
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
		changed, syncErr := autoSyncWireGuardPeerKeys(ctx)
		if changed {
			if _, rerr := execLocal("ping", "-c", "1", "-W", "2", wgServerIP); rerr == nil {
				return nil
			}
		}

		localDiag, _ := wgDiagLocal()
		remoteDiag, _ := wgDiagRemote(ctx)
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

func wgDiagRemote(ctx infraRunContext) (string, error) {
	client, err := dialArcWithKey(ctx.Addr)
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

func autoSyncWireGuardPeerKeys(ctx infraRunContext) (bool, error) {
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

	client, err := dialArcWithKey(ctx.Addr)
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
	localPatched, localChanged, lerr := patchWGPeerInConf(localConf, wgServerIP+"/32", remotePub, ctx.WG.Endpoint, "25")
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

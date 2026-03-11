package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func installServerWireGuard(ctx infraRunContext) error {
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		id, err := readRemoteOSID(client)
		if err != nil {
			return err
		}
		if err := installRemotePackages(client, id,
			[]string{"wireguard", "wireguard-tools"},
			[]string{"wireguard-tools", "nftables"},
		); err != nil {
			return err
		}
		return ensureWireGuardKernelRemote(client, id)
	})
}

func writeServerWireGuardConfig(ctx infraRunContext) error {
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		_, _ = runRemoteCommand(client, "sudo -n systemctl stop wg-quick@"+wgInterface+" || true", false, "")

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
		_, err := runRemoteCommand(client, "sudo -n sh -lc "+shSingleQuote(script), false, "")
		return err
	})
}

func openServerFirewall(ctx infraRunContext) error {
	script := fmt.Sprintf(`set -eu
if command -v ufw >/dev/null 2>&1; then
	if sudo -n ufw status 2>/dev/null | grep -q 'Status: active'; then
		sudo -n ufw allow %d/udp >/dev/null
	fi
fi
`, wgPort)
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		_, err := runRemoteCommand(client, script, false, "")
		return err
	})
}

func enableServerWireGuard(ctx infraRunContext) error {
	cmd := fmt.Sprintf("sudo -n systemctl enable wg-quick@%s && sudo -n systemctl restart wg-quick@%s && sudo -n systemctl is-active --quiet wg-quick@%s", wgInterface, wgInterface, wgInterface)
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		_, err := runRemoteCommand(client, cmd, false, "")
		return err
	})
}

func installLocalWireGuard(_ infraRunContext) error {
	id, err := localOSID()
	if err != nil {
		return err
	}
	if err := installLocalPackages(id, []string{"wireguard", "wireguard-tools"}, []string{"wireguard-tools"}); err != nil {
		return err
	}
	return ensureWireGuardKernelLocal(id)
}

func writeLocalWireGuardConfig(ctx infraRunContext) error {
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

	_, _ = execLocal("sudo", "-n", "systemctl", "stop", "wg-quick@"+wgInterface)

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
}

func enableLocalWireGuard(_ infraRunContext) error {
	unit := "wg-quick@" + wgInterface
	if _, err := execLocal("sudo", "-n", "systemctl", "enable", unit); err != nil {
		return err
	}
	if _, err := execLocal("sudo", "-n", "systemctl", "restart", unit); err != nil {
		return localWGServiceError(unit, err)
	}
	if _, err := execLocal("sudo", "-n", "systemctl", "is-active", "--quiet", unit); err != nil {
		return localWGServiceError(unit, err)
	}
	return nil
}

func localWGServiceError(unit string, cause error) error {
	status, _ := execLocal("sudo", "-n", "systemctl", "status", "--no-pager", "-l", unit)
	journal, _ := execLocal("sudo", "-n", "journalctl", "-u", unit, "-b", "--no-pager", "-n", "120")
	if status == "" {
		return cause
	}
	if journal != "" {
		return fmt.Errorf("%v; status:\n%s\n\njournal:\n%s", cause, status, journal)
	}
	return fmt.Errorf("%v; status:\n%s", cause, status)
}

func verifyTunnelConnectivity(ctx infraRunContext) error {
	_, err := execLocal("ping", "-c", "1", "-W", "2", wgServerIP)
	if err == nil {
		return nil
	}

	changed, syncErr := autoSyncWireGuardPeerKeys(ctx)
	if changed {
		if _, retryErr := execLocal("ping", "-c", "1", "-W", "2", wgServerIP); retryErr == nil {
			return nil
		}
	}

	localDiag, _ := wgDiagLocal()
	remoteDiag, _ := wgDiagRemote(ctx)
	if syncErr != nil {
		return fmt.Errorf("tunnel verification failed (ping %s): %v\n\nauto-sync error: %v\n\nlocal wg diag:\n%s\n\nremote wg diag:\n%s", wgServerIP, err, syncErr, localDiag, remoteDiag)
	}
	return fmt.Errorf("tunnel verification failed (ping %s): %v\n\nlocal wg diag:\n%s\n\nremote wg diag:\n%s", wgServerIP, err, localDiag, remoteDiag)
}

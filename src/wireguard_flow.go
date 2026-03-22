package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	remotePatched, remoteChanged, rerr := patchWGPeerInConf(remoteConf, wgDesktopIP+"/32", localPub, "", "")
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

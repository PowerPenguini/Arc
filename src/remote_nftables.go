package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	lhRedirectNftPath        = "/etc/nftables.d/lh_redirect.nft"
	lhRedirectServiceName    = "arc-lh-redirect-nftable.service"
	lhRedirectServicePath    = "/etc/systemd/system/arc-lh-redirect-nftable.service"
	lhRedirectSysctlConfPath = "/etc/sysctl.d/99-arc-route-localnet.conf"
)

const lhRedirectNftContentTpl = `table ip lh_redirect {
  chain prerouting {
    type nat hook prerouting priority dstnat; policy accept;

    # Expose localhost services over WireGuard by DNATing wg destination to loopback.
    iifname "%s" ip daddr %s dnat to 127.0.0.1
  }
}
`

const lhRedirectServiceContentTpl = `[Unit]
Description=ARC nftables redirect rules
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStartPre=-%s delete table ip lh_redirect
ExecStart=%s -f /etc/nftables.d/lh_redirect.nft
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`

const lhRedirectSysctlContentTpl = `net.ipv4.conf.all.route_localnet=1
net.ipv4.conf.%s.route_localnet=1
`

func ensureRemoteLHRedirectNftablesService(ctx infraRunContext) error {
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
	installCmd := ""
	switch id {
	case "ubuntu", "debian":
		installCmd = "apt-get update\napt-get install -y nftables"
	case "arch", "manjaro":
		installCmd = "pacman -Sy --noconfirm nftables"
	default:
		return fmt.Errorf("unsupported remote OS ID=%q (supported: ubuntu, debian, arch, manjaro)", id)
	}

	if _, err := runRemoteCommand(client, "sudo -n sh -lc "+shSingleQuote("set -eu\n"+installCmd), false, ""); err != nil {
		return err
	}

	nftBin, err := detectRemoteNFTBinary(client)
	if err != nil {
		return err
	}

	nftContent := fmt.Sprintf(lhRedirectNftContentTpl, wgInterface, wgServerIP)
	serviceContent := fmt.Sprintf(lhRedirectServiceContentTpl, nftBin, nftBin)
	sysctlContent := fmt.Sprintf(lhRedirectSysctlContentTpl, wgInterface)

	script := fmt.Sprintf(
		"set -eu\n"+
			"cat > %s <<'EOF'\n%sEOF\n"+
			"chmod 0644 %s\n"+
			"sysctl -w net.ipv4.conf.all.route_localnet=1\n"+
			"sysctl -w net.ipv4.conf.%s.route_localnet=1\n"+
			"sysctl --system >/dev/null\n"+
			"install -d -m 0755 /etc/nftables.d\n"+
			"cat > %s <<'EOF'\n%sEOF\n"+
			"chmod 0644 %s\n"+
			"rm -f /etc/systemd/system/arc-lh-redirect-nftables.service\n"+
			"cat > %s <<'EOF'\n%sEOF\n"+
			"chmod 0644 %s\n",
		lhRedirectSysctlConfPath, sysctlContent, lhRedirectSysctlConfPath, wgInterface,
		lhRedirectNftPath, nftContent, lhRedirectNftPath,
		lhRedirectServicePath, serviceContent, lhRedirectServicePath,
	)
	if _, err := runRemoteCommand(client, "sudo -n sh -lc "+shSingleQuote(script), false, ""); err != nil {
		return err
	}
	if _, err := runRemoteCommand(client, "sudo -n systemctl daemon-reload", false, ""); err != nil {
		return err
	}
	if _, err := runRemoteCommand(client, "sudo -n systemctl enable --now "+lhRedirectServiceName, false, ""); err != nil {
		return err
	}
	if _, err := runRemoteCommand(client, "sudo -n systemctl is-active --quiet "+lhRedirectServiceName, false, ""); err != nil {
		status, _ := runRemoteCommand(client, "sudo -n systemctl status --no-pager -l "+lhRedirectServiceName, false, "")
		journal, _ := runRemoteCommand(client, "sudo -n journalctl -u "+lhRedirectServiceName+" -b --no-pager -n 120", false, "")
		if status != "" {
			if journal != "" {
				return fmt.Errorf("%v; status:\n%s\n\njournal:\n%s", err, status, journal)
			}
			return fmt.Errorf("%v; status:\n%s", err, status)
		}
		return err
	}
	return nil
}

func detectRemoteNFTBinary(client *ssh.Client) (string, error) {
	script := `set -eu
p="$(command -v nft || true)"
if [ -z "$p" ]; then
  for c in /usr/sbin/nft /usr/bin/nft /sbin/nft /bin/nft; do
    if [ -x "$c" ]; then p="$c"; break; fi
  done
fi
[ -x "$p" ] || { echo "nft binary not found"; exit 1; }
printf '%s' "$p"
`
	out, err := runRemoteCommand(client, "sh -lc "+shSingleQuote(script), false, "")
	if err != nil {
		return "", fmt.Errorf("detect remote nft binary: %w", err)
	}
	p := strings.TrimSpace(out)
	if p == "" {
		return "", fmt.Errorf("detect remote nft binary: empty path")
	}
	return p, nil
}

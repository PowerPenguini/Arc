package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

func configureServerZsh(ctx infraRunContext) error {
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		id, err := readRemoteOSID(client)
		if err != nil {
			return err
		}
		if err := installRemotePackages(client, id,
			[]string{"zsh", "waypipe", "libwayland-client0", "wayland-protocols"},
			[]string{"zsh", "waypipe", "wayland"},
		); err != nil {
			return err
		}

		script := fmt.Sprintf(`set -eu
zsh_bin="$(command -v zsh || true)"
[ -n "$zsh_bin" ] || { echo "zsh not found"; exit 1; }
current_shell="$(getent passwd %s | cut -d: -f7)"
[ "$current_shell" = "$zsh_bin" ] && exit 0
sudo -n chsh -s "$zsh_bin" %s || sudo -n usermod -s "$zsh_bin" %s
`, arcUser, arcUser, arcUser)
		_, err = runRemoteCommand(client, script, false, "")
		return err
	})
}

func configureLocalZsh(_ infraRunContext) error {
	id, err := localOSID()
	if err != nil {
		return err
	}
	if err := installLocalPackages(id, []string{"zsh", "waypipe"}, []string{"zsh", "waypipe"}); err != nil {
		return err
	}

	targetUser := strings.TrimSpace(os.Getenv("USER"))
	if targetUser == "" {
		return fmt.Errorf("cannot resolve local username")
	}
	script := fmt.Sprintf(`set -eu
zsh_bin="$(command -v zsh || true)"
[ -n "$zsh_bin" ] || { echo "zsh not found"; exit 1; }
current_shell="$(getent passwd %s | cut -d: -f7)"
[ "$current_shell" = "$zsh_bin" ] && exit 0
sudo -n chsh -s "$zsh_bin" %s || sudo -n usermod -s "$zsh_bin" %s
`, shSingleQuote(targetUser), shSingleQuote(targetUser), shSingleQuote(targetUser))
	_, err = execLocal("sh", "-lc", script)
	return err
}

func configureRemoteWaypipe(ctx infraRunContext) error {
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		id, err := readRemoteOSID(client)
		if err != nil {
			return err
		}
		if err := installRemotePackages(client, id,
			[]string{"waypipe", "libwayland-client0", "wayland-protocols"},
			[]string{"waypipe", "wayland"},
		); err != nil {
			return err
		}

		script := `set -eu
install -d -m 0700 "$HOME/.config/arc"
install -d -m 0700 "$HOME/.cache/xdg-runtime"
cat > "$HOME/.config/arc/waypipe.env" <<'EOF'
# ARC managed: fallback runtime dir for headless servers using waypipe.
if [ -z "${XDG_RUNTIME_DIR:-}" ] || [ ! -d "${XDG_RUNTIME_DIR:-}" ]; then
	export XDG_RUNTIME_DIR="$HOME/.cache/xdg-runtime"
fi
if [ ! -d "$XDG_RUNTIME_DIR" ]; then
	mkdir -p "$XDG_RUNTIME_DIR"
	chmod 700 "$XDG_RUNTIME_DIR" || true
fi
EOF
chmod 600 "$HOME/.config/arc/waypipe.env"
`
		_, err = runRemoteCommand(client, script, false, "")
		return err
	})
}

func configureLocalWaypipeService() error {
	configDir, systemdDir, localBinDir, err := arcConfigPaths()
	if err != nil {
		return err
	}
	for _, dir := range []string{configDir, systemdDir, localBinDir} {
		if err := ensureDir0700(dir); err != nil {
			return err
		}
	}

	envPath := filepath.Join(configDir, "waypipe-client.env")
	envData := []byte(strings.Join([]string{
		"ARC_REMOTE_USER=arc",
		"ARC_REMOTE_HOSTS=remotehost",
		"ARC_WAYPIPE_DISPLAY=wayland-0",
		"",
	}, "\n"))
	if err := writeFile0600(envPath, envData); err != nil {
		return err
	}

	runnerPath := filepath.Join(localBinDir, "arc-waypipe-forward")
	runner := `#!/bin/sh
set -eu

host="${ARC_REMOTE_HOSTS:-remotehost}"
user="${ARC_REMOTE_USER:-arc}"
display_name="${ARC_WAYPIPE_DISPLAY:-wayland-arc}"
probe_opts="-o BatchMode=yes -o ConnectTimeout=2 -o ConnectionAttempts=1 -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR"
ssh_opts="-q -o LogLevel=QUIET -o ServerAliveInterval=2 -o ServerAliveCountMax=1 -o TCPKeepAlive=yes"
remote_keepalive='sh -lc ". \"$HOME/.config/arc/waypipe.env\" 2>/dev/null || true; while :; do sleep 3600; done"'

while :; do
	if ! ssh $probe_opts "${user}@${host}" true >/dev/null 2>&1; then
		sleep 2
		continue
	fi
	waypipe --display "$display_name" ssh $ssh_opts "${user}@${host}" "$remote_keepalive" || true
	sleep 1
done
`
	if err := os.WriteFile(runnerPath, []byte(runner), 0o700); err != nil {
		return err
	}

	servicePath := filepath.Join(systemdDir, "arc-waypipe.service")
	service, err := renderTemplateFile("templates/arc_waypipe.service.tmpl", map[string]string{})
	if err != nil {
		return err
	}
	if err := os.WriteFile(servicePath, []byte(service), 0o644); err != nil {
		return err
	}

	return activateLocalWaypipeService(execLocal)
}

func activateLocalWaypipeService(execFn localExecFunc) error {
	_, _ = execFn("systemctl", "--user", "import-environment", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS")
	if _, err := execFn("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if _, err := execFn("systemctl", "--user", "enable", "arc-waypipe.service"); err != nil {
		return err
	}
	if _, err := execFn("systemctl", "--user", "restart", "arc-waypipe.service"); err != nil {
		return err
	}
	return nil
}

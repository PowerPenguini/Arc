package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

func configureRemoteClipboardCompositor(ctx infraRunContext) error {
	return withArcClient(ctx.Addr, func(client *ssh.Client) error {
		id, err := readRemoteOSID(client)
		if err != nil {
			return err
		}
		if err := installRemotePackages(client, id,
			[]string{"wl-clipboard", "libwayland-dev", "pkg-config"},
			[]string{"wl-clipboard", "wayland", "pkgconf"},
		); err != nil {
			return err
		}

		clipdBinary, err := buildArcClipdBinary()
		if err != nil {
			return err
		}
		if err := uploadRemoteFile(client, ".local/bin/arc-clipd", clipdBinary, 0o700); err != nil {
			return err
		}

		clipdService, err := renderTemplateFile("templates/arc_clipd.service.tmpl", map[string]string{})
		if err != nil {
			return err
		}

		script := fmt.Sprintf(`set -eu
install -d -m 0700 "$HOME/.config/arc"
install -d -m 0700 "$HOME/.config/systemd/user"
install -d -m 0700 "$HOME/.local/bin"

cat > "$HOME/.config/arc/clipd.env" <<'EOF'
# ARC managed: runtime and socket for the hidden Wayland clipboard sidecar used by Codex.
if [ -z "${XDG_RUNTIME_DIR:-}" ] || [ ! -d "${XDG_RUNTIME_DIR:-}" ]; then
	export XDG_RUNTIME_DIR="$HOME/.cache/xdg-runtime"
fi
if [ ! -d "$XDG_RUNTIME_DIR" ]; then
	mkdir -p "$XDG_RUNTIME_DIR"
	chmod 700 "$XDG_RUNTIME_DIR" || true
fi
export ARC_CLIPD_DISPLAY=arc-clipd-0
export ARC_CLIPD_SEAT=seat0
EOF
chmod 600 "$HOME/.config/arc/clipd.env"

cat > "$HOME/.local/bin/arc-remote-clipboard-put-image" <<'EOF'
#!/bin/sh
set -eu

. "$HOME/.config/arc/clipd.env" 2>/dev/null || true
display_name="${ARC_CLIPD_DISPLAY:-arc-clipd-0}"
kind="${1:-}"
case "$kind" in
	png) mime='image/png' ;;
	jpeg) mime='image/jpeg' ;;
	webp) mime='image/webp' ;;
	*) echo "unsupported image kind: $kind" >&2; exit 2 ;;
esac
exec "$HOME/.local/bin/arc-clipd" put --socket "$display_name" --type "$mime"
EOF
chmod 700 "$HOME/.local/bin/arc-remote-clipboard-put-image"

cat > "$HOME/.local/bin/codex-wayland" <<'EOF'
#!/bin/sh
set -eu
. "$HOME/.config/arc/clipd.env" 2>/dev/null || true
export WAYLAND_DISPLAY="${ARC_CLIPD_DISPLAY:-arc-clipd-0}"
unset DISPLAY
export XDG_SESSION_TYPE=wayland
export OZONE_PLATFORM=wayland
export ELECTRON_OZONE_PLATFORM_HINT=wayland

if [ -n "${XDG_RUNTIME_DIR:-}" ] && [ ! -d "$XDG_RUNTIME_DIR" ]; then
	mkdir -p "$XDG_RUNTIME_DIR"
	chmod 700 "$XDG_RUNTIME_DIR" || true
fi

if command -v systemctl >/dev/null 2>&1; then
	if ! systemctl --user is-active --quiet arc-clipd.service; then
		systemctl --user restart arc-clipd.service >/dev/null 2>&1 || true
	fi
fi

socket_path="${XDG_RUNTIME_DIR:-$HOME/.cache/xdg-runtime}/${WAYLAND_DISPLAY}"
try=0
while [ ! -S "$socket_path" ] && [ "$try" -lt 20 ]; do
	try=$((try + 1))
	sleep 0.1
done

if command -v codex >/dev/null 2>&1; then
	exec codex "$@"
fi
if [ -x "$HOME/.bun/bin/codex" ]; then
	exec "$HOME/.bun/bin/codex" "$@"
fi

echo "codex-wayland: codex binary not found in PATH or ~/.bun/bin/codex" >&2
exit 127
EOF
chmod 700 "$HOME/.local/bin/codex-wayland"

cat > "$HOME/.config/systemd/user/arc-clipd.service" <<'EOF'
%sEOF
chmod 644 "$HOME/.config/systemd/user/arc-clipd.service"
`, clipdService)
		if _, err := runRemoteCommand(client, script, false, ""); err != nil {
			return err
		}
		return activateRemoteClipboardCompositor(client)
	})
}

func activateRemoteClipboardCompositor(client *ssh.Client) error {
	for _, cmd := range []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable arc-clipd.service",
		"systemctl --user restart arc-clipd.service",
	} {
		if _, err := runRemoteCommand(client, cmd, false, ""); err != nil {
			return err
		}
	}
	return nil
}

func configureLocalImageClipboardSync() error {
	id, err := localOSID()
	if err != nil {
		return err
	}
	if err := installLocalPackages(id, []string{"wl-clipboard"}, []string{"wl-clipboard"}); err != nil {
		return err
	}

	configDir, systemdDir, localBinDir, err := arcConfigPaths()
	if err != nil {
		return err
	}
	for _, dir := range []string{configDir, systemdDir, localBinDir} {
		if err := ensureDir0700(dir); err != nil {
			return err
		}
	}

	envPath := filepath.Join(configDir, "clipboard-sync.env")
	envData := []byte(strings.Join([]string{
		"ARC_REMOTE_USER=arc",
		"ARC_REMOTE_HOSTS=remotehost",
		"ARC_REMOTE_CLIPBOARD_DISPLAY=arc-clipd-0",
		"ARC_CLIPBOARD_POLL_SECONDS=2",
		"",
	}, "\n"))
	if err := writeFile0600(envPath, envData); err != nil {
		return err
	}

	runnerPath := filepath.Join(localBinDir, "arc-clipboard-sync")
	runner := `#!/bin/sh
set -eu

host="${ARC_REMOTE_HOSTS:-remotehost}"
user="${ARC_REMOTE_USER:-arc}"
display_name="${ARC_REMOTE_CLIPBOARD_DISPLAY:-arc-clipd-0}"
poll_seconds="${ARC_CLIPBOARD_POLL_SECONDS:-2}"
probe_opts="-o BatchMode=yes -o ConnectTimeout=2 -o ConnectionAttempts=1 -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR"
ssh_opts="-q -o LogLevel=QUIET -o ServerAliveInterval=2 -o ServerAliveCountMax=1 -o TCPKeepAlive=yes"
last_hash=''

pick_image_kind() {
	types="$(wl-paste --list-types 2>/dev/null || true)"
	printf '%s\n' "$types" | grep -Fx 'image/png' >/dev/null 2>&1 && { printf '%s\n' png; return 0; }
	printf '%s\n' "$types" | grep -Fx 'image/jpeg' >/dev/null 2>&1 && { printf '%s\n' jpeg; return 0; }
	printf '%s\n' "$types" | grep -Fx 'image/webp' >/dev/null 2>&1 && { printf '%s\n' webp; return 0; }
	return 1
}

while :; do
	if [ -z "${WAYLAND_DISPLAY:-}" ]; then
		sleep "$poll_seconds"
		continue
	fi
	if ! command -v wl-paste >/dev/null 2>&1; then
		echo "arc-clipboard-sync: wl-paste missing" >&2
		sleep "$poll_seconds"
		continue
	fi
	kind="$(pick_image_kind || true)"
	if [ -z "$kind" ]; then
		last_hash=''
		sleep "$poll_seconds"
		continue
	fi
	tmp="$(mktemp)"
	if ! wl-paste --no-newline --type "image/$kind" >"$tmp" 2>/dev/null; then
		rm -f "$tmp"
		sleep "$poll_seconds"
		continue
	fi
	hash="$(sha256sum "$tmp" | awk '{print $1}')"
	if [ "$hash" = "$last_hash" ]; then
		rm -f "$tmp"
		sleep "$poll_seconds"
		continue
	fi
	sent=0
	if ssh $probe_opts "${user}@${host}" true >/dev/null 2>&1; then
		if ssh $ssh_opts "${user}@${host}" "ARC_CLIPD_DISPLAY='$display_name' ~/.local/bin/arc-remote-clipboard-put-image '$kind'" <"$tmp"; then
			last_hash="$hash"
			sent=1
		fi
	fi
	rm -f "$tmp"
	if [ "$sent" -eq 0 ]; then
		echo "arc-clipboard-sync: could not reach remote host or wl-copy failed" >&2
	fi
	sleep "$poll_seconds"
done
`
	if err := os.WriteFile(runnerPath, []byte(runner), 0o700); err != nil {
		return err
	}

	servicePath := filepath.Join(systemdDir, "arc-clipboard-sync.service")
	service, err := renderTemplateFile("templates/arc_clipboard_sync.service.tmpl", map[string]string{})
	if err != nil {
		return err
	}
	if err := os.WriteFile(servicePath, []byte(service), 0o644); err != nil {
		return err
	}

	return activateLocalClipboardSyncService(execLocal)
}

func activateLocalClipboardSyncService(execFn localExecFunc) error {
	_, _ = execFn("systemctl", "--user", "import-environment", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS")
	if _, err := execFn("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if _, err := execFn("systemctl", "--user", "enable", "arc-clipboard-sync.service"); err != nil {
		return err
	}
	if _, err := execFn("systemctl", "--user", "restart", "arc-clipboard-sync.service"); err != nil {
		return err
	}
	return nil
}

func buildArcClipdBinary() ([]byte, error) {
	clipdDir, err := localClipdProjectDir()
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(clipdDir, "Cargo.toml")
	releasePath := filepath.Join(clipdDir, "target", "release", "arc-clipd")

	var lastErr error
	for _, cmd := range [][]string{
		{"cargo", "build", "--manifest-path", manifestPath, "--release", "--offline"},
		{"cargo", "build", "--manifest-path", manifestPath, "--release"},
	} {
		var stderr bytes.Buffer
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Stderr = &stderr
		c.Stdout = &stderr
		if err := c.Run(); err == nil {
			return os.ReadFile(releasePath)
		} else {
			lastErr = fmt.Errorf("%w (%s)", err, strings.TrimSpace(stderr.String()))
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("build arc-clipd: %w", lastErr)
	}
	return nil, fmt.Errorf("build arc-clipd: unknown error")
}

func localClipdProjectDir() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot resolve source path for clipd build")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
	clipdDir := filepath.Join(repoRoot, "clipd")
	if _, err := os.Stat(filepath.Join(clipdDir, "Cargo.toml")); err != nil {
		return "", fmt.Errorf("cannot resolve clipd project at %s: %w", clipdDir, err)
	}
	return clipdDir, nil
}

type sshRemoteFileSession struct {
	*ssh.Session
}

func (s *sshRemoteFileSession) SetStdin(reader *bytes.Reader) {
	s.Stdin = reader
}

func uploadRemoteFile(client *ssh.Client, remotePath string, data []byte, mode os.FileMode) error {
	return uploadRemoteFileWithSessionFactory(func() (remoteFileSession, error) {
		session, err := client.NewSession()
		if err != nil {
			return nil, err
		}
		return &sshRemoteFileSession{Session: session}, nil
	}, remotePath, data, mode)
}

func uploadRemoteFileWithSessionFactory(factory func() (remoteFileSession, error), remotePath string, data []byte, mode os.FileMode) error {
	session, err := factory()
	if err != nil {
		return fmt.Errorf("cannot open ssh session: %w", err)
	}
	defer session.Close()

	session.SetStdin(bytes.NewReader(data))
	remoteDir := filepath.ToSlash(filepath.Dir(remotePath))
	remotePath = filepath.ToSlash(remotePath)
	tmpPath := remotePath + ".tmp.arc-upload"
	cmd := fmt.Sprintf(
		"/bin/sh -lc %s",
		shSingleQuote(fmt.Sprintf(
			"set -eu\ninstall -d -m 0700 \"$HOME/%s\"\ncat > \"$HOME/%s\"\nchmod %04o \"$HOME/%s\"\nmv -f \"$HOME/%s\" \"$HOME/%s\"\n",
			remoteDir,
			tmpPath,
			mode.Perm(),
			tmpPath,
			tmpPath,
			remotePath,
		)),
	)
	if out, err := session.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("upload %s: %w (%s)", remotePath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

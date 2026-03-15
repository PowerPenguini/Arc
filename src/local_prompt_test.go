package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureLocalArcZshPrompt_CreatesFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := ensureLocalArcZshPrompt(); err != nil {
		t.Fatalf("ensureLocalArcZshPrompt: %v", err)
	}

	rcb, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	rc := string(rcb)
	if !strings.Contains(rc, arcPromptStart) || !strings.Contains(rc, arcPromptEnd) {
		t.Fatalf(".zshrc missing ARC prompt block markers")
	}
	if !strings.Contains(rc, "ARC AUTO SSH (local)") {
		t.Fatalf(".zshrc missing ARC AUTO SSH block")
	}
	if strings.Contains(rc, "pub.remotehost") {
		t.Fatalf(".zshrc should not reference pub.remotehost")
	}
	if !strings.Contains(rc, `HISTFILE="$__arc_state_dir/.zsh_history_local"`) {
		t.Fatalf(".zshrc missing local history fallback")
	}
	if !strings.Contains(rc, "if __arc_vpn_path_healthy; then") || !strings.Contains(rc, "HISTFILE=/home/arc/.zsh_history_shared") {
		t.Fatalf(".zshrc missing conditional shared history selection")
	}
	if !strings.Contains(rc, "setopt SHARE_HISTORY") {
		t.Fatalf(".zshrc missing SHARE_HISTORY option")
	}
}

func TestArcPromptBlocks_ContainSharedHistoryConfig(t *testing.T) {
	for _, block := range []string{arcPromptBlockLocal, arcPromptBlockRemote} {
		if !strings.Contains(block, `export PATH="$HOME/.local/bin:$PATH"`) {
			t.Fatalf("prompt block missing ~/.local/bin PATH bootstrap")
		}
		if !strings.Contains(block, "HISTFILE=/home/arc/.zsh_history_shared") && !strings.Contains(block, `HISTFILE="$__arc_state_dir/.zsh_history_local"`) {
			t.Fatalf("prompt block missing history file configuration")
		}
		if !strings.Contains(block, "setopt SHARE_HISTORY") {
			t.Fatalf("prompt block missing SHARE_HISTORY")
		}
		if !strings.Contains(block, "setopt EXTENDED_HISTORY") {
			t.Fatalf("prompt block missing EXTENDED_HISTORY")
		}
		if !strings.Contains(block, "setopt NO_HIST_SAVE_BY_COPY") {
			t.Fatalf("prompt block missing NO_HIST_SAVE_BY_COPY")
		}
	}

	if !strings.Contains(arcPromptBlockLocal, "if __arc_vpn_path_healthy; then") {
		t.Fatalf("local prompt block missing VPN-gated shared history")
	}
	if strings.Contains(arcPromptBlockLocal, "pub.remotehost") {
		t.Fatalf("local prompt block should not reference pub.remotehost")
	}
	if !strings.Contains(arcPromptBlockLocal, `HISTFILE="$__arc_state_dir/.zsh_history_local"`) {
		t.Fatalf("local prompt block missing local history fallback")
	}
	if !strings.Contains(arcPromptBlockRemote, "HISTFILE=/home/arc/.zsh_history_shared") {
		t.Fatalf("remote prompt block missing shared HISTFILE")
	}
}

func TestArcPromptBlocks_ContainCtrlArrowWordBindings(t *testing.T) {
	for _, block := range []string{arcPromptBlockLocal, arcPromptBlockRemote} {
		if !strings.Contains(block, "bindkey -M \"$__arc_map\" '^[[1;5D' backward-word") {
			t.Fatalf("prompt block missing ctrl-left xterm binding")
		}
		if !strings.Contains(block, "bindkey -M \"$__arc_map\" '^[[5D' backward-word") {
			t.Fatalf("prompt block missing ctrl-left rxvt binding")
		}
		if !strings.Contains(block, "bindkey -M \"$__arc_map\" '^[[1;5C' forward-word") {
			t.Fatalf("prompt block missing ctrl-right xterm binding")
		}
		if !strings.Contains(block, "bindkey -M \"$__arc_map\" '^[[5C' forward-word") {
			t.Fatalf("prompt block missing ctrl-right rxvt binding")
		}
		if !strings.Contains(block, "terminfo[kLFT5]") {
			t.Fatalf("prompt block missing ctrl-left terminfo binding")
		}
		if !strings.Contains(block, "terminfo[kRIT5]") {
			t.Fatalf("prompt block missing ctrl-right terminfo binding")
		}
		if !strings.Contains(block, "zle -N backward-word __arc_fancy_backward_word") {
			t.Fatalf("prompt block missing backward-word widget")
		}
		if !strings.Contains(block, "zle -N forward-word __arc_fancy_forward_word") {
			t.Fatalf("prompt block missing forward-word widget")
		}
	}
}

func TestArcPromptBlocks_ClearVisibleHintBeforeDelete(t *testing.T) {
	for _, block := range []string{arcPromptBlockLocal, arcPromptBlockRemote} {
		if !strings.Contains(block, "__arc_fancy_clear_hint()") {
			t.Fatalf("prompt block missing hint clearing helper")
		}
		if !strings.Contains(block, "__arc_fancy_delete_char() {\n\t# Delete should not accept or edit the visible history hint.\n\t__arc_fancy_clear_hint\n\tzle .delete-char\n\t__arc_fancy_refresh\n}") {
			t.Fatalf("prompt block missing guarded delete-char widget")
		}
	}
}

func TestArcPromptBlockLocal_ContainsWaypipeAutoForwarding(t *testing.T) {
	if !strings.Contains(arcPromptBlockLocal, "WAYLAND_DISPLAY") {
		t.Fatalf("local prompt block missing WAYLAND_DISPLAY detection")
	}
	if !strings.Contains(arcPromptBlockLocal, "command -v waypipe") {
		t.Fatalf("local prompt block missing waypipe availability check")
	}
	if !strings.Contains(arcPromptBlockLocal, "__arc_waypipe_service_name='arc-waypipe.service'") {
		t.Fatalf("local prompt block missing waypipe service name")
	}
	if !strings.Contains(arcPromptBlockLocal, "systemctl --user show-environment") {
		t.Fatalf("local prompt block missing waypipe env health check")
	}
	if !strings.Contains(arcPromptBlockLocal, "__arc_waypipe_systemd_env_matches") {
		t.Fatalf("local prompt block missing waypipe env helper")
	}
	if !strings.Contains(arcPromptBlockLocal, "systemctl --user restart \"$__arc_waypipe_service_name\"") {
		t.Fatalf("local prompt block missing waypipe service restart")
	}
	if !strings.Contains(arcPromptBlockLocal, "__arc_waypipe_ensure_active || true") {
		t.Fatalf("local prompt block missing quiet waypipe activation")
	}
	if strings.Contains(arcPromptBlockLocal, "cannot reach remotehost or pub.remotehost") {
		t.Fatalf("local prompt block should not keep public fallback errors")
	}
	if !strings.Contains(arcPromptBlockLocal, "ARC_WAYPIPE_HINT_ONCE") {
		t.Fatalf("local prompt block missing one-time waypipe hint guard")
	}
	if !strings.Contains(arcPromptBlockLocal, "clip-status()") {
		t.Fatalf("local prompt block missing clipboard sync status helper")
	}
	if !strings.Contains(arcPromptBlockLocal, "arc-clipboard-sync.service") {
		t.Fatalf("local prompt block missing clipboard sync service controls")
	}
}

func TestArcPromptBlockRemote_ContainsWaypipeRuntimeSetup(t *testing.T) {
	if !strings.Contains(arcPromptBlockRemote, "$HOME/.config/arc/waypipe.env") {
		t.Fatalf("remote prompt block missing waypipe env file integration")
	}
	if !strings.Contains(arcPromptBlockRemote, "XDG_RUNTIME_DIR") {
		t.Fatalf("remote prompt block missing XDG_RUNTIME_DIR fallback")
	}
	if !strings.Contains(arcPromptBlockRemote, "\"$XDG_RUNTIME_DIR\"/wayland-*(N) \"$XDG_RUNTIME_DIR\"/waypipe-*(N)") {
		t.Fatalf("remote prompt block missing Wayland socket autodetection")
	}
	if !strings.Contains(arcPromptBlockRemote, "OZONE_PLATFORM=wayland") {
		t.Fatalf("remote prompt block missing Wayland ozone preference")
	}
	if !strings.Contains(arcPromptBlockRemote, "CHROMIUM_FLAGS") {
		t.Fatalf("remote prompt block missing Chromium Wayland flags")
	}
	if !strings.Contains(arcPromptBlockRemote, "clipd-status()") {
		t.Fatalf("remote prompt block missing clipd status helper")
	}
	if !strings.Contains(arcPromptBlockRemote, "codex-wayland") {
		t.Fatalf("remote prompt block missing codex-wayland helper")
	}
	if !strings.Contains(arcPromptBlockRemote, "codex()") {
		t.Fatalf("remote prompt block missing codex wrapper")
	}
}

func TestArcTemplates_ContainClipboardServices(t *testing.T) {
	clipdService, err := renderTemplateFile("templates/arc_clipd.service.tmpl", map[string]string{})
	if err != nil {
		t.Fatalf("render clipd service: %v", err)
	}
	if !strings.Contains(clipdService, "arc-clipd.service") && !strings.Contains(clipdService, "clipboard compositor sidecar") {
		t.Fatalf("clipd service template missing identifying text")
	}
	if !strings.Contains(clipdService, "--socket \"${ARC_CLIPD_DISPLAY:-arc-clipd-0}\"") {
		t.Fatalf("clipd service template missing socket selection")
	}

	syncService, err := renderTemplateFile("templates/arc_clipboard_sync.service.tmpl", map[string]string{})
	if err != nil {
		t.Fatalf("render clipboard sync service: %v", err)
	}
	if !strings.Contains(syncService, "arc-clipboard-sync") {
		t.Fatalf("clipboard sync template missing helper ExecStart")
	}
	if !strings.Contains(syncService, "clipboard-sync.env") {
		t.Fatalf("clipboard sync template missing env file")
	}
}

func TestRemoteWestonProvisioning_ContainsRobustCodexWrapper(t *testing.T) {
	if !strings.Contains(mustTemplateFile("templates/prompt_remote.zsh"), "codex-wayland") {
		t.Fatalf("remote prompt template should expose codex-wayland")
	}
	if !strings.Contains(arcPromptBlockRemote, "clipd-restart()") {
		t.Fatalf("remote prompt block missing clipd restart helper")
	}
	remoteProvisioning := mustTemplateFile("templates/prompt_remote.zsh")
	if !strings.Contains(remoteProvisioning, "codex()") {
		t.Fatalf("remote prompt template should expose codex wrapper")
	}
	if !strings.Contains(remoteProvisioning, "cw()") {
		t.Fatalf("remote prompt template should expose cw alias")
	}
}

func TestRemoteCodexWrapperForcesWaylandSession(t *testing.T) {
	src, err := os.ReadFile("clipboard_flow.go")
	if err != nil {
		t.Fatalf("read clipboard_flow.go: %v", err)
	}
	text := string(src)
	for _, snippet := range []string{
		`unset DISPLAY`,
		`export XDG_SESSION_TYPE=wayland`,
		`export OZONE_PLATFORM=wayland`,
		`export ELECTRON_OZONE_PLATFORM_HINT=wayland`,
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("codex wrapper should contain %q", snippet)
		}
	}
}

func TestArcTmuxBlockRemote_ContainsWaylandEnvPropagation(t *testing.T) {
	if !strings.Contains(arcTmuxBlockRemote, "update-environment") {
		t.Fatalf("remote tmux block missing update-environment setting")
	}
	if !strings.Contains(arcTmuxBlockRemote, "COLORTERM") {
		t.Fatalf("remote tmux block missing COLORTERM propagation")
	}
	if !strings.Contains(arcTmuxBlockRemote, "WAYLAND_DISPLAY") {
		t.Fatalf("remote tmux block missing WAYLAND_DISPLAY propagation")
	}
	if !strings.Contains(arcTmuxBlockRemote, "XDG_RUNTIME_DIR") {
		t.Fatalf("remote tmux block missing XDG_RUNTIME_DIR propagation")
	}
}

func TestArcTmuxBlockRemote_EnablesTruecolor(t *testing.T) {
	if !strings.Contains(arcTmuxBlockRemote, `default-terminal "tmux-256color"`) {
		t.Fatalf("remote tmux block missing tmux-256color default terminal")
	}
	for _, term := range []string{"xterm-256color:RGB", "tmux-256color:RGB", "screen-256color:RGB"} {
		if !strings.Contains(arcTmuxBlockRemote, term) {
			t.Fatalf("remote tmux block missing RGB terminal feature for %s", term)
		}
	}
}

func TestArcPromptBlockLocal_SeedsTruecolorForRemoteTmux(t *testing.T) {
	if !strings.Contains(arcPromptBlockLocal, "env TERM=${__arc_tmux_term} COLORTERM=truecolor tmux new-session -A -D -s ${__arc_tmux_session}") {
		t.Fatalf("local prompt block missing truecolor tmux launch command")
	}
}

func TestEnsureLocalArcZshPrompt_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 2; i++ {
		if err := ensureLocalArcZshPrompt(); err != nil {
			t.Fatalf("ensureLocalArcZshPrompt (run %d): %v", i, err)
		}
	}

	rcb, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	rc := string(rcb)
	if c := strings.Count(rc, arcPromptStart); c != 1 {
		t.Fatalf("expected 1 ARC_PROMPT_START, got %d", c)
	}
	if c := strings.Count(rc, arcPromptEnd); c != 1 {
		t.Fatalf("expected 1 ARC_PROMPT_END, got %d", c)
	}
	if c := strings.Count(rc, "ARC AUTO SSH (local)"); c != 1 {
		t.Fatalf("expected 1 ARC AUTO SSH block, got %d", c)
	}
}

func TestEnsureLocalArcZshPrompt_ReplacesExistingBlockAndPreservesOtherLines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rcPath := filepath.Join(home, ".zshrc")
	initial := strings.Join([]string{
		"line-before",
		arcPromptStart,
		"old prompt content",
		arcPromptEnd,
		"line-after",
		"",
	}, "\n")
	if err := os.WriteFile(rcPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write .zshrc: %v", err)
	}

	if err := ensureLocalArcZshPrompt(); err != nil {
		t.Fatalf("ensureLocalArcZshPrompt: %v", err)
	}

	rcb, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	rc := string(rcb)
	if strings.Contains(rc, "old prompt content") {
		t.Fatalf("expected old prompt content to be removed")
	}
	if !strings.Contains(rc, "line-before") || !strings.Contains(rc, "line-after") {
		t.Fatalf("expected other lines to be preserved")
	}
	if strings.Count(rc, arcPromptStart) != 1 || strings.Count(rc, arcPromptEnd) != 1 {
		t.Fatalf("expected exactly one ARC prompt block after replacement")
	}
}

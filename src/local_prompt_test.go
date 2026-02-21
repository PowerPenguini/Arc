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
	if !strings.Contains(rc, "HISTFILE=/home/arc/.zsh_history_shared") {
		t.Fatalf(".zshrc missing shared history file")
	}
	if !strings.Contains(rc, "setopt SHARE_HISTORY") {
		t.Fatalf(".zshrc missing SHARE_HISTORY option")
	}
}

func TestArcPromptBlocks_ContainSharedHistoryConfig(t *testing.T) {
	for _, block := range []string{arcPromptBlockLocal, arcPromptBlockRemote} {
		if !strings.Contains(block, "HISTFILE=/home/arc/.zsh_history_shared") {
			t.Fatalf("prompt block missing shared HISTFILE")
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
	if !strings.Contains(arcPromptBlockLocal, "systemctl --user start \"$__arc_waypipe_service_name\"") {
		t.Fatalf("local prompt block missing waypipe service start")
	}
	if !strings.Contains(arcPromptBlockLocal, "sw: warning: waypipe service is not active; continuing with plain ssh/tmux") {
		t.Fatalf("local prompt block missing waypipe fallback warning")
	}
	if !strings.Contains(arcPromptBlockLocal, "ARC_WAYPIPE_HINT_ONCE") {
		t.Fatalf("local prompt block missing one-time waypipe hint guard")
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
}

func TestArcTmuxBlockRemote_ContainsWaylandEnvPropagation(t *testing.T) {
	if !strings.Contains(arcTmuxBlockRemote, "update-environment") {
		t.Fatalf("remote tmux block missing update-environment setting")
	}
	if !strings.Contains(arcTmuxBlockRemote, "WAYLAND_DISPLAY") {
		t.Fatalf("remote tmux block missing WAYLAND_DISPLAY propagation")
	}
	if !strings.Contains(arcTmuxBlockRemote, "XDG_RUNTIME_DIR") {
		t.Fatalf("remote tmux block missing XDG_RUNTIME_DIR propagation")
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

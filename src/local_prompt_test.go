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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureLocalArcBashPrompt_CreatesFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := ensureLocalArcBashPrompt(); err != nil {
		t.Fatalf("ensureLocalArcBashPrompt: %v", err)
	}

	rcb, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
	}
	rc := string(rcb)
	if !strings.Contains(rc, arcPromptStart) || !strings.Contains(rc, arcPromptEnd) {
		t.Fatalf(".bashrc missing ARC prompt block markers")
	}
	if !strings.Contains(rc, "ARC AUTO SSH (local)") {
		t.Fatalf(".bashrc missing ARC AUTO SSH block")
	}

	pb, err := os.ReadFile(filepath.Join(home, ".bash_profile"))
	if err != nil {
		t.Fatalf("read .bash_profile: %v", err)
	}
	profile := string(pb)
	if !strings.Contains(profile, ". ~/.bashrc") && !strings.Contains(profile, "source ~/.bashrc") {
		t.Fatalf(".bash_profile should source ~/.bashrc, got: %q", profile)
	}
}

func TestEnsureLocalArcBashPrompt_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 2; i++ {
		if err := ensureLocalArcBashPrompt(); err != nil {
			t.Fatalf("ensureLocalArcBashPrompt (run %d): %v", i, err)
		}
	}

	rcb, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
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

	pb, err := os.ReadFile(filepath.Join(home, ".bash_profile"))
	if err != nil {
		t.Fatalf("read .bash_profile: %v", err)
	}
	profile := string(pb)
	if c := strings.Count(profile, ". ~/.bashrc"); c > 1 {
		t.Fatalf("expected no duplicate sourcing of ~/.bashrc, got %d", c)
	}
}

func TestEnsureLocalArcBashPrompt_ReplacesExistingBlockAndPreservesOtherLines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rcPath := filepath.Join(home, ".bashrc")
	initial := strings.Join([]string{
		"line-before",
		arcPromptStart,
		"old prompt content",
		arcPromptEnd,
		"line-after",
		"",
	}, "\n")
	if err := os.WriteFile(rcPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write .bashrc: %v", err)
	}

	if err := ensureLocalArcBashPrompt(); err != nil {
		t.Fatalf("ensureLocalArcBashPrompt: %v", err)
	}

	rcb, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
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

func TestEnsureLocalArcBashPrompt_DoesNotDuplicateProfileSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	profilePath := filepath.Join(home, ".bash_profile")
	if err := os.WriteFile(profilePath, []byte("source ~/.bashrc\n"), 0o600); err != nil {
		t.Fatalf("write .bash_profile: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := ensureLocalArcBashPrompt(); err != nil {
			t.Fatalf("ensureLocalArcBashPrompt (run %d): %v", i, err)
		}
	}

	pb, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .bash_profile: %v", err)
	}
	profile := string(pb)
	if c := strings.Count(profile, "source ~/.bashrc"); c != 1 {
		t.Fatalf("expected exactly one source line, got %d", c)
	}
}

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const (
	arcPromptStart = "### ARC_PROMPT_START"
	arcPromptEnd   = "### ARC_PROMPT_END"
)

var (
	arcPromptBlockRemote = mustTemplateFile("templates/prompt_remote.zsh")
	arcPromptBlockLocal  = mustTemplateFile("templates/prompt_local.zsh")
	arcTmuxBlockRemote   = mustTemplateFile("templates/tmux_remote.conf")
)

func stripArcPromptBlock(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	out := make([][]byte, 0, len(lines))
	skip := false
	for _, ln := range lines {
		if bytes.Equal(ln, []byte(arcPromptStart)) {
			skip = true
			continue
		}
		if bytes.Equal(ln, []byte(arcPromptEnd)) {
			skip = false
			continue
		}
		if !skip {
			out = append(out, ln)
		}
	}
	// Preserve the trailing newline behaviour of the original file: join with \n.
	return bytes.Join(out, []byte("\n"))
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func ensureArcPromptInZshrc(rcPath string, promptBlock string) error {
	rcb, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		rcb = nil
	}

	rcb = stripArcPromptBlock(rcb)
	if len(rcb) > 0 && rcb[len(rcb)-1] != '\n' {
		rcb = append(rcb, '\n')
	}
	if len(rcb) > 0 {
		// Ensure a blank line before the prompt block for readability.
		rcb = append(rcb, '\n')
	}
	rcb = append(rcb, []byte(promptBlock)...)
	rcb = append(rcb, '\n')

	return atomicWriteFile(rcPath, rcb, 0o600)
}

func ensureLocalArcZshPrompt() error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("cannot resolve home dir")
	}
	rc := filepath.Join(home, ".zshrc")

	if err := ensureArcPromptInZshrc(rc, arcPromptBlockLocal); err != nil {
		return err
	}
	return nil
}

package workflow

import "testing"

func TestDefaultSetupSteps_OrderAndCount(t *testing.T) {
	steps := DefaultSetupSteps()
	if len(steps) != 34 {
		t.Fatalf("expected 34 setup steps, got %d", len(steps))
	}
	if steps[0].Label != "Server: detect privileged mode" {
		t.Fatalf("unexpected first step: %q", steps[0].Label)
	}
	if steps[len(steps)-1].Label != "Local: configure persistent waypipe tunnel" {
		t.Fatalf("unexpected last step: %q", steps[len(steps)-1].Label)
	}
	if steps[len(steps)-2].Label != "Server: configure waypipe runtime" {
		t.Fatalf("unexpected penultimate step: %q", steps[len(steps)-2].Label)
	}
	if steps[len(steps)-3].Label != "Verify: verify /home/arc NFS mount" {
		t.Fatalf("unexpected third-from-end step: %q", steps[len(steps)-3].Label)
	}
	if steps[5].Label != "Server: install ARC tmux config" {
		t.Fatalf("missing tmux config step: %q", steps[5].Label)
	}
	if steps[4].Label != "Server: install ARC zsh prompt" {
		t.Fatalf("missing server zsh prompt step: %q", steps[4].Label)
	}
	if steps[6].Label != "Server: install zsh" {
		t.Fatalf("missing server zsh install step: %q", steps[6].Label)
	}
	if steps[7].Label != "Server: set zsh as default shell for arc" {
		t.Fatalf("missing server zsh default shell step: %q", steps[7].Label)
	}
	if steps[13].Label != "Server: apply nftables redirect service" {
		t.Fatalf("missing server nftables step: %q", steps[13].Label)
	}
	if steps[14].Label != "Local: add hosts aliases" {
		t.Fatalf("unexpected local phase start: %q", steps[14].Label)
	}
	if steps[16].Label != "Local: install ARC local prompt" {
		t.Fatalf("missing local prompt step: %q", steps[16].Label)
	}
	if steps[17].Label != "Local: install zsh" {
		t.Fatalf("missing local zsh install step: %q", steps[17].Label)
	}
	if steps[18].Label != "Local: set zsh as default shell" {
		t.Fatalf("missing local zsh default shell step: %q", steps[18].Label)
	}
	if steps[22].Label != "Local: enable wg0" {
		t.Fatalf("missing local wg enable step: %q", steps[22].Label)
	}
	if steps[23].Label != "Verify: add arc authorized_keys" {
		t.Fatalf("unexpected verification phase start: %q", steps[23].Label)
	}
	if steps[26].Label != "Server: resolve arc UID/GID for NFS squash" {
		t.Fatalf("missing arc UID/GID squash step: %q", steps[26].Label)
	}
	if steps[30].Label != "Local: configure /home/arc automount" {
		t.Fatalf("missing local NFS automount step: %q", steps[30].Label)
	}
}

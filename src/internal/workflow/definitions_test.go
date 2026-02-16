package workflow

import "testing"

func TestDefaultSetupSteps_OrderAndCount(t *testing.T) {
	steps := DefaultSetupSteps()
	if len(steps) != 20 {
		t.Fatalf("expected 20 setup steps, got %d", len(steps))
	}
	if steps[0].Label != "Server: detect privileged mode" {
		t.Fatalf("unexpected first step: %q", steps[0].Label)
	}
	if steps[len(steps)-1].Label != "Verify: verify tunnel connectivity" {
		t.Fatalf("unexpected last step: %q", steps[len(steps)-1].Label)
	}
	if steps[10].Label != "Local: add hosts aliases" {
		t.Fatalf("unexpected local phase start: %q", steps[10].Label)
	}
	if steps[12].Label != "Local: install ARC local prompt" {
		t.Fatalf("missing local prompt step: %q", steps[12].Label)
	}
	if steps[17].Label != "Verify: add arc authorized_keys" {
		t.Fatalf("unexpected verification phase start: %q", steps[17].Label)
	}
}

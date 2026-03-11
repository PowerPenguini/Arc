package workflow

import "testing"

func TestDefaultSetupSteps_OrderAndCount(t *testing.T) {
	steps := DefaultSetupSteps()
	if len(steps) != 31 {
		t.Fatalf("expected 31 setup steps, got %d", len(steps))
	}
	if steps[0].ID != StepDetectPrivilegedMode {
		t.Fatalf("unexpected first step ID: %q", steps[0].ID)
	}
	if steps[len(steps)-1].ID != StepConfigureImageClipboard {
		t.Fatalf("unexpected last step ID: %q", steps[len(steps)-1].ID)
	}

	idx := map[StepID]int{}
	for i, s := range steps {
		idx[s.ID] = i
	}

	assertBefore := func(a, b StepID) {
		ia, oka := idx[a]
		ib, okb := idx[b]
		if !oka || !okb {
			t.Fatalf("missing steps for ordering check: %q -> %q", a, b)
		}
		if ia >= ib {
			t.Fatalf("unexpected order: %q(%d) should be before %q(%d)", a, ia, b, ib)
		}
	}

	// Bootstrap key auth for arc must be in place before any remote infra step that dials as arc.
	assertBefore(StepEnsureArcSSHAccess, StepConfigureServerZsh)
	assertBefore(StepEnsureArcSSHAccess, StepInstallServerWireGuard)
	assertBefore(StepEnsureArcSSHAccess, StepInstallServerArcZshPrompt)
	assertBefore(StepEnsureArcSSHAccess, StepInstallServerArcTmux)

	// Keep core workflow ordering guarantees.
	assertBefore(StepEnableServerWG, StepEnableLocalWG)
	assertBefore(StepEnableLocalWG, StepVerifyTunnelConnectivity)
	assertBefore(StepVerifyTunnelConnectivity, StepResolveArcUIDGID)
	assertBefore(StepVerifyLocalArcNFSMount, StepConfigureRemoteWaypipe)
	assertBefore(StepConfigureRemoteWaypipe, StepConfigureLocalWaypipe)
	assertBefore(StepConfigureLocalWaypipe, StepConfigureClipboardComp)
	assertBefore(StepConfigureClipboardComp, StepConfigureImageClipboard)
}

func TestSetupStepDefinitions_ValidAndUnique(t *testing.T) {
	defs := SetupStepDefinitions()
	if err := ValidateStepDefinitions(defs); err != nil {
		t.Fatalf("ValidateStepDefinitions: %v", err)
	}
	seen := map[StepID]struct{}{}
	for _, def := range defs {
		if _, ok := seen[def.ID]; ok {
			t.Fatalf("duplicate step ID: %q", def.ID)
		}
		seen[def.ID] = struct{}{}
	}
	if len(seen) != 31 {
		t.Fatalf("expected 31 unique step IDs, got %d", len(seen))
	}
}

func TestDefaultSetupSteps_MatchesDefinitions(t *testing.T) {
	defs := SetupStepDefinitions()
	steps := DefaultSetupSteps()
	if len(defs) != len(steps) {
		t.Fatalf("len mismatch definitions=%d steps=%d", len(defs), len(steps))
	}
	for i := range defs {
		if defs[i].ID != steps[i].ID {
			t.Fatalf("ID mismatch at %d: %q != %q", i, defs[i].ID, steps[i].ID)
		}
		if defs[i].Label != steps[i].Label {
			t.Fatalf("label mismatch at %d: %q != %q", i, defs[i].Label, steps[i].Label)
		}
	}
}

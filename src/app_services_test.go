package main

import (
	"arc/internal/app"
	"arc/internal/workflow"
	"strings"
	"testing"
)

func TestRuntimeStepRegistryMatchesDefinitions(t *testing.T) {
	if err := validateRuntimeStepRegistry(); err != nil {
		t.Fatalf("validateRuntimeStepRegistry: %v", err)
	}
	defs := workflow.SetupStepDefinitions()
	if len(runtimeStepExecutors) != len(defs) {
		t.Fatalf("executor count mismatch: executors=%d defs=%d", len(runtimeStepExecutors), len(defs))
	}
}

func TestRuntimeServicesSetupDefinitionMatchesWorkflow(t *testing.T) {
	svc := newRuntimeServices()
	steps := svc.SetupDefinition()
	defs := workflow.SetupStepDefinitions()
	if len(steps) != len(defs) {
		t.Fatalf("setup definition count mismatch: steps=%d defs=%d", len(steps), len(defs))
	}
	for i := range defs {
		if steps[i].ID != defs[i].ID {
			t.Fatalf("step ID mismatch at %d: %q != %q", i, steps[i].ID, defs[i].ID)
		}
		if steps[i].Label != defs[i].Label {
			t.Fatalf("step label mismatch at %d: %q != %q", i, steps[i].Label, defs[i].Label)
		}
	}
}

func TestRunSetupStep_RequiresStepID(t *testing.T) {
	svc := newRuntimeServices()
	_, err := svc.RunSetupStep(app.SetupStepRequest{})
	if err == nil {
		t.Fatalf("expected error for missing step ID")
	}
	if !strings.Contains(err.Error(), "missing step ID") {
		t.Fatalf("unexpected error: %v", err)
	}
}

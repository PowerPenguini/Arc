package app

import (
	"arc/internal/workflow"
	"testing"
)

type fakeServices struct {
	lastReq SetupStepRequest
	steps   []workflow.Step
}

func (f *fakeServices) CheckLocalSudo() error { return nil }

func (f *fakeServices) ParseSSHConnectTarget(string) (string, string, string, error) {
	return "", "", "", nil
}

func (f *fakeServices) SetupDefinition() []workflow.Step {
	out := make([]workflow.Step, len(f.steps))
	copy(out, f.steps)
	return out
}

func (f *fakeServices) RunSetupStep(req SetupStepRequest) (SetupStepResult, error) {
	f.lastReq = req
	return SetupStepResult{}, nil
}

func TestRunSetupStepCmd_UsesStepIDFromDefinition(t *testing.T) {
	fake := &fakeServices{}
	m := model{svc: fake}
	m.steps = []setupStep{{ID: workflow.StepDetectPrivilegedMode, Label: "x"}}

	cmd := m.runSetupStepCmd(0)
	msg := cmd()
	if _, ok := msg.(setupStepDoneMsg); !ok {
		t.Fatalf("unexpected message type: %T", msg)
	}
	if fake.lastReq.StepID != workflow.StepDetectPrivilegedMode {
		t.Fatalf("unexpected step ID: got %q want %q", fake.lastReq.StepID, workflow.StepDetectPrivilegedMode)
	}
}

func TestStartSetupWorkflow_UsesServiceDefinition(t *testing.T) {
	fake := &fakeServices{steps: []workflow.Step{{ID: workflow.StepCreateArcUser, Label: "create"}}}
	m := model{svc: fake}

	_ = m.startSetupWorkflow()
	if len(m.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(m.steps))
	}
	if m.steps[0].ID != workflow.StepCreateArcUser {
		t.Fatalf("unexpected step ID: %q", m.steps[0].ID)
	}
	if m.steps[0].State != stepRunning {
		t.Fatalf("expected first step to be running, got %v", m.steps[0].State)
	}
}

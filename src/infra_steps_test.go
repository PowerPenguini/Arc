package main

import (
	"arc/internal/workflow"
	"testing"
)

func TestInfraStepHandlersCoverInfraWorkflowSteps(t *testing.T) {
	custom := map[workflow.StepID]struct{}{
		workflow.StepDetectPrivilegedMode:      {},
		workflow.StepCreateArcUser:             {},
		workflow.StepAddArcToSudoers:           {},
		workflow.StepCreateArcHushlogin:        {},
		workflow.StepInstallServerArcZshPrompt: {},
		workflow.StepInstallServerArcTmux:      {},
		workflow.StepAddLocalHostsAliases:      {},
		workflow.StepEnsureArcSSHAccess:        {},
		workflow.StepInstallLocalArcPrompt:     {},
		workflow.StepVerifyArcSSHLogin:         {},
	}

	for _, def := range workflow.SetupStepDefinitions() {
		if _, ok := custom[def.ID]; ok {
			continue
		}
		if _, ok := infraStepHandlers[def.ID]; !ok {
			t.Fatalf("missing infra handler for %q", def.ID)
		}
	}
}

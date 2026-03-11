package main

import (
	"arc/internal/workflow"
	"fmt"
)

var infraStepHandlers = map[workflow.StepID]infraStepFunc{
	workflow.StepConfigureServerZsh:         configureServerZsh,
	workflow.StepInstallServerWireGuard:     installServerWireGuard,
	workflow.StepWriteServerWGConf:          writeServerWireGuardConfig,
	workflow.StepOpenServerFirewall:         openServerFirewall,
	workflow.StepEnableServerWG:             enableServerWireGuard,
	workflow.StepApplyServerNFTables:        ensureRemoteLHRedirectNftablesService,
	workflow.StepConfigureLocalZsh:          configureLocalZsh,
	workflow.StepInstallLocalWireGuard:      installLocalWireGuard,
	workflow.StepWriteLocalWGConf:           writeLocalWireGuardConfig,
	workflow.StepEnableLocalWG:              enableLocalWireGuard,
	workflow.StepVerifyTunnelConnectivity:   verifyTunnelConnectivity,
	workflow.StepResolveArcUIDGID:           verifyRemoteArcIdentity,
	workflow.StepInstallRemoteNFS:           installRemoteNFS,
	workflow.StepExportRemoteArcNFS:         configureRemoteArcNFS,
	workflow.StepInstallLocalNFSClient:      func(infraRunContext) error { return installLocalNFSClient() },
	workflow.StepConfigureLocalArcAutomount: func(infraRunContext) error { return configureLocalArcAutomount() },
	workflow.StepVerifyLocalArcNFSMount:     func(infraRunContext) error { return verifyLocalArcNFSMount() },
	workflow.StepConfigureRemoteWaypipe:     configureRemoteWaypipe,
	workflow.StepConfigureLocalWaypipe:      func(infraRunContext) error { return configureLocalWaypipeService() },
	workflow.StepConfigureClipboardComp:     configureRemoteClipboardCompositor,
	workflow.StepConfigureImageClipboard:    func(infraRunContext) error { return configureLocalImageClipboardSync() },
}

func runInfraStep(ctx infraRunContext, stepID workflow.StepID) error {
	handler, ok := infraStepHandlers[stepID]
	if !ok {
		return fmt.Errorf("unknown infra step ID: %q", stepID)
	}
	return handler(ctx)
}

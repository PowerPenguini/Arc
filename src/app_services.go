package main

import (
	"arc/internal/app"
	"arc/internal/workflow"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type runtimeServices struct{}

type stepExecutor func(req app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error

var (
	validateExecutorsOnce sync.Once
	validateExecutorsErr  error
)

var runtimeStepExecutors = map[workflow.StepID]stepExecutor{
	workflow.StepDetectPrivilegedMode:       execDetectPrivilegedMode,
	workflow.StepCreateArcUser:              execCreateArcUser,
	workflow.StepAddArcToSudoers:            execAddArcToSudoers,
	workflow.StepCreateArcHushlogin:         execCreateArcHushlogin,
	workflow.StepInstallServerArcZshPrompt:  execInstallServerArcZshPrompt,
	workflow.StepInstallServerArcTmux:       execInstallServerArcTmux,
	workflow.StepInstallServerZsh:           execInfraStep,
	workflow.StepSetServerDefaultShell:      execInfraStep,
	workflow.StepDetectServerOS:             execInfraStep,
	workflow.StepInstallServerWireGuard:     execInfraStep,
	workflow.StepWriteServerWGConf:          execInfraStep,
	workflow.StepOpenServerFirewall:         execInfraStep,
	workflow.StepEnableServerWG:             execInfraStep,
	workflow.StepApplyServerNFTables:        execInfraStep,
	workflow.StepAddLocalHostsAliases:       execAddLocalHostsAliases,
	workflow.StepEnsureLocalSSHKey:          execEnsureLocalSSHKey,
	workflow.StepInstallLocalArcPrompt:      execInstallLocalArcPrompt,
	workflow.StepInstallLocalZsh:            execInfraStep,
	workflow.StepSetLocalDefaultShell:       execInfraStep,
	workflow.StepDetectLocalOS:              execInfraStep,
	workflow.StepInstallLocalWireGuard:      execInfraStep,
	workflow.StepWriteLocalWGConf:           execInfraStep,
	workflow.StepEnableLocalWG:              execInfraStep,
	workflow.StepAddArcAuthorizedKey:        execAddArcAuthorizedKey,
	workflow.StepVerifyArcSSHLogin:          execVerifyArcSSHLogin,
	workflow.StepVerifyTunnelConnectivity:   execVerifyTunnelConnectivity,
	workflow.StepResolveArcUIDGID:           execInfraStep,
	workflow.StepInstallRemoteNFS:           execInfraStep,
	workflow.StepExportRemoteArcNFS:         execInfraStep,
	workflow.StepInstallLocalNFSClient:      execInfraStep,
	workflow.StepConfigureLocalArcAutomount: execInfraStep,
	workflow.StepVerifyLocalArcNFSMount:     execInfraStep,
	workflow.StepConfigureRemoteWaypipe:     execInfraStep,
	workflow.StepConfigureLocalWaypipe:      execInfraStep,
}

func newRuntimeServices() app.Services { return runtimeServices{} }

func (runtimeServices) CheckLocalSudo() error {
	_, err := execLocal("sudo", "-n", "true")
	return err
}

func (runtimeServices) ParseSSHConnectTarget(target string) (user, host, addr string, err error) {
	return parseSSHConnectTarget(target)
}

func (runtimeServices) SetupDefinition() []workflow.Step {
	if err := validateRuntimeStepRegistry(); err != nil {
		return []workflow.Step{{
			ID:    "setup.registry.invalid",
			Label: "Setup registry invalid",
			State: workflow.StepFailed,
			Err:   err.Error(),
		}}
	}
	return workflow.DefaultSetupSteps()
}

func (runtimeServices) RunSetupStep(req app.SetupStepRequest) (app.SetupStepResult, error) {
	if err := validateRuntimeStepRegistry(); err != nil {
		return app.SetupStepResult{}, err
	}
	if req.StepID == "" {
		return app.SetupStepResult{}, fmt.Errorf("missing step ID")
	}

	executor, ok := runtimeStepExecutors[req.StepID]
	if !ok {
		return app.SetupStepResult{}, fmt.Errorf("unknown step ID: %q", req.StepID)
	}

	res := app.SetupStepResult{}
	wg := fromAppWG(req.WG)
	if wg.Endpoint == "" && strings.TrimSpace(req.Host) != "" {
		cfg, err := buildWGConfig(req.Host)
		if err != nil {
			return res, err
		}
		wg = cfg
	}

	if err := executor(req, wg, &res); err != nil {
		return res, err
	}
	return res, nil
}

func validateRuntimeStepRegistry() error {
	validateExecutorsOnce.Do(func() {
		defs := workflow.SetupStepDefinitions()
		if err := workflow.ValidateStepDefinitions(defs); err != nil {
			validateExecutorsErr = err
			return
		}
		for _, def := range defs {
			if _, ok := runtimeStepExecutors[def.ID]; !ok {
				validateExecutorsErr = fmt.Errorf("missing executor for step ID: %q", def.ID)
				return
			}
		}
		for id := range runtimeStepExecutors {
			found := false
			for _, def := range defs {
				if def.ID == id {
					found = true
					break
				}
			}
			if !found {
				validateExecutorsErr = fmt.Errorf("executor without step definition: %q", id)
				return
			}
		}
	})
	return validateExecutorsErr
}

func attachWG(res *app.SetupStepResult, wg wgConfig) {
	appWG := toAppWG(wg)
	res.WG = &appWG
}

func execDetectPrivilegedMode(req app.SetupStepRequest, _ wgConfig, res *app.SetupStepResult) error {
	client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
	if err != nil {
		return err
	}
	sudoOK, err := canRunPrivileged(req.BootstrapUser, client, req.Password)
	_ = client.Close()
	if err != nil {
		return err
	}
	res.UseSudo = &sudoOK
	return nil
}

func execCreateArcUser(req app.SetupStepRequest, _ wgConfig, _ *app.SetupStepResult) error {
	client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
	if err != nil {
		return err
	}
	err = ensureArcUser(client, req.UseSudo, req.Password)
	_ = client.Close()
	return err
}

func execAddArcToSudoers(req app.SetupStepRequest, _ wgConfig, _ *app.SetupStepResult) error {
	client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
	if err != nil {
		return err
	}
	err = ensureArcSudoers(client, req.UseSudo, req.Password)
	_ = client.Close()
	return err
}

func execCreateArcHushlogin(req app.SetupStepRequest, _ wgConfig, _ *app.SetupStepResult) error {
	client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
	if err != nil {
		return err
	}
	err = ensureArcHushLogin(client, req.UseSudo, req.Password)
	_ = client.Close()
	return err
}

func execInstallServerArcZshPrompt(req app.SetupStepRequest, _ wgConfig, _ *app.SetupStepResult) error {
	return ensureArcZshPrompt(req.Addr)
}

func execInstallServerArcTmux(req app.SetupStepRequest, _ wgConfig, _ *app.SetupStepResult) error {
	return ensureArcTmuxConfig(req.Addr)
}

func execInfraStep(req app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error {
	ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
	if err := runInfraStep(ctx, req.StepID); err != nil {
		return err
	}
	attachWG(res, wg)
	return nil
}

func execAddLocalHostsAliases(req app.SetupStepRequest, _ wgConfig, _ *app.SetupStepResult) error {
	return ensureLocalArcHostsAliases(req.Host)
}

func execEnsureLocalSSHKey(_ app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error {
	if err := ensureLocalSSHKeyPair(); err != nil {
		return err
	}
	pubPath := filepath.Join(userSSHDir(), "id_ed25519.pub")
	pubKeyLine, err := readPublicKeyLine(pubPath)
	if err != nil {
		return err
	}
	res.PubKeyLine = pubKeyLine
	attachWG(res, wg)
	return nil
}

func execInstallLocalArcPrompt(_ app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error {
	if err := ensureLocalArcZshPrompt(); err != nil {
		return err
	}
	attachWG(res, wg)
	return nil
}

func execAddArcAuthorizedKey(req app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error {
	if strings.TrimSpace(req.PubKeyLine) == "" {
		return fmt.Errorf("missing public key line")
	}
	client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
	if err != nil {
		return err
	}
	err = ensureArcAuthorizedKey(client, req.UseSudo, req.Password, req.PubKeyLine)
	_ = client.Close()
	if err != nil {
		return err
	}
	attachWG(res, wg)
	return nil
}

func execVerifyArcSSHLogin(req app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error {
	if err := verifyArcKeyLogin(req.Host, req.Addr); err != nil {
		return err
	}
	if err := ensureArcZshPrompt(req.Addr); err != nil {
		return err
	}
	attachWG(res, wg)
	return nil
}

func execVerifyTunnelConnectivity(req app.SetupStepRequest, wg wgConfig, res *app.SetupStepResult) error {
	if err := execInfraStep(req, wg, res); err != nil {
		return err
	}
	res.ReadyAs = arcUser + "@" + req.Host
	return nil
}

func toAppWG(c wgConfig) app.WGConfig {
	return app.WGConfig{
		ServerPriv: c.ServerPriv,
		ServerPub:  c.ServerPub,
		ClientPriv: c.ClientPriv,
		ClientPub:  c.ClientPub,
		ServerConf: c.ServerConf,
		ClientConf: c.ClientConf,
		Endpoint:   c.Endpoint,
	}
}

func fromAppWG(c app.WGConfig) wgConfig {
	return wgConfig{
		ServerPriv: c.ServerPriv,
		ServerPub:  c.ServerPub,
		ClientPriv: c.ClientPriv,
		ClientPub:  c.ClientPub,
		ServerConf: c.ServerConf,
		ClientConf: c.ClientConf,
		Endpoint:   c.Endpoint,
	}
}

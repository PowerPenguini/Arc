package app

import "arc/internal/workflow"

type WGConfig struct {
	ServerPriv       string
	ServerPub        string
	ClientPriv       string
	ClientPub        string
	MobileClientPriv string
	MobileClientPub  string

	ServerConf       string
	ClientConf       string
	MobileClientConf string
	Endpoint         string
}

type SetupStepRequest struct {
	BootstrapUser string
	Host          string
	Addr          string
	Password      string
	UseSudo       bool
	WG            WGConfig
	StepID        workflow.StepID
}

type SetupStepResult struct {
	UseSudo *bool
	ReadyAs string
	WG      *WGConfig
}

type Services interface {
	CheckLocalSudo() error
	ParseSSHDeviceTarget(target string) (user, host, addr string, err error)
	SetupDefinition() []workflow.Step
	RunSetupStep(req SetupStepRequest) (SetupStepResult, error)
	BuildMobilePayload(host string, wg WGConfig) (string, error)
}

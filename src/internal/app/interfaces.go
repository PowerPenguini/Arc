package app

import "arc/internal/workflow"

type WGConfig struct {
	ServerPriv string
	ServerPub  string
	ClientPriv string
	ClientPub  string

	ServerConf string
	ClientConf string
	Endpoint   string
}

type SetupStepRequest struct {
	BootstrapUser string
	Host          string
	Addr          string
	Password      string
	UseSudo       bool
	PubKeyLine    string
	WG            WGConfig
	StepID        workflow.StepID
}

type SetupStepResult struct {
	UseSudo    *bool
	PubKeyLine string
	ReadyAs    string
	WG         *WGConfig
}

type Services interface {
	CheckLocalSudo() error
	ParseSSHConnectTarget(target string) (user, host, addr string, err error)
	SetupDefinition() []workflow.Step
	RunSetupStep(req SetupStepRequest) (SetupStepResult, error)
}

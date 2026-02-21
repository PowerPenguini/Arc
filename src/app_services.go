package main

import (
	"arc/internal/app"
	"fmt"
	"path/filepath"
	"strings"
)

type runtimeServices struct{}

func newRuntimeServices() app.Services { return runtimeServices{} }

func (runtimeServices) CheckLocalSudo() error {
	_, err := execLocal("sudo", "-n", "true")
	return err
}

func (runtimeServices) ParseSSHConnectTarget(target string) (user, host, addr string, err error) {
	return parseSSHConnectTarget(target)
}

func (runtimeServices) RunSetupStep(req app.SetupStepRequest) (app.SetupStepResult, error) {
	res := app.SetupStepResult{}
	wg := fromAppWG(req.WG)
	if wg.Endpoint == "" && strings.TrimSpace(req.Host) != "" {
		cfg, err := buildWGConfig(req.Host)
		if err != nil {
			return res, err
		}
		wg = cfg
	}

	switch req.Index {
	case 0:
		// Server: detect privileged mode.
		client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
		if err != nil {
			return res, err
		}
		sudoOK, err := canRunPrivileged(req.BootstrapUser, client, req.Password)
		if err != nil {
			_ = client.Close()
			return res, err
		}
		res.UseSudo = &sudoOK
		_ = client.Close()
		return res, nil

	case 1:
		client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
		if err != nil {
			return res, err
		}
		err = ensureArcUser(client, req.UseSudo, req.Password)
		_ = client.Close()
		return res, err

	case 2:
		client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
		if err != nil {
			return res, err
		}
		err = ensureArcSudoers(client, req.UseSudo, req.Password)
		_ = client.Close()
		return res, err

	case 3:
		client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
		if err != nil {
			return res, err
		}
		err = ensureArcHushLogin(client, req.UseSudo, req.Password)
		_ = client.Close()
		return res, err

	case 4:
		return res, ensureArcZshPrompt(req.Addr)

	case 5:
		return res, ensureArcTmuxConfig(req.Addr)

	case 6:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 0); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 7:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 1); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 8:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 2); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 9:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 3); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 10:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 4); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 11:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 5); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 12:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 6); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 13:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 13); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 14:
		if err := ensureLocalArcHostsAliases(req.Host); err != nil {
			return res, err
		}
		return res, nil

	case 15:
		if err := ensureLocalSSHKeyPair(); err != nil {
			return res, err
		}
		pubPath := filepath.Join(userSSHDir(), "id_ed25519.pub")
		pubKeyLine, err := readPublicKeyLine(pubPath)
		if err != nil {
			return res, err
		}
		res.PubKeyLine = pubKeyLine
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 16:
		if err := ensureLocalArcZshPrompt(); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 17:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 7); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 18:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 8); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 19:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 9); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 20:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 10); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 21:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 11); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 22:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 12); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 23:
		if strings.TrimSpace(req.PubKeyLine) == "" {
			return res, fmt.Errorf("missing public key line")
		}
		client, err := dialWithPassword(req.BootstrapUser, req.Addr, req.Password)
		if err != nil {
			return res, err
		}
		err = ensureArcAuthorizedKey(client, req.UseSudo, req.Password, req.PubKeyLine)
		_ = client.Close()
		if err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 24:
		if err := verifyArcKeyLogin(req.Host, req.Addr); err != nil {
			return res, err
		}
		if err := ensureArcZshPrompt(req.Addr); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	case 25:
		if err := runInfraStep(infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}, 14); err != nil {
			return res, err
		}
		res.ReadyAs = arcUser + "@" + req.Host
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 26:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 15); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 27:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 16); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 28:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 17); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 29:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 18); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 30:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 19); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 31:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 20); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 32:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 21); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil
	case 33:
		ctx := infraRunContext{Addr: req.Addr, Host: req.Host, WG: wg}
		if err := runInfraStep(ctx, 22); err != nil {
			return res, err
		}
		appWG := toAppWG(wg)
		res.WG = &appWG
		return res, nil

	default:
		return res, fmt.Errorf("unknown step index: %d", req.Index)
	}
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

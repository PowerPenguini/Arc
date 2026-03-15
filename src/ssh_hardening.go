package main

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func hardenServerSSH(ctx infraRunContext) error {
	if err := withArcClient(ctx.Addr, func(client *ssh.Client) error {
		script, err := renderTemplateFile("templates/ssh_harden_server_access.sh.tmpl", map[string]string{
			"WGInterface": wgInterface,
			"WGPort":      fmt.Sprintf("%d", wgPort),
		})
		if err != nil {
			return err
		}
		if _, err := runRemoteCommand(client, script, false, ""); err != nil {
			return fmt.Errorf("apply remote SSH hardening: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := execLocal(
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		arcUser+"@remotehost",
		"true",
	); err != nil {
		return fmt.Errorf("verify hardened SSH over WireGuard: %w", err)
	}

	return nil
}

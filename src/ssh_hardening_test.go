package main

import (
	"strings"
	"testing"
)

func TestSSHHardeningTemplate_ContainsExpectedRestrictions(t *testing.T) {
	script, err := renderTemplateFile("templates/ssh_harden_server_access.sh.tmpl", map[string]string{
		"WGInterface": wgInterface,
		"WGPort":      "51820",
	})
	if err != nil {
		t.Fatalf("render ssh hardening template: %v", err)
	}

	for _, snippet := range []string{
		"PermitRootLogin no",
		"PasswordAuthentication no",
		"KbdInteractiveAuthentication no",
		"PubkeyAuthentication yes",
		"MaxAuthTries 3",
		"X11Forwarding no",
		"AllowAgentForwarding no",
		`public_if="$(ip route get 1.1.1.1`,
		`iifname "wg0" accept`,
		`iifname "$public_if" udp dport 51820 accept`,
		`iifname "$public_if" drop`,
		"arc-public-lockdown.service",
		"ufw allow in on wg0 proto tcp to any port 22",
		"ufw allow 51820/udp",
		"ufw deny 22/tcp",
	} {
		if !strings.Contains(script, snippet) {
			t.Fatalf("hardening script missing %q", snippet)
		}
	}
}

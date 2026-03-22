package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBuildMobilePayload_UsesWireGuardHostWhenConfigPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	raw, err := buildMobilePayload("example.com", wgConfig{
		MobileClientConf: "[Interface]\nPrivateKey = test\nAddress = 10.0.0.3/32\n",
	})
	if err != nil {
		t.Fatalf("buildMobilePayload returned error: %v", err)
	}

	var payload mobilePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload.Host != wgServerIP {
		t.Fatalf("unexpected host: got %q want %q", payload.Host, wgServerIP)
	}
	if payload.Port != 22 {
		t.Fatalf("unexpected port: got %d want 22", payload.Port)
	}
	if payload.Username != arcUser {
		t.Fatalf("unexpected username: got %q want %q", payload.Username, arcUser)
	}
	if payload.WireGuardTunnelName != defaultMobileWGTunnelName {
		t.Fatalf("unexpected tunnel name: got %q want %q", payload.WireGuardTunnelName, defaultMobileWGTunnelName)
	}
	if payload.WireGuardConfig == "" {
		t.Fatalf("wireguard config was empty")
	}
	if !strings.Contains(payload.WireGuardConfig, "Address = "+wgMobileCIDR) {
		t.Fatalf("expected mobile wireguard config in payload")
	}
	if !strings.Contains(payload.PrivateKeyPEM, "PRIVATE KEY") {
		t.Fatalf("private key was not embedded")
	}
	if strings.Contains(payload.PrivateKeyPEM, "BEGIN OPENSSH PRIVATE KEY") {
		t.Fatalf("expected mobile payload to use RSA PEM, got OpenSSH private key")
	}
}

func TestBuildMobilePayload_UsesOriginalHostWithoutWireGuard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	raw, err := buildMobilePayload("example.com", wgConfig{})
	if err != nil {
		t.Fatalf("buildMobilePayload returned error: %v", err)
	}

	var payload mobilePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload.Host != "example.com" {
		t.Fatalf("unexpected host: got %q want %q", payload.Host, "example.com")
	}
	if payload.WireGuardConfig != "" {
		t.Fatalf("expected wireguard config to be empty")
	}
}

func TestBuildMobilePayload_UsesDedicatedMobileKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := ensureLocalSSHKeyPair(); err != nil {
		t.Fatalf("ensureLocalSSHKeyPair returned error: %v", err)
	}

	desktopKey, err := os.ReadFile(userSSHDir() + "/id_ed25519")
	if err != nil {
		t.Fatalf("read desktop key: %v", err)
	}

	raw, err := buildMobilePayload("example.com", wgConfig{})
	if err != nil {
		t.Fatalf("buildMobilePayload returned error: %v", err)
	}

	var payload mobilePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if strings.TrimSpace(payload.PrivateKeyPEM) == strings.TrimSpace(string(desktopKey)) {
		t.Fatalf("expected mobile payload to use a dedicated mobile key")
	}
	if !strings.Contains(payload.PrivateKeyPEM, "BEGIN PRIVATE KEY") {
		t.Fatalf("expected PKCS8 private key in payload")
	}
}

func TestEnsureLocalMobileSSHKeyPair_RecreatesPublicKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := ensureLocalMobileSSHKeyPair(); err != nil {
		t.Fatalf("ensureLocalMobileSSHKeyPair returned error: %v", err)
	}

	pubPath := userMobileSSHPublicKeyPath()
	if err := os.Remove(pubPath); err != nil {
		t.Fatalf("remove mobile public key: %v", err)
	}

	if err := ensureLocalMobileSSHKeyPair(); err != nil {
		t.Fatalf("ensureLocalMobileSSHKeyPair recreate returned error: %v", err)
	}

	pub, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("read recreated public key: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(pub)), "ssh-rsa ") {
		t.Fatalf("expected recreated public key to be ssh-rsa, got %q", strings.TrimSpace(string(pub)))
	}
}

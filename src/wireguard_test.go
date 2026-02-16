package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenWGKeyPair_Base64AndLength(t *testing.T) {
	priv, pub, err := genWGKeyPair()
	if err != nil {
		t.Fatalf("genWGKeyPair: %v", err)
	}
	pb, err := base64.StdEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("decode priv: %v", err)
	}
	ub, err := base64.StdEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("decode pub: %v", err)
	}
	if len(pb) != 32 {
		t.Fatalf("expected priv 32 bytes, got %d", len(pb))
	}
	if len(ub) != 32 {
		t.Fatalf("expected pub 32 bytes, got %d", len(ub))
	}
}

func TestBuildWGConfig_RendersExpectedFields(t *testing.T) {
	wg, err := buildWGConfig("example.com")
	if err != nil {
		t.Fatalf("buildWGConfig: %v", err)
	}
	if wg.Endpoint != "example.com:51820" {
		t.Fatalf("unexpected endpoint: %q", wg.Endpoint)
	}
	if !strings.Contains(wg.ServerConf, "[Interface]") || !strings.Contains(wg.ServerConf, "[Peer]") {
		t.Fatalf("server conf missing sections")
	}
	if !strings.Contains(wg.ServerConf, "ListenPort = 51820") {
		t.Fatalf("server conf missing ListenPort")
	}
	if !strings.Contains(wg.ClientConf, "Endpoint = example.com:51820") {
		t.Fatalf("client conf missing endpoint")
	}
	if !strings.Contains(wg.ClientConf, "AllowedIPs = "+wgServerIP+"/32") {
		t.Fatalf("client conf missing AllowedIPs")
	}
}

func TestWGPublicKeyFromPrivateKeyB64_MatchesGenerated(t *testing.T) {
	priv, pub, err := genWGKeyPair()
	if err != nil {
		t.Fatalf("genWGKeyPair: %v", err)
	}
	derived, err := wgPublicKeyFromPrivateKeyB64(priv)
	if err != nil {
		t.Fatalf("wgPublicKeyFromPrivateKeyB64: %v", err)
	}
	if derived != pub {
		t.Fatalf("derived pub mismatch: got %q want %q", derived, pub)
	}
}

func TestPatchWGPeerInConf_UpdatesMatchingAllowedIPs(t *testing.T) {
	conf := strings.Join([]string{
		"[Interface]",
		"Address = 10.66.66.2/32",
		"PrivateKey = AAAA",
		"",
		"[Peer]",
		"PublicKey = OLD",
		"Endpoint = wrong:51820",
		"AllowedIPs = 10.66.66.1/32",
		"",
	}, "\n")

	out, changed, err := patchWGPeerInConf(conf, "10.66.66.1/32", "NEWPUB", "example.com:51820", "25")
	if err != nil {
		t.Fatalf("patchWGPeerInConf: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if !strings.Contains(out, "PublicKey = NEWPUB") {
		t.Fatalf("missing updated PublicKey")
	}
	if !strings.Contains(out, "AllowedIPs = 10.66.66.1/32") {
		t.Fatalf("missing AllowedIPs")
	}
	if !strings.Contains(out, "Endpoint = example.com:51820") {
		t.Fatalf("missing updated Endpoint")
	}
	if !strings.Contains(out, "PersistentKeepalive = 25") {
		t.Fatalf("missing keepalive")
	}
}

func TestParseOSRelease_ID(t *testing.T) {
	m := parseOSRelease("NAME=Ubuntu\nID=ubuntu\nVERSION_ID=\"24.04\"\n")
	if m["ID"] != "ubuntu" {
		t.Fatalf("expected ID ubuntu, got %q", m["ID"])
	}
	if m["VERSION_ID"] != "24.04" {
		t.Fatalf("expected VERSION_ID 24.04, got %q", m["VERSION_ID"])
	}
}

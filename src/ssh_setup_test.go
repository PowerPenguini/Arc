package main

import "testing"

func TestParseSSHDeviceTarget_UserHostPort(t *testing.T) {
	user, host, addr, err := parseSSHDeviceTarget("root@example.com:2202")
	if err != nil {
		t.Fatalf("parseSSHDeviceTarget: %v", err)
	}
	if user != "root" {
		t.Fatalf("user = %q, want root", user)
	}
	if host != "example.com" {
		t.Fatalf("host = %q, want example.com", host)
	}
	if addr != "example.com:2202" {
		t.Fatalf("addr = %q, want example.com:2202", addr)
	}
}

func TestParseSSHDeviceTarget_SSHURI(t *testing.T) {
	user, host, addr, err := parseSSHDeviceTarget("ssh://arc@192.168.1.10")
	if err != nil {
		t.Fatalf("parseSSHDeviceTarget: %v", err)
	}
	if user != "arc" {
		t.Fatalf("user = %q, want arc", user)
	}
	if host != "192.168.1.10" {
		t.Fatalf("host = %q, want 192.168.1.10", host)
	}
	if addr != "192.168.1.10:22" {
		t.Fatalf("addr = %q, want 192.168.1.10:22", addr)
	}
}

func TestParseSSHDeviceTarget_BareIPv6(t *testing.T) {
	user, host, addr, err := parseSSHDeviceTarget("root@2001:db8::7")
	if err != nil {
		t.Fatalf("parseSSHDeviceTarget: %v", err)
	}
	if user != "root" {
		t.Fatalf("user = %q, want root", user)
	}
	if host != "2001:db8::7" {
		t.Fatalf("host = %q, want 2001:db8::7", host)
	}
	if addr != "[2001:db8::7]:22" {
		t.Fatalf("addr = %q, want [2001:db8::7]:22", addr)
	}
}

func TestParseSSHDeviceTarget_RejectsMissingUser(t *testing.T) {
	_, _, _, err := parseSSHDeviceTarget("example.com")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

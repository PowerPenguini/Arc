package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestKnownHostTargetsForAddr_DeduplicatesAndKeepsPort(t *testing.T) {
	targets := knownHostTargetsForAddr("example.com:2222", "example.com", "remotehost", "remotehost", "rh")

	want := []knownHostTarget{
		{Host: "example.com", Port: "2222"},
		{Host: "remotehost", Port: "2222"},
		{Host: "rh", Port: "2222"},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("unexpected targets:\n got %#v\nwant %#v", targets, want)
	}
}

func TestKnownHostRemovalKeys_DefaultAndCustomPort(t *testing.T) {
	got22 := knownHostRemovalKeys(knownHostTarget{Host: "remotehost", Port: "22"})
	want22 := []string{"remotehost", "[remotehost]:22"}
	if !reflect.DeepEqual(got22, want22) {
		t.Fatalf("unexpected default-port removal keys: got %#v want %#v", got22, want22)
	}

	got2222 := knownHostRemovalKeys(knownHostTarget{Host: "example.com", Port: "2222"})
	want2222 := []string{"example.com", "[example.com]:2222"}
	if !reflect.DeepEqual(got2222, want2222) {
		t.Fatalf("unexpected custom-port removal keys: got %#v want %#v", got2222, want2222)
	}
}

func TestSyncLocalKnownHosts_ReplacesEntriesAndAppendsScannedKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	type call struct {
		name string
		args []string
	}
	var calls []call

	execFn := func(name string, args ...string) (string, error) {
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		if name == "ssh-keyscan" {
			host := args[len(args)-1]
			return host + " ssh-ed25519 AAAATEST", nil
		}
		return "", nil
	}

	targets := []knownHostTarget{
		{Host: "example.com", Port: "2222"},
		{Host: "remotehost", Port: "22"},
	}
	if err := syncLocalKnownHosts(execFn, targets...); err != nil {
		t.Fatalf("syncLocalKnownHosts returned error: %v", err)
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	raw, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "example.com ssh-ed25519 AAAATEST") {
		t.Fatalf("missing scanned key for example.com: %q", text)
	}
	if !strings.Contains(text, "remotehost ssh-ed25519 AAAATEST") {
		t.Fatalf("missing scanned key for remotehost: %q", text)
	}

	var got []string
	for _, c := range calls {
		got = append(got, c.name+" "+strings.Join(c.args, " "))
	}
	wantPrefixes := []string{
		"ssh-keygen -R example.com -f " + knownHostsPath,
		"ssh-keygen -R [example.com]:2222 -f " + knownHostsPath,
		"ssh-keygen -R remotehost -f " + knownHostsPath,
		"ssh-keygen -R [remotehost]:22 -f " + knownHostsPath,
		"ssh-keyscan -T 5 -p 2222 example.com",
		"ssh-keyscan -T 5 remotehost",
	}
	for _, want := range wantPrefixes {
		found := false
		for _, line := range got {
			if line == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing exec call %q in %#v", want, got)
		}
	}
}

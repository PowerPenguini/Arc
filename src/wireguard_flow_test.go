package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestActivateLocalWaypipeService_CommandOrder(t *testing.T) {
	var got [][]string
	execFn := func(name string, args ...string) (string, error) {
		cmd := append([]string{name}, args...)
		got = append(got, cmd)
		return "", nil
	}

	if err := activateLocalWaypipeService(execFn); err != nil {
		t.Fatalf("activateLocalWaypipeService: %v", err)
	}

	want := [][]string{
		{"systemctl", "--user", "import-environment", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"},
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "arc-waypipe.service"},
		{"systemctl", "--user", "restart", "arc-waypipe.service"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected command order:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestActivateLocalWaypipeService_IgnoresImportFailure(t *testing.T) {
	var got [][]string
	execFn := func(name string, args ...string) (string, error) {
		cmd := append([]string{name}, args...)
		got = append(got, cmd)
		if len(got) == 1 {
			return "", errors.New("missing env")
		}
		return "", nil
	}

	if err := activateLocalWaypipeService(execFn); err != nil {
		t.Fatalf("activateLocalWaypipeService: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 commands, got %d", len(got))
	}
}

func TestActivateLocalWaypipeService_ReturnsRestartError(t *testing.T) {
	wantErr := errors.New("restart failed")
	execFn := func(name string, args ...string) (string, error) {
		if len(args) >= 2 && args[1] == "restart" {
			return "", wantErr
		}
		return "", nil
	}

	err := activateLocalWaypipeService(execFn)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected restart error, got %v", err)
	}
}

package main

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

type fakeUploadSession struct {
	stdin   *bytes.Buffer
	command string
}

func (s *fakeUploadSession) SetStdin(reader *bytes.Reader) {
	if s.stdin == nil {
		s.stdin = &bytes.Buffer{}
	}
	_, _ = s.stdin.ReadFrom(reader)
}

func (s *fakeUploadSession) CombinedOutput(cmd string) ([]byte, error) {
	s.command = cmd
	return nil, nil
}

func (s *fakeUploadSession) Close() error { return nil }

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

func TestActivateLocalClipboardSyncService_CommandOrder(t *testing.T) {
	var got [][]string
	execFn := func(name string, args ...string) (string, error) {
		cmd := append([]string{name}, args...)
		got = append(got, cmd)
		return "", nil
	}

	if err := activateLocalClipboardSyncService(execFn); err != nil {
		t.Fatalf("activateLocalClipboardSyncService: %v", err)
	}

	want := [][]string{
		{"systemctl", "--user", "import-environment", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"},
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "arc-clipboard-sync.service"},
		{"systemctl", "--user", "restart", "arc-clipboard-sync.service"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected command order:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestActivateLocalClipboardSyncService_ReturnsRestartError(t *testing.T) {
	wantErr := errors.New("restart failed")
	execFn := func(name string, args ...string) (string, error) {
		if len(args) >= 2 && args[1] == "restart" {
			return "", wantErr
		}
		return "", nil
	}

	err := activateLocalClipboardSyncService(execFn)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected restart error, got %v", err)
	}
}

func TestUploadRemoteFile_UsesTempPathThenRename(t *testing.T) {
	var stdin bytes.Buffer
	session := &fakeUploadSession{stdin: &stdin}

	err := uploadRemoteFileWithSessionFactory(func() (remoteFileSession, error) {
		return session, nil
	}, ".local/bin/arc-clipd", []byte("payload"), 0o700)
	if err != nil {
		t.Fatalf("uploadRemoteFileWithSessionFactory: %v", err)
	}
	if stdin.String() != "payload" {
		t.Fatalf("unexpected stdin payload: %q", stdin.String())
	}
	if session.command == "" {
		t.Fatalf("expected remote command to be recorded")
	}
	if !bytes.Contains([]byte(session.command), []byte(`.local/bin/arc-clipd.tmp.arc-upload`)) {
		t.Fatalf("remote command should upload to temp path before rename: %s", session.command)
	}
	if !bytes.Contains([]byte(session.command), []byte(`mv -f "$HOME/.local/bin/arc-clipd.tmp.arc-upload" "$HOME/.local/bin/arc-clipd"`)) {
		t.Fatalf("remote command should rename temp path into final location: %s", session.command)
	}
}

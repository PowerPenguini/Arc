package main

import (
	"os"
	"strings"
	"testing"
)

func TestHostsSource_DoesNotAddPublicAliases(t *testing.T) {
	src, err := os.ReadFile("hosts.go")
	if err != nil {
		t.Fatalf("read hosts.go: %v", err)
	}
	text := string(src)

	if strings.Contains(text, `fmt.Sprintf("%s\tpub.remotehost", ip)`) {
		t.Fatalf("hosts.go should not append pub.remotehost aliases")
	}
	if strings.Contains(text, `fmt.Sprintf("%s\tpub.rh", ip)`) {
		t.Fatalf("hosts.go should not append pub.rh aliases")
	}
	if strings.Contains(text, `"remotehost":     wgServerIP`) && strings.Contains(text, `"pub.remotehost": pubIP`) {
		t.Fatalf("hosts.go should not keep public host mapping alongside remotehost")
	}
}

func TestLocalRunners_UseWireGuardHostOnly(t *testing.T) {
	for _, path := range []string{"infra_shell_steps.go", "clipboard_flow.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		if !strings.Contains(text, `ARC_REMOTE_HOSTS=remotehost`) {
			t.Fatalf("%s should default ARC_REMOTE_HOSTS to remotehost", path)
		}
		if strings.Contains(text, `ARC_REMOTE_HOSTS=remotehost pub.remotehost`) {
			t.Fatalf("%s should not keep public host fallback in ARC_REMOTE_HOSTS", path)
		}
		if strings.Contains(text, `hosts="${ARC_REMOTE_HOSTS:-remotehost pub.remotehost}"`) {
			t.Fatalf("%s should not iterate over public host fallback", path)
		}
	}
}

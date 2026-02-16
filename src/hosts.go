package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func resolveHostToIP(host string) (string, error) {
	h := strings.TrimSpace(host)
	if h == "" {
		return "", fmt.Errorf("host is empty")
	}
	if ip := net.ParseIP(h); ip != nil {
		return h, nil
	}
	ips, err := net.LookupIP(h)
	if err != nil {
		return "", fmt.Errorf("dns lookup failed for %q: %w", h, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("dns lookup returned no IPs for %q", h)
	}
	// Prefer IPv4 to keep /etc/hosts simple.
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return ips[0].String(), nil
}

func ensureLocalHostsMappings(m map[string]string) error {
	hostsRaw, err := execLocal("sudo", "-n", "cat", "/etc/hosts")
	if err != nil {
		return fmt.Errorf("cannot read /etc/hosts: %w", err)
	}

	var out []string
	for _, ln := range strings.Split(hostsRaw, "\n") {
		trim := strings.TrimSpace(ln)
		if trim == "" || strings.HasPrefix(trim, "#") {
			out = append(out, ln)
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) < 2 {
			out = append(out, ln)
			continue
		}
		// Remove any existing occurrences of aliases we manage.
		hasManaged := false
		nf := []string{fields[0]}
		for _, f := range fields[1:] {
			if _, ok := m[f]; ok {
				hasManaged = true
				continue
			}
			nf = append(nf, f)
		}
		if hasManaged {
			// Drop the line if it only mapped managed aliases.
			if len(nf) > 1 {
				out = append(out, strings.Join(nf, "\t"))
			}
			continue
		}
		out = append(out, ln)
	}

	// Append managed mappings in a stable order.
	if ip := strings.TrimSpace(m["remotehost"]); ip != "" {
		out = append(out, fmt.Sprintf("%s\tremotehost", ip))
	}
	if ip := strings.TrimSpace(m["pub.remotehost"]); ip != "" {
		out = append(out, fmt.Sprintf("%s\tpub.remotehost", ip))
	}

	newHosts := strings.Join(out, "\n")
	if !strings.HasSuffix(newHosts, "\n") {
		newHosts += "\n"
	}

	tmp, err := os.CreateTemp("/tmp", "arc-hosts-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := os.WriteFile(tmpPath, []byte(newHosts), 0o644); err != nil {
		return fmt.Errorf("write temp hosts file: %w", err)
	}
	if _, err := execLocal("sudo", "-n", "install", "-m", "0644", tmpPath, "/etc/hosts"); err != nil {
		return fmt.Errorf("cannot update /etc/hosts (sudo install): %w", err)
	}
	return nil
}

func ensureLocalArcHostsAliases(pubHost string) error {
	pubIP, err := resolveHostToIP(pubHost)
	if err != nil {
		return err
	}
	// "remotehost" should point at the server's WG/LAN address.
	return ensureLocalHostsMappings(map[string]string{
		"remotehost":     wgServerIP,
		"pub.remotehost": pubIP,
	})
}

package workflow

func DefaultSetupSteps() []Step {
	return []Step{
		{Label: "Server: detect privileged mode"},
		{Label: "Server: create arc user"},
		{Label: "Server: add arc to sudoers"},
		{Label: "Server: create ~/.hushlogin for arc"},
		{Label: "Server: install ARC bash prompt"},
		{Label: "Server: detect OS"},
		{Label: "Server: install WireGuard"},
		{Label: "Server: write wg0.conf"},
		{Label: "Server: open firewall (ufw)"},
		{Label: "Server: enable wg0"},
		{Label: "Local: add hosts aliases"},
		{Label: "Local: ensure SSH key"},
		{Label: "Local: install ARC local prompt"},
		{Label: "Local: detect OS"},
		{Label: "Local: install WireGuard"},
		{Label: "Local: write wg0.conf"},
		{Label: "Local: enable wg0"},
		{Label: "Verify: add arc authorized_keys"},
		{Label: "Verify: verify arc SSH login"},
		{Label: "Verify: verify tunnel connectivity"},
	}
}

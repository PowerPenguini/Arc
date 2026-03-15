package workflow

const (
	StepDetectPrivilegedMode       StepID = "server.detect_privileged_mode"
	StepCreateArcUser              StepID = "server.create_arc_user"
	StepAddArcToSudoers            StepID = "server.add_arc_to_sudoers"
	StepCreateArcHushlogin         StepID = "server.create_arc_hushlogin"
	StepInstallServerArcZshPrompt  StepID = "server.install_arc_zsh_prompt"
	StepInstallServerArcTmux       StepID = "server.install_arc_tmux_config"
	StepConfigureServerZsh         StepID = "server.configure_zsh"
	StepInstallServerWireGuard     StepID = "server.install_wireguard"
	StepWriteServerWGConf          StepID = "server.write_wg_conf"
	StepOpenServerFirewall         StepID = "server.open_ufw_wireguard"
	StepEnableServerWG             StepID = "server.enable_wg"
	StepApplyServerNFTables        StepID = "server.apply_nftables_redirect"
	StepAddLocalHostsAliases       StepID = "local.add_hosts_aliases"
	StepEnsureArcSSHAccess         StepID = "verify.ensure_arc_ssh_access"
	StepInstallLocalArcPrompt      StepID = "local.install_arc_prompt"
	StepConfigureLocalZsh          StepID = "local.configure_zsh"
	StepInstallLocalWireGuard      StepID = "local.install_wireguard"
	StepWriteLocalWGConf           StepID = "local.write_wg_conf"
	StepEnableLocalWG              StepID = "local.enable_wg"
	StepVerifyArcSSHLogin          StepID = "verify.verify_arc_ssh_login"
	StepVerifyTunnelConnectivity   StepID = "verify.verify_tunnel_connectivity"
	StepResolveArcUIDGID           StepID = "server.resolve_arc_uid_gid"
	StepInstallRemoteNFS           StepID = "server.install_nfs_server"
	StepExportRemoteArcNFS         StepID = "server.export_arc_nfs"
	StepInstallLocalNFSClient      StepID = "local.install_nfs_client"
	StepConfigureLocalArcAutomount StepID = "local.configure_arc_automount"
	StepVerifyLocalArcNFSMount     StepID = "verify.verify_arc_nfs_mount"
	StepConfigureRemoteWaypipe     StepID = "server.configure_waypipe_runtime"
	StepConfigureLocalWaypipe      StepID = "local.configure_waypipe_tunnel"
	StepConfigureClipboardComp     StepID = "server.configure_clipboard_compositor"
	StepHardenServerSSH            StepID = "server.harden_ssh_access"
	StepConfigureImageClipboard    StepID = "local.configure_image_clipboard_sync"
)

func SetupStepDefinitions() []StepDef {
	return []StepDef{
		{ID: StepDetectPrivilegedMode, Label: "Server: detect privileged mode"},
		{ID: StepCreateArcUser, Label: "Server: create arc user"},
		{ID: StepAddArcToSudoers, Label: "Server: add arc to sudoers"},
		{ID: StepCreateArcHushlogin, Label: "Server: create ~/.hushlogin for arc"},
		{ID: StepAddLocalHostsAliases, Label: "Local: add hosts aliases"},
		{ID: StepEnsureArcSSHAccess, Label: "Verify: ensure arc SSH access"},
		{ID: StepVerifyArcSSHLogin, Label: "Verify: verify arc SSH login"},
		{ID: StepConfigureServerZsh, Label: "Server: install and configure zsh"},
		{ID: StepInstallServerWireGuard, Label: "Server: install WireGuard"},
		{ID: StepWriteServerWGConf, Label: "Server: write wg0.conf"},
		{ID: StepOpenServerFirewall, Label: "Server: open firewall (ufw)"},
		{ID: StepEnableServerWG, Label: "Server: enable wg0"},
		{ID: StepApplyServerNFTables, Label: "Server: apply nftables redirect service"},
		{ID: StepInstallServerArcZshPrompt, Label: "Server: install ARC zsh prompt"},
		{ID: StepInstallServerArcTmux, Label: "Server: install ARC tmux config"},
		{ID: StepInstallLocalArcPrompt, Label: "Local: install ARC local prompt"},
		{ID: StepConfigureLocalZsh, Label: "Local: install and configure zsh"},
		{ID: StepInstallLocalWireGuard, Label: "Local: install WireGuard"},
		{ID: StepWriteLocalWGConf, Label: "Local: write wg0.conf"},
		{ID: StepEnableLocalWG, Label: "Local: enable wg0"},
		{ID: StepVerifyTunnelConnectivity, Label: "Verify: verify tunnel connectivity"},
		{ID: StepResolveArcUIDGID, Label: "Server: resolve arc UID/GID for NFS squash"},
		{ID: StepInstallRemoteNFS, Label: "Server: install NFS server"},
		{ID: StepExportRemoteArcNFS, Label: "Server: export /home/arc over NFS (WireGuard only)"},
		{ID: StepInstallLocalNFSClient, Label: "Local: install NFS client"},
		{ID: StepConfigureLocalArcAutomount, Label: "Local: configure /home/arc automount"},
		{ID: StepVerifyLocalArcNFSMount, Label: "Verify: verify /home/arc NFS mount"},
		{ID: StepConfigureRemoteWaypipe, Label: "Server: configure waypipe runtime"},
		{ID: StepConfigureLocalWaypipe, Label: "Local: configure persistent waypipe tunnel"},
		{ID: StepConfigureClipboardComp, Label: "Server: configure clipboard compositor"},
		{ID: StepHardenServerSSH, Label: "Server: harden SSH access"},
		{ID: StepConfigureImageClipboard, Label: "Local: configure image clipboard sync"},
	}
}

func DefaultSetupSteps() []Step {
	defs := SetupStepDefinitions()
	steps := make([]Step, 0, len(defs))
	for _, def := range defs {
		steps = append(steps, Step{
			ID:    def.ID,
			Label: def.Label,
		})
	}
	return steps
}

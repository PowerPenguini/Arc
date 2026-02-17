# ARC

![ARC logo](assets/logo.svg)

ARC is a seamless server-client workflow tool where the local machine is treated as a thin client by default.

## Feature Highlights

- Thin-client-first workflow:
  - local machine is primarily an access/control node,
  - the main runtime context is a remote tmux session.

- Session-centric remote flow (`sw`, `sl`, `x`):
  - on local:
  - `sw` attaches/creates remote tmux session `arc`,
  - `sw <name>` attaches/creates named remote tmux session,
  - `sl` lists remote tmux sessions,
  - `x` (or `x <name>`) kills remote tmux session (`arc` by default),
  - connection order is always `arc@remotehost` then `arc@pub.remotehost`.
  - on remote:
  - `sw` detaches current tmux client (keeps session alive),
  - `sw <name>` switches to named tmux session (creates it if missing),
  - `sl` lists tmux sessions on the host,
  - `x` kills current tmux session, `x <name>` kills named session.

- Automatic handoff on local Bash start:
  - on first interactive local shell startup, ARC attempts to auto-connect to the remote `ARC` shell,
  - connection order: `arc@remotehost` -> `arc@pub.remotehost`.

- Local hosts alias management:
  - updates local hosts mappings for `remotehost` and `pub.remotehost`,
  - keeps SSH targets stable for seamless switching and auto-connect behavior.

- WireGuard-aware remote access strategy:
  - prefers private/WireGuard path (`remotehost`) and falls back to public endpoint (`pub.remotehost`) when needed.

- NFS-backed remote home:
  - remote exports `/home/arc` via NFS (WireGuard-only access scope),
  - local machine mounts it as `/home/arc` via systemd automount,
  - setup includes NFS verification step.

- Prompt integration:
  - dedicated ARC prompt block is managed for local and remote Bash environments,
  - remote prompt shows active session name in a top bar,
  - VPN-aware indicators are included in prompt logic.

- Remote tmux config management:
  - ARC installs managed `~/.tmux.conf` block (mouse on, scroll bindings, hidden status line),
  - ARC-specific tmux keybinds are configured during setup.

## Setup Workflow (High Level)

ARC setup currently runs these groups:
- server bootstrap (`arc` user, sudoers, hushlogin, remote prompt, remote tmux config),
- WireGuard setup (remote + local),
- access verification (SSH key login + tunnel checks),
- NFS setup (remote export + local automount + mount verification).

## Core Components

- `src/internal/app` - application orchestration and state model.
- `src/app_services.go` - runtime service adapter.
- `src/ssh_setup.go` - SSH/remote operation helpers.
- `src/components` - rendering components.

## Notes

- `DefaultLocalSteps` and `DefaultInfraSteps` are retained as fallback definitions.

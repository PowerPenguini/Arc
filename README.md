# ARC

ARC is a seamless server-client workflow tool where the local machine is treated as a thin client by default.

## Feature Highlights

- Thin-client-first workflow:
  - local machine is primarily an access/control node,
  - the main runtime context is the remote `ARC` shell.

- `sw` command behavior:
  - on local: `sw` opens an interactive remote shell (`arc@remotehost` first, then `arc@pub.remotehost` fallback),
  - on remote: `sw` exits the current SSH session quickly.

- Automatic shell handoff on local Bash start:
  - on first interactive local shell startup, ARC attempts to auto-connect to the remote `ARC` shell,
  - connection order: `arc@remotehost` -> `arc@pub.remotehost`.

- Local hosts alias management:
  - updates local hosts mappings for `remotehost` and `pub.remotehost`,
  - keeps SSH targets stable for seamless switching and auto-connect behavior.

- WireGuard-aware remote access strategy:
  - prefers private/WireGuard path (`remotehost`) and falls back to public endpoint (`pub.remotehost`) when needed.

- Prompt integration:
  - dedicated ARC prompt block is managed for local and remote Bash environments,
  - VPN-aware indicators are included in prompt logic.

## Core Components

- `src/internal/app` - application orchestration and state model.
- `src/app_services.go` - runtime service adapter.
- `src/ssh_setup.go` - SSH/remote operation helpers.
- `src/components` - rendering components.

## Notes

- `DefaultLocalSteps` and `DefaultInfraSteps` are retained as fallback definitions.

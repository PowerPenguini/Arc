# arc-clipd

`arc-clipd` is a tiny Wayland clipboard-only sidecar intended for ARC.

It is deliberately narrower than a real compositor:

- one socket
- one synthetic seat
- no rendering
- no outputs
- no shells
- clipboard protocols only

The server advertises:

- `wl_seat`
- `wl_data_device_manager`
- `ext_data_control_manager_v1`

The current implementation focuses on being small and easy to provision from ARC rather than on
desktop completeness.

## Build

```bash
cargo check --offline
cargo build --release --offline
```

## Run

```bash
export XDG_RUNTIME_DIR=/run/user/$(id -u)
cargo run --offline -- --socket arc-clipd-0 --seat seat0
```

## Inject clipboard contents

```bash
printf 'hello' | cargo run --offline -- put --socket arc-clipd-0 --type text/plain
```

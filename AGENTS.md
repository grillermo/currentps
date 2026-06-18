@/Users/grillermo/.codex/RTK.md

# currentps Agent Notes

## Project Summary

`currentps` is a Go terminal UI for inspecting running processes by rolling average CPU usage. It polls `ps` every 2 seconds, augments processes with listening ports from `lsof`, supports filtering by process name or port, persists exclusions in `currentps_excluded.txt`, and can kill the selected process with `F2`.

## Tech Stack

- Go module: `currentps`
- Go version: `1.26.1`
- UI: Charmbracelet Bubble Tea and Lip Gloss
- Target platforms: macOS and Linux
- External runtime commands: `ps -eo %cpu,pid,args` and `lsof -iTCP -sTCP:LISTEN -nP -P -F pn`

## Development Commands

Run all commands through `rtk`.

```sh
rtk go test ./...
rtk gofmt -w .
rtk go build -o /tmp/currentps-build-check .
rtk ./currentps
```

## Code Organization

- `main.go`: loads exclusions, initializes the Bubble Tea model, and starts the alt-screen TUI.
- `model.go`: owns application state, key handling, sorting/filtering, selection, exclusions, kill behavior, and rendering.
- `poller.go`: parses `ps` output and fetches live process snapshots.
- `ports.go`: parses `lsof` field output and formats displayed ports.
- `exclusions.go`: loads and appends persistent excluded process names.
- `*_test.go`: unit tests for parsing, formatting, model updates, filtering, and exclusions.

## Implementation Guidance

- Keep the app as a small single-package Go program unless a change clearly needs more structure.
- Prefer pure helper functions for parsing, formatting, filtering, and list-building so behavior remains unit-testable without launching the TUI.
- Do not make tests depend on live system process state, `ps`, or `lsof`; use fixture strings and direct model messages instead.
- Keep the Process Name column aligned with htop-style comm names (`ucomm` on macOS, `comm` elsewhere). Do not append the PID to the visible name; the PID column already carries it.
- Keep internal process identity separate from the visible name so processes with the same comm value do not merge.
- Be careful with process actions: `F2` sends `SIGKILL` to the selected process PID and then removes stale state from all model maps.
- Keep filtering case-insensitive for names and substring-based for port numbers.
- Keep port lists sorted, deduplicated, and truncated through `formatPorts`.
- Avoid writing to `currentps_excluded.txt` in tests; use `t.TempDir()`.

## Verification

- Run `rtk go test ./...` after behavior changes.
- Run `rtk go build -o /tmp/currentps-build-check .` after changes that affect startup, dependencies, or OS command integration.
- If rendering changes, inspect model `View()` output through focused tests where practical.

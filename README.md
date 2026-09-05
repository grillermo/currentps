# currentps

Terminal UI that shows your running processes sorted by average CPU usage. Lets you filter, select, exclude, and kill processes without leaving the terminal.

## What it does

- Polls `ps` every 2 seconds and tracks a rolling average CPU% per process
- Shows listening ports next to each process
- Filter by name or port number
- Exclude processes persistently (saved to `currentps_excluded.txt`)
- Kill a selected process with F2

## Prerequisites

- Go 1.21+
- macOS or Linux (uses `ps -eo %cpu,pid,args` and `lsof`/`ss` for ports)

## Install

```sh
go install github.com/grillermo/currentps@latest
```

Or build from source:

```sh
git clone https://github.com/grillermo/currentps
cd currentps
go build -o currentps .
```

## Run

```sh
./currentps
```

## Kill everything on a port

```sh
currentps kill 11434
```

Lists every process listening on that port, with all of them selected. Deselect
the ones you want to keep, then press `Enter` to `SIGKILL` the rest.

| Key            | Action                     |
|----------------|----------------------------|
| `↑` / `↓`      | Navigate list              |
| `Space`        | Toggle the process under the cursor |
| `a`            | Toggle all                 |
| `Enter`        | Kill the selected processes |
| `Esc` / `q`    | Cancel without killing     |

Exits `0` when nothing is listening on the port or the kills succeed, `1` when
cancelled or a kill fails, `2` on a bad argument.

## Keybindings

| Key        | Action                        |
|------------|-------------------------------|
| `↑` / `↓` | Navigate list                 |
| `Enter`    | Select process                |
| `/`        | Start filtering               |
| `Esc`      | Deselect / exit filter        |
| `F1`       | Exclude selected (persistent) |
| `F2`       | Kill selected process (SIGKILL) |
| `q` / `Ctrl+C` | Quit                    |

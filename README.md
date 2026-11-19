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

## Kill everything matching a name

```sh
currentps kill 'claude*'
```

Any argument that isn't all digits is a case-insensitive glob (`*`, `?`,
`[abc]`) matched against each process's name and its argv[0] basename, so
`claude*` catches Claude Code even though its process name is its version
number. Quote the pattern so your shell doesn't expand it. `currentps` never
lists itself. The same confirmation list and keys apply.

Exits `0` when nothing matches or the kills succeed, `1` when
cancelled or a kill fails, `2` on a bad port or pattern.

## Keybindings

Press `/` to filter by name, PID or port. While filtering, printable keys
(space included) type into the query and `Esc` goes back to the list with the
filter kept.

| Key                 | Action                                        |
|---------------------|-----------------------------------------------|
| `/`                 | Start filtering                               |
| `↑` / `↓`, `j` / `k` | Navigate list (`j` / `k` type while filtering) |
| `Space`             | Toggle the process under the cursor           |
| `Ctrl+A`            | Tick all visible (unfiltered) processes       |
| `Ctrl+X`            | Untick all visible processes                  |
| `Tab` / `Shift+Tab` | Highlight the next / previous action button   |
| `Enter`             | Run the highlighted action button             |
| `F1`                | Exclude ticked (persistent)                   |
| `F2`                | Kill ticked processes (SIGKILL)               |
| `F3`                | Copy command                                  |
| `Esc`               | Leave the filter; on the list, clear the filter if set, else quit |
| `q`                 | Quit (types `q` while filtering)              |
| `Ctrl+C`            | Quit                                          |

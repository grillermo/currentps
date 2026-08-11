# Compact Port Column Design

## Goal

Give the Command column 14 additional terminal cells by reducing the Port column from 20 to 6 cells, while retaining the displayed rolling-average CPU column.

## Layout

The process table will continue to render these columns in the existing order:

`Avg CPU%`, `PID`, `Port`, `Process Name`, and `Command`.

Only the Port column width changes, from 20 to 6. The saved width is added to the dynamically calculated Command column. The CPU average remains visible and processes continue to be sorted by it.

## Port Formatting

`formatPorts` will truncate a formatted comma-separated port list to at most six runes. A truncated value uses the existing ellipsis convention: five visible runes followed by `…`.

## Tests

Update the port-formatting truncation test to assert the six-rune limit and ellipsis. Add a focused view-rendering test that verifies the table keeps `Avg CPU%`, has the six-character Port header field, and gives Command the reclaimed width.

## Scope

No polling, process sorting, filtering, keybindings, or process-action behavior changes.

# Keel

Keel is a local-first task tracker for terminal-heavy workflows. It stores the canonical task ledger as Markdown so people and AI agents can read and write the same file without a server or database.

## Install

```sh
go install ./cmd/keel
```

For local development from this repo:

```sh
./keel
make install-local
keel
```

`make install-local` symlinks the repo launcher to `~/bin/keel`. If `~/bin` is not on your PATH, add `export PATH="$HOME/bin:$PATH"` to your shell profile.

## Usage

```sh
keel                         # open the Bubble Tea TUI
keel add "Review parser"     # append a task
keel add --category work --tag ai "Review parser"
keel inbox                   # print open tasks
keel edit                    # open the ledger in $EDITOR
keel doctor                  # validate the ledger
keel export                  # print canonical Markdown
keel config                  # print resolved config
keel ai "plan my day"        # ask OpenCode to inspect/update the ledger
```

The default ledger is `~/tasks.md`, intentionally placed in the home directory so local agents and shell tools can find it without project-specific discovery. Override it with `--ledger` or with `~/.config/keel/config.toml`:

```toml
ledger = "~/tasks.md"
theme = "harbor"
```

## Ledger Grammar

Canonical task lines use this shape:

```md
- [ ] id:K-20260530-9QRT status:todo priority:p2 due:2026-06-01 category:work +ai Review parser edge cases
  Indented lines are task notes.
  - [ ] sub:S-1 status:todo Draft acceptance criteria
  @time start:2026-05-30T09:00:00Z end:2026-05-30T09:45:00Z duration:2700

- [ ] recur:R-20260530-ABCD every:weekly days:sun at:18:30 priority:p1 category:home Make pasta
```

Rules:

- Checkbox must be `[ ]` or `[x]`.
- `id:<id>` is required and must be unique.
- `status:` is one of `todo`, `doing`, `done`, or `blocked`.
- `priority:` is one of `p0`, `p1`, `p2`, or `p3`.
- `due:` uses `YYYY-MM-DD`.
- `category:` is optional and can be edited in the TUI.
- `id:`, `status:`, `priority:`, `due:`, and `category:` may appear once each.
- `[x]` must agree with `status:done`; blank checkboxes with `status:done` are canonicalized to `[x]`.
- `+tag` tokens are preserved.
- Unknown `key:value` metadata before the title is preserved.
- The first non-token starts the task title.
- Indented checkbox lines are subtasks. Use `sub:S-N` IDs unique within the parent task.
- Indented non-empty lines after a task become notes.
- Indented `@time` lines are structured timer sessions. Active sessions omit `end:` and `duration:`.
- Recurring tasks use `recur:<id>` instead of `id:<id>`.
- `every:` is `daily`, `weekly`, or `custom`.
- `days:` is required for `weekly` and `custom`, using comma-separated days like `mon,wed,fri`.
- `at:` is optional and uses comma-separated `HH:MM` times.

## TUI

Keel uses a simple terminal workbench with one selectable list and one detail pane. The filter strip keeps the mental model small:

- All: open work.
- Today: due and active work.
- Done: completed work.
- Recurring: recurring definitions.

All tasks are grouped by due date, then priority. Recurring tasks have their own filter and appear as virtual due-today cards in All/Today when their schedule matches the current day. When a timer is running, Keel shows a persistent Timer band with the active task and elapsed time. Keel autosyncs the ledger from disk every few seconds so edits from agents or another terminal appear without a manual reload. Search matches titles, categories, tags, notes, and subtasks. Keys: `h/l` filter, `j/k` move, `a` add task or recurring task, `A` add subtask, `e` edit title, `u` edit due date, `c` edit category, `t` edit tags, `p` cycle priority, `s` cycle status, `D` mark done, `T` start or stop the timer, `/` search, `r` reload, `q` quit. Inline text edits are saved with `Enter` and cancelled with `Esc`. Due dates use `YYYY-MM-DD`; submitting a blank due date clears it.

Recurring add examples from the Recurring filter:

```text
daily 09:00 Make pasta
weekly mon 10:00 Weekly review
mon,wed,fri 08:30 Stretch
```

## AI Agent Flow

`keel ai` runs OpenCode headlessly with [docs/keel-agent-skill.md](docs/keel-agent-skill.md) and the ledger attached:

```sh
keel ai
keel ai "Break today into subtasks and tag agent-suitable work with +ai"
```

The skill teaches agents the ledger grammar, subtask format, timer session format, and clarification rules.

## Safety Model

Writes use temp-file, fsync, rename, and directory sync where supported. Keel records the ledger hash when loading and refuses to save over a changed file. `keel doctor --fix` refuses to repair symlink ledgers and writes a `.bak` backup before canonicalizing parsed tasks.

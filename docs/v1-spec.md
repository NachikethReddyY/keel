# Keel v1 Spec

## Scope

Keel v1 is a single Go binary with Cobra commands, a Bubble Tea TUI, Bubbles components, Lipgloss styling, TOML config, and a Markdown ledger as the canonical store.

The default ledger is `~/tasks.md`. That path is deliberately boring and global so humans, Codex sessions, and other local agents can inspect and update the same task file without asking where the project stores state.

The TUI does not implement a rich text editor. Full ledger editing is delegated to `$EDITOR`, resolved as `$EDITOR`, `nvim`, `vim`, then `vi`, and launched with direct argv rather than a shell.

## Commands

- `keel`: open the TUI.
- `keel add`: append one task.
- `keel inbox`: print open tasks.
- `keel edit`: open the ledger in an external editor.
- `keel doctor`: validate parseability.
- `keel doctor --fix`: rewrite canonical parsed tasks after creating a backup.
- `keel export`: print canonical Markdown.
- `keel config`: print resolved config.

## Conflict Behavior

Keel hashes the ledger on load. Any save compares the current file hash to the loaded hash. If the file changed, Keel refuses to write and asks the user to reload.

## Repair Behavior

Repair is intentionally conservative:

- Refuse to repair a symlink ledger.
- Preserve a `.bak` copy before writing.
- Only canonicalize tasks that parsed successfully.
- Keep unknown metadata tokens that appeared before the title.

## TUI

The TUI is a single workbench rather than a multi-tab board. It has:

- Filter strip: All, Today, Done, Recurring.
- Task list: one focused, keyboard-driven list.
- Detail pane: selected task metadata, notes, and edit hints.
- Inline editing: `e` changes title, `u` changes due date, blank due clears it.
- Organization: `c` edits `category:`, `t` edits user-defined `+tags`.
- Search: matches title, ID, category, tags, notes, and subtasks.
- All filter grouping: due date first, priority second, then title.
- Subtasks: `A` adds an indented child task under the selected task.
- Timer: `T` starts or stops one active timer. Timer sessions are stored as indented `@time` records below the task.
- Timer visibility: while a timer is active, a persistent Timer band appears below the header with the active task and elapsed time.
- Recurring: separate Recurring filter for definitions; due-today recurring tasks appear as virtual cards in All and Today.
- Autosync: the TUI polls the ledger every few seconds and reloads when the file hash changes on disk.

## TUI States

- First run: empty list with capture prompt and visible ledger path.
- Empty state: visible per-filter empty messages.
- Parse error: blocking error screen with doctor guidance.
- Search no-results: designed list empty state.
- Saving: status line changes while save commands run.
- Conflict: status line shows reload requirement.
- Readonly or permission failures: surfaced as save errors.

## Timer Records

Timer sessions are canonical Markdown children of their task:

```md
  @time start:2026-05-30T09:00:00Z end:2026-05-30T09:45:00Z duration:2700
```

An active timer omits `end:` and `duration:`. Keel permits one active timer across the ledger so dashboard totals do not double-count focused work.

## Recurring Tasks

Recurring definitions are stored in the ledger, separate from normal one-off tasks:

```md
- [ ] recur:R-20260530-ABCD every:daily at:09:00 priority:p1 Make pasta
- [ ] recur:R-20260530-EFGH every:weekly days:mon at:10:00 priority:p2 Weekly review
- [ ] recur:R-20260530-IJKL every:custom days:mon,wed,fri at:08:30 priority:p2 Stretch
```

Rules:

- `recur:<id>` is required and unique among recurring tasks.
- `every:` is `daily`, `weekly`, or `custom`.
- `days:` is required for `weekly` and `custom`.
- `at:` is optional and accepts one or more comma-separated `HH:MM` times.
- Recurring definitions are managed from the Recurring filter.
- Matching recurrences appear as virtual tasks for the current day; they are not duplicated into permanent normal tasks.

## AI Agent Flow

`keel ai [prompt]` shells out to `opencode run` with:

- `docs/keel-agent-skill.md`
- the resolved ledger path, defaulting to `~/tasks.md`
- the user's prompt

The command prompts for one line when no prompt argument is supplied. The attached skill instructs agents to preserve IDs, subtasks, timer sessions, unknown metadata, and notes, and to ask for clarification when edits are ambiguous.

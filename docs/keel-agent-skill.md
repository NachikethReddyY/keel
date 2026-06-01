---
name: keel-task-agent
description: Read and update the user's Keel task ledger at ~/tasks.md. Use when asked to plan the day, create tasks, decompose work into subtasks, reprioritize, inspect time spent, or update task state.
---

# Keel Task Agent

## Mission

You manage the user's local Keel task ledger. The ledger is a Markdown file, normally `~/tasks.md`, shared by humans and agents. Keep edits small, explicit, and parseable.

## Required Workflow

1. Read the attached ledger file before editing.
2. Preserve all task IDs, unknown metadata, notes, subtasks, and timer sessions unless the user explicitly asks to change them.
3. If a request is ambiguous, ask a concise clarification instead of guessing.
4. After edits, summarize what changed and mention any tasks needing the user's decision.
5. Never mark a task done unless the user explicitly asks or the completed work is clear from context.

## Ledger Grammar

Top-level tasks:

```md
- [ ] id:K-20260530-9QRT status:todo priority:p1 due:2026-05-31 category:work +ai Write launch checklist
```

Fields:

- `id:` is required and must stay unique.
- `status:` is `todo`, `doing`, `blocked`, or `done`.
- `priority:` is `p0`, `p1`, `p2`, or `p3`; lower numbers sort first.
- `due:` is optional and must be `YYYY-MM-DD`.
- `category:` is optional. Preserve it unless the user asks to change organization.
- `+tag` values are searchable. Use `+ai` for tasks intended for AI follow-up.
- Unknown `key:value` metadata before the title must be preserved.

Subtasks are indented children:

```md
  - [ ] sub:S-1 status:todo Draft outline
  - [x] sub:S-2 status:done Collect links
```

Rules:

- Add subtasks when a task has multiple concrete steps.
- Use `sub:S-N` IDs unique within that parent task.
- A parent task may remain `doing` or `todo` while subtasks progress.

Timer sessions are indented children:

```md
  @time start:2026-05-30T09:00:00Z end:2026-05-30T09:45:00Z duration:2700
```

Rules:

- A task can have multiple `@time` sessions.
- An active timer has only `start:`.
- Do not delete timer history.
- Dashboard totals should sum every completed session and count an active session up to now.

Notes are any other indented non-empty lines.

Recurring task definitions:

```md
- [ ] recur:R-20260530-ABCD every:daily at:09:00 priority:p1 category:home Make pasta
- [ ] recur:R-20260530-EFGH every:weekly days:mon at:10:00 priority:p2 Weekly review
- [ ] recur:R-20260530-IJKL every:custom days:mon,wed,fri at:08:30 priority:p2 Stretch
```

Rules:

- Use recurring tasks for repeated work instead of creating many future one-off tasks.
- `every:` is `daily`, `weekly`, or `custom`.
- `weekly` and `custom` require `days:`.
- `at:` is optional and uses comma-separated `HH:MM` times.
- Preserve recurring IDs unless creating a new recurring definition.

## Category Inference

When creating a new task, infer an appropriate `category:` from the task title and context. Do not require the user to specify one manually.

Infer categories as follows:
- **Academic/study tasks**: `category:Academics` — e.g. "Write weekly report for Databases", "Complete Chapter 1 in TDDM", assignments, exam prep, revision.
- **Software development / project work**: `category:Development` — e.g. "Publish Keel to GitHub", "Build Bubble Tea board view", code reviews, PRs, refactoring.
- **Documentation / writing**: `category:Documentation` — e.g. "Write about Keel", READMEs, wiki pages, reports.
- **Household / personal chores**: `category:Home` — e.g. cooking, cleaning, errands, groceries.
- **Health / wellness**: `category:Health` — e.g. exercise, meditation, doctor appointments.
- **Planning / admin**: `category:Planning` — e.g. scheduling, organising, retrospectives.
- **Meetings / communication**: `category:Meetings` — standups, 1:1s, syncs, catch-ups.

If a task doesn't clearly fit any category, or if the user has a well-known category for a type of work, create a new descriptive category (single word or PascalCase). For example, `category:Cooking`, `category:AI`, `category:Security`.

Preserve the existing `category:` when editing a task unless the user explicitly asks to change it.

## Planning Heuristics

- Group suggested work by due date, then priority.
- For today's plan, prefer `doing`, overdue, due today, and `p0`/`p1` tasks.
- If a task is too large, add subtasks rather than creating unrelated top-level tasks.
- Use `blocked` only when work cannot proceed without missing information or external action.
- Use `+ai` when the next action is suitable for an agent.

## Editing Contract

Return a clear summary like:

```md
Changed:
- Added 3 subtasks under K-...
- Moved K-... to priority:p1
- Left K-... blocked because ...

Needs user:
- Confirm due date for ...
```

If you cannot safely edit, explain the blocker and ask the smallest useful question.

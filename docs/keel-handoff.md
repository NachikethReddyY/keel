# Handoff: Keel Terminal Task Tracker

## Context

The user wants to build **Keel**, a local-first CLI/TUI task tracker for terminal-heavy Vim/NeoVim workflows. The tool should let both the user and AI agents write, edit, and monitor tasks through a shared local task ledger. The user explicitly approved the core stack and asked for an implementation plan, not implementation yet.

The session used the `preflight` and `frontend-design` skills. Foundations were locked, and four preflight reviewer agents ran. The review result was **human-in-loop / revise**, not auto-approved, because design and security flagged high risk and all reviewers recommended revision.

## Goals

The next agent should revise the implementation plan into a build-ready v1 spec, then optionally begin implementation if the user approves.

Focus areas:

- Make the Markdown ledger grammar precise.
- Define safe file write, conflict, and repair behavior.
- Clarify TUI interaction hierarchy and states.
- Harden `$EDITOR` integration.
- Expand the test plan around filesystem, editor, parser, and concurrency edge cases.

## Constraints & Decisions

Approved stack:

- Language: Go
- CLI: Cobra
- TUI: Bubble Tea
- Components: Bubbles
- Styling: Lipgloss
- Storage: Markdown ledger
- Config: YAML/TOML was approved generally, but reviewers recommend choosing one. Prefer TOML for v1.
- Editor integration: `$EDITOR`, with fallback to `nvim`, `vim`, then `vi`
- Distribution: single Go binary

Locked foundations:

- No database for v1. Markdown ledger is canonical.
- No TypeScript. Use Go structs only.
- Validation via Go parser plus `keel doctor`.
- Routing via Cobra command tree plus Bubble Tea TUI tab state.
- No auth. Local files only.
- Styling via Lipgloss theme tokens.
- UI via Bubble Tea, Bubbles, Lipgloss.
- No client/server communication in v1.
- Suggested folder layout: `cmd/keel`, `internal/ledger`, `internal/parser`, `internal/tui`, `internal/editor`, `internal/config`, `internal/export`, `testdata/`.

Initial product shape:

- Binary: `keel`
- Commands: `keel`, `add`, `inbox`, `edit`, `doctor`, `export`, `config`
- Default ledger: `~/tasks.md`, configurable
- TUI tabs: Board, Today, AI Feed, Search, Edit
- Shortcuts: `j/k`, `h/l`, `a`, `e`, `space`, `s`, `d`, `p`, `/`, `g/G`, `?`, `q`

## Preflight Review Summary

Decision: **human_in_loop / revise**

Key reviewer findings:

- Engineering: architecture is solid, but atomic writes, concurrent edit semantics, ID generation, and Markdown grammar need sharper rules.
- Design: multi-tab TUI is high-risk by scope; clarify primary workflow, first-run state, conflict state, no-results state, narrow-terminal behavior, and whether Edit is a real tab or `$EDITOR` handoff.
- QA: test plan must add coverage for concurrent writes, config precedence, `doctor --fix`, editor failures, symlinks/permissions/path expansion, large ledgers, export stability, and TUI persistence.
- Security: `$EDITOR` must be launched with direct argv, never shell interpolation. Ledger/config paths and repair writes need protections against symlink/path abuse. Treat Markdown/config as untrusted input.

Important correction for orchestrator synthesis:

- Design reviewer returned `confidence: 0.88`, likely intended as 8.8 or 88%. Under strict preflight rules, do not round up. Treat as below 8 unless normalized with user approval.
- QA reviewer returned `confidence: 86`, likely intended as 8.6 or 86%. Normalize only if the orchestration policy allows it; otherwise flag invalid schema. Either way, high risk already blocks auto-approval.

## Files & References

No project files have been created yet.

Relevant skill files:

- `/Users/nr/.agents/skills/preflight/SKILL.md`
- `/Users/nr/.agents/skills/frontend-design/SKILL.md`
- `/Users/nr/.agents/skills/handoff/SKILL.md`

Reviewer role files already read:

- `/Users/nr/.agents/skills/preflight/agents/eng-reviewer.md`
- `/Users/nr/.agents/skills/preflight/agents/design-reviewer.md`
- `/Users/nr/.agents/skills/preflight/agents/qa-reviewer.md`
- `/Users/nr/.agents/skills/preflight/agents/security-reviewer.md`
- `/Users/nr/.agents/skills/preflight/agents/orchestrator.md`

Preflight persistence note:

- The previous turn created `/Users/nr/.temp`, but the orchestrator JSON was not written before the user interrupted. If the next agent continues preflight formally, it should write the final JSON to `/Users/nr/.temp/preflight-<timestamp>.json`.

## Suggested Skills

- `preflight`: revise and re-run the plan review after hardening the spec.
- `frontend-design`: use for the terminal UX spec, interaction states, dense information hierarchy, and keyboard-first polish.

## Next Steps

1. Revise the plan with explicit v1 decisions:
   - Config format: TOML only.
   - Edit flow: external `$EDITOR` handoff only for v1; no complex in-TUI text editor.
   - Ledger write safety: temp file, fsync, rename, permission preservation, backup for repair.
   - Conflict model: detect ledger mtime/hash changes before write; reload and ask user rather than overwrite.
   - Path safety: expand `~`, reject unsafe symlink writes for repair/fix operations, handle permission errors clearly.
   - Editor execution: direct `exec.Command` argv; no shell.

2. Define the Markdown grammar:
   - Canonical task line fields, token ordering, duplicate token policy, unknown metadata preservation, multiline notes, checkbox/status conflict rules, duplicate/missing ID behavior.

3. Define TUI states:
   - first-run, empty, no-results, loading/reloading, saving, success, parse-error, conflict, readonly, external-editor-return, repair-preview.

4. Expand tests:
   - Parser fixtures and golden round trips.
   - CLI integration with temp HOME.
   - Atomic write and conflict tests.
   - Config precedence matrix.
   - Editor failure tests using fake editor scripts.
   - Filesystem edge cases: permissions, symlinks, missing directories, path expansion.
   - TUI model tests for keybindings, resize, parse errors, and persistence.

5. Ask the user whether to approve the revised spec for implementation.

# AGENTS.md

This file defines the rules, conventions, architectural routing, and execution standards that AI agents must follow when working in this repository.

---

## Rule Priority

When rules conflict, follow this order:

1. **User instructions** — always take precedence.
2. **This file** — applies to all tasks across all agent interfaces and environments.
3. **Language-specific standards** (`.instructions/*.md`, listed at the end of this file) — they add rules for their tasks, but never override this file: when one conflicts with this file, this file wins.
4. Existing project conventions.
5. Default AI behavior.

Never override explicit user instructions.

---

## Language

| Context | Language |
|---|---|
| Responses to the user | Brazilian Portuguese (pt-BR) |
| Documentation (`*.md`) | English — except translated mirrors a project standard requires (e.g. `docs/pt-br/`) |
| Code comments | English |
| User-facing strings (UI, CLI output) | Follow the project's existing language |
| Commit messages | English |

Do **not** mix languages inside the same file.

---

## Code Comments

- Write every comment (functions, types, blocks, inline) in **English**.
- A comment explains **strictly what the code does** — its behavior, inputs, outputs and side effects.
- Never reference external material in a comment: no documentation pages, code reviews, issues, tickets, conversations, decision logs or other files.

---

## Documentation Maintenance

- When a change alters a public API, CLI command/flag, configuration option, or other user-visible behavior, update the relevant documentation **in the same change** — do not defer it to a follow-up.
- **`CHANGELOG.md`**: if the project already has one, add an entry for every user-facing change (skip internal refactors with no visible effect). Do not create a `CHANGELOG.md` for a project that has never had one — ask first.
- If the CHANGELOG follows Keep a Changelog, add entries under `[Unreleased]`; never edit released sections.
- **`README.md`**: keep usage examples, flags, and setup/installation steps in sync with the code. Do not restructure or rewrite unrelated sections while doing this.
- Internal-only changes (refactors, renamed private helpers, test-only edits) do not require a documentation update.

---

## Agent Behavior

- Make **minimal and precise changes**.
- Modify **only files relevant to the task**.
- Prefer **editing existing files** over creating new ones; create a file only when the task needs it (new tests included).
- Respect the **existing project structure**.
- Prefer **simple and readable** solutions.
- Avoid unnecessary refactoring or large rewrites; if one is truly required, **ask the user before proceeding**.

### Verification

- **Investigate before changing:** read the relevant code and confirm external facts (package names, CLI flags, versions) instead of assuming them.
- **Run the project's checks** (tests, linters, build) before reporting a task as done.
- **Report results faithfully:** if something failed or was skipped, say so with the output.

---

## Communication & Decision Making

- Respond **concisely**. Match response length to request complexity.
- Do not add preamble before acting ("I'll now look at X and then...") — just act.
- End with a short report — what changed, what was verified, what is pending. Don't restate the diff.

### Requirement Gathering & Non-Deduction Protocol

Ask only when there is genuine ambiguity or more than one reasonable solution; when the request is clear, proceed. When you do ask:

1. **Do not assume or infer details.** Never guess architecture, design choices, API formats, or missing logic.
2. **Analyze first:** do a preliminary analysis of the code and the request, then ask every open question at once.
3. **Provide selectable options:** present each question with its options through the agent interface's selectable-question tool (e.g. a multiple-choice prompt), so the user picks an option instead of typing it. Only when the interface has no such tool, list numbered questions with lettered options.
4. **Highlight the recommended option:** mark it as **[RECOMENDADO]** (in the response language) and briefly state the technical tradeoff or reason for the recommendation.

---

## Git Operations

- **Allowed without asking:** read-only commands (`status`, `diff`, `log`, `show`).
- **Never without explicit confirmation:** `commit`, `branch`, `tag`, `stash`, `push`, `reset --hard`, `rebase`, `clean`, `branch -D`, force operations, and anything that changes a remote or a hosting service (e.g. `gh` commands that create or modify).
- When a task needs any of these, do not run them: prepare the changes and give the user the exact commands to run in their own terminal.

---

## Model Selection for Subagents

When delegating to subagents, pick the smallest model tier that can do the job safely:

| Tier | Use for | Model |
|---|---|---|
| Small | Mechanical work: search, formatting, single-file micro-fixes | The platform's smallest/fastest model |
| Medium | Scoped features, tests, multi-file updates, debugging | The platform's default model |
| Large | Architecture, cross-domain trade-offs, critical refactors | The platform's most capable model |

---

## Circuit Breaker

To prevent infinite loops, token waste, and unintended codebase degradation, agents must observe strict self-throttling rules.

### Trigger Conditions

An agent must **immediately halt execution** if:

1. It encounters **> 2 consecutive failed attempts** on the same compilation, test, or execution error.
2. The planned change unexpectedly requires modifying **> 3 contextually unrelated files**.
3. A subagent or smaller model identifies that the task exceeds its assigned tier.

### Report

Stop and report to the user, in the response language: what was attempted, the last error (with output), files modified so far, and a suggested next step.

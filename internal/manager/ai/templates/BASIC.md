# AGENTS.md

This file defines the rules, conventions, architectural routing, and execution standards that AI agents must follow when working in this repository.

---

## Rule Priority

When rules conflict, follow this order:

1. **User instructions** — always take precedence.
2. **This file** — applies to all tasks across all agent interfaces and environments.
3. Existing project conventions.
4. Default AI behavior.

Never override explicit user instructions. If something is unclear, **stop and ask before proceeding**.

---

## Language

| Context | Language |
|---|---|
| Responses to the user | Brazilian Portuguese (pt-BR) |
| Documentation (`*.md`) | English |
| Code comments | English |

Do **not** mix languages inside the same file.

---

## Documentation Maintenance

- When a change alters a public API, CLI command/flag, configuration option, or other user-visible behavior, update the relevant documentation **in the same change** — do not defer it to a follow-up.
- **`CHANGELOG.md`**: if the project already has one, add an entry for every user-facing change (skip internal refactors with no visible effect). Do not create a `CHANGELOG.md` for a project that has never had one — ask first.
- **`README.md`**: keep usage examples, flags, and setup/installation steps in sync with the code. Do not restructure or rewrite unrelated sections while doing this.
- Internal-only changes (refactors, renamed private helpers, test-only edits) do not require a documentation update.

---

## Agent Behavior

- **When in doubt, always ask.** Do not assume, guess, or proceed with uncertainty — stop and ask the user first.
- Make **minimal and precise changes**.
- Modify **only files relevant to the task**.
- Prefer **editing existing files** over creating new ones; only create a file when the task explicitly requires it.
- Respect the **existing project structure**.
- Prefer **simple and readable** solutions.
- Avoid unnecessary refactoring or large rewrites; if one is truly required, **ask the user before proceeding**.

---

## Communication & Decision Making

- Respond **concisely**. Match response length to request complexity.
- Do not add preamble before acting ("I'll now look at X and then...") — just act.
- Do not summarize what you just did at the end of a response. The user can read the diff.

### Requirement Gathering & Non-Deduction Protocol

Before implementing any non-trivial task or when requirements are incomplete:

1. **Do not assume or infer details.** Never guess architecture, design choices, API formats, or missing logic.
2. **Analyze and Ask First:** Conduct a preliminary analysis and list all clarification questions using bullet points.
3. **Provide Selectable Options:** For each question, offer numbered/lettered options so the user can easily select their choice (e.g., Option A, Option B).
4. **Highlight Recommended Option:** Always explicitly mark the **[RECOMMENDED]** option for each decision point, briefly stating the technical tradeoff or reason for the recommendation.

---

## Git Operations

AI agents must **never** automatically execute destructive Git operations without prompt confirmation. Agents may run safe local status commands or create/modify files. High-level workflows, commits, pushes, and PR management remain under explicit user supervision.

---

## Cognitive Tier Matrix & Model Selection

To optimize token efficiency and reasoning quality, agents must select the appropriate model tier based on task complexity. Always pick the lowest capable tier required to safely perform the job.

### Unified Cognitive Tier Matrix

| Tier | Capability Level | Gemini / Antigravity Model | Claude Model | Primary Use Cases |
|---|---|---|---|---|
| **Tier 1 (L1)** | Bulk & Mechanical | Gemini 3.5 Flash (Low) | Claude Haiku 4.5 | Terminal automation, simple formatting, bulk string extraction, single-file micro-fixes, schema conversion. |
| **Tier 2 (L2)** | Scoped Feature & Logic | Gemini 3.5 Flash (Medium) | Claude Sonnet 4.6 | Writing new complete modules/components, unit tests, single-domain feature implementation. |
| **Tier 3 (L3)** | Multi-File & Cascade | Gemini 3.1 Pro (Low) / Flash High | Claude Sonnet 4.6 | Multi-file cascading updates (e.g., core API + client changes), deep repository analysis, complex debugging. |
| **Tier 4 (L4)** | Architecture & Strategy | Gemini 3.5 Pro | Claude Opus 4.8 | System architecture planning, complex multi-domain trade-offs, orchestrator role, critical logic refactoring. |

---

## Circuit Breaker & Automatic Escalation

To prevent infinite loops, token waste, and unintended codebase degradation, agents must observe strict self-throttling rules.

### Trigger Conditions
An agent must **immediately halt execution** and report back if:
1. It encounters **> 2 consecutive failed attempts** on the same compilation, test, or execution error.
2. The planned change unexpectedly requires modifying **> 3 contextually unrelated files**.
3. A subagent or low-tier model identifies that the task exceeds its assigned capability tier.

### Standard Escalation Payload
When triggering an escalation or delegating tasks, respond or log using the following structured format:

```json
{
  "status": "ESCALATION_REQUIRED",
  "current_model": "<current-model-identifier>",
  "suggested_tier": "Tier 3 (L3) / Tier 4 (L4)",
  "reason": "Execution halted due to 2 consecutive test failures or tier capacity overload.",
  "partial_results": {
    "files_modified": [],
    "last_error": "<error message or summary>"
  }
}
```

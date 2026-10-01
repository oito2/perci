🌐 [Português](../../pt-br/architecture/managers.md) | **English** | 🏠 [Index](../index.md)

---

# Managers (`internal/manager/*`)

Backs the **Dev Tools** GUI category — Repositórios (`manager/repo`, `manager/gitignore`) and IA: Contextos (`manager/ai`) — plus the Backup/Restore actions on **Docker → Gerenciar Containers** (`manager/db`); see [Docker application stack](./appstack.md).

## `manager/ai` — AI Context Generation

Generates AI agent context files for a **target project** (the folder picked via the GUI's folder picker, `PickAIContextFolder`) — `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, and `.instructions/*.md` language/topic guides, from a catalog defined in `context.go` (`models`).

- Model/guide selection is a checklist in the GUI's **IA: Contextos** screen (`GetAIContextItems`/`ApplyAIContext`, `cmd/prci-gui/service_aicontext.go`, part of `DevToolsService`), one entry per `manager/ai.Models()` catalog item; an empty selection is valid and generates only the general rules, no language section.
- Clicking **Aplicar** always overwrites (`GenerateSharedFiles(..., overwrite=true, ...)`) with no extra confirmation dialog — the click itself is the confirmation, the same convention every other non-interactive GUI action follows. (`ConfirmOverwrite`, which had CLI/TUI/non-interactive variants, was deleted along with the TUI/CLI.)
- `.instructions/*.md` files referenced via `@.instructions/...` in the generated `AGENTS.md` are actually written to disk (`WriteInstruction`), not just referenced.
- The generated `AGENTS.md` lists active standards as an agent-agnostic index: an explicit "read the applicable file before starting" instruction, then one line per model with its `AppliesTo` text (which tasks it covers) and the `@.instructions/...` path. Only Claude Code turns `@path` into content (via `CLAUDE.md`'s `@AGENTS.md` import); Antigravity turns a bare `@path` into a path reference and Codex has no import mechanism at all, so for both the index text is what makes the agent open the right file. Inlining the files instead was ruled out: two standards already overflow Codex's 32 KiB `AGENTS.md` budget and Antigravity's 24 KB per-rule cap.
- The `PROJECT-DOCUMENTATION.md` guide in the catalog ("Documentação de projeto") — the standard this documentation site itself follows — is optional, not forced onto every generated project; see its own "When NOT to Use" section.
- Project-type documentation complements ("Documentação: Servidor MCP/Plugin Moodle/Aplicação Desktop/Aplicação Mobile/Aplicação Web" → `PROJECT-DOCUMENTATION-{MCP,MOODLE,DESKTOP,MOBILE,WEB}.md`) layer type-specific rules on top of that base. A catalog `Model` can declare `Requires` (another model's name); `GenerateSharedFiles` pulls any required model in automatically when it wasn't selected (`withRequired`), logs that it did, and keeps catalog order so `AGENTS.md` references the base before its complements.
- A `Model` can also list `Legacy` filenames (pre-rename names of its instruction file, e.g. `DOCUMENTATION-SITE.md` for the base): `DetectActiveModels` still counts them as active, and an overwrite run (every GUI **Aplicar**) deletes them after writing the current file — or when the model is deselected — so projects migrate on their next apply.
- Moodle-specific placeholders (`{{MOODLE_VERSION}}`, `{{MOODLE_FULLVERSION}}`, `{{MOODLE_PATH}}`, `{{WORKSPACE_PATH}}`) are auto-filled by detecting `version.php` (or `public/version.php`, Moodle 5.1+ layout) in the picked folder; when it isn't found, the literal placeholders are left in place and a warning is shown in the Execução terminal. `version.php` reads are capped at 1 MiB.
- There's no bulk "clear all generated files" action in the GUI — `RemoveSharedFiles` (the function behind the old `prci ai clear` command) still exists in the package but has no caller anywhere; unchecking a previously-selected model in **IA: Contextos** only removes that model's own `.instructions/*.md` file (`GenerateSharedFiles` diffing against `DetectActiveModels`), not the shared `AGENTS.md`/`CLAUDE.md`/ignore files — and only while that file is still exactly what Perci generated (`isPristine`): an edited or hand-written copy is kept, with a warning. Old-named copies (`Model.Legacy`) are removed on overwrite runs, as before.

## `manager/db` — Database Operations

MariaDB backup (`Backup`), restore (`Restore`), and optimization (`Optimize`, `OptimizeMoodle` — Moodle-specific tuning). No dedicated GUI screen of its own — invoked directly by [Docker → Gerenciar Containers](./appstack.md#docker--gerenciar-containers)'s Backup/Restore actions on the MariaDB row, against `appstack.MariaDBContainerName` and `cfg.Docker.MariaDB.DBUser`/`DBPass`. `Optimize`/`OptimizeMoodle` aren't wired up on the new stack (deliberately out of scope — see [Docker application stack](./appstack.md)). Backup/restore paths are validated and `~`-expanded before use. The backup is a `mariadb-dump --all-databases` with `--single-transaction --skip-lock-tables` (a consistent snapshot without locking tables) and `--routines --events` (stored routines and events included).

## `manager/repo` — Git Repository Control

Global Git identity configuration (`ConfigureGlobal`), repo init/clone with per-repo identity (`Init`, `Clone`), local identity override (`ApplyLocalIdentityAt`), and Code of Conduct file generation (`CreateConduct`: `CODE_OF_CONDUCT.md` at the root plus its Portuguese mirror at `docs/pt-br/codigo-de-conduta.md`, from embedded Contributor Covenant templates, with the reporting contact e-mail the user types in **Repositórios → Arquivos** — pre-filled with the folder's Git e-mail; no fixed address is ever written. A `CODIGO_DE_CONDUTA.md` left at the root by older versions is reported, not deleted). `Clone` clones straight into the picked folder, so it refuses a folder that isn't empty (`IsEmptyDir` — hidden files count) before running `git`; the GUI checks the same thing up front (`RepoFolderState.IsEmpty`) and shows a warning instead of the form.

The folder cards come from `config.yaml`'s `repo_folders`: **Novo repositório** adds one, **Remover** forgets one (nothing on disk is touched), and entries whose folder no longer exists are pruned whenever the list is read.

## `manager/gitignore`

Generates a `.gitignore` tailored to the project (`Generate`). The project type is detected from marker files at the folder's **root** only (`DetectStacks`, a dictionary in `gitignore.go`), independent of `.instructions/`:

| Type | Marker | Type | Marker |
|---|---|---|---|
| Go | `go.mod` | Node.js | `package.json` |
| Flutter | `pubspec.yaml` with `sdk: flutter` | Python | `pyproject.toml`, `requirements.txt`, `setup.py` |
| Dart | `pubspec.yaml` without Flutter | Ruby | `Gemfile` |
| Moodle | `version.php` with `$plugin->component` | Rust | `Cargo.toml` |
| PHP | `composer.json` | Java | `pom.xml`, `build.gradle`, `build.gradle.kts` |
| Shell | any `*.sh` | | |

Editors and OS sections are always included; with no type recognized, generic sections (environment, logs, build/dependencies) are used instead. Several types can match at once, and a pattern shared by two sections is written once. An existing `.gitignore` is **merged, never overwritten**: its content stays as is and only the missing patterns are appended, under their section title — running it again adds nothing.

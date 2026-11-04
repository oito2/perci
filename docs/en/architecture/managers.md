🌐 [Português](../../pt-br/architecture/managers.md) | **English** | 🏠 [Index](../index.md)

---

# Managers (`internal/manager/*`)

Backs the **Repositórios** GUI category (`manager/gitaccount`, with `manager/repo` and `manager/gitignore` for the per-repository files), the **Dev Tools → IA: Contextos** screen (`manager/ai`) and the Backup/Restore actions on **Docker → Gerenciar Containers** (`manager/db`); see [Docker application stack](./appstack.md).

## `manager/ai` — AI Context Generation

Generates AI agent context files for a **target project** (the folder picked with `PickAIContextFolder`) on the **Dev Tools → IA: Contextos** screen (`cmd/prci-gui/service_aicontext.go`, part of `DevToolsService`), for the checked agents and standards.

- **Agents** (`agents.go`): Claude Code, Gemini (CLI and Code Assist), ChatGPT CLI (Codex), OpenCode and Antigravity, one checkbox each (all checked for a folder without `AGENTS.md`). `AGENTS.md` ends with `<!-- Perci: generated for … -->`, which `DetectAgents` reads back (an older `AGENTS.md` counts Codex, Antigravity and the agents whose files exist).
- **Files** (`GenerateSharedFiles`): always `AGENTS.md`, the checked standards under `.instructions/` and `.instructions/PROJECT.md`; then, per agent:

  | Agent | Files |
  |---|---|
  | Claude Code | `CLAUDE.md` (`@AGENTS.md` + `@.instructions/PROJECT.md`); `.claude/rules/perci/<STANDARD>.md` — a copy of each file-type standard under a `paths:` frontmatter, so Claude Code loads it only when it works on a matching file; `permissions.deny` read rules merged into `.claude/settings.json` |
  | Gemini | `GEMINI.md` (`@./AGENTS.md` + `@./.instructions/PROJECT.md`); `.aiexclude` (Code Assist) and `.geminiignore` (CLI) |
  | OpenCode | `.instructions/PROJECT.md` added to `instructions` in `opencode.json` (OpenCode doesn't follow references in AGENTS.md); other keys kept, `opencode.jsonc` never touched |
  | Codex, Antigravity | `AGENTS.md` only |

  Unchecking an agent removes its files while they are still exactly what Perci wrote (an edited or hand-written file is kept, with a warning), Perci's entry from `opencode.json` and Perci's rules from `settings.json`.
- **AGENTS.md** is `templates/BASIC.md` plus the list of checked standards. Each entry names its file in a code span with its "applies to" line, never as an `@` import: Claude Code would load every standard on every session. Agents read a standard before a task it covers. With every standard checked, `AGENTS.md` stays around 11 KB (under Antigravity's 24,000-byte rule cap and Codex's 32 KiB).
- **Project rules** (`project.go`): `.instructions/PROJECT.md` is created once and never rewritten. It is second in AGENTS.md's priority (after the user's instructions), and agents write the rules the user defines for the project there. On an Aplicar, lines of an existing `AGENTS.md` that no Perci version generated (`templates/legacy/` keeps the earlier rule sets) are moved into a "Moved from AGENTS.md (date)" section first, never twice.
- **Read blocking** (`claude_settings.go`): one canonical list (`templates/ignore/patterns`) — secrets, dependencies, VCS, caches, logs, minified files, audio/video/fonts, archives and binaries. Images, PDFs and lockfiles stay readable. `dist/`, `build/`, `out/`, `vendor/`, `env/` and `.next/` are left out when `git ls-files` shows tracked files under them. Claude Code gets `Read(...)` rules (a directory pattern becomes `Read(dir/**)`, `.claude/` is skipped), and every rule an earlier version generated but the current list doesn't is removed, keeping the user's own rules. Gemini gets the same list as `.aiexclude`/`.geminiignore`. **OpenCode gets no rules**: its documented `permission` setting is deprecated in its configuration schema and neither form documents how a path pattern matches. Codex and Antigravity have no project-level mechanism.
- **Screen**: standards grouped by base with their complements indented, a Portuguese description each (`Model.Description`; `AppliesTo` stays English for the agents), and a state badge from `GuideStatus` — applied, outdated (identical to a committed earlier version, `template_history.go`, guarded by `TestTemplateHistory`) or edited. A folder without standards starts on the project type its files suggest (`SuggestPreset`). **Aplicar** first shows the files to create, update or remove (`Preview`: the real generation on a temporary copy, with git's listing from the project; links are copied as content, never written through).
- The catalog covers Bash, Go (with the MCP server and Wails v3 complements), Rust + Tauri v2 (with Vanilla TS, SvelteKit and React complements), PHP + Slim 4 (with PDO, Doctrine DBAL and Eloquent complements), Moodle, Dart + Flutter, Godot 4, MySQL/MariaDB, TypeScript/Node, Docker/Compose, CI (GitHub Actions), Website, Landing Page Design and the documentation standards. Every standard has a `Last verified` date and ends with a `Quality` section. A model can declare `Requires` (pulled in automatically, base listed first) and `Legacy` file names (migrated on Aplicar).
- **Moodle placeholders**: `{{MOODLE_VERSION}}`/`{{MOODLE_FULLVERSION}}` are filled from a Moodle checkout's `version.php` (or `public/version.php`) or from a plugin's `$plugin->requires`; with neither, they read "unknown (…)" and a warning is shown — no raw placeholder is ever written. `version.php` reads are capped at 1 MiB.
- **Generated files and symlinks**: written with `fsutil.WriteGenerated` — through a symlink, keeping a `<file>.perci-bak` of a different previous version. A `CLAUDE.md`/`GEMINI.md` that links to `AGENTS.md` is left as is.
- **Folder approval**: the target folder must have been picked in the native folder dialog during the session (or be a folder already registered); otherwise the GUI refuses with "pasta não escolhida no seletor".

## `manager/db` — Database Operations

MariaDB backup (`Backup`) and restore (`Restore`). No dedicated GUI screen of its own — invoked directly by [Docker → Gerenciar Containers](./appstack.md#docker--gerenciar-containers)'s Backup/Restore actions on the MariaDB row, against `appstack.MariaDBContainerName` and `cfg.Docker.MariaDB.DBUser`/`DBPass`. Backup/restore paths are validated and `~`-expanded before use. The backup is a `mariadb-dump --all-databases` with `--single-transaction --skip-lock-tables` (a consistent snapshot without locking tables) and `--routines --events` (stored routines and events included). The dump is written to a temporary file next to the destination and replaces it only when it completes.

## `manager/gitaccount` — Git Accounts

Backs the **Repositórios** category (`cmd/prci-gui/service_repositories.go`, `RepositoriesService`). The user guide is [Git accounts and repositories](../guides/workflows/repositories.md). An account (`config.GitAccount`, `git_accounts` in config) is a provider user with a folder, a commit identity and an SSH key:

- **Validation** (`Normalize`, `Validate`): unique name and ID (the slug of the name), a known provider, host and user name formats, a port up to 65535, and a folder that is absolute, exists and neither contains nor lies inside another account's folder (also compared with symlinks resolved).
- **Git configuration** (`Apply`, `Remove`, `Forget`): `WriteGitconfig` writes `~/.perci/git/<id>.gitconfig` (`[user]` name/e-mail and `core.sshCommand`) with `git config --file` on a temporary file renamed over it; one `includeIf.gitdir:<folder>/.path` value in the global configuration points to it, added and removed only with `git config --global`. `core.sshCommand` is `ssh -i <key> -o IdentitiesOnly=yes -o UserKnownHostsFile=~/.perci/git/known_hosts` (plus the port); a key or known_hosts path with whitespace is refused.
- **Server keys** (`knownhosts.go`): the official keys of github.com and gitlab.com are built in (`TrustPinnedHostKeys`); a self-hosted server is scanned with `ssh-keyscan` (`ScanHostKeys`) and its keys are written only after the user confirms the fingerprints — the service accepts only the keys of its own last scan.
- **SSH keys** (`sshkey.go`): `GenerateKey` creates an ed25519 pair at `~/.ssh/perci_<id>` and never replaces an existing file. Passphrases reach `ssh-keygen` and `ssh-add` through `SSH_ASKPASS` (with `SSH_ASKPASS_REQUIRE=force`), never on a command line: the askpass program is Perci's own executable (`IsAskpassCall`/`RespondAskpass`), which answers a limited number of times so a wrong passphrase fails at once. `AgentHasKey` reads `ssh-add -l`; `UnlockKey` loads the key into the session's agent. `TestConnection` runs `ssh -T` and checks that the greeting names the account user.
- **Token** (`token.go`): stored, read and deleted with `secret-tool`; `config.yaml` only records `has_token`.
- **Repositories** (`repos.go`): `Discover` walks the account folder, never following symlinks or entering hidden folders, `node_modules`, `vendor` or a repository (a `.git` file counts, for worktrees and submodules). `ReadRepo` reads the local state only — `git config --local --get-regexp`, `git --no-optional-locks status --porcelain=v2 --branch`, `git log -1` — up to 8 repositories at a time; "last Verificar" is the modification time of `.git/FETCH_HEAD`. `ClassifyRemote` sorts the origin: SSH of the account (`ok`), HTTPS of the account (`https`), none, or another owner/host (`foreign`, listed apart).
- **Network** (`sync.go`): every network command runs with `GIT_TERMINAL_PROMPT=0`, empty `GIT_ASKPASS`/`SSH_ASKPASS` and `GIT_SSH_COMMAND` = the account's `core.sshCommand` plus `BatchMode=yes` and `ConnectTimeout=15`, so it never prompts; 90 seconds per command, 30 minutes per clone. `Fetch` runs `git fetch --prune origin` 4 at a time; `Pull` fetches, re-reads the state and runs `git merge --ff-only @{upstream}`, skipping (not failing) repositories that can't fast-forward.
- **Actions** (`actions.go`, `newrepo.go`): `FixURL` checks the SSH URL with `git ls-remote` before `git remote set-url`; `SetRemote` and `Init` take a repository name (`group/name` for GitLab; `..` refused); `CloneURL` accepts a name or a URL of the account only; `DestDir` confines a new repository to a new or empty folder strictly inside the account folder and outside other repositories; a failed `Clone` removes the parent folders it created. Every service method that receives a folder re-lists the account's repositories and accepts only a listed one.
- **Provider API** (`remoteapi.go`): `ListRemoteRepos` lists the repositories the account user owns — GitHub `/user/repos?affiliation=owner` with a token or `/users/<user>/repos?type=owner` without, at `api.github.com` or `<host>/api/v3`; GitLab `/projects?owned=true` or `/users/<user>/projects`, at `<host>/api/v4` — 100 per page, up to 10 pages, 20-second timeout. The token goes only in a request header, and the SSH URL is built locally, not taken from the response.
- **Export/import** (`transfer.go`): the file is YAML with `version: 1` and the accounts with their repositories (`path` relative to the account folder, SSH `url`); unknown keys are refused, so the Docker export can't be read as one. `PlanImport` matches an account to a registered one by provider, host and user; `Import` merges into it (new pending repositories only) or creates the account — folder, key (`~/.ssh/perci_<id>`, reused when present, otherwise generated without a passphrase), built-in server keys, `Apply` — and records the repositories in `pending`. `ClonePending` clones them one at a time and drops the cloned ones from `pending`; the service first runs the connection test.

## `manager/repo` — Global Identity and Code of Conduct

Global Git identity (`ConfigureGlobal`/`GetGlobalIdentity`, the **Identidade padrão** tab of Configurar Conta) and Code of Conduct generation (`CreateConduct`, a per-repository action on an account screen: `CODE_OF_CONDUCT.md` at the root plus its Portuguese mirror at `docs/pt-br/codigo-de-conduta.md`, from embedded Contributor Covenant templates, with the reporting contact e-mail the user types — pre-filled with the repository's own e-mail or, without one, the account's; no fixed address is ever written. A `CODIGO_DE_CONDUTA.md` left at the root by older versions is reported, not deleted). `ConfigureGlobal` never overwrites a `credential.helper` that is already configured; when it has to choose one and `git-credential-libsecret` can't be found, it falls back to `cache` (password kept in memory for 15 minutes) and prints a warning pointing to **Pré-requisitos → Libsecret Tools**. `Init`, `Clone` and `ApplyLocalIdentityAt` served the removed Dev Tools → Repositórios screen and are no longer called by the GUI.

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

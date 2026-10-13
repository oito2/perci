🌐 [Português](../../pt-br/architecture/dev-environment.md) | **English** | 🏠 [Index](../index.md)

---

# Dev Environment (`internal/dev/*`)

Backs the **Desenvolvimento** GUI category.

| Package | Responsibility |
|---|---|
| `prereqs` | 23 essential items (CMake, Ninja, Clang, the GTK3/GTK4/WebKitGTK 6.0 development headers, Git, curl, mkcert, `librsvg2-bin`, libsecret with its `libsecret-1` development files, etc.) plus grouped Docker Engine/Node.js installs. On Fedora, Docker Engine is the distribution's `moby-engine` + `docker-compose` + `docker-buildx`, and the GitHub CLI is the native `gh` package. Each item has its own Instalar/Atualizar/Desinstalar (`InstallOne`/`UpdateOne`/`UninstallOne`): updating upgrades only the item's own packages (`apt-get install --only-upgrade` after an index refresh in the same password prompt, or `dnf upgrade`), moves Node.js to the latest LTS through nvm carrying over the global npm packages, and reinstalls the version-pinned Wails3 CLI and AppImageTool. `Prereq.Requires` lists what must be installed first (Curl for the GitHub CLI, Node.js and AppImageTool; Flatpak for Flatpak Builder; Go, from Linguagens e SDKs, for the Wails3 CLI): `Catalogue` lists those requirements first, and `MissingRequirements` keeps Instalar disabled with a "Requer: …" hint until they are installed. Uninstalling always asks for confirmation; removing Node.js deletes the whole `~/.nvm` folder (every nvm-installed Node version and its global npm packages), and the confirmation spells that out, and it also removes the nvm lines from the shell rc files. Removing any package-based item (this includes the terminals and IDEs below that go through the same helper: Alacritty, GNOME Console, VS Code, VSCodium) first simulates the removal and **refuses** — "também removeria …" — when the package manager would take other packages with it. Libsecret Tools never removes the shared `libsecret-1-0` library on Debian/Ubuntu, and on Fedora also installs `git-credential-libsecret`. Docker Engine's description warns that the `docker` group is equivalent to root access. Unprivileged network steps (nvm, `npx`, `git`, `curl` downloads) are bounded by a 30-minute limit (`internal/dev/nettimeout`); `apt`/`dnf` are never interrupted. |
| `golang` | Go SDK installer. `LatestRelease(ctx)` fetches the releases JSON once and returns both the version and its checksum together (`Install(ctx, exe, stdout, version, checksumSHA256)` — pass an empty checksum to fall back to fetching it). The privileged part is one batch (one password prompt): root-owned copy of the tarball, checksum checked again, extraction to `/usr/local/go-staging`, then swap with rollback (`installScript`). Local PATH exports via `internal/shellrc`, with a quoted line that also works in fish (the older unquoted one is replaced/removed). |
| `flutter` | Flutter SDK via `git` checkout, PATH exports, automatic Android tooling acceptance, `flutter doctor` wrapper. The install accepts the Android SDK licenses on the user's behalf (`sdkmanager --licenses`); the Flutter description on the **Linguagens e SDKs** screen says so and links the terms. An existing `java` is accepted only when it is version 17 or newer (what the Android command-line tools need); otherwise JDK 21 is installed, in the same privileged batch as the Flutter prerequisites. The JDK comes from `openjdk-21-jdk` on Debian/Ubuntu and from Eclipse Temurin 21 (Adoptium's signed repository) on Fedora, whose own repositories no longer ship JDK 21. The Android cmdline-tools archive (a pinned build) is verified against the SHA-1 that Google's package manifest (`repository2-3.xml`) lists for it before extraction, and staged in the same parent directory as the destination to avoid `EXDEV` cross-filesystem errors on rename. |
| `sdks` | The **Linguagens e SDKs** screen: Go and Flutter, each with its own Instalar/Atualizar/Desinstalar (`InstallOne`/`UpdateOne`/`UninstallOne`), same table as Pré-requisitos. Updating Go installs the latest stable release only when it is newer than the installed one; updating Flutter runs `flutter upgrade`. `SDK.Requires` names the Pré-requisitos items each installer needs — Curl for Go (the tarball is downloaded with `curl`), Git for Flutter (`git clone`) — and `MissingRequirements` keeps Instalar disabled with a "Requer: …" hint until they are installed. Uninstalling always asks for confirmation. |
| `ide` | VS Code, VSCodium, and Zed Editor setup. The VS Code/VSCodium installers don't add a second apt source when a `packages.microsoft.com/repos/code` source already exists (it would conflict on `Signed-By`); uninstalling Zed also removes its `.desktop` entry. |
| `llm` | Unified manager for Claude Code, Antigravity CLI, Codex, OpenCode. Uninstalling also removes what the official installers leave besides the binary: `~/.local/share/claude` (when `claude` is the native install's link into it) and OpenCode's `~/.opencode/bin` plus its `# opencode` rc lines; user settings are kept. Codex is pinned (`@openai/codex@0.162.1`). |
| `terminal` | Kitty, Alacritty, GNOME Console wrappers, Starship prompt, file-manager context-menu integrations. |
| `mcpservers` | The **IA: MCPs** screen: registers MCP servers with Claude Code, Codex and Antigravity (see [`mcpservers`](#ai-mcps-and-skills)). |
| `agentskills` | The **IA: SKILLs** screen: installs third-party agent skills through the `skills` CLI (see [`agentskills`](#ai-mcps-and-skills)). |
| `nettimeout` | The 30-minute limit and the `curl` timeout flags shared by every unprivileged network command. |
| `localbin` | Shared helper for ensuring `~/.local/bin` is exported on `PATH` (`EnsureInPath`, built on `internal/shellrc`, written to the current shell's rc file — `fish_add_path` for fish). `RefreshPath` rebuilds the GUI process's own `PATH` (`~/.local/bin` + nvm's default Node) at startup and after every action, so a tool installed there during the session is found without restarting Perci. |

## Two Catalogs, Two GUI Screens

`llm` and `terminal` each get their own GUI screen ("Aplicativos: IA" and "Aplicativos: Terminais", both under **Desenvolvimento**) rather than being merged into one — the split happens only at the presentation layer; the underlying catalogs stay in their own packages. (`mcp`, a third catalog a single merged TUI screen used to also cover before the TUI/CLI split was removed, was itself removed 2026-09-11 in favor of the separate, GUI-only "IA: MCPs" screen — see `internal/dev/mcpservers`.) See [Adding a New LLM, IDE, or Terminal](../../../CONTRIBUTING.md#adding-a-new-ai-app-ide-or-terminal).

### Terminals: Context Menu and Starship

- **Context menu** — every "Aplicativos: Terminais" run ends with `terminal.SyncContextMenuEntries`, which matches the "Abrir no <terminal>" entries to what is actually installed (`InstalledMap`, after refreshing `PATH`): created for every installed terminal with a `DirFlag` — including ones installed outside Perci — and removed for the others, even when an item of the run failed. Entries live in the user's home, so no password is asked: Nautilus and Nemo scripts (`~/.local/share/{nautilus,nemo}/scripts/Abrir no <terminal>` — Nautilus shows them under the **Scripts** submenu) and a Dolphin service menu (`~/.local/share/kio/servicemenus/perci-<terminal>.desktop`).
- **Starship** — not a checklist item; two extra buttons on the same screen: **Aplicar Starship** (`InstallStarship`) and **Remover Starship** (`UninstallStarship`, with a confirmation), which deletes `~/.local/bin/starship` and the init lines in bash/zsh/fish, keeping `~/.config/starship.toml` as `starship.toml.perci-bak`.

## Pinned Versions

Nothing Perci downloads or runs through a package runner is "whatever is latest today". The official installers of Starship, Claude Code, Kitty and similar tools are the exception: they are fetched over HTTPS with `curl` timeouts (`internal/dev/nettimeout`) and always give the current release.

| Used by | What is pinned | Where |
|---|---|---|
| Dev Tools → IA: SKILLs | `skills@1.7.0` (npm, [vercel-labs/skills](https://github.com/vercel-labs/skills)); every skill is installed from its repository at a full commit SHA (`https://github.com/<owner>/<repo>#<sha>`) | `agentskills.skillsCLI`, `Skill.Ref` |
| Dev Tools → IA: MCPs (Filesystem) | `@modelcontextprotocol/server-filesystem@2026.8.31` (npm) | `mcpservers.filesystemServerPkg` |
| Dev Tools → IA: MCPs (SQLite) | `mcp-server-sqlite@2025.4.25` (PyPI, via `uvx`) | `mcpservers.sqliteServerPkg` |
| Dev Tools → IA: MCPs (Dart + Flutter, Codex) | the `flutter/agent-plugins` marketplace at a fixed commit | `mcpservers.dartFlutterCodexMarket` |
| Pré-requisitos → Node.js | the nvm installer, by tag (`v0.40.8`) | `prereqs.nvmVersion` |
| Aplicativos: IA → Codex | `@openai/codex@0.162.1` (npm) | `llm.codexPackage` |
| Linux → Fontes → JetBrains Mono NF | the release archive, by version and SHA-256 | `fonts` catalogue |

To upgrade one, check the new release (`npm view <pkg> version`, pypi.org, the repository's latest commit), bump the constant and rebuild. An MCP server already registered keeps the version it was registered with until it's updated in the screen, and **Atualizar** on an installed SKILL reinstalls it at the pinned commit.

## AI: MCPs and Skills

`mcpservers` registers a server per agent (Claude Code, Codex, Antigravity) and records, in `config.yaml`, which agents each registration reached (`mcp_servers[].agents`); `agentskills` does the same for skills (`agent_skills[].agents`). When some agent is missing from a registration (for instance one failed while another succeeded), the screen tags the row **parcial: …**, listing the agents that did accept it. Other rules:

- A server of the same name that Perci did not register (found in Antigravity's `mcp_config.json`, or in Claude Code's or Codex's own list) is never overwritten or removed; adding it is refused with the "já existe um servidor com esse nome que não foi registrado pelo Perci" message.
- Filesystem and SQLite are refused up front when `npx`/`uvx` isn't available.
- Folders and MCP parameters (the Filesystem folder, the SQLite file) must have been picked in the native dialog during the session or already be recorded for that server; anything else is refused with "pasta não escolhida no seletor". The same rule applies to adding a repository folder.
- The `skills` CLI runs with `DISABLE_TELEMETRY=1`, and the screen shows a notice that skills are third-party content.

## PATH Management

Every package that needs to persist a PATH export uses the shared `internal/shellrc.AppendIfMissing`/`RemoveEntry` helpers against `~/.bashrc` — no package reimplements rc-file editing on its own. Rewrites (`RemoveEntry`) are atomic, preserve the rc file's existing permissions and keep a dotfiles symlink pointing at its target.

## EXDEV / Atomic Install Pattern

Any installer that downloads into a temp location and then moves the result into its final destination stages that temp file/directory in the **same parent directory** as the destination (same filesystem) before renaming — this is what avoids `EXDEV` cross-filesystem rename errors (previously misread as permission errors, unnecessarily escalating to `sudo`). Applied consistently in `golang` and `flutter`.

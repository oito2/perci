🌐 [Português](../../pt-br/architecture/dev-environment.md) | **English** | 🏠 [Index](../index.md)

---

# Dev Environment (`internal/dev/*`)

Backs the **Desenvolvimento** GUI category.

| Package | Responsibility |
|---|---|
| `prereqs` | 18 individually toggleable essential packages (compilers, `build-essential`, Git, curl, mkcert, etc.) plus grouped Docker Engine/Node.js installs. On Fedora, Docker Engine is the distribution's `moby-engine` + `docker-compose` + `docker-buildx`, and the GitHub CLI is the native `gh` package. Removing Node.js deletes the whole `~/.nvm` folder (every nvm-installed Node version and its global npm packages) — the GUI asks for confirmation first, spelling that out. |
| `golang` | Go SDK installer. `LatestRelease(ctx)` fetches the releases JSON once and returns both the version and its checksum together (`Install(ctx, exe, stdout, version, checksumSHA256)` — pass an empty checksum to fall back to fetching it). The privileged part is one batch (one password prompt): root-owned copy of the tarball, checksum checked again, extraction to `/usr/local/go-staging`, then swap with rollback (`installScript`). Local PATH exports via `internal/shellrc`, with a quoted line that also works in fish (the older unquoted one is replaced/removed). |
| `flutter` | Flutter SDK via `git` checkout, PATH exports, automatic Android tooling acceptance, `flutter doctor` wrapper. The JDK comes from `openjdk-21-jdk` on Debian/Ubuntu and from Eclipse Temurin 21 (Adoptium's signed repository) on Fedora, whose own repositories no longer ship JDK 21. Downloads the Android cmdline-tools without checksum verification — a documented, accepted limitation (no official checksum is published for that specific artifact); staged in the same parent directory as the destination to avoid `EXDEV` cross-filesystem errors on rename. |
| `ide` | VS Code, VSCodium, and Zed Editor setup. |
| `llm` | Unified manager for Claude Code, Antigravity CLI, Codex, OpenCode. Uninstalling also removes what the official installers leave besides the binary: `~/.local/share/claude` (when `claude` is the native install's link into it) and OpenCode's `~/.opencode/bin` plus its `# opencode` rc lines; user settings are kept. |
| `mcp` | Catalog of local Node.js MCP servers (`servers.yaml`, embedded at build time — recompile to pick up catalog edits). |
| `terminal` | Kitty, Alacritty, GNOME Console wrappers, Starship prompt, file-manager context-menu integrations. |
| `localbin` | Shared helper for ensuring `~/.local/bin` is exported on `PATH` (`EnsureInPath`, built on `internal/shellrc`, written to the current shell's rc file — `fish_add_path` for fish). `RefreshPath` rebuilds the GUI process's own `PATH` (`~/.local/bin` + nvm's default Node) at startup and after every action, so a tool installed there during the session is found without restarting Perci. |

## Two Catalogs, Two GUI Screens

`llm` and `terminal` each get their own GUI screen ("Aplicativos: IA" and "Aplicativos: Terminais", both under **Desenvolvimento**) rather than being merged into one — the split happens only at the presentation layer; the underlying catalogs stay in their own packages. (`mcp`, a third catalog a single merged TUI screen used to also cover before the TUI/CLI split was removed, was itself removed 2026-09-11 in favor of the separate, GUI-only "IA: MCPs" screen — see `internal/dev/mcpservers`.) See [Adding a New LLM, IDE, or Terminal](../../../CONTRIBUTING.md#adding-a-new-ai-app-ide-or-terminal).

### Terminals: Context Menu and Starship

- **Context menu** — every "Aplicativos: Terminais" run ends with `terminal.SyncContextMenuEntries`, which matches the "Abrir no <terminal>" entries to what is actually installed (`InstalledMap`, after refreshing `PATH`): created for every installed terminal with a `DirFlag` — including ones installed outside Perci — and removed for the others, even when an item of the run failed. Entries live in the user's home, so no password is asked: Nautilus and Nemo scripts (`~/.local/share/{nautilus,nemo}/scripts/Abrir no <terminal>` — Nautilus shows them under the **Scripts** submenu) and a Dolphin service menu (`~/.local/share/kio/servicemenus/perci-<terminal>.desktop`).
- **Starship** — not a checklist item; two extra buttons on the same screen: **Aplicar Starship** (`InstallStarship`) and **Remover Starship** (`UninstallStarship`, with a confirmation), which deletes `~/.local/bin/starship` and the init lines in bash/zsh/fish, keeping `~/.config/starship.toml` as `starship.toml.perci-bak`.

## Pinned Package Versions

The tools Perci runs through `npx`/`uvx` are pinned, never "whatever is latest today":

| Used by | Package | Constant |
|---|---|---|
| Dev Tools → IA: SKILLs | `skills@1.7.0` (npm, [vercel-labs/skills](https://github.com/vercel-labs/skills)) | `agentskills.skillsCLI` |
| Dev Tools → IA: MCPs (Filesystem) | `@modelcontextprotocol/server-filesystem@2026.8.31` (npm) | `mcpservers.filesystemServerPkg` |
| Dev Tools → IA: MCPs (SQLite) | `mcp-server-sqlite@2025.4.25` (PyPI, via `uvx`) | `mcpservers.sqliteServerPkg` |

To upgrade one, check the new release (`npm view <pkg> version`, pypi.org), bump the constant and rebuild. An MCP server already registered keeps the version it was registered with until it's updated in the screen.

## PATH Management

Every package that needs to persist a PATH export uses the shared `internal/shellrc.AppendIfMissing`/`RemoveEntry` helpers against `~/.bashrc` — no package reimplements rc-file editing on its own. Rewrites (`RemoveEntry`) are atomic, preserve the rc file's existing permissions and keep a dotfiles symlink pointing at its target.

## EXDEV / Atomic Install Pattern

Any installer that downloads into a temp location and then moves the result into its final destination stages that temp file/directory in the **same parent directory** as the destination (same filesystem) before renaming — this is what avoids `EXDEV` cross-filesystem rename errors (previously misread as permission errors, unnecessarily escalating to `sudo`). Applied consistently in `golang` and `flutter`.

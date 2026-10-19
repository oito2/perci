# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [1.5.0] - 2026-10-19

### Changed
- Desenvolvimento → **IDEs**: Zed Editor, VS Code and VSCodium share one screen, a table like Linguagens e SDKs with Instalar, Atualizar and Desinstalar per IDE; Desinstalar asks for confirmation. Zed Editor needs Curl from Pré-requisitos. Android Studio and Antigravity IDE keep their own screens.

### Removed
- Linux → **Aplicativos: MegaSync**: Perci no longer installs or removes MEGAsync. To remove one installed by an earlier version, see the uninstallation guide. Moodle plugin mounts still add their `.megaignore` exclusion.

## [1.4.1] - 2026-10-14

### Fixed
- Docker: browsers installed as Flatpaks (Firefox, Zen, LibreWolf, Chrome…) didn't trust the `*.localhost` certificate (`SEC_ERROR_UNKNOWN_ISSUER`), since `mkcert -install` doesn't look at their profiles. Creating or recreating the Nginx container now registers mkcert's CA in them too.
- Docker: Firefox and its derivatives refused the `*.localhost` certificate (`SSL_ERROR_BAD_CERT_DOMAIN`), a wildcard they don't accept. The certificate now also names `localhost` and every app, and is regenerated when an app's name is missing — use **Recriar** on Nginx once to update an existing one.

## [1.4.0] - 2026-10-14

### Added
- Linux → Pós-instalação (Ubuntu Budgie 26.04): **Aplicar backports-budgie** adds the Ubuntu Budgie team's `ppa:ubuntubudgie/backports-budgie`, refreshes the package index and upgrades the system, with one password. Not offered on 24.04, which the PPA doesn't build for.
- Desenvolvimento → Linguagens e SDKs: **Rust (rustup)** for the current user — rustc, cargo, clippy, rustfmt and rust-analyzer, from a version-pinned, checksum-verified rustup installer; Atualizar runs `rustup update`.
- Desenvolvimento → Pré-requisitos: **Build Essential** (GCC and make), **WebKitGTK 4.1 Dev Headers**, **Libxdo Dev** and **Flatpak Builder Tools** (`flatpak-node-generator` and `flatpak-cargo-generator.py` at a pinned commit, for offline Flatpak builds) — with OpenSSL and Librsvg Tools below, everything a Tauri v2 app needs to build its AppImage, `.deb` and `.rpm`.
- Docker → **Exportar/Importar**: its own menu item with **Exportar** and **Importar** tabs. Both list Nginx, MariaDB, each app and each Moodle plugin mount to check one by one; checking an item also checks what it depends on, checking a Moodle checks the plugin mounts on it, and missing dependencies are pointed out.
- Moodle plugin mounts are part of the export (without this machine's MEGA exclusion). Importing them makes the mounts, their units and the MEGA exclusion with one password; a mount whose development folder or Moodle code doesn't exist on the machine shows as unavailable and is skipped with a warning.

### Changed
- Desenvolvimento → Pré-requisitos: **OpenSSL** also installs its development headers (`libssl-dev`/`openssl-devel`) and **Librsvg Tools** the librsvg ones (`librsvg2-dev`/`librsvg2-devel`). Removing OpenSSL removes only the headers.
- Importing merges: the checked items are added or replace the app with the same folder, and everything else on the machine stays — apps missing from the file no longer leave the stack. Files exported by earlier versions still import.
- Exportar/Importar left Docker → Gerenciar Contêineres. The uninstall backup also carries the Moodle plugin mounts.

### Fixed
- Desenvolvimento → Pré-requisitos: OpenSSL, Librsvg Tools and Libsecret Tools showed as installed when only their command was present, so their development headers could never be installed; they now count as installed only with the headers too.

## [1.3.1] - 2026-10-14

### Changed
- Dev Tools → Plugins Moodle: the mounted plugins (status, **Reiniciar contêiner**, **Remover**) moved to their own **Plugins montados** tab, and the message shown after mounting points to it.

### Fixed
- Dev Tools → Plugins Moodle: when the Moodle's `html` folder is synced by MEGAsync, the mount destination is excluded in the nearest `.megaignore` (`-dpG:<path>`) before it is mounted, and the line is removed with **Remover**. Without it, MEGAsync could take the mount for a move between syncs and send the plugin's development files to its `.debris` folder. The mount is refused when the exclusion can't be written.

## [1.3.0] - 2026-10-13

### Added
- Dev Tools → IA: Contextos: a **Tipo de projeto** selector that checks the standards of a project kind at once — Servidor MCP em Go, Aplicativo Go + Wails, Plugin Moodle, Aplicativo Rust + Tauri v2, Website, Sistema PHP + Slim and Jogo Godot (Android); the checkboxes stay editable.
- New standards: Go (with MCP server and Wails v3 complements), Rust + Tauri v2 (with Vanilla TS + Vite, SvelteKit and React + Vite frontend complements), PHP + Slim 4 (with PDO, Doctrine DBAL and Eloquent complements), Godot 4 (GDScript), Website, and the "Documentação: Jogo" documentation complement.
- Dev Tools → IA: MCPs: Context7 (`@upstash/context7-mcp@4.4.1`), gopls MCP (`gopls mcp`, needs `gopls` on `PATH`), Chrome DevTools MCP (`chrome-devtools-mcp@1.10.1`, isolated profile, usage statistics off), Svelte MCP (`@sveltejs/mcp@0.1.26`) and DBHub (`@bytebase/dbhub@1.5.0`, configured through a `dbhub.toml` picked in the file dialog). Each server shows a short note under its name.
- Dev Tools → IA: SKILLs: Go (Wails, Testing, Security, Lint, Error Handling, Continuous Integration), Rust Best Practices, Svelte (official Code Writer and Best Practices), React Best Practices (Vercel), Vite, Vitest, Web accessibility/SEO/performance, Web App Testing (Playwright), MariaDB (official Query Optimization and Features), SQL Code Review, GitHub Actions Hardening, MCP Security Audit, Security Review (Sentry) and Godot Mobile — each pinned to a full commit.

### Changed
- The project documentation standard covers GitHub community files (SECURITY.md, issue and pull request templates), repository metadata (description, topics, homepage — prepared as commands for the user) and releases; the desktop complement covers Tauri bundles and updater signing, the web complement GitHub Pages, and the Moodle complement the plugin license declaration and the release ZIP for the Moodle Marketplace. `SECURITY.md` is a canonical root file mirrored as `docs/pt-br/seguranca.md`.
- Dev Tools → IA: MCPs: the Filesystem server is pinned to `@modelcontextprotocol/server-filesystem@1.0.0` (the package moved to semantic versioning); **Atualizar** re-registers it.
- Dev Tools → IA: SKILLs: "Godot: Master" is installed from the repository's new address (`thedivergentai/Game-Design-Agentic-Skills`), at the same commit.

### Removed
- Dev Tools → IA: MCPs: SQLite (the server is archived and allows writes; DBHub replaces it). Dev Tools → IA: SKILLs: "Godot: Core UI Knowledge", "Moodle: Plugin Development", "Moodle: Education Expert", "SVG Logo Designer" and "PHP: Specialist". Where one of them is still installed it stays listed, tagged **fora do catálogo**, with only **Remover**.

### Fixed
- Release builds use Wails' `production` tag: the published binary no longer runs in debug mode with the webview's developer tools enabled.
- Dev Tools → IA: Contextos: the `.claude/settings.json` read rules no longer block `dist/`, `build/`, `out/` or images (`*.png`, `*.jpg`, `*.jpeg`, `*.gif`, `*.webp`) when the project's git repository tracks files there, so versioned source code and screenshots stay readable by Claude Code.

## [1.2.0] - 2026-10-13

### Added
- Ubuntu Budgie 24.04/26.04 and Fedora Budgie 44 support in Linux → Pós-instalação. Ubuntu Budgie has its own profiles without the GNOME-only packages (GNOME Tweaks, and the Flatpak plugin that would install GNOME Software); on 26.04, which already ships VLC, the Flatpak step installs only the web-app manager. Fedora Budgie uses the Fedora actions, shown as "Fedora 44 Budgie".
- Dev Tools → IA: SKILLs and IA: MCPs show "parcial: …" when some agents rejected an item, listing the ones that accepted it.
- Docker → Importar configurações lists the commands the imported apps will run before asking for confirmation.

### Changed
- Linux → Pós-instalação: each item now has its own **Aplicar** button instead of one checklist run, so the administrator password is asked only for the item being applied. An item already in place (detected on the system, or recorded after a successful run when it leaves nothing to detect) shows a disabled **Aplicado** button; an item whose prerequisite isn't applied yet shows "Requer: <item>". Recorded items are tied to the machine (`/etc/machine-id`), so after reinstalling the OS they can be applied again; held packages count as installed.
- Desenvolvimento → Pré-requisitos: a table with **Instalar**, **Atualizar** and **Desinstalar** buttons on each item instead of a checklist. **Atualizar** is new: it upgrades the item's own packages, moves Node.js to the latest LTS (keeping its global npm packages) and reinstalls the pinned Wails3 CLI and AppImageTool. Items others need (Curl, Flatpak) come first, and an item whose requirement isn't installed (Curl, Flatpak, or Go for the Wails3 CLI) can't be installed yet and shows "Requer: …". Uninstalling always asks for confirmation.
- Desenvolvimento → Linguagens e SDKs: the same table as Pré-requisitos — **Instalar**, **Atualizar** and **Desinstalar** on Go and Flutter. **Atualizar** installs the latest stable Go when it is newer than the installed one and runs `flutter upgrade` for Flutter. Go requires Curl and Flutter requires Git (both from Pré-requisitos), shown as "Requer: …" until installed. Uninstalling always asks for confirmation.
- Removing a prerequisite (and Alacritty, GNOME Console, VS Code, VSCodium, GitHub CLI) is refused when the package manager would also remove other packages, naming them; "Libsecret Tools" no longer removes the shared library. On Fedora it also installs `git-credential-libsecret`.
- Dev Tools → IA: SKILLs: skills are pinned to a commit, **Atualizar** reinstalls them at that commit for the same agents, the skills CLI runs with telemetry disabled, and the screen notes that skills are third-party content.
- Dev Tools → IA: Contextos: `.claudeignore` is no longer generated (Claude Code ignores it); the read restrictions go to `.claude/settings.json` as `permissions.deny` rules, merged with the existing file. Generated files are written through symlinks and the previous different version is kept as `<file>.perci-bak`. The generated `MOODLE.md` refers to the project as `.` instead of its absolute path.
- Folders and MCP parameters sent by the screens must have been picked in the file dialog (or registered earlier); adding a repository folder requires the dialog.
- Closing the window (with the tray off) or **Sair** in the tray while an action runs no longer quits Perci: the window running the action shows its Execução tab with a notice.
- Linux → Atualizar Sistema finishes "concluído com avisos" when snap, Flatpak, journal or autoremove steps fail, instead of reporting plain success.
- Docker: app containers use `--mount` instead of `-v` (the workspace path may not contain `,` or `"`), get `host.docker.internal` (Xdebug can reach the IDE after **Recriar**), and `moodledata` is created with mode 0750. **Editar**/**Recriar** build the image before removing the running container. When Nginx is stopped, its configuration is tested with `nginx -t` before being written. The `mkcert` certificate is renewed when it is less than 30 days from expiring.
- Docker images pin Composer, the redis and xdebug PECL extensions, and take Node.js from the official `node` image; PHP 7.4 and 8.0 images (Debian bullseye, out of support) take their packages from archive.debian.org. Every Perci image rebuilds once.
- Fonts downloaded by Linux → Gerenciar Fontes go to `~/.local/share/fonts/perci/<font>` and are removed completely; removing a packaged font on Debian/Ubuntu is refused when another package depends on it.
- Installers pin nvm (v0.40.8) and the Codex CLI (0.162.1), and every network download or tool run without privileges has a time limit that also stops the processes it started, so a stalled connection no longer blocks Perci until restart. The Flutter clone is instead stopped only when the transfer stalls.
- Terminal output reaches the Execução tab in batches of about 30 ms, keeping long actions smooth.
- Uninstalling Perci asks for the password once and removes the `perci` alias only when it points to `prci`; a development build no longer updates itself.
- Wails updated to v3.0.0-beta.28: on Linux, right-clicking the system tray icon no longer runs the left-click action as well, on top of opening the menu.
- Release pipeline: lint, tests and ShellCheck run in their own job before the build.

### Removed
- Linux → Aplicativos: Linux Toys (it ran a remote installer as root without integrity checks). An installation made by an earlier version stays; remove it with the package manager if needed.
- Linux → Aplicativos: Flatpak: Resources, Google Chrome, Vivaldi, Minecraft, Minecraft Java, Ente Auth, Font Downloader and FreeTube are no longer in the list. Copies already installed stay installed; remove them with `flatpak uninstall` or the software center if needed.

### Fixed
- Linux → Atualizar Sistema no longer fails on Debian/Ubuntu when a package's configuration file was edited by the user.
- Linux → Pós-instalação: "Habilitar RPM Fusion" installs whichever repository is missing instead of staying "Aplicar" forever; the swapfile item reports failures and undoes a half-done setup; Fedora codecs only count as applied with the multimedia group installed.
- Uninstalling Perci with a backup requested removes nothing when the backup fails.
- Docker → MariaDB backup no longer destroys the previous backup file when the dump fails; importing a configuration keeps the local MariaDB root and data passwords and checks the credentials before removing anything; Recriar MariaDB refuses diverging credentials without removing the database.
- Dev Tools → Plugins Moodle: a failed mount undoes the unit, the record and the folder; removing keeps the unit while the folder is still mounted; folder names with `%` work; the screen opens with far fewer Docker calls.
- Reinstalling Android Studio or Antigravity no longer destroys the previous installation when the new one fails.
- Checklists no longer run the previous screen's action while the new one loads; IDE screens refresh their buttons after an action; screens no longer show data from a previous folder or scope when answers arrive late; errors are shown instead of a permanent "Verificando...".
- Removing Go, Flutter or Node.js entries from the shell rc no longer deletes unrelated lines or leaves blank lines, and works on fish without `~/.config/fish`; uninstalling Node.js removes the nvm lines.
- An existing `credential.helper` (gh, GCM) is kept; a warning is shown when only the `cache` helper is available.
- Flutter requires Java 17 or newer and installs the JDK in the same password prompt as its other prerequisites; installing Kitty no longer opens a Kitty window; Kitty, Starship and Zed make sure `~/.local/bin` is on the PATH; uninstalling Zed removes its menu entry.
- The KDE Dolphin "Abrir no <terminal>" menu entry is executable, so Dolphin no longer asks to trust it.
- Dev Tools → IA: Contextos keeps a `CLAUDE.md` that is a link to `AGENTS.md` instead of writing the Claude stub through it over `AGENTS.md`.
- Docker → Recriar Nginx renews the certificate before removing the running container, so a renewal failure leaves Nginx running.
- An action started right after the previous one finished could be refused with "outra ação já está em execução": the end of an action is now announced only after it releases the lock.
- Docker → Criar Container: the first-time `mkcert` warning is shown at the top of the screen, above the form, instead of breaking the form layout.
- Opening Perci again while it is already running (e.g. from the distro menu with the tray enabled) no longer starts a second copy with another tray icon; it brings the running window to the front.

### Security
- Commands run with administrator rights are resolved to absolute paths in system directories, never through the user's `PATH`.
- Dev Tools → Plugins Moodle refuses mount destinations outside the Moodle folder (including through symbolic links or a modified `components.json`).
- Docker quick actions and logs only accept Perci's own containers; a second Nginx or MariaDB container is refused.
- MCP servers, Claude Code and Codex registrations never overwrite or remove a same-name entry Perci didn't create; the Filesystem and SQLite servers are refused when `npx`/`uvx` is missing.
- Signed repositories (VS Code, VSCodium, MegaSync) are configured non-interactively, never add a second apt source when one exists, require HTTPS redirects, and the MEGA key is checked against its pinned fingerprint.
- `install.sh` only downloads from GitHub hosts, refuses redirects to HTTP, has timeouts, and verifies the release's build attestation when the GitHub CLI is installed and logged in.
- The self-update no longer downloads `checksums.txt` to check for a new version, but still requires and verifies it before installing.
- The Docker Engine prerequisite description warns that the `docker` group grants administrator-equivalent access; the Flutter description states that the Android SDK licenses are accepted during installation.

## [1.1.0] - 2026-10-09

### Added
- Dev Tools → Plugins Moodle (first item): mount a plugin development folder inside one or more Moodle containers, so the plugin appears as a regular folder in the Moodle code on the host (editor) and in the containers. Type and name come from the folder's `version.php` (or are chosen when the folder is still empty), the destination follows each Moodle's layout (`public/` on 5.1+), and the table shows whether each mount is applied, with restart and remove actions.
- Desenvolvimento → Pré-requisitos: "Librsvg Tools" item (`librsvg2-bin`, `librsvg2-tools` on Fedora) providing `rsvg-convert`.

### Changed
- Linux → Atualizar Sistema: the "remove unused" option now cleans unused Flatpak runtimes in both the user and the system installation.
- Desenvolvimento → Pré-requisitos: "Libsecret Tools" also installs `libsecret-1-dev` on Debian/Ubuntu (the `libsecret-1` development files; Fedora already installed `libsecret-devel`).

### Fixed
- Linux → Atualizar Sistema: asks for the administrator password only once — system Flatpaks are now updated inside the same privileged batch as the packages, instead of Flatpak requesting its own authorization.

## [1.0.1] - 2026-10-02

### Added
- System tray menu: "Abrir Aplicativo" item that shows the main window.

### Changed
- Generated `AGENTS.md` (Dev Tools → Definir Contextos): new Code Comments and Verification rules; questions use the agent's selectable-option prompt; Git commands are handed to the user instead of run by the agent; model selection is tier-based (no model names) and applies to subagents; escalation is a plain report instead of JSON; translated documentation mirrors are allowed; AGENTS.md rules take precedence over the selected standards.
- AI context standards: Moodle (current Hook API, ES modules, `core_external`, PHP table matching Moodle 4.1–5.3 and the perci container options, no MCP-specific section), Bash (fixed boilerplate and pattern examples), MySQL/MariaDB (MariaDB-compatible examples, `ALGORITHM=INSTANT`), Landing Page (no invented social proof, accessibility rules), Flutter (explicit Dart SDK constraint, documentation via the project documentation standard).
- Collapsed sidebar: category icons (no background, bolder on hover) instead of initials, and a smaller logo that fits the rail.
- Tab names: "Configurações" (Home → Configurações), "Definir Contextos", "SKILLs" and "MCPs" (Dev Tools) instead of "Perci".
- Tab panels fill the window height in the compact (tray) window too, with the same bottom margin as the sides.
- Small spacing and wording adjustments in Linux → Atualizar Sistema and Dev Tools → IA: SKILLs / IA: MCPs.

### Fixed
- Docker → Moodle containers: "Versão do Moodle" offers 4.1, 4.2-4.3 and 4.4-4.5 instead of a single 4.x, each with the PHP versions that Moodle release supports (4.4/4.5 can now use PHP 8.2/8.3); 5.1+ offers only PHP 8.3/8.4, since Moodle 5.2 and 5.3 require PHP 8.3. Containers saved as 4.x keep working.
- Dev Tools → Repositórios (also in the tray window): adding or selecting a repository no longer duplicates the "Novo repositório" card.

## [1.0.0] - 2026-10-01

First public release: a Linux desktop app (Go + Wails, GTK4/WebKitGTK 6.0) with five sidebar categories.

### Added
- **Home** — system overview; update and uninstall Perci itself; settings for theme, sidebar mascot, app icon, workspace folder, Flatpak scope and an optional system tray icon with a compact window.
- **Linux** — system update; distro/desktop-aware post-installation for Linux Mint (Cinnamon, XFCE), Ubuntu 24.04/26.04, Fedora, ZorinOS (Core, Lite) and Pop!_OS 24.04 COSMIC; fonts; file templates; Flatpak apps; Linux Toys; MegaSync from MEGA's signed repository.
- **Desenvolvimento** — build prerequisites; Go and Flutter SDKs; AI CLIs (Claude Code, Antigravity CLI, Codex, OpenCode); terminals (Kitty, Alacritty, Black Box, GNOME Console) with "Abrir no <terminal>" file-manager entries and the Starship prompt; Zed, VS Code, VSCodium, Android Studio and Antigravity IDE.
- **Docker** — per-project containers (Nginx, MariaDB, Moodle, PHP, generic, Node, PHP + Node) with local HTTPS, start/stop/recreate/logs, configuration export/import and MariaDB backup/restore.
- **Dev Tools** — repositories (clone, init, global and per-repo git identity, `.gitignore` by project type, Code of Conduct); AI contexts (`AGENTS.md`, `CLAUDE.md`, `.instructions/`); AI skills from skills.sh; MCP servers for Claude Code, Codex and Antigravity.
- Privileged actions ask for the administrator password once per action (PolicyKit).
- Installation through `install.sh` or `make install`; release artifacts (`prci-linux-amd64`, `perci-menu.tar.gz`, `checksums.txt`) with build provenance attestation.

[Unreleased]: https://github.com/oito2/perci/compare/v1.5.0...HEAD
[1.5.0]: https://github.com/oito2/perci/compare/v1.4.1...v1.5.0
[1.4.1]: https://github.com/oito2/perci/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/oito2/perci/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/oito2/perci/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/oito2/perci/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/oito2/perci/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/oito2/perci/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/oito2/perci/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/oito2/perci/releases/tag/v1.0.0

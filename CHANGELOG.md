# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

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

[Unreleased]: https://github.com/oito2/perci/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/oito2/perci/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/oito2/perci/releases/tag/v1.0.0

# Documentation Complement — Desktop Applications

## Overview

Type-specific documentation requirements for **desktop applications**. This complements the base standard in `.instructions/PROJECT-DOCUMENTATION.md` — apply both. Like the base, it's a standard for when documentation is created or updated, not a task to run on its own.

---

## 1. Operating System Requirements

In the README's **Prerequisites & Quick Installation** section, include a support matrix covering **Windows, macOS and Linux**:

| OS | Status | Minimum version / distributions | Runtime dependencies |
|---|---|---|---|
| Linux | Supported | e.g. Debian 12+/Ubuntu 24.04+, Fedora 41+ | e.g. `libgtk-4-1`, `libwebkitgtk-6.0-4` |
| Windows | Not supported | — | — |
| macOS | Not supported | — | — |

- Document requirements, build and install steps **only for the operating systems the project actually supports** (check the build files, CI workflows, and platform-specific code).
- Every unsupported OS still appears in the matrix, **explicitly marked as not supported** — never with invented commands. `docs/en/getting-started/installation.md` gets one section per OS, with the unsupported ones stating so in one line.
- For Linux, list runtime dependencies per package manager (`apt`, `dnf`, ...) the project supports.

---

## 2. Building Executables and Installers

Document the commands to produce each distributable artifact **the project actually produces** (check the `Makefile`, build scripts, packaging config, CI release workflow):

| Platform | Possible artifacts |
|---|---|
| Windows | `.exe`, `.msi` |
| macOS | `.app`, `.dmg` |
| Linux | `.AppImage`, `.deb`, `.rpm`, Flatpak, `.tar.gz` |

For each: the exact build command, its build-time dependencies, the output path, and code signing/notarization steps when the project has them. Artifacts the project doesn't produce are not documented.

---

## 3. Installation Footprint and Uninstallation

Document what installing puts on the system — binary location, desktop entry/menu shortcut, icons, autostart entries, config and data directories (per OS, e.g. `~/.config/<app>`, `%APPDATA%\<app>`, `~/Library/Application Support/<app>`) — and write `docs/en/getting-started/uninstallation.md` removing every one of them.

---

## 4. User Interface Screenshots

Add placeholders for images/GIFs demonstrating the UI, stored under `docs/img/` and referenced with relative paths:

```markdown
![Main window](docs/img/main-window.png)
<!-- TODO: capture a screenshot of the main window -->
```

- One hero screenshot or GIF in the README, right after the Overview.
- Screenshots of the main screens in `docs/en/concepts/how-{{PROJECT_NAME}}-works.md` or the relevant guides.
- Keep each placeholder's `TODO` comment until a real capture replaces it — never ship a broken image link without it. Both language trees reference the same image files.

---

## 5. Troubleshooting

Include OS-specific issues in `docs/en/troubleshooting/common-issues.md` (missing runtime libraries, sandbox/permission restrictions, graphics/driver problems, unsigned-app warnings) — only the ones that actually apply to this project.

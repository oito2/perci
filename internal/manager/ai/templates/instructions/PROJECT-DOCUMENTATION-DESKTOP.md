# Documentation Complement — Desktop Applications

Type-specific documentation requirements for **desktop applications**. Complements `.instructions/PROJECT-DOCUMENTATION.md` — apply both; this file lists only what it adds. Build mechanics live in the stack standards (`.instructions/TAURI.md`, `.instructions/WAILS.md`) when the project has them; this file covers what to document.

Last verified: 2026-11-04

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

### Stack specifics

Take build commands, bundle targets and output paths from the project's build files and CI — the stack standards explain them:

- **Tauri v2** (`TAURI.md`): `tauri build` bundles the targets in `bundle.targets` (`deb`, `rpm`, `appimage`, `msi`, `nsis`, `app`, `dmg`); `--bundles` selects some. Document the OS each artifact must be built on (`msi` only on Windows, `app`/`dmg` on macOS). With the updater plugin, document where the update manifest is published and that the private signing key is provided only through `TAURI_SIGNING_PRIVATE_KEY`/`TAURI_SIGNING_PRIVATE_KEY_PASSWORD` or CI secrets — never committed.
- **Wails v3** (`WAILS.md`): `wails3 package` runs the project's per-OS `package` Taskfile tasks (generated defaults: AppImage, `.deb`, `.rpm`, Arch package on Linux; NSIS installer on Windows; `.app` on macOS). `wails3 tool package -format deb|rpm|archlinux|dmg` builds a single package from the project's package config (`dmg` only on macOS) — document it when the project calls it.

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

---

## Quality

In addition to the base standard's checks:

- [ ] The OS matrix lists Linux, Windows and macOS, with unsupported ones marked as such; the supported ones match the build files and CI.
- [ ] Every documented artifact is produced by the project's build/CI, with the real command and output path (run it, or check the CI workflow).
- [ ] `uninstallation.md` removes everything the install footprint lists (binary, desktop entry, icons, autostart, config/data dirs).
- [ ] Each screenshot placeholder has its `TODO` comment; no broken image links (`lychee --offline` from the base checks).

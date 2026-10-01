🌐 [Português](../../pt-br/architecture/system-management.md) | **English** | 🏠 [Index](../index.md)

---

# System Management (`internal/system/*`)

Backs the **Linux** GUI category.

| Package | Responsibility |
|---|---|
| `postinstall` | Distro/DE-specific post-installation scripts. Mint Cinnamon and Mint XFCE share a consolidated `runMint` implementation; Ubuntu, Fedora, and Zorin Core/Lite each have their own flow. Uses `tee`/`debconf-set-selections` instead of `bash -c` string interpolation for things like sysctl config and MS TTF fonts EULA acceptance. |
| `fonts` | Install/remove JetBrains Mono, Noto Fonts, Carlito, Caladea, and others. The "JetBrains Mono NF" download is pinned to a specific version (v3.4.0) with SHA-256 checksum verification — not a "latest" fetch. Carlito, Caladea and the Noto fonts come from distribution packages on both families (`apt` on Debian/Ubuntu, `dnf` on Fedora; removal on Fedora uses `rpm -e`, which refuses when another package still requires the font). Installed state is an exact match on the font family names `fc-list` reports. |
| `templates` | Blank document templates (Office, LibreOffice, text, code) in the user's Templates/Modelos folder. Falls back to checking for a literal `Modelos` folder on pt-BR locales when `xdg-user-dir TEMPLATES` doesn't resolve one; creates the templates directory if missing before writing into it. Never overwrites an existing file, and only removes plain file names inside that folder. |
| `apps` | Flatpak catalog (`Catalogue` slice in `catalogue.go`) — single-step install/uninstall with visual confirmation. `EnsureFlatpak` (also used by the post-install profiles) installs flatpak when missing and always ensures the Flathub remote in the configured scope; its steps (`FlatpakSetupSteps`) join the caller's own install batch, so installing apps at system scope asks for the password once. A batch where some app failed reports "concluído com avisos" (`executor.ErrCompletedWithWarnings`) instead of success. |
| `update` | Unified `apt`/`dnf` + Flatpak + Snap update pass, run from the **Atualizar Sistema** screen. Flatpak apps are updated in both the user and the system installation. Two cleanups are opt-in checkboxes, unchecked by default: vacuuming journal entries older than 7 days, and removing orphaned packages (`autoremove`) plus unused Flatpak runtimes. |
| `linuxtoys` | Installer for community terminal tools ("Toys"). Downloads the official installer script, then runs it as root through pkexec (one prompt) — its own inner `sudo` calls can't ask for a password inside the GUI. The script reaches root on stdin (`bash -s`), never as a file path a same-user process could swap during the password dialog. |
| `megasync` | MEGA desktop sync client. Installed from MEGA's official signed apt/dnf repository (GPG-verified), not a manually downloaded, unverified `.deb`/`.rpm`. Safe to re-run (the key is dearmored with `--batch --yes` into a temp file first); uninstalling also removes MEGA's repository and key, in the same privileged batch. |

Every install/download flow embedding a `curl | sh`-style upstream installer script runs with `pipefail` set (the one exception is `nvm`'s install script, which isn't compatible with `set -u`/`nounset`).

## Distro/DE Gating

None of these packages implement their own distro detection — they're called from Go methods in `cmd/prci-gui` (bound to the GUI's JS frontend) after a check against `internal/distro.Detect()`/`DetectDE()`. See [Core infrastructure](./core-infra.md#internaldistro).

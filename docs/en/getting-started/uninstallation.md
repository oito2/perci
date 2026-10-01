🌐 [Português](../../pt-br/getting-started/uninstallation.md) | **English** | 🏠 [Index](../index.md)

---

# Uninstallation

## Built-in Uninstaller (Recommended)

Open Perci, go to **Home → Visão Geral**, and use the **Desinstalar Perci** section. A confirmation dialog offers three optional steps before the binary itself is removed:

| Option | What it does |
|---|---|
| Back up configuration | Exports the Docker stack definition (Nginx, MariaDB credentials, every app) to a YAML file you pick — the same format as **Docker → Exportar configurações**, so it can be imported on a new install. |
| Remove Docker containers | Removes every registered container (apps first, then MariaDB, then Nginx). Project data on disk is **not** touched. |
| Remove `~/.perci` | Deletes Perci's configuration directory. |

Whatever you choose, the uninstaller always removes:

- The binary (`/usr/local/bin/prci`) and the `perci` alias symlink.
- The application menu entry: `/usr/share/applications/perci.desktop`, the icon set `/usr/share/icons/hicolor/<size>/apps/perci.png` (512, 256, 128, 64, 48 and 32) and, from older installs, `/usr/share/pixmaps/perci.png`.
- The tray autostart entry, `~/.config/autostart/perci.desktop` (if the tray was ever enabled).

The window closes on its own once the uninstall finishes.

## Manual Uninstall

If Perci no longer opens, remove the same files by hand:

```bash
sudo rm -f /usr/local/bin/prci /usr/local/bin/perci
sudo rm -f /usr/share/applications/perci.desktop /usr/share/pixmaps/perci.png
sudo rm -f /usr/share/icons/hicolor/{512x512,256x256,128x128,64x64,48x48,32x32}/apps/perci.png
rm -f ~/.config/autostart/perci.desktop
rm -rf ~/.perci    # optional: configuration
```

## What Stays on the System

Perci never removes what it installed or created **for you** — that belongs to your system, not to Perci. After uninstalling, these remain until you remove them yourself:

| Leftover | How to remove it |
|---|---|
| Docker containers (if you didn't pick the removal option) | `docker rm -f <name>` — see [Docker application stack](../guides/environments/docker.md) for the container names. |
| Base images `perci-php<version>` / `perci-node<version>` | `docker image ls 'perci-*'`, then `docker image rm <image>`. |
| The `docker-php-network` network | `docker network rm docker-php-network` |
| Project data under your workspace (`workspace/localhost/{html,data,logs}`, MariaDB data) | Delete the folders manually, **only** if you no longer need the projects or databases. |
| Per-app wrappers in `~/.local/bin` (`php-<folder>`, `composer-<folder>`, `npm-<folder>`, …) | Deleting an app in Perci removes its wrappers. For apps still registered when you uninstall Perci (or deleted by an older version): `ls ~/.local/bin/*-<folder>` and remove them. |
| File-manager context-menu entries ("Abrir no <terminal>"): `~/.local/share/nautilus/scripts/Abrir no *`, `~/.local/share/nemo/scripts/Abrir no *`, `~/.local/share/kio/servicemenus/perci-*.desktop` | Unchecking the terminal in **Desenvolvimento → Aplicativos: Terminais** removes its entries. Otherwise, delete those files. |
| Everything installed through Perci's screens (packages, fonts, Flatpak apps, SDKs, IDEs, AI context files, …) | Uninstall each one through its own package manager or tool — or, for most of them, through the matching Perci screen **before** uninstalling Perci. |

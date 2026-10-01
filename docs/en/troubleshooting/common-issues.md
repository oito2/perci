🌐 [Português](../../pt-br/troubleshooting/common-issues.md) | **English** | 🏠 [Index](../index.md)

---

# Common Issues

## Installation

**`install.sh` fails with "Required tool 'jq' is not installed."**
Install `curl`, `jq`, and `sha256sum` (usually part of `coreutils`) first — the script checks for all three before doing anything else and refuses to proceed without them, on purpose, since checksum verification depends on them.

**"Checksum mismatch" during install**
The script aborts rather than installing a binary that doesn't match the published `checksums.txt`. Re-run the installer — if the mismatch persists, it may indicate a corrupted download or a release in a bad state; do not work around this check manually.

**The window doesn't open, with errors about missing `libgtk-4` / `libwebkitgtk-6.0` libraries**
The GTK4/WebKitGTK 6.0 runtime libraries aren't installed. Install them as shown in [Installation → Runtime Requirements](../getting-started/installation.md#runtime-requirements-linux).

**Perci crashes right at startup with `bwrap: setting up uid map: Permission denied` (Ubuntu 23.10+, Zorin OS 18+)**
Not a Perci bug: these releases set `kernel.apparmor_restrict_unprivileged_userns=1`, which blocks the sandbox WebKitGTK starts through `bwrap` — it affects other GTK4/WebKit apps too. Allow user namespaces for `bwrap` with an AppArmor profile, such as `/etc/apparmor.d/bwrap`:

```text
abi <abi/4.0>,
include <tunables/global>

profile bwrap /usr/bin/bwrap flags=(unconfined) {
  userns,
  include if exists <local/bwrap>
}
```

then reload AppArmor with `sudo systemctl reload apparmor` and start Perci again.

**A post-installation entry I expect to see under Linux isn't in the list**
Post-installation entries under **Linux** are gated on distro + desktop environment — only the entry matching your detected distro/DE shows up, the rest are left out of the list entirely (see [Installation → Supported Distributions](../getting-started/installation.md#supported-distributions)). Check `distro`/`de` in `~/.perci/config.yaml` — if auto-detection got your setup wrong, override them there (see [Configuration reference](../reference/configuration.md)).

## Docker Application Stack

**A new Container Aplicativo isn't reachable at its `*.localhost` URL**
Check that "Criar Container Nginx" ran first (it's the reverse proxy every app routes through) and that the browser trusts the mkcert wildcard certificate — `mkcert -install` runs automatically the first time the Nginx container is created, but only registers the CA for the invoking user. Also confirm the app's container is actually running (**Docker → Gerenciar Containers**).

**Creating the Nginx container for the first time hangs or fails silently**
On first creation, `mkcert -install` may escalate to root on its own (mkcert's own binary, not Perci's privilege mechanism) to register the local CA — something the GUI can't answer, since it has no interactive terminal to type a password into (the "Docker → Criar Container" screen already warns about this when it detects it's the first time). If it happens: open a terminal outside Perci and run `mkcert -install` manually once — after that the certificate already exists and "Criar Container Nginx" never needs to call that part of mkcert again.

**Editing a container caused a brief outage of just that app**
Expected — "Editar" is remove-and-recreate with the new parameters (the only real way to change a running container's image/PHP/Node version). Volumes (`html`/`data`) are preserved; other containers are untouched.

**Recreating MariaDB changed my database root password**
It shouldn't — the generator reuses `cfg.Docker.MariaDB.DBRootPass` from the existing config instead of generating a new one when the container is already provisioned. If the actual MariaDB root password doesn't match, the volume and the config have likely drifted out of sync manually; resolve by hand rather than regenerating.

**Removing an app didn't free up disk space**
By design — "Remover" only removes the container and its `config.yaml` entry, never the `html`/`data` folders on disk. Delete those manually if you're sure you don't need them.

**My "Comando do worker em background" (or another feature added to a shared image's build recipe) doesn't seem to do anything**
A `perci-php<version>`/`perci-php<version>-node<version>`/`perci-node<version>` image built by an older Perci is rebuilt automatically the next time an app needs it (its `perci.hash` label no longer matches — see [Docker application stack](../architecture/appstack.md)), but an existing container keeps running on the image it was created from. After that rebuild the terminal lists the containers still on the old image: **Recriar** each of them in **Docker → Gerenciar Containers**.

**`composer-<folder>`/`npm-<folder>` fails with "Permission denied" writing `vendor/` or `node_modules/`**
Wrappers created by older versions ran as root inside PHP-family and combo containers, leaving those folders root-owned on the host; today's wrappers run as `www-data` (your own UID). Give the folders back to your user once, from the project folder: `sudo chown -R "$USER": vendor node_modules`.

## AI Context / Skills

**Clicking "Aplicar" in IA: Contextos overwrote my customized `AGENTS.md`**
That's by design — **Aplicar** always regenerates and overwrites `AGENTS.md`/`CLAUDE.md`/`.instructions/*.md` for every checked model, with no confirmation dialog; the click itself is the confirmation, same convention as every other non-interactive GUI action. Back up any manual edits to those files before clicking Aplicar again, or keep your customizations in a file the generator doesn't touch.

**Moodle placeholders (`{{MOODLE_VERSION}}`, etc.) weren't filled in**
The generator looks for `version.php` (or `public/version.php`) in the folder you picked in **IA: Contextos**. If it's not found, the GUI leaves the literal placeholders in place and logs a warning instead of prompting for values — fill them in by hand, or pick the project's actual root folder and click Aplicar again.

**Removing DaisyUI, "Dart: Oficial" or "Flutter: Oficial" in IA: SKILLs removed my other skills too**
Fixed: older versions removed those three entries (which install every skill of their repository) with `skills remove --skill '*'`, which the `skills` CLI treats as "every installed skill of these agents" in the chosen scope. Perci now looks up, with `skills list --json`, which installed skills came from the entry's repository and removes only those, by name — and refuses the removal if that lookup fails. Reinstall the skills you lost from the same screen.

## Development Environment

**After installing Go, my fish shell lost `/usr/bin` (and most commands) from `PATH`**
Fixed: older versions wrote `export PATH=$PATH:/usr/local/go/bin` to `~/.config/fish/config.fish`; in fish that unquoted line leaves `PATH` with just `/bin` and Go's `bin`. The line is now quoted (`export PATH="$PATH:/usr/local/go/bin"`, valid in bash, zsh and fish). To repair an existing install, uncheck Go in **Linguagens e SDKs** and run it (removes Go and both forms of the line), then check it again to reinstall. By hand: delete the old line from `config.fish` and open a new shell.

## Known, Accepted Limitations (Not Bugs)

- **Android cmdline-tools** (`internal/dev/flutter`) and **`appimagetool`** (`internal/dev/prereqs`) are downloaded without checksum verification. Neither Google nor the AppImage project publishes an official checksum for these specific artifacts — this is a documented, accepted risk, not an oversight. See the [internal decisions log](../architecture/decisions.md) for the full reasoning (and why `appimagetool` specifically stays on the `continuous` release tag rather than a "stable" one).
- **Arch-based distros are not supported.** `internal/distro` only detects Debian and Fedora families; there is no plan to add Arch support unless that changes.
- perci itself has no telemetry, background process, or daemon — if something looks "stuck," it's a single synchronous command; check the terminal output directly rather than looking for a service to restart.

## Still Stuck?

Check the [Configuration reference](../reference/configuration.md) for the exact expected behavior of the setting or screen you're using, or open an issue with the version shown on **Home → Visão Geral** and the exact steps that misbehaved.

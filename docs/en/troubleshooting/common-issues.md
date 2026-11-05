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
Post-installation entries under **Linux** are gated on distro + desktop environment — only the entry matching your detected distro/DE shows up, the rest are left out of the list entirely (see [Installation → Supported Distributions](../getting-started/installation.md#supported-distributions)). Perci detects the distribution from `/etc/os-release` and the desktop from `XDG_CURRENT_DESKTOP`; there is no setting to override them (the `distro`/`de` keys older docs mentioned are never read). If the entry is missing, check those two values in a terminal (`cat /etc/os-release`, `echo $XDG_CURRENT_DESKTOP`) and compare with the supported list.

## Linux and General Use

**The window won't close, or "Sair" in the tray does nothing, and the Execução tab shows a notice**
An action is still running. Perci doesn't quit under a running action (an interrupted package-manager run could leave the system half-configured); wait for it to finish on the **Execução** tab and close it again.

**An action ended as "concluído com avisos" instead of success**
The main work finished, but one or more secondary steps failed — for example Snap, Flatpak, journal vacuum or `autoremove` during **Atualizar Sistema**, or one app in a Flatpak or font batch. The warning lines in the terminal name what failed; re-run the action after fixing the cause.

**Post-installation items I applied before show up as available again**
The record of items without a detectable state is tied to the machine's `/etc/machine-id` (`postinstall_machine_id`). After reinstalling the OS while keeping `$HOME` (or the first time you run a version that records the ID) the old records no longer count, and those items are offered once more. Applying them again is safe; items that can be detected on the system (packages, sysctl file, swap, `fstrim.timer`, Flathub) were never affected.

**A font install fails with "nenhuma fonte .ttf encontrada" or a font removal is refused**
Downloaded fonts are installed under `~/.local/share/fonts/perci/<font>`; an archive that carries no `.ttf` fails instead of installing nothing. On Debian-family systems a packaged font that another package depends on is not removed (`dpkg --purge` refuses) — remove the dependent package first.

## Docker Application Stack

**Every row in Gerenciar Containers says "Docker inacessível"**
Perci couldn't query Docker, so the containers' status is unknown — hover the status to see Docker's message. Usually the Docker daemon isn't running (`sudo systemctl start docker`) or your user can't use its socket: add it to the `docker` group (`sudo usermod -aG docker "$USER"`), then log out and back in. Check with `docker ps -a` in a terminal.

**A row in Gerenciar Containers says "não encontrado"**
The container is registered in Perci but doesn't exist in Docker — it was removed outside Perci, or an import stopped before creating it. Use **Recriar** on that row.

**A new Container Aplicativo isn't reachable at its `*.localhost` URL**
Check that "Criar Container Nginx" ran first (it's the reverse proxy every app routes through) and that the browser trusts the mkcert wildcard certificate — `mkcert -install` runs automatically the first time the Nginx container is created, but only registers the CA for the invoking user. Also confirm the app's container is actually running (**Docker → Gerenciar Containers**).

**The browser says the `*.localhost` certificate isn't trusted (`SEC_ERROR_UNKNOWN_ISSUER`)**
The browser doesn't have mkcert's CA. `mkcert -install` covers the system store, Chrome/Chromium and Firefox's own `~/.mozilla/firefox` profiles; for browsers installed as Flatpaks (Firefox, Zen, LibreWolf, Chrome…), Perci registers it when the Nginx container is created or recreated — close the browser and use **Recriar** on the Nginx row in **Docker → Gerenciar Containers**, then open the browser again. To do it by hand, close the browser and run `certutil -A -d "sql:<profile folder>" -t "C,," -n "mkcert development CA" -i "$(mkcert -CAROOT)/rootCA.pem"`, or import `rootCA.pem` from `mkcert -CAROOT` in the browser's certificate settings (Authorities), trusting it to identify websites.

**Firefox or Zen says `SSL_ERROR_BAD_CERT_DOMAIN` for a `*.localhost` app**
Certificates made by older versions only named `*.localhost`, a wildcard Firefox and its derivatives refuse. Use **Recriar** on the Nginx row in **Docker → Gerenciar Containers**: the certificate is regenerated naming every app.

**Creating the Nginx container for the first time hangs or fails silently**
On first creation, `mkcert -install` may escalate to root on its own (mkcert's own binary, not Perci's privilege mechanism) to register the local CA — something the GUI can't answer, since it has no interactive terminal to type a password into (the "Docker → Criar Container" screen already warns about this when it detects it's the first time). If it happens: open a terminal outside Perci and run `mkcert -install` manually once — after that the certificate already exists and "Criar Container Nginx" never needs to call that part of mkcert again.

**A mounted Moodle plugin stays "Reinício pendente" or the plugin's pages fail**
A running container only sees a new mount after it restarts — use the row's restart action in the **Plugins montados** tab of **Dev Tools → Plugins Moodle** (it also restarts Nginx). "Montagem inativa" means the host mount is gone (unit removed or failed: `systemctl status <unit>`); remove the row and create it again. Don't use a symbolic link for a plugin instead: inside the container the link points outside the mounted code, and PHP's `__DIR__` resolves it, so `config.php` isn't found.

**Moodle shows a mounted plugin as "missing from disk" and its files went to `.debris`**
The development folder and the Moodle's `html` folder are synced by MEGAsync, which identifies files by inode and took the mount for a move between syncs, sending the files to the `.debris` folder of the development folder's sync. Restore them from MEGA (Rubbish bin or version history, which keep the original paths), then remove the row in the **Plugins montados** tab of **Dev Tools → Plugins Moodle** and create it again: Perci now excludes the destination in the nearest `.megaignore` before mounting (see [Moodle Plugin Mounts](../architecture/appstack.md#moodle-plugin-mounts-moodleplugingo)). Mounts created by an older version don't have that line — remove and recreate them.

**An app can't be created or recreated: "o caminho … não pode ser montado no contêiner"**
Project folders are bind-mounted with `--mount`, whose value can't carry a comma, a double quote or a line break. Rename the folder (or the workspace folder) without those characters.

**An app container doesn't start after its `html` or `logs` folder was deleted**
With `--mount`, Docker no longer recreates a missing source folder, so starting (or the restart at boot) fails. Use **Recriar** in Docker → Gerenciar Containers: it creates the folders again.

**The PHP 7.4 / 8.0 image builds, but I'm worried about updates**
These two images are based on Debian bullseye, which is out of support, so their packages come from `archive.debian.org` and they receive no more security updates. Use them only for legacy projects; prefer 8.1 or newer when the project allows.

**Xdebug can't reach my IDE from an app created before**
Containers now start with `--add-host host.docker.internal:host-gateway`; an existing container doesn't have it until you **Recriar** it in **Docker → Gerenciar Containers**.

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

## Git Accounts and Repositories

**Verificar, Baixar or a clone fails with "confirme a chave do servidor … em Configurar Conta"**
The account's server key isn't trusted yet. github.com and gitlab.com are trusted when the account is saved; for a self-hosted host, open the account in **Configurar Conta**, use **Verificar chave do servidor** and confirm the fingerprints after comparing them with the ones your server administrator publishes.

**"a chave está bloqueada — desbloqueie-a em Configurar Conta" or "a chave tem senha e não há agente SSH nesta sessão"**
The key has a passphrase and isn't loaded in the session's SSH agent; Perci never asks for a passphrase during a network action. Use **Desbloquear chave** in the account's details. Without an SSH agent in the session (no `SSH_AUTH_SOCK`), use a key without a passphrase, or start an agent for your desktop session.

**"o servidor recusou a chave — cadastre a chave pública …" or "a chave pertence ao usuário …"**
The public key isn't registered on the provider, or it is registered on another user. Copy it from the account's details, add it on the provider's SSH keys page (the link is next to it), then use **Testar conexão**.

**A repository under the account folder doesn't show in the table**
Repositories whose `origin` belongs to another owner or host — organization repositories, other users' repositories, remotes with an SSH host alias such as `git@github-work:` — are listed apart, under the table, as hidden. Accounts are personal by design. A folder inside another repository, a hidden folder, `node_modules` and `vendor` aren't searched.

**Every repository shows "HTTPS"**
Their `origin` uses an HTTPS URL. **Ajustar todas as URLs** switches them to the SSH URL of the same repository, after checking that the account key reaches each one; a repository the key can't reach keeps its URL.

**Baixar skipped a repository**
Baixar only fast-forwards. A repository with uncommitted changes to tracked files, without an upstream branch, with a detached HEAD or that diverged from the remote is skipped, with the reason on the Execução tab. Commit, stash or merge it with `git` in a terminal.

**The account's repository list says the token was refused, or the request was refused (rate limit)**
A refused token has expired or was revoked: create a new one on the provider and save it in **Configurar Conta**. Without a token, GitHub limits anonymous API requests; saving a token raises the limit and adds the private repositories.

**Novo Repositório refuses a URL ("não é um repositório de …")**
Only repositories owned by the account user can be cloned from the account screen. Clone others with `git clone` in a folder outside the account folders.

## AI Context / Skills

**Clicking "Aplicar" in IA: Contextos replaced my customized `AGENTS.md`**
`AGENTS.md` and the standards in `.instructions/` are regenerated by every Aplicar (the confirmation lists them first, and a `.perci-bak` keeps the previous version). Rules of your own belong in `.instructions/PROJECT.md`, which Perci never rewrites; on the first Aplicar of 1.7.0, lines you had added to `AGENTS.md` are moved there automatically. A standard marked **editado** on the screen was changed by hand and will be replaced.

**Aplicar fails with "pasta não escolhida no seletor"**
Folders handed to the AI Context, Repositórios and MCP screens must have been picked in the native folder dialog in this session (or already be registered). Use the folder-picker button (**Selecionar pasta**) instead of typing or pasting a path.

**A folder on the Contextos tab shows "pendente", and Atualizar and Alterar are disabled**
The folder doesn't exist on this machine — usually a folder imported before its repository was cloned, or a folder that was moved or renamed. Create or clone it at the path shown: the row turns "desatualizado" and **Atualizar** writes the context. For a folder that moved, **Remover** it from the list (nothing is removed from disk) and define the context again at the new path.

**A folder with a context isn't on the Contextos tab**
Folders whose context was applied before 1.8.0 aren't on the list. Open the folder on **Definir Contexto**, type a **Nome** and click **Registrar** on the "Contexto não registrado" alert.

**Importar stays disabled with a warning about a name**
Each imported folder needs a name not used by another folder on this machine nor by another folder of the import. Change the name in its row; folders marked "já registrada aqui" are kept as they are and can't be imported again.

**There is no `.claudeignore` anymore, and `*.perci-bak` files appeared**
Claude Code ignores `.claudeignore`, so Perci stopped generating it: the equivalent read rules are merged into the project's `.claude/settings.json` (`permissions.deny`); `.aiexclude` and `.geminiignore` are only for Gemini. If an old `.claudeignore` is found, a warning says it can be deleted. A `<file>.perci-bak` is the previous version of a generated file that Perci replaced with different content — delete it when you no longer need it.

**A file was "kept" with the warning "não foi gerado pelo Perci ou foi editado"**
Unchecking an agent or a standard removes only files that are still exactly what Perci wrote. A `CLAUDE.md`, `GEMINI.md`, `.aiexclude` or rule file you wrote or changed is kept; delete it by hand if you no longer need it.

**The agent can't read images, lockfiles or a vendored folder**
Regenerate the context with **Aplicar**: current versions keep images, PDFs and lockfiles readable, and keep `vendor/`, `dist/`, `build/`, `out/`, `env/` and `.next/` readable when git tracks files in them; read rules older versions added are removed. OpenCode, Codex and Antigravity get no read rules at all.

**Registering an MCP server fails with "já existe um servidor com esse nome que não foi registrado pelo Perci"**
An agent (Antigravity, Claude Code or Codex) already has a server with that name that Perci didn't create; Perci never overwrites or removes those. Rename or remove it in the agent's own configuration, then register again.

**An MCP server or SKILL row says "parcial: …"**
The registration reached only the agents listed after "parcial"; the others failed or weren't available. Check the terminal output of the run, fix the cause (for example a missing CLI) and run the action on that row again.

**MOODLE.md says the Moodle version is "unknown"**
The generator reads the version from a Moodle checkout's `version.php` (or `public/version.php`) or from a plugin's `$plugin->requires`, in the folder you picked in **IA: Contextos**. With neither, it writes "unknown (…)" and logs a warning — pick the project's actual root folder and click Aplicar again, or tell the agent the target version.

**Removing DaisyUI, "Dart: Oficial" or "Flutter: Oficial" in IA: SKILLs removed my other skills too**
Fixed: older versions removed those three entries (which install every skill of their repository) with `skills remove --skill '*'`, which the `skills` CLI treats as "every installed skill of these agents" in the chosen scope. Perci now looks up, with `skills list --json`, which installed skills came from the entry's repository and removes only those, by name — and refuses the removal if that lookup fails. Reinstall the skills you lost from the same screen.

## Development Environment

**After installing Go, my fish shell lost `/usr/bin` (and most commands) from `PATH`**
Fixed: older versions wrote `export PATH=$PATH:/usr/local/go/bin` to `~/.config/fish/config.fish`; in fish that unquoted line leaves `PATH` with just `/bin` and Go's `bin`. The line is now quoted (`export PATH="$PATH:/usr/local/go/bin"`, valid in bash, zsh and fish). To repair an existing install, click **Desinstalar** on Go in **Linguagens e SDKs** (removes Go and both forms of the line), then **Instalar** to reinstall. By hand: delete the old line from `config.fish` and open a new shell.

**Uninstalling an item is refused with "… também removeria: …"**
Before removing a prerequisite, terminal (Alacritty, GNOME Console) or IDE (VS Code, VSCodium) package, Perci simulates the removal and stops when the package manager would also remove other packages — nothing is removed. Look at the listed packages; if you are sure, remove it from a terminal yourself.

**Flutter's install mentions Android licenses**
Installing Flutter accepts the Android SDK licenses on your behalf (`sdkmanager --licenses`); the terms are linked in the item's description. Don't install it if you don't agree to them.

**A long download or `npx` step ends with a timeout error**
Unprivileged network commands (nvm, `npx`, `git`, `curl` downloads) are cut off after 30 minutes so a stalled connection can't hold the single running action forever. Check the connection and run the action again. `apt`/`dnf` steps are never interrupted this way. The Flutter clone (its full history is large) has no total limit: git aborts it when the transfer stays below 1 KB/s for 2 minutes.

## Known, Accepted Limitations (Not Bugs)

- **Arch-based distros are not supported.** `internal/distro` only detects Debian and Fedora families; there is no plan to add Arch support unless that changes.
- perci itself has no telemetry, background process, or daemon — if something looks "stuck," it's a single synchronous command; check the terminal output directly rather than looking for a service to restart.

## Still Stuck?

Check the [Configuration reference](../reference/configuration.md) for the exact expected behavior of the setting or screen you're using, or open an issue with the version shown on **Home → Visão Geral** and the exact steps that misbehaved.

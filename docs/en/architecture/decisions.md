🌐 [Português](../../pt-br/architecture/decisions.md) | **English** | 🏠 [Index](../index.md)

---

# Internal Decisions Log

Internal technical decisions and accepted risks that aren't obvious from reading the code alone.

## 2026-07-14 — appimagetool: keep downloading from the "continuous" GitHub release tag

**Context**: A code review flagged `internal/dev/prereqs/catalogue.go` (`installAppImageTool`) for
downloading `appimagetool-<arch>.AppImage` from AppImageKit's `continuous` release tag without
checksum verification, running with `RequiresSudo: true`. The suggested fix was to pin a specific
stable, versioned release and its published checksum instead.

**Finding**: verified directly against the GitHub API
(`gh api repos/AppImage/AppImageKit/releases`). The last tagged "stable" release (`13`, published
2020-12-31) has every asset explicitly renamed with an `obsolete-` prefix by the AppImageKit
project itself (e.g. `obsolete-appimagetool-x86_64.AppImage`). The `continuous` tag is the actual
maintained distribution channel and has been for years — not an oversight in our code. No release,
stable or continuous, publishes a `checksums.txt` or per-file `.sha256`; only `.zsync` files exist,
which support incremental/delta downloads and provide no integrity guarantee against a compromised
source.

**Decision**: keep `installAppImageTool` downloading from the `continuous` tag over HTTPS, with no
checksum verification. Pinning a "stable" version would install AppImageKit's own admittedly
obsolete tooling, and there is nothing upstream to verify a downloaded asset against either way.
This is the same accepted-risk category as the `curl | sh` installer pattern used elsewhere in
`internal/dev/*` (Starship, nvm, Zed, Claude Code, Kitty, Linux Toys): HTTPS transport is the only
integrity guarantee available, because the upstream project doesn't publish anything stronger.

**Revisit if**: AppImageKit ever starts publishing checksums or GPG signatures for `continuous`
assets, or ships a new tagged stable release that isn't marked obsolete.

**Superseded (2026-09-29)**: the tool moved to its own repository (`AppImage/appimagetool`), which
publishes tagged releases with a SHA-256 digest per asset in the GitHub API. `installAppImageTool`
now pins version `1.9.1`, verifies that digest, and installs the file through
`executor.PrivilegedInstall` (root re-verifies the checksum on a root-owned copy and installs it as
`root:root` 0755). The accepted risk above no longer applies.

## 2026-09-14 — Dropped arm64 from published releases when switching to the Wails GUI

**Context**: Removing the TUI/CLI (`internal/tui`, `internal/app`, `cmd/prci`) left `cmd/prci-gui`
(Wails) as the only binary to build and release. The old release pipeline cross-compiled the
pure-Go TUI/CLI binary for both `linux/amd64` and `linux/arm64` with a plain `GOARCH=arm64 go
build` — no cgo involved.

**Finding**: `cmd/prci-gui` requires cgo (GTK3/WebKit2GTK bindings via Wails' Linux backend).
`GOARCH=arm64 go build` alone does not cross-compile a cgo-dependent binary — it needs an arm64
cross-toolchain (e.g. `CC=aarch64-linux-gnu-gcc`) and the arm64 versions of the GTK3/WebKit2GTK
`-dev` packages installed on the build host, neither of which `.github/workflows/release.yml`
(a standard `ubuntu-latest` amd64 runner) provisions.

**Decision**: `make release`/`release.yml` now only build/publish `linux/amd64`. Setting up arm64
releases would need either a native arm64 GitHub-hosted runner or a dedicated cross-compilation
toolchain step — treated as separate, future work, not blocking this change (no known perci user
was running the previous arm64 build; the project has no confirmed production users yet). Building
from source on an arm64 machine directly is unaffected and documented as the fallback
(`docs/en/getting-started/installation.md`).

**Revisit if**: someone actually needs perci on arm64 Linux, or GitHub Actions arm64 runners become
freely available for public repos.

## 2026-09-14 — Migrated the GUI from Wails v2 to v3, adopting the new GTK4/WebKitGTK 6.0 default stack instead of the legacy GTK3 opt-in

**Context**: A system-tray feature is planned for `cmd/prci-gui`. Wails v2 has no native systray
support and no multi-window support either; both are needed (systray for the feature itself,
multi-window because the planned tray interaction opens a separate window from the main one).

**Finding**: Wails v3 (`v3.0.0-beta.22`) provides both natively. It also changes the default Linux
system dependency from GTK3 + WebKit2GTK 4.1 to GTK4 + WebKitGTK 6.0 — a deliberate upstream
choice, not a side effect: GTK4/WebKitGTK 6.0 is v3's stable, default stack, with GTK3/WebKit2GTK
4.1 kept only as a legacy opt-in behind `-tags gtk3`. v2 required `-tags
desktop,production,webkit2_41` on every build; v3's default stack needs no build tags at all. The
frontend binding mechanism also changed: v2 auto-injected a `window.go.main.App.*` global at
runtime, while v3 generates ES module bindings ahead of time via `wails3 generate bindings`.

**Decision**: migrate `cmd/prci-gui` to Wails v3 on its default GTK4/WebKitGTK 6.0 stack (not the
`-tags gtk3` legacy path — no reason to carry the older, non-default stack forward). Build tags are
dropped entirely (`go build ./cmd/prci-gui`, no `-tags`). Generated bindings are committed to
`frontend/dist/bindings/`, the same vendoring treatment already used for xterm.js — regenerated via
`make generate-bindings` only when a bound `App` method's signature changes or a new one is added,
not as part of the normal build. `go build`/`go vet`/`go test`/`gofmt` all pass post-migration.

**Revisit if**: Wails v3 stabilizes past beta with a breaking change to the bindings or systray API,
or the planned system-tray feature is dropped and multi-window/systray support turns out to be
unused.

## 2026-09-29 — Frontend split into classic scripts sharing one global scope

**Context**: The whole frontend was one inline `<script type="module">` (~2600 lines, one closure)
in `index.html`. The Content-Security-Policy added the same day could only allow it by hash, which
changed on every edit.

**Decision**: move the code to `frontend/dist/js/`, cut at the existing section boundaries:
`bootstrap.js` is the only ES module (imports the Wails bindings and runtime, exposes `App`/`Events`/
`Browser`), and the rest are classic `defer` scripts that share one global scope, loaded in order.
That keeps the exact semantics of the old closure — every screen reads and updates the same shared
state — without a bundler (a deliberate constraint of the project) and without rewriting ~2600 lines
into explicit module imports/exports. Verified by rendering every sidebar item and the tray's compact
window in headless Chrome, before and after, with a mocked Wails runtime: identical visible content,
no errors, the same backend calls. `script-src 'self'`, no hash.

**Revisit if**: the frontend adopts a build step, or shared globals start colliding (a proper ES
module split with explicit exports would then be the next step).

## 2026-09-29 — One long-running action at a time, app-wide

**Context**: `runAction` started every action on its own goroutine with no serialization; only the
frontend's per-window `running` flag prevented overlaps, and the tray's compact window has its own.
Two actions at once interleave their terminal output, race on the package manager's lock and on the
Nginx config, and prompt for the password twice.

**Decision** (chosen by the user among a global or a per-domain lock): a single package-level mutex
(`runMu`) for every service and window. A second action is refused with a synchronous error returned
only to its caller (never through `action-done`, which every window receives); a panic in an action
becomes a failed action instead of killing the process.

**Revisit if**: a real need appears to run actions from different domains in parallel.

## 2026-09-29 — Tray autostart starts Perci hidden

**Context**: The tray's autostart entry said "Perci iniciado oculto na bandeja", but the main window
always opened at login.

**Decision** (user's choice): the autostart entry runs `prci --hidden`; `main.go` creates the main
window hidden only when that flag is present **and** the tray is enabled (otherwise the app would be
unreachable). The entry is rewritten at every start while the tray is on, so older entries get the
flag.

**Revisit if**: Wails gains single-instance handling, so opening Perci from the menu while it's in
the tray focuses that instance instead of starting a second one.

## 2026-09-29 — Linux Toys installer runs as root, from stdin

**Context**: The official linux.toys installer escalates with its own `sudo`, which can't ask for a
password inside the GUI (no terminal) — without cached credentials the install just failed.

**Decision** (user's choice, over installing the release's `.deb`/`.rpm` or dropping the item): run
the official script as root through pkexec (one prompt). The script reaches root on stdin (`bash -s`),
never as a file path, so a process of the same user can't swap it during the password dialog; a
non-tty stdin also makes it take its own non-interactive branch. The accepted "always latest, no
checksum" risk of this installer is unchanged.

**Revisit if**: the script stops working as root, or the project wants checksum-verified installs —
the release publishes `.deb`/`.rpm` with SHA-256 digests.

## 2026-09-29 — MariaDB credentials over existing data are refused, not rewritten

**Context**: The `mariadb` image applies `MYSQL_USER`/`MYSQL_PASSWORD`/`MYSQL_ROOT_PASSWORD` only to
an empty data directory. Editing or re-creating MariaDB with other credentials over existing data
left `config.yaml` with credentials the database didn't know.

**Decision** (user's choice, over applying the change with `ALTER USER`): refuse with a clear message
(keep the credentials, change them inside the database first, or remove the data directory).
`data_user`/`data_pass` record the credentials the data directory was initialized with and survive
removing the container from the stack; configs from before they existed get a warning, not a refusal.

**Revisit if**: changing the database credentials from Perci becomes a real need.

## 2026-09-29 — "Atualizar Sistema" no longer deletes logs or removes packages on its own

**Context**: Every system update also vacuumed the journal (`journalctl --vacuum-time=7d`) and ran `apt-get autoremove`/`dnf autoremove` — deleting logs someone may need for diagnosis and removing packages the user never asked to remove.

**Decision** (the user's choice): both become checkboxes on the screen, unchecked by default; unused Flatpak runtimes follow the autoremove option. The update itself covers `apt`/`dnf` (`full-upgrade` only — the separate `upgrade` was redundant), Snap, and Flatpak in both the user and the system installation.

**Revisit if**: users routinely check both boxes, making opt-out the better default.

## 2026-09-29 — Removing Node.js still deletes `~/.nvm`, after an explicit warning

**Context**: The Node.js prerequisite is installed through nvm, and removing it deletes the whole `~/.nvm` folder — every Node version and global npm package there, including ones Perci didn't install.

**Decision** (the user's choice, over removing only the LTS Perci installed): keep deleting `~/.nvm`, but the GUI shows what will be lost and asks for confirmation before running. The nvm installer keeps coming from nvm's `HEAD`.

**Revisit if**: users keep their own Node versions under nvm and lose them, or nvm's `HEAD` installer breaks.

## 2026-09-29 — Base images are rebuilt when what they were built from changes

**Context**: `perci-php*`/`perci-node*`/combo images were built once and never again, so fixes to their Dockerfile, `php.ini` or `supervisord.conf` never reached machines that already had them.

**Decision** (the user's choice, over a manual "rebuild" action): each image carries a `perci.hash` label (a hash of its build files and build args). An image whose label doesn't match is rebuilt automatically the next time an app needs it, and the terminal names the containers that keep the old image until they're recreated.

**Revisit if**: rebuilds get slow or frequent enough to warrant asking first.

## 2026-09-29 — Self-update compares versions locally as semver

**Context**: Any difference between the running version and the latest release counted as an update — a local build newer than the release was offered a downgrade.

**Decision** (the user's choice, over adding `golang.org/x/mod/semver`): a small local comparison (MAJOR.MINOR.PATCH plus prerelease, build metadata ignored) in `selfupdate.IsNewer`. Versions that aren't semver (e.g. a `dev` build) keep the old behavior: any difference counts as newer.

**Revisit if**: release tags stop following semver.

## 2026-09-29 — Fedora: Flutter's JDK is Eclipse Temurin 21

**Context**: Fedora 44's repositories only ship JDK 25 (`java-latest-openjdk`), and the Android/Flutter toolchain Perci sets up targets JDK 21. Checked in a `fedora:44` container.

**Decision** (the user's choice): on Fedora, install `temurin-21-jdk` from Adoptium's GPG-signed repository (`distro.InstallFromSignedRepo`); Debian/Ubuntu keep `openjdk-21-jdk` from their own archives.

**Revisit if**: Fedora ships a JDK 21 package again, or the toolchain moves past JDK 21.

## 2026-09-29 — An empty `workspace_path` means "not chosen", everywhere

**Context**: `config.Load` filled in `~/workspace` only when `config.yaml` didn't exist; a file without `workspace_path` came back empty. Importar configurações adopts the exported workspace only when the local one is empty, so whether it adopted or refused depended on whether any other setting had ever been saved.

**Decision** (the user's choice, over always filling the default in): `Load` never fills a field in. Empty means "not chosen yet" with or without a file — the Configurações screen says so, Importar configurações adopts the exported workspace — and `Config.Workspace()` resolves `~/workspace` for code that needs an actual folder. The `distro`/`de` keys, never read by anything, were removed.

**Revisit if**: another setting needs a default applied on load.

## 2026-09-29 — `npx`/`uvx` packages are pinned

**Context**: The skills.sh CLI and the Filesystem/SQLite MCP servers ran as "whatever `npx`/`uvx` resolves as latest", so a compromised or broken release would reach every machine on its next run.

**Decision** (the user's choice): pin `skills@1.7.0`, `@modelcontextprotocol/server-filesystem@2026.8.31` and `mcp-server-sqlite@2025.4.25` (constants in `internal/dev/agentskills` and `internal/dev/mcpservers`), each checked to run; upgrading is a deliberate bump.

**Revisit if**: keeping the versions current becomes a burden, or the registries add a way to verify releases.

## 2026-09-29 — Unwired code left from the TUI: removed, except two features

**Context**: A `deadcode` pass (golang.org/x/tools) found functions with no caller, left from the TUI/CLI.

**Decision** (the user's choice): remove the bulk "update everything installed" helpers (`ide`/`llm`/`terminal.Update`, `checklist.UpdateInstalled`), `appstack.ContainerLogs` and `flutter.Upgrade`. Keep `internal/dev/terminal/contextmenu.go` (file-manager "open terminal here" entries) and `terminal.UninstallStarship`, to be wired into the GUI later. `golang.EnsurePathInBashrc` turned out to be a missing call, not dead code: installing Go never added `/usr/local/go/bin` to PATH, and now does.

**Revisit if**: the two kept features stay unwired — then remove them too. (Both wired on 2026-10-01 — see below.)

## 2026-09-29 — Frontend tests in plain Node, and a CI workflow

**Context**: The frontend had no automated test, and the only workflow (`release.yml`) ran the Go checks only when a tag was published.

**Decision** (the user's choice, over bringing the headless-Chrome harness into the repository): a `node:test` suite with no npm dependencies — the classic scripts run in a Node `vm` context with a minimal fake DOM — plus `node --check` on every script (`make test-frontend`). A new `ci.yml` runs gofmt, go vet, golangci-lint, `go test -race` and the frontend suite on every push to `main` and every pull request, with the same minimal permissions and SHA-pinned actions as `release.yml`; it uses the runner's preinstalled Node instead of adding a setup action.

**Revisit if**: a regression slips through that only a real browser would have caught — then add the headless-Chrome walk as an optional CI job.

## 2026-10-01 — Terminal context-menu entries follow the installed state; Starship gets a remove button

**Context**: `internal/dev/terminal/contextmenu.go` and `terminal.UninstallStarship` were kept unwired on 2026-09-29, to be wired into the GUI later.

**Decision** (the user's choice): every "Aplicativos: Terminais" run ends by syncing the "Abrir no <terminal>" entries with what is installed (`SyncContextMenuEntries`) — no extra screen or option, and terminals installed before (or outside Perci) are covered. Starship gets a second extra button, **Remover Starship**, with a confirmation and always enabled (the function is idempotent); the frontend's `MULTISELECT_SCREENS` entry now takes a list of `extras`.

**Revisit if**: users want the entries without the terminal being managed by Perci, or per terminal — then a separate option per item.

## 2026-10-01 — Wildcard SKILL entries are removed by name; Go installs in one batch with a fish-safe PATH line

**Context**: Writing tests for `internal/dev/{agentskills,golang,llm,sdks}` surfaced three bugs: removing an `Arg: "*"` SKILL entry ran `skills remove --skill '*'`, which deletes every skill of the four agents in the scope (confirmed against `skills@1.7.0` in a temporary HOME); installing Go made one pkexec call per step; and the unquoted Go PATH line broke `PATH` under fish (confirmed on fish 4.0.2).

**Decision** (the user's choices): the wildcard entries are removed by the names of the skills whose recorded source is the entry's repository — looked up with `skills list --json` (machine-readable, with the source normalized to `owner/repo`) rather than parsing the human-only output of `skills add --list`; a failed lookup refuses the removal. Go's extraction and swap run as one privileged script that also re-verifies the checksum on a root-owned copy. The Go PATH line is quoted (valid in bash, zsh and fish); the legacy line is still recognized.

**Revisit if**: the `skills` CLI changes its `list --json` shape or stops recording `source` — then the catalogue needs explicit skill names per repository.

## 2026-10-01 — Alacritty in one batch, OpenCode leftovers removed, workspace path from the dialog only

**Context**: Testing `internal/dev/terminal`, `internal/dev/llm` and the GUI's bound methods found: Alacritty on the Debian family ran `add-apt-repository universe` (absent on Debian itself), `apt-get update` and `apt-get install` as three password prompts; uninstalling OpenCode left `~/.opencode/bin` and the PATH lines its installer adds to the shell rc; and `SetWorkspacePath` stored any path the page sent, unlike every other bound method taking a path.

**Decision** (the user's choices): Alacritty installs through `distro.InstallPkgs` (one batch; the package is in Debian's main and in Ubuntu's universe, enabled by default). Uninstalling OpenCode removes `~/.opencode/bin` (`~/.opencode` only when left empty) and only the exact `# opencode` lines its installer wrote; settings in `~/.config/opencode` stay. `SetWorkspacePath` accepts only a folder picked in the native dialog this session (`requireApprovedPath`), since Docker creates folders under the workspace and mounts it into containers.

**Revisit if**: a supported Debian-family distro ships without `universe` enabled, or OpenCode's installer changes its install directory or rc lines.

## 2026-10-02 — Moodle version buckets follow each release's PHP range

**Context**: The "5.1+" bucket offered PHP 8.2–8.4, but it covers Moodle 5.1, 5.2 and 5.3, and 5.2/5.3 require PHP 8.3–8.4. The single "4.x" bucket offered PHP 8.0–8.1, while Moodle 4.1 supports 7.4–8.1, 4.2/4.3 support 8.0–8.2 and 4.4/4.5 support 8.1–8.3 (docs.moodle.org PHP requirements).

**Decision** (the user's choices): "5.1+" offers only PHP 8.3/8.4 (still valid for 5.1). "4.x" is split into three buckets with their exact ranges: `4.1` (7.4–8.1), `4.2-4.3` (8.0–8.2) and `4.4-4.5` (8.1–8.3), all on the classic Nginx recipe. `4.x` is no longer offered but stays valid, with the 4.1 range, for containers already saved with it, and the edit form keeps showing it.

**Revisit if**: a new Moodle release changes its PHP range or Nginx layout, or perci adds a PHP version.

## 2026-10-09 — System Flatpaks update inside the privileged batch

**Context**: "Atualizar Sistema" asked for the administrator password several times on a Zorin machine. Packages, Snap and the journal already ran as one privileged batch; Flatpak ran afterwards as the user, so updating or cleaning the system installation went through Flatpak's own authorization (polkit), whose outcome depends on each distro's rules and the user's groups.

**Decision** (the user's choice): the system installation's `flatpak update --system` and, with the autoremove option, `flatpak uninstall --system --unused` run as root inside the same batch (both Soft, so a Flatpak failure doesn't abort the update). The user installation is updated afterwards without privileges, and the autoremove option now cleans unused runtimes in both installations instead of only the configured scope.

**Revisit if**: another step of the update starts asking for authorization outside the batch.

## 2026-10-09 — Moodle plugins are bind-mounted on the host, not linked

**Context**: One plugin developed in its own folder (e.g. `oito2-hub/mdle-example`) must run in several Moodle containers (4.1 and 5.1) and be visible inside each Moodle's code in the editor. A symbolic link fails in Docker: the app container mounts only its own folder, and PHP resolves links in `__DIR__`, so `require_once(__DIR__ . '/../../config.php')` looks outside Moodle (tested in `perci-php81`). A Docker-only mount works in the container but leaves an empty folder in the editor. A host bind mount was tested end to end: after a container restart the plugin is a real folder inside it (`__DIR__` stays in Moodle), two containers share one source, files written by the container keep the user as owner, and Nginx's read-only mount sees it.

**Decision** (the user's choices): **Dev Tools → Plugins Moodle** replaces the unreleased Links Simbólicos screen. The plugin is mounted with a systemd mount unit (`Before=docker.service`), its type and name come from `version.php` (locked) or, for an empty folder, from the form; the Moodle is picked from a list of Perci's Moodle containers and the destination follows its `lib/components.json`. The status column checks the real state (device and inode on the host, in the container and in Nginx); the row restarts the container and Nginx to apply, and removing restarts them too. A destination with content is refused.

**Revisit if**: Moodle stops resolving plugin pages through `__DIR__`, the containers start mounting the whole workspace, or systemd mount units aren't available on a supported distro.

## 2026-10-13 — Post-install items are applied one at a time

**Context**: Linux → Pós-instalação was a checklist with one "Executar" button; each selected item escalated on its own, so a run asked for the administrator password once per item, and nothing showed what had already been applied.

**Decision** (the user's choices): every item has its own **Aplicar** button, which turns into a disabled **Aplicado** once the item is in place; the "Executar" button is gone and an applied item is not run again. Applied state is detected on the system where the item leaves one (packages, sysctl file, swap, `fstrim.timer`, Flathub apps, Ubuntu components, RPM Fusion); the items that leave none (package index refresh, system upgrade, driver detection, cleanup) are recorded in `postinstall_applied` after a successful run. An item whose dependency isn't applied keeps its button disabled with a "Requer: <item>" hint.

**Revisit if**: an item gains a detectable state, or re-running an applied item becomes necessary.

## 2026-10-13 — Prerequisites are managed one item at a time, in dependency order

**Context**: Desenvolvimento → Pré-requisitos was a checklist applied as one batch (check to install, uncheck to remove), with no way to update an item and nothing stopping an item from being installed before what its installer needs (curl for the GitHub CLI, Node.js and AppImageTool; Go for the Wails3 CLI).

**Decision** (the user's choices): a table with one row per item and its own **Instalar**, **Atualizar** and **Desinstalar** buttons — Instalar only while the item isn't installed, the other two only while it is. Atualizar upgrades the item's own packages through the package manager, moves Node.js to the latest LTS (via nvm, carrying over the global npm packages of the default version) and reinstalls the version-pinned Wails3 CLI and AppImageTool. Requirements: Curl → GitHub CLI, Node.js, AppImageTool; Flatpak → Flatpak Builder; Go (Linguagens e SDKs) → Wails3 CLI; those come first in the list, and a dependent item's Instalar stays disabled with "Requer: …" until they are installed. Uninstalling is always allowed for an installed item (an installed dependent keeps working) and always asks for confirmation.

**Revisit if**: an installer starts needing another catalogue item, or the pinned versions start following upstream releases.

## 2026-10-13 — Linguagens e SDKs uses the Pré-requisitos table

**Context**: Linguagens e SDKs (Go and Flutter) was the same install/remove checklist Pré-requisitos used to be. Go's installer downloads with `curl` and Flutter's starts with `git clone`, both Pré-requisitos items.

**Decision** (the user's choice: same layout and behavior as Pré-requisitos): one table row per SDK with Instalar/Atualizar/Desinstalar, shared with Pré-requisitos (one frontend screen, one service helper). Atualizar installs the latest stable Go only when it is newer than the installed one, and runs `flutter upgrade` for Flutter. Go requires Curl and Flutter requires Git, shown as "Requer: … (Pré-requisitos)" until installed. Uninstalling always asks for confirmation.

**Revisit if**: an SDK installer stops depending on those tools, or more SDKs are added with requirements between them.

## 2026-10-13 — Budgie support (Ubuntu Budgie 24.04/26.04, Fedora Budgie 44)

**Context**: Budgie 10.9 (Ubuntu Budgie 24.04) sets `XDG_CURRENT_DESKTOP=Budgie:GNOME` and Budgie 10.10 (Ubuntu Budgie 26.04, Fedora Budgie 44, Wayland with labwc) sets `Budgie` — read from the upstream session files. Ubuntu Budgie reports `ID=ubuntu`, so it already fell into the Ubuntu GNOME profile, which installs `gnome-tweaks` and `gnome-software-plugin-flatpak`; the latter depends on `gnome-software` (checked in `ubuntu:24.04`/`ubuntu:26.04`), a second software store on a desktop that uses Nemo and the snap store. Ubuntu Budgie 26.04 ships VLC as a package. Fedora's post-install actions don't depend on the desktop; the Budgie spin uses KDE apps (Dolphin, Konsole, Discover) and ships the Flathub remote.

**Decision** (the user's choices): `DetectDE` recognizes Budgie (before GNOME). Ubuntu Budgie gets its own 24.04/26.04 profiles: the Ubuntu list without `gnome-tweaks` and `gnome-software-plugin-flatpak`, and on 26.04 the Flatpak step installs only the web-app manager. Fedora Budgie gets a "Fedora 44 Budgie" profile with the Fedora actions.

**Revisit if**: Ubuntu Budgie adopts GNOME Software, or a Budgie release changes `XDG_CURRENT_DESKTOP`.

## 2026-10-13 — Code review hardening

**Context**: A full code review (security, bugs, consistency, documentation) raised more than a hundred findings, several with more than one reasonable fix. They were implemented together before publishing 1.2.0.

**Decision** (the user's choices): implement the whole plan before release. Remove Linux Toys entirely, with no uninstaller. Keep Flutter's full-history clone. Bound unprivileged network commands (`curl --connect-timeout/--max-time`, a 30-minute limit); nothing interrupts `apt`/`dnf` and there is no Cancel button. Record, per agent, which agents accepted an SKILL or MCP server, and show "parcial" when one is missing. Never overwrite an existing `credential.helper`; on Fedora "Libsecret Tools" installs `git-credential-libsecret`, and a fall back to `cache` is warned about. Warn in Flutter's description that the Android SDK licenses are accepted. Generate `MOODLE_PATH` as the relative `.`. Tie the post-install record to `/etc/machine-id`. Closing or quitting during an action doesn't quit: the main window opens on the Execução tab with a notice. Split the release workflow into reusable `test` and `build` jobs and add ShellCheck to CI. Keep the official installers, with timeouts, and pin nvm (tag), Codex (version) and the Nerd Font (SHA-256). Pin SKILLs by commit, turn the `skills` CLI telemetry off and show a third-party-content notice. Pin composer/pecl, take Node from the official Node image instead of NodeSource, and renew the mkcert certificate before it expires. Stop generating `.claudeignore` (Claude Code ignores it): merge `permissions.deny` read rules into `.claude/settings.json` and keep `.aiexclude`. Validate the Nginx config with `nginx -t` in a throwaway container when Nginx is stopped. Group terminal output every ~30 ms. Use `--mount` instead of `-v`, refusing a comma in the workspace path.

**Revisit if**: PHP 7.4/8.0 packages disappear from `archive.debian.org` (those images would have to be dropped), the `skills` CLI gains a lockfile for pinned installs, or a Cancel button for long actions is wanted.

## 2026-10-14 — Moodle plugin mounts are excluded from MEGA sync

**Context**: MEGAsync identifies files by inode. A plugin mount shows the development folder's inodes inside the Moodle's `html` folder; with both folders in different MEGA syncs, MEGAsync took the mount for a move between syncs and sent the development files to its `.debris` folder.

**Decision** (the user's choice, over refusing or only warning): before mounting, Perci writes `-dpG:<destination>` to the nearest `.megaignore` above the destination, refuses the mount when it can't, and removes the line with the mount. A line the user already had is left alone. Tested with real mounts, edits through the Moodle path, a MEGAsync restart and a reboot.

**Revisit if**: MEGAsync stops honoring `.megaignore` path rules before its inode move detection, or another sync client with the same behavior needs support.

## 2026-10-14 — Docker → Exportar/Importar, item by item, merging

**Context**: Export/Import lived under Gerenciar Contêineres, always carried the whole Docker stack, replaced it on import, and left the Moodle plugin mounts out.

**Decision** (the user's choices): its own menu item with Exportar and Importar tabs. Both list Nginx, MariaDB, each app and each plugin mount to check one by one; checking an item checks its dependencies. Only definitions travel (no project files or databases). Import merges: checked items are added or replace the app with the same folder, nothing else leaves. Plugin mounts are imported with one password; one that can't be made on the machine (missing development folder or Moodle code) is skipped with a warning.

**Revisit if**: the files and databases themselves need to travel too.

## 2026-10-14 — Rust and the Tauri v2 build dependencies

**Context**: Developing with Rust + Tauri v2 needs Rust, a C toolchain, the WebKitGTK 4.1/libxdo/OpenSSL/librsvg headers and, for Flathub, the offline source generators; Perci had none of these except GTK3. OpenSSL and Librsvg Tools only installed the command, and were detected by it.

**Decision** (the user's choices): Rust is a Linguagens e SDKs item (rustup, current user, pinned and checksum-verified installer, plus rust-analyzer). The libraries are separate Pré-requisitos items like the existing ones (Build Essential, WebKitGTK 4.1 Dev Headers, Libxdo Dev; OpenSSL and Librsvg Tools gain their headers and are detected by them too). Flatpak Builder Tools is an item pinned to a commit. Not added: `rpm` (Tauri builds the `.rpm` in Rust), `patchelf` (a real `tauri build` produced the AppImage, `.deb` and `.rpm` without it), `libayatana-appindicator3-dev` (only for tray icons), and the Flatpak SDK and extensions (`flatpak-builder --install-deps-from=flathub` installs the ones the manifest names).

**Revisit if**: a Tauri release needs `patchelf` or `rpmbuild` on the build machine, or a project needs a tray icon.

## 2026-10-14 — backports-budgie only on Ubuntu Budgie 26.04

**Context**: The user asked for an "Aplicar backports-budgie" post-install action (`add-apt-repository ppa:ubuntubudgie/backports-budgie`, `apt update`, `apt full-upgrade`). The PPA publishes to 22.04, 26.04 and a newer development series, not to 24.04: its `noble` repository answers 404, so `apt update` would fail there.

**Decision** (the user's choice): offer it only on Ubuntu Budgie 26.04, right after "Atualizar pacotes do sistema", in one privileged batch with the unattended apt options; it counts as applied while an enabled apt source points at the PPA.

**Revisit if**: the PPA starts building for another release Perci supports.

## 2026-10-19 — MEGAsync installer removed, IDEs in one table

**Context**: Linux → Aplicativos: MegaSync installed and removed MEGAsync from MEGA's signed repository. Zed Editor, VS Code and VSCodium each had their own three-button screen.

**Decision** (the user's choices): the MegaSync screen and `internal/system/megasync` are removed with no uninstaller left, like Linux Toys; the uninstallation guide lists the commands to remove the package, repository and key by hand. The `.megaignore` exclusion of Moodle plugin mounts stays, since it protects the development files of anyone running MEGAsync. Zed Editor, VS Code and VSCodium move to one **IDEs** item table (same pattern as Linguagens e SDKs, with requirements and an uninstall confirmation); Android Studio and Antigravity IDE keep their local-file screens.

**Revisit if**: MEGAsync installation is wanted back, or another IDE installed from a downloaded file joins the catalogue.

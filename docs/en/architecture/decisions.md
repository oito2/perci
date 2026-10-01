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

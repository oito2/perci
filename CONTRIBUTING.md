# Contributing to perci.gnl

🌐 **Language:** English · [Português](docs/pt-br/contribuindo.md)

Thank you for your interest in contributing! This document explains how to set up the development environment, our project conventions, and how to submit your changes.

---

## Development Environment

**Requirements:**
- Go 1.26 or higher
- Git
- System packages needed to build/run the GUI (Wails, cgo): `libgtk-4-dev libwebkitgtk-6.0-dev` (Debian/Ubuntu-based; see `cmd/prci-gui/README.md`)

**Setup:**

```bash
git clone https://github.com/oito2/perci.git
cd perci
go mod download
```

**Common Commands:**

```bash
# Build (development mode)
go build ./cmd/prci-gui

# Build with version injection
go build -ldflags "-X github.com/oito2/perci/internal/version.Version=v1.0.0" -o prci ./cmd/prci-gui

# Run directly
go run ./cmd/prci-gui

# Running tests
go test ./...
go test -race ./...

# Static analysis
go vet ./...
golangci-lint run

# Frontend smoke test (plain Node, no npm install): node --check on every
# script, then the node:test suite in cmd/prci-gui/frontend/test
make test-frontend

# Makefile shortcuts (same flags above)
make build
make test
make lint
```

### Writing Tests

- Privileged or system-changing flows are tested with `executor.Executor{DryRun: true, UsePolicyKit: true}` and asserted on the dry-run trace — typically that a flow asks for the password **once** (`strings.Count(out, "pkexec") == 1`). Set `Executor.LookPath` to fake which commands are installed; never rely on what the machine running the test has.
- Commands that must really run (Docker, mkcert) are replaced by a small script placed first on `PATH` with `t.Setenv("PATH", ...)` — see `internal/appstack/stack_test.go` and `internal/manager/db/backup_test.go`.
- Never touch real system paths: point `HOME` at `t.TempDir()`, and use the package's test seams (`selfupdate.executablePath`, `uninstallMenuPaths`, `serviceBase.emitFn`).
- A bound method that takes a folder or file path from the frontend must refuse a bad one before doing anything: add it to the table in `cmd/prci-gui/path_guards_test.go` (empty, relative, missing and not-picked paths).
- GUI actions are tested in DryRun through `newTestBase` with `isolateActions` (temporary `HOME`, empty `PATH`) — see `cmd/prci-gui/actions_test.go`. Under DryRun, shell-based "is it installed?" checks always answer yes, and downloads done by Go itself (not through the executor) still hit the network: leave those out.
- The frontend suite (`cmd/prci-gui/frontend/test`) loads the classic scripts into a Node `vm` context with a minimal DOM (`harness.mjs`); call the scripts' functions from the returned context, and use `evalIn` for their top-level `let` bindings.

CI (`.github/workflows/ci.yml`) runs gofmt, go vet, golangci-lint, `go test -race` and `make test-frontend` on every push to `main` and every pull request.

---

## Project Structure

```
perci/
├── cmd/prci-gui/       # Entry point — Wails GUI (the only interface; there is no CLI/TUI anymore)
│   ├── main.go
│   ├── service_base.go # Shared plumbing (wailsApp/exe, pickFolder, runAction, eventWriter) embedded by every service below
│   ├── service_*.go    # One application.Service per sidebar category (HomeService, LinuxService, DevSetupService, DockerService, DevToolsService, TrayService), exposed to the frontend via generated ES module bindings (frontend/dist/bindings/)
│   ├── catalog.go      # Sidebar categories/items shown in the GUI
│   └── frontend/dist/  # index.html + js/ (no build step — daisyUI/Tailwind/xterm.js vendored, strict CSP)
├── internal/
│   ├── ui/             # Terminal-panel primitives (PrintHeader, Info, Err, Success, Step…) — plain colored ANSI text, streamed into the GUI's xterm.js panel
│   ├── executor/       # Single point for sudo/pkexec escalation, incl. RunSudoSequence (one authentication for a whole batch of privileged commands)
│   ├── config/         # ~/.perci/config.yaml
│   ├── distro/         # Distro family detection (Debian/Fedora)
│   ├── sets/           # Set structures
│   ├── checklist/      # Shared "simple checklist" loop used by the checklist screens
│   ├── shellrc/        # Helpers for editing the user's shell rc files
│   ├── version/        # Version string (injected via -ldflags)
│   ├── selfupdate/     # Self-update via GitHub Releases (checksum-verified) + uninstall + menu icons
│   ├── system/         # "Linux" category (update, fonts, apps, templates, postinstall…)
│   ├── appstack/       # Per-project Docker application stack (Nginx/MariaDB/PHP/Node containers)
│   ├── dev/            # "Desenvolvimento" category (SDKs, AI apps, IDEs, terminals, MCP)
│   └── manager/        # "Dev Tools" category (AI context, .gitignore, repositories)
│       ├── ai/         # AI Context generation (CLAUDE.md, AGENTS.md, .instructions/*.md)
│       ├── db/         # MariaDB backup/restore/optimize
│       ├── gitignore/  # .gitignore generation based on the detected project type
│       └── repo/       # Git identity (global, init, clone, local ident, conduct)
├── packaging/          # perci.desktop + hicolor icon sets (application menu entry)
├── docs/
│   ├── en/             # Documentation site (English, canonical)
│   ├── pt-br/          # Documentation site (Portuguese mirror)
│   └── img/            # Logos, icons and screenshots
├── CODE_OF_CONDUCT.md  # Contributor Covenant
└── install.sh          # One-line installer
```

See [docs/en/index.md](docs/en/index.md) for the full documentation site — this file only covers what's needed to start contributing.

> **History note:** Perci used to ship as a Bubble Tea TUI plus a non-interactive CLI (`internal/tui`, `internal/app`, `cmd/prci`), before this became a Wails GUI-only app (2026-09-14). `internal/theme` and `internal/prompt` were removed in the same change — no domain package depends on `charmbracelet/*` anymore.

---

## Coding Conventions

- **Language:** All source code, comments, and documentation must be in English; all strings displayed to the user must be in Brazilian Portuguese (pt-BR).
- **Error Handling:** Always wrap errors using `fmt.Errorf("context: %w", err)`.
- **No Direct Sudo:** All privileged operations must go through `executor.Executor` using `RequiresSudo: true` (single command) or `RunSudoSequence` (a batch that must authenticate only once).
- **No Interactive Prompts in Domain Packages:** Domain functions (`internal/system/*`, `internal/dev/*`, `internal/manager/*`, `internal/appstack`) take plain parameters and never read stdin or show a form — all input/confirmation happens in the GUI (Wails frontend + the bound services in `cmd/prci-gui`, one `application.Service` per domain: `HomeService`/`LinuxService`/`DevSetupService`/`DockerService`/`DevToolsService`/`TrayService`). A GUI button click is itself the confirmation; there is no "are you sure?" prompt inside domain code.
- **Domain Function Signatures:**
  ```go
  func DoSomething(ctx context.Context, exe *executor.Executor, stdout io.Writer) error
  ```
- **UI Flow Pattern:** Terminal-panel output from a domain function follows `PrintHeader → Info/Warning → Err or Success` (streamed live into the GUI's Execução tab via `cmd/prci-gui/service_base.go`'s `eventWriter`).
- **Hidden, Not Disabled:** Menu items that are incompatible with the current distro or desktop environment must be left out of the list entirely, never shown greyed out.
- **No Premature Abstraction:** Implement only what is required.
- **Distro Detection:** Always use `distro.Detect()` — never write local distro detection functions in domain packages. Supported families: Debian, Fedora (Arch is not supported).

---

## Adding a New Menu Action

1. Add the entry to the relevant category's `Items` slice in `cmd/prci-gui/catalog.go` (`Title`, `Desc`, `ActionID`).
2. Implement the domain function in the appropriate package (plain parameters, no interactivity — see Coding Conventions above).
3. Add a Go method to the matching domain's `service_*.go` file in `cmd/prci-gui` (`HomeService`/`LinuxService`/`DevSetupService`/`DockerService`/`DevToolsService`/`TrayService`) that calls it — exposed to the frontend via the generated ES module bindings in `frontend/dist/bindings/` (run `make generate-bindings` after adding a method or changing one's signature).
4. Wire the screen in the frontend: a container `<div>` in `cmd/prci-gui/frontend/dist/index.html`, its render function in `frontend/dist/js/screens/`, an entry in `screenTable()` (`js/navigation.js`) and, when it must refresh after an action finishes, in `ACTION_DONE_HANDLERS` (`js/core.js`) — or reuse an existing mode (simple/multiselect/singleapp/etc.) when the screen fits one already. Chain `.catch(failRun)` onto every call that starts an action after `startExecution()`.

---

## Adding a New Flatpak App

Edit `internal/system/apps/catalogue.go` and add an entry to the `Catalogue` slice:

```go
{Name: "App Name", FlatID: "com.example.AppID"},
```

---

## Adding a New AI App, IDE, or Terminal

Edit the corresponding `catalogue.go` file inside `internal/dev/llm/`, `internal/dev/ide/`, or `internal/dev/terminal/` and add an entry to the `Catalogue` slice. Then implement the installation/uninstallation flow inside the respective `install.go` and `uninstall.go` files (plain functions, no prompts — see Coding Conventions above).

---

## Submitting Changes

1. Fork the repository and create a new feature branch.
2. Implement your changes following the conventions listed above.
3. Run `make lint`, `make test` and `make test-frontend` — all must pass (CI runs the same checks).
4. Open a Pull Request with a clear description of what was changed and why.

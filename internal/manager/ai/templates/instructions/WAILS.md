# Wails v3 Desktop Application Standard

Complement to `GO.md` for desktop applications built with Wails v3 (`github.com/wailsapp/wails/v3`), a Go backend rendering an HTML/JS frontend in the system webview.
Apply it when changing the application setup, services, events, windows, tray, frontend assets, build or packaging. Every rule in `GO.md` still applies.

---

## Version

- Wails v3 is in **beta**; the API below was checked against `v3.0.0-beta.28`. APIs can change between betas: confirm symbols in the version pinned in `go.mod` before using them.

```bash
go list -m github.com/wailsapp/wails/v3
go doc github.com/wailsapp/wails/v3/pkg/application <Symbol>
```

- The `wails3` CLI version must match the module version: `go install github.com/wailsapp/wails/v3/cmd/wails3@<same version as go.mod>`.
- Upgrade Wails as a dedicated change, then regenerate bindings and run the app manually.

---

## Project Layout

```text
cmd/<app>/
├── main.go            # application.New, windows, tray, wiring only
├── service_*.go       # bound services: thin adapters over internal/
└── frontend/
    ├── dist/          # assets embedded into the binary
    └── bindings/      # generated JS/TS bindings (path chosen by -d)
internal/              # all business logic, with no Wails import
```

- Business logic lives in `internal/` packages that **do not import Wails**, so it builds and tests without cgo, GTK or a display.
- Service methods validate input, call `internal/`, and translate results/errors for the frontend.

---

## Application Setup

```go
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name: "Example",
		Services: []application.Service{
			application.NewService(&ProjectService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.example.app",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				application.InvokeAsync(focusMainWindow)
			},
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Example",
		Width:  1100,
		Height: 720,
		URL:    "/",
	})

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
```

- `//go:embed all:frontend/dist` (the `all:` prefix includes files starting with `.` or `_`).
- Use `SingleInstance` for apps that own a tray icon or shared state; the `UniqueID` is a reverse-DNS identifier, constant across versions. Treat `SecondInstanceData.Args` as untrusted input.
- Cleanup belongs in `ServiceShutdown` or `Options.OnShutdown`; both block shutdown until they return, so keep them short.
- Wails' own messages go through `Options.Logger` (`*slog.Logger`); point it at the same stderr logger as the rest of the app.

---

## Services & Bindings

- A service is a pointer to a named struct passed to `application.NewService`. Its **exported methods** become frontend bindings (except `ServiceName`, `ServiceStartup`, `ServiceShutdown` and `ServeHTTP`).
- Optional lifecycle interfaces:

| Method | Called | Notes |
|---|---|---|
| `ServiceStartup(ctx context.Context, options application.ServiceOptions) error` | At startup, in registration order | `ctx` is cancelled right before shutdown; a non-nil error aborts `app.Run` |
| `ServiceShutdown() error` | At shutdown, in reverse order | Errors go to `Options.ErrorHandler` |
| `ServiceName() string` | For logs | Overridden by `ServiceOptions.Name` |

- A bound method whose **first parameter is `context.Context`** receives a context that is cancelled when the frontend cancels the call (the generated binding returns a cancellable promise). Use it for anything slow.
- Parameters and results must be JSON-serializable; use small named structs, not `map[string]any`.
- Return `error` as the last result; the JS promise rejects with it. Error text is shown to users: make it readable, and keep internals (stack traces, absolute paths, command lines with secrets) in the log.
- Panics in bound methods are recovered by Wails and returned as errors; still treat them as bugs.
- Assume bound methods can run concurrently: guard shared service state with a mutex.
- Unexported helper methods stay unexported so they are not exposed to the frontend.

### Regenerating bindings

After **any** change to a service's exported method set or signatures, or to types they use, regenerate the bindings and include them in the same change:

```bash
wails3 generate bindings -d <output-dir> ./...
```

- Use the exact flags the project already uses (e.g. `-names`, `-b`, `-ts`, `-d`); look in its `Makefile`/`Taskfile` first. `-clean` defaults to true and wipes the output directory.
- Follow the project's policy on committing generated bindings; if they are committed, never hand-edit them.

---

## Events

```go
func init() {
	application.RegisterEvent[ProgressEvent]("action:progress")
}

app.Event.Emit("action:progress", ProgressEvent{ID: id, Line: line})
```

```js
import { Events } from "@wailsio/runtime";

const off = Events.On("action:progress", (event) => render(event.data));
```

- Register custom events with `application.RegisterEvent[T]` from an `init` function, with a constant name: the binding generator then emits typed event definitions, and `Emit` checks the payload type.
- Use namespaced, stable names (`domain:verb`). Payloads are small structs.
- Events emitted by the frontend (`Events.Emit`) and received in Go (`app.Event.On`) are **untrusted input**: validate them like any bound-method argument.
- Unsubscribe listeners that are no longer needed (`On` returns a cancel function on both sides).

---

## Windows & Tray

- Create windows with `app.Window.NewWithOptions`; give windows that are looked up later a `Name`.
- Intercept closing with `window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { ... e.Cancel() })`, e.g. to hide to the tray or to block closing while an action runs.
- Tray: `tray := app.SystemTray.New()`, then `SetIcon`, `SetTooltip`, `SetMenu(menu)`; build menus with `application.NewMenu()` and `menu.Add(label).OnClick(func(*application.Context) { ... })`. Always offer a way to show the window and to quit.
- Code that must run on the UI thread from a goroutine uses `application.InvokeSync` / `application.InvokeAsync`.

---

## Long-Running Actions

- Bound methods return quickly. An action that runs commands, downloads or installs: validates input, starts a goroutine, and **streams progress through events** (output lines, step index, final status).
- Allow **one mutating action at a time** (or one per resource): reject a second start with a clear error instead of queueing silently, and expose the "running" state to the frontend.
- Every action is cancellable through a `context.Context` owned by the service; quitting the app or closing the window cancels it, and shutdown waits for it to stop.
- Subprocess rules from `GO.md` apply. Privilege escalation in a GUI (e.g. `pkexec`) uses an absolute path and a fixed argument list — never a command string built from frontend input.
- Final status is always reported (success, failure with message, or cancelled), even when the frontend reloaded meanwhile.

---

## Frontend Security

The webview runs with the same privileges as the Go process through the bindings. Treat the frontend as an **untrusted client**.

- Strict Content Security Policy, set as a `<meta http-equiv="Content-Security-Policy">` in `index.html` (or as a header via `AssetOptions.Middleware`):

```html
<meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'" />
```

- No inline `<script>` blocks or inline event handlers (`onclick=`); no `'unsafe-eval'`; no remote scripts or stylesheets — vendor third-party assets into `frontend/dist`.
- Insert backend data with `textContent` or proper escaping; never build `innerHTML` from command output, file contents or error messages.
- **Validate in Go, never in JS alone.** Every path, identifier, URL or option arriving from the frontend is checked in the service: allowed values, confinement to expected directories (see `GO.md` → File System), length limits.
- Never expose generic primitives as bindings (run any command, read any file, write any path). Expose specific operations.
- Release builds use the `production` build tag: without it, Wails runs in debug mode and enables the webview devtools.

---

## Linux Build Dependencies

Wails v3 on Linux uses cgo with **GTK 4 + WebKitGTK 6.0** by default (`pkg-config: gtk4 webkitgtk-6.0`). The `gtk3` build tag selects the legacy GTK 3 + WebKit2GTK 4.1 stack.

| Package manager | Build dependencies (default stack) |
|---|---|
| `apt` (Debian/Ubuntu) | `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev` |
| `dnf` (Fedora) | `gcc-c++ pkgconf-pkg-config gtk4-devel webkitgtk6.0-devel` |
| `pacman` (Arch) | `gcc pkgconf gtk4 webkitgtk-6.0` |
| `zypper` (openSUSE) | `gtk4-devel webkitgtk-6_0-devel` (plus a C compiler and pkg-config) |

Runtime dependencies of a packaged app: `libgtk-4-1 libwebkitgtk-6.0-4` (Debian/Ubuntu), `gtk4 webkitgtk6.0` (Fedora), `gtk4 webkitgtk-6.0` (Arch).

Check a machine with `wails3 doctor` and `wails3 tool capabilities`.

---

## Build & Packaging

```bash
go build -tags production -trimpath \
    -ldflags "-s -w -X <module>/internal/version.Version=${VERSION}" \
    -o bin/<app> ./cmd/<app>
```

- Because of cgo, build natively per OS/architecture (or with a matching cross toolchain); `GOOS`/`GOARCH` alone do not cross-compile.
- Projects scaffolded with `wails3 init` use the generated `Taskfile` (`wails3 build`, `wails3 package`); Linux packages (`.deb`, `.rpm`, Arch) come from `wails3 tool package -format <fmt> -config <nfpm.yaml>`, AppImage from `wails3 generate appimage`. Use whichever mechanism the project already has; do not introduce a second one.
- A Linux install ships the binary, a `.desktop` file and hicolor icons; uninstall instructions remove all of them.

---

## Testing

- Unit-test `internal/` packages normally (`GO.md` → Testing); they need no GTK.
- Test services **without a window**: construct the service struct directly and call its methods. Put Wails-specific dependencies behind small interfaces (e.g. an `emitter` with `Emit(name string, data ...any) bool`) so tests inject a recording fake instead of `*application.App`.
- Packages that import `github.com/wailsapp/wails/v3/pkg/application` compile only with cgo and the GTK/WebKitGTK development packages: install them in CI before `go vet`, `golangci-lint` and `go test`.
- Add tests that read `frontend/dist/index.html` and assert the CSP, the absence of inline scripts and remote assets, and that referenced script files exist.
- Run the app manually (`go run ./cmd/<app>`) after UI-affecting changes and say so in the report if it could not be done.

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| Changing a bound method without regenerating bindings | Frontend calls the wrong ID/signature at runtime | `wails3 generate bindings` in the same change |
| Business logic inside service methods | Untestable without cgo/GTK | Logic in `internal/`, services as adapters |
| Blocking bound method for a long action | Frozen UI, no progress, no cancel | Goroutine + events + context |
| Trusting paths or values from JS | Arbitrary file/command access from the webview | Validate and confine in Go |
| `innerHTML` with backend output | Script injection in a privileged webview | `textContent` / escaping |
| Inline scripts, `'unsafe-inline'` in `script-src`, remote CDNs | Weakens CSP, runtime network dependency | Bundled files under `frontend/dist`, `script-src 'self'` |
| Release build without `-tags production` | Devtools enabled for end users | `-tags production` |
| Several concurrent mutating actions | Conflicting system changes | One action at a time, explicit busy state |

---

## Quality

In addition to the `GO.md` checks (with the Linux build dependencies installed):

```bash
wails3 generate bindings -d <output-dir> ./...   # with the project's flags; then
git status --short                               # bindings must show no unexpected diff
go vet ./...
go test -race ./...
go build -tags production ./cmd/<app>
```

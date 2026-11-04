# Rust + Tauri v2 Standard

Base standard for applications built with **Tauri v2** and a Rust backend: desktop (Linux, Windows, macOS) and, where the project targets them, Android and iOS. Apply it to any task that touches `src-tauri/`, the Tauri configuration, capabilities, plugins, bundling or the app's release pipeline. The frontend complements (`TAURI-VANILLA.md`, `TAURI-SVELTEKIT.md`, `TAURI-REACT.md`) add rules for the webview side and are always used together with this file.

Last verified: 2026-11-04

Tauri v1 APIs, config keys (`tauri.allowlist`, `build.distDir`, `build.devPath`) and the `@tauri-apps/api/tauri` import path do not apply — never introduce them.

---

## Project Layout

The official scaffold (`npm create tauri-app@latest`) produces this layout. Keep it.

```text
<project>/
├── package.json            # frontend scripts + "tauri": "tauri" script
├── src/                    # frontend (see the frontend complement)
└── src-tauri/
    ├── Cargo.toml          # Rust crate: lib + bin, tauri / tauri-build / plugins
    ├── build.rs            # tauri_build::build()
    ├── tauri.conf.json     # app, build, bundle and plugin configuration
    ├── capabilities/       # one JSON/TOML file per capability
    │   └── default.json
    ├── icons/              # generated with `tauri icon`
    ├── gen/                # generated (schemas, mobile projects) — do not hand-edit gen/schemas
    └── src/
        ├── lib.rs          # run(): Builder, plugins, state, invoke_handler
        └── main.rs         # desktop entry point, calls <crate>_lib::run()
```

| File | Rule |
|---|---|
| `src/main.rs` | Keep `#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]`; it prevents an extra console window on Windows release builds. No logic here. |
| `src/lib.rs` | Keep `#[cfg_attr(mobile, tauri::mobile_entry_point)]` on `pub fn run()` and the `crate-type = ["staticlib", "cdylib", "rlib"]` lib section in `Cargo.toml` — mobile targets need both. |
| Commands | Group commands in modules (`src/commands/*.rs`) and register them in `lib.rs`. Commands written directly in `lib.rs` cannot be `pub` (glue-code limitation, error `E0255`). |
| `tauri.conf.json` | Always keep `"$schema": "https://schema.tauri.app/config/2"`. Platform overrides go in `tauri.linux.conf.json`, `tauri.windows.conf.json`, `tauri.macos.conf.json`, `tauri.android.conf.json`, `tauri.ios.conf.json`. |
| `identifier` | Reverse-domain, unique, never `com.tauri.dev`/`com.example.*` in a release. Changing it later changes the app's data directories — ask first. |

---

## Rust Conventions

- **Edition:** follow `Cargo.toml` (the current scaffold uses `edition = "2024"`). Do not downgrade it.
- **Formatting:** `cargo fmt`, default settings.
- **Lints:** `cargo clippy` must pass with `-D warnings`. Do not add `#[allow(...)]` to silence a lint without the user's approval.
- **Errors:** one error enum per crate/domain with `thiserror`, implementing `serde::Serialize` so commands can return it (see below). Never `Result<T, String>` as a long-term API.
- **No panics in command paths:** no `unwrap()`, `expect()`, `panic!`, indexing that can go out of bounds, or `todo!()` inside commands, event handlers or anything they call. A panic is not turned into an error for the frontend, and with the scaffold's release profile (`panic = "abort"`) it terminates the app. `.expect(...)` on the final `Builder::run(...)` in `lib.rs` (startup) is acceptable.
- **State:** register with `Builder::manage(...)`; read with `tauri::State<'_, T>`. Wrap mutable state in `std::sync::Mutex` (no `Arc` needed — Tauri already shares it). Use `tokio::sync::Mutex` only when the guard must be held across an `.await`.
- **Blocking work:** file I/O, hashing, network and anything slow go in `async` commands (they run off the main thread). Synchronous commands run on the main thread and freeze the UI.

### Serializable command error

```rust
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error(transparent)]
    Io(#[from] std::io::Error),
    #[error("invalid input: {0}")]
    InvalidInput(String),
}

#[derive(serde::Serialize)]
#[serde(tag = "kind", content = "message")]
#[serde(rename_all = "camelCase")]
enum ErrorKind {
    Io(String),
    InvalidInput(String),
}

impl serde::Serialize for Error {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        let message = self.to_string();
        let kind = match self {
            Self::Io(_) => ErrorKind::Io(message),
            Self::InvalidInput(_) => ErrorKind::InvalidInput(message),
        };
        kind.serialize(serializer)
    }
}

pub type Result<T> = std::result::Result<T, Error>;
```

The frontend receives `{ kind: "io" | "invalidInput", message: string }` as the rejection value, typed as `CommandError` (see [Frontend IPC](#frontend-ipc)). Never put secrets, full stack traces or absolute paths of the user's machine in error messages.

---

## Commands

- Annotate with `#[tauri::command]`, register **all** commands in a single `tauri::generate_handler![...]` call (only the last `invoke_handler` call is used). Command names must be unique.
- **Typed arguments:** take owned, typed parameters (`String`, `PathBuf`, structs deriving `serde::Deserialize`), not `serde_json::Value`.
- **Async commands:** borrowed arguments (`&str`, `State<'_, T>`) in an `async fn` require the command to return a `Result`. Prefer owned types plus `Result<T, Error>`.
- **Never trust the webview.** Treat every argument as attacker-controlled: validate lengths, ranges, enums, and canonicalize and check every path against an allowed base directory before touching the filesystem. The webview only says *what* it wants; Rust decides *whether* it is allowed.
- Return only the data the UI needs. Large binary payloads use `tauri::ipc::Response` instead of JSON serialization.
- Commands that need the window or app use `tauri::WebviewWindow` / `tauri::AppHandle` parameters. When writing testable commands, make them generic over `R: tauri::Runtime`.

```rust
use std::sync::Mutex;
use tauri::State;

#[derive(Default)]
pub struct AppState {
    counter: u32,
}

#[tauri::command]
pub async fn increment(by: u32, state: State<'_, Mutex<AppState>>) -> crate::Result<u32> {
    if by == 0 || by > 100 {
        return Err(crate::Error::InvalidInput("`by` must be between 1 and 100".into()));
    }
    let mut guard = state
        .lock()
        .map_err(|_| crate::Error::InvalidInput("state unavailable".into()))?;
    guard.counter = guard.counter.saturating_add(by);
    Ok(guard.counter)
}
```

---

## Frontend IPC

Rules for every frontend; the complements add only framework-specific parts.

- Import from the v2 modules: `invoke` and `Channel` from `@tauri-apps/api/core`, `listen`/`emit` from `@tauri-apps/api/event`.
- **One typed wrapper per command** in the frontend's API folder (`src/api/`, or `src/lib/api/` in SvelteKit) — the only place that imports `@tauri-apps/api/core`. UI code never calls `invoke("...")` with a string literal.
- Argument keys are **camelCase** by default (Rust `invoke_message` ↔ JS `invokeMessage`); only change it with `#[tauri::command(rename_all = "snake_case")]` consistently across the project.
- Type rejections with `CommandError`, kept in sync with the Rust `ErrorKind`, and handle every promise — never leave one unhandled.
- `listen()` returns a **Promise** of the unlisten function: keep it and call it when the view that registered the listener is torn down.

```typescript
// src/api/errors.ts — the canonical rejection type
export type CommandError = { kind: "io" | "invalidInput"; message: string };

// src/api/counter.ts
import { invoke } from "@tauri-apps/api/core";

export function increment(by: number): Promise<number> {
  return invoke<number>("increment", { by });
}
```

### Markup safety

The webview can reach IPC, so an XSS is a path into the backend.

- Render data from commands, events, files, URLs or user input as **text** (escaped); never through the framework's raw-HTML path (`innerHTML`, `{@html}`, `dangerouslySetInnerHTML`).
- Validate the scheme before building `href`/`src` from untrusted data; open external links through the opener plugin, not by navigating the webview.
- No `eval`, `new Function` or string-based `setTimeout`.
- `VITE_*` and `TAURI_ENV_*` variables ship inside the bundle: never put secrets in them, in `localStorage` or in frontend state.

---

## Permissions and Capabilities

Tauri v2 exposes nothing to the frontend unless a **capability** grants a **permission** to a window/webview.

- Capability files live in `src-tauri/capabilities/` (JSON or TOML) and use `"$schema": "../gen/schemas/desktop-schema.json"` (or `mobile-schema.json`). All files in that folder are enabled automatically; once `app.security.capabilities` lists identifiers in `tauri.conf.json`, **only** the listed ones are used.
- **Least privilege:** grant specific permissions (`core:window:allow-set-title`, `fs:allow-read-text-file`) instead of broad sets. Never grant a plugin's `*:default` or an `allow-*` permission "just in case".
- **Per-window capabilities:** scope each capability with `"windows": ["main"]` (window **labels**, not titles). A window in several capabilities gets the union of all of them — keep privileged windows separate. Only privileged windows may create new windows.
- **Platform-specific:** use `"platforms": ["linux", "macOS", "windows"]` or `["iOS", "android"]` for permissions that exist only on one family.
- **Scopes:** plugins such as `fs`, `http` and `shell` need an explicit `allow` scope; `deny` always wins over `allow`. Use base-directory variables (`$APPDATA`, `$APPLOCALDATA`, `$DOCUMENT`, ...) — never `$HOME/**` or `/**`.
- **App commands:** by default every command in `invoke_handler` is callable by every window. To restrict them, declare them with `tauri_build::AppManifest::new().commands(&[...])` in `build.rs` and grant the generated permissions per capability.
- **Remote content:** do not add a `"remote": { "urls": [...] }` capability without explicit user approval — remote pages then reach the IPC layer.
- Use `npm run tauri permission ...` / `npm run tauri capability new` (or `cargo tauri ...`) to manage them, and commit the result.

```json
{
  "$schema": "../gen/schemas/desktop-schema.json",
  "identifier": "main-capability",
  "description": "Main window: core defaults and app data folder access",
  "windows": ["main"],
  "permissions": [
    "core:default",
    "opener:default",
    "fs:allow-read-text-file",
    { "identifier": "fs:scope", "allow": [{ "path": "$APPDATA" }, { "path": "$APPDATA/**/*" }] }
  ]
}
```

---

## Content Security Policy and Webview Hardening

- The scaffold ships `"app": { "security": { "csp": null } }` (CSP disabled). Before any release, set a restrictive `app.security.csp`. Tauri appends nonces and hashes for bundled scripts and styles at compile time, so the policy only lists what is unique to the app.
- Allow IPC explicitly: `connect-src ipc: http://ipc.localhost`. Add `asset: http://asset.localhost` to `img-src`/`default-src` only if the asset protocol is used.
- No remote scripts (CDNs). Ship every script inside the bundle. Add `'wasm-unsafe-eval'` to `script-src` only if the frontend uses WebAssembly.
- Never use `dangerousDisableAssetCspModification` or `'unsafe-eval'` without explicit user approval.
- Consider `app.security.freezePrototype: true` (hardens against prototype pollution); test the app with it before shipping.
- Keep `app.withGlobalTauri` `false` unless the frontend has no bundler and needs `window.__TAURI__`.

```json
"security": {
  "csp": {
    "default-src": "'self'",
    "connect-src": "ipc: http://ipc.localhost",
    "img-src": "'self' data: blob:",
    "style-src": "'self' 'unsafe-inline'"
  }
}
```

---

## Plugins

- Use official plugins (`tauri-plugin-*` crates + `@tauri-apps/plugin-*` npm packages) before writing custom code for the same feature. Add them with `npm run tauri add <name>` (or `cargo tauri add <name>`): it adds the crate, the npm package and the `.plugin(...)` registration.
- Every plugin needs its permissions granted in a capability; dangerous commands are blocked by default.
- **Versions aligned:** keep `tauri` / `@tauri-apps/api` / `@tauri-apps/cli` and each plugin's crate / npm package on the same major.minor. The CLI checks this and refuses mismatches; do not pass `--ignore-version-mismatches` to get around it.
- Desktop-only plugins (e.g. updater, global-shortcut) are added as target-specific dependencies (`[target.'cfg(any(target_os = "macos", windows, target_os = "linux"))'.dependencies]`) when the project also targets mobile.
- `shell.open` is replaced by the **opener** plugin. The **shell** plugin's `allow-execute`/`allow-spawn` must list exact commands with argument `validator` regexes — never an unrestricted shell.

---

## Events and Channels

| Mechanism | Use for | Notes |
|---|---|---|
| Command (`invoke`) | Request/response | Typed, returns a value or a rejected promise |
| Event (`Emitter::emit`, `emit_to`, `listen`) | Small, occasional notifications | Not type-safe, JSON payloads only, not for high throughput or low latency |
| Channel (`tauri::ipc::Channel<T>`) | Streams: progress, process output, chunked data | Fast and ordered; the frontend creates the `Channel` and passes it as a command argument |

- Event payloads are `Clone + serde::Serialize` structs; name events in kebab-case (`download-progress`).
- Prefer `emit_to(label, ...)` over global `emit` when only one webview needs the event.
- Events are not covered by capabilities' fine-grained data control — never send secrets through them.
- Channel message enums use `#[serde(rename_all = "camelCase", tag = "event", content = "data")]` so the frontend gets a discriminated union.

---

## Filesystem, Paths and External Processes

- Resolve app directories through `app.path()` (`app_data_dir()`, `app_config_dir()`, ...) via the `tauri::Manager` trait — never hardcode `~/.config/...` or `%APPDATA%`.
- Do not grant the frontend access to `$APPLOCALDATA` (webview data lives there on Linux/Windows) or to the user's home tree.
- Prefer doing filesystem work in Rust commands with validated paths over exposing the `fs` plugin to the webview.
- Spawn external programs from Rust with fixed program paths and argument arrays; never build a shell command string from webview input.

---

## Updater

Only when the project ships auto-updates (`tauri-plugin-updater`):

- Generate keys with `npm run tauri signer generate -- -w ~/.tauri/<app>.key` (or `cargo tauri signer generate -w ...`). **Never commit the private key** or its password; keep them in a secret store.
- Provide them at build time through `TAURI_SIGNING_PRIVATE_KEY` and `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` environment variables (`.env` files are not read).
- `tauri.conf.json`: `bundle.createUpdaterArtifacts: true`; `plugins.updater.pubkey` holds the public key **content** (not a path); `plugins.updater.endpoints` are HTTPS URLs. Do not enable `dangerousInsecureTransportProtocol`.
- Losing the private key means existing installs can no longer be updated — say so to the user whenever keys are generated.

---

## Building and Platforms

The CLI runs through the npm script (`npm run tauri <cmd>`) or the Cargo subcommand (`cargo tauri <cmd>`, installed with `cargo install tauri-cli --version "^2.0.0" --locked`). Use whichever the project already uses.

| Task | Command |
|---|---|
| Dev run | `npm run tauri dev` |
| Release build + bundles | `npm run tauri build` |
| Only some bundles | `npm run tauri build -- --bundles deb,appimage` |
| Debug build | `npm run tauri build -- --debug` |
| Environment report | `npm run tauri info` |
| Icons | `npm run tauri icon <source.png>` |
| Android / iOS | `npm run tauri android init` / `android dev` / `android build`; `ios ...` only on macOS hosts |

Bundle types per OS: Linux `deb`, `rpm`, `appimage`; Windows `msi`, `nsis`; macOS `app`, `dmg`. Where they can be built: `msi` only on Windows; `nsis` on Windows, or cross-compiled from Linux/macOS with caveats (last resort: needs NSIS, LLVM/LLD and `cargo-xwin`, less tested, signing needs an external tool); `app`/`dmg` only on macOS; Linux bundles on Linux. Output goes to `src-tauri/target/release/bundle/` (`target/<triple>/release/bundle/` with `--target`).

### System prerequisites

| OS | Requirements |
|---|---|
| Debian/Ubuntu | `sudo apt install libwebkit2gtk-4.1-dev build-essential curl wget file libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev` |
| Fedora | `sudo dnf install webkit2gtk4.1-devel openssl-devel curl wget file libappindicator-gtk3-devel librsvg2-devel libxdo-devel` and `sudo dnf group install "c-development"` |
| Arch | `sudo pacman -S --needed webkit2gtk-4.1 base-devel curl wget file openssl appmenu-gtk-module libappindicator-gtk3 librsvg xdotool` |
| Windows | Microsoft C++ Build Tools ("Desktop development with C++"), WebView2 runtime, `rustup default stable-msvc`; building `msi` also needs the **VBSCRIPT** optional feature enabled (`failed to run light.exe` means it is off) |
| macOS | Xcode (or `xcode-select --install` for desktop only) |
| Android | Android Studio + SDK/NDK, `JAVA_HOME`, `ANDROID_HOME`, `NDK_HOME`, rustup Android targets |
| iOS | macOS host, full Xcode, CocoaPods, rustup iOS targets |

Tauri v2 needs **WebKitGTK 4.1** on Linux (`webkit2gtk-4.0` is v1). Re-check package names against the official prerequisites page when a distro release changes.

---

## Testing

- **Rust unit tests** next to the code (`#[cfg(test)] mod tests`) for every pure function and validator; keep command bodies thin wrappers around testable functions.
- **Runtime tests:** the `tauri` crate's `test` feature exposes `tauri::test` (`mock_builder()`, `mock_context()`, `noop_assets()`, `get_ipc_response()`, `assert_ipc_response()`) to run commands against a mock runtime without a webview. The module is marked unstable — keep these tests few and focused.
- **Frontend tests** mock IPC with `@tauri-apps/api/mocks` (`mockIPC`, `mockWindows`, `clearMocks`) — see the frontend complement.
- **End-to-end:** WebDriver (WebdriverIO's Tauri service) only when the project already has it.

---

## Security and Release

- **Dependency audit:** `cargo audit` (RustSec advisories), or `cargo deny check` when the project has a `deny.toml`; plus `npm audit` or the package manager's equivalent (`pnpm audit`, `yarn npm audit`). Upgrade the affected dependency; ignoring an advisory needs a written reason and the user's approval.
- **Windows:** sign installers and binaries with an **Authenticode** certificate (`bundle.windows.certificateThumbprint`, `digestAlgorithm`, `timestampUrl`, or `bundle.windows.signCommand` for cloud/HSM signing). Unsigned apps trigger SmartScreen warnings.
- **macOS:** sign with a **Developer ID** certificate and **notarize** (`APPLE_CERTIFICATE`, `APPLE_CERTIFICATE_PASSWORD`, `APPLE_SIGNING_IDENTITY`; notarization through `APPLE_ID` + `APPLE_PASSWORD` + `APPLE_TEAM_ID` or an App Store Connect API key). Unnotarized apps are blocked by Gatekeeper.
- Certificates, passwords and API keys come from CI secrets only — never commit them or put them in `tauri.conf.json`.

---

## CI Basics

- Linux runners install the WebKitGTK 4.1 build dependencies first (the official workflow uses `libwebkit2gtk-4.1-dev libappindicator3-dev librsvg2-dev patchelf xdg-utils` on Ubuntu 22.04).
- Run the frontend checks, then the [Quality](#quality) commands.
- Release builds use `tauri-apps/tauri-action` with a per-OS matrix; signing secrets (updater key, Apple/Windows certificates) come from CI secrets only.
- Cache Cargo with `swatinem/rust-cache` pointed at `src-tauri`.

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `unwrap()`/`expect()` in commands | Panic instead of a rejected promise; aborts the app in release | `?` with a serializable `Error` |
| Trusting paths/IDs from the webview | Path traversal, privilege escalation from an XSS | Validate and canonicalize in Rust against an allowed base |
| `"csp": null` in a release | No XSS mitigation in the webview | Restrictive `app.security.csp` |
| `fs:default` + `$HOME/**` scope | Webview can read the whole home folder | Narrow permissions + `$APPDATA`-style scopes |
| One capability for all windows | Every window gets every permission | Per-window capabilities by label |
| Global events for streams | Slow, unordered under load | `tauri::ipc::Channel` |
| Synchronous heavy command | Freezes the UI (runs on the main thread) | `async fn` command |
| Committing the updater private key | Anyone can ship signed updates | Secret store + env vars at build time |
| Mismatched Tauri crate/npm versions | Build fails or undefined behavior | Same major.minor everywhere |
| v1 config/API (`allowlist`, `distDir`, `@tauri-apps/api/tauri`) | Does not exist in v2 | v2 capabilities, `frontendDist`, `@tauri-apps/api/core` |

---

## Quality

Run from the project root before reporting a task as done:

```bash
cargo fmt --manifest-path src-tauri/Cargo.toml --all -- --check
cargo clippy --manifest-path src-tauri/Cargo.toml --all-targets -- -D warnings
cargo test --manifest-path src-tauri/Cargo.toml
(cd src-tauri && cargo audit)    # or: cargo deny --manifest-path src-tauri/Cargo.toml check
npm audit                        # or the package manager's equivalent
npm run tauri build -- --debug   # when the change touches config, capabilities, plugins or bundling
```

Also run the frontend checks listed in the frontend complement. A capability or permission identifier that does not exist fails the build — never "fix" it by widening permissions.

# Tauri v2 Frontend — Vanilla TypeScript + Vite

Complement for Tauri v2 apps whose webview is **plain TypeScript bundled with Vite** (no UI framework), as produced by the `vanilla-ts` template of `create-tauri-app`. It **complements `TAURI.md` and must always be used together with it** — the Rust, capability, CSP and build rules live there.

---

## Project Layout

```text
<project>/
├── index.html
├── vite.config.ts
├── tsconfig.json           # "strict": true — keep it
├── src/
│   ├── main.ts             # entry: wires DOM events to the API layer
│   ├── api/                # typed wrappers around invoke/listen/Channel (only place that imports @tauri-apps/api)
│   ├── ui/                 # DOM rendering helpers
│   └── styles.css
└── src-tauri/              # see TAURI.md
```

---

## Calling Rust

- Import from the v2 modules: `invoke` and `Channel` from `@tauri-apps/api/core`, `listen`/`emit` from `@tauri-apps/api/event`, windows from `@tauri-apps/api/webviewWindow`. Do not use `window.__TAURI__` when the code is bundled by Vite.
- **One typed wrapper per command** in `src/api/`. UI code never calls `invoke` with a string literal directly.
- Argument keys are **camelCase** (Rust `file_name` ↔ `fileName`) unless the command uses `rename_all`.
- Rejections carry the serialized Rust error (`{ kind, message }`, see `TAURI.md`); type it and handle it — never leave a promise unhandled.

```typescript
import { invoke, Channel } from "@tauri-apps/api/core";

export type CommandError = { kind: "io" | "invalidInput"; message: string };

export function increment(by: number): Promise<number> {
  return invoke<number>("increment", { by });
}

export type Progress =
  | { event: "started"; data: { total: number } }
  | { event: "progress"; data: { done: number } }
  | { event: "finished"; data: Record<string, never> };

export async function runImport(path: string, onProgress: (p: Progress) => void): Promise<void> {
  const channel = new Channel<Progress>();
  channel.onmessage = onProgress;
  await invoke("run_import", { path, onEvent: channel });
}
```

### Events

`listen()` returns a **Promise** of the unlisten function. Await it, keep the handle, and call it when the view that registered the listener is torn down — an SPA does not unregister listeners on its own.

```typescript
import { listen, type UnlistenFn } from "@tauri-apps/api/event";

let stopSync: UnlistenFn | undefined;

export async function watchSync(onDone: (count: number) => void): Promise<void> {
  stopSync?.();
  stopSync = await listen<number>("sync-complete", (event) => onDone(event.payload));
}

export function unwatchSync(): void {
  stopSync?.();
  stopSync = undefined;
}
```

---

## DOM Safety

The webview can reach IPC, so an XSS is a path into the backend.

- Write text with `textContent` / `innerText`; build elements with `document.createElement` + `append`.
- **Never** assign `innerHTML`, `outerHTML` or call `insertAdjacentHTML` / `document.write` with data from commands, events, files, URLs or user input. Static markup literals written in the source are the only acceptable `innerHTML` content.
- Set attributes with `setAttribute` / properties, never by concatenating HTML strings; do not build `href`/`src` from untrusted data without validating the scheme.
- No `eval`, `new Function` or string-based `setTimeout`.
- Query elements once, type them (`querySelector<HTMLInputElement>`), and handle `null`.

```typescript
function renderItem(list: HTMLUListElement, name: string): void {
  const li = document.createElement("li");
  li.textContent = name;
  list.append(li);
}
```

---

## Vite Configuration for Tauri

Keep the template's Tauri-specific options and add `envPrefix` and `build` as the official Vite guide shows. `server.port` must match `build.devUrl` in `tauri.conf.json` (the template uses `1420`), and `build.frontendDist` must point to Vite's output (`../dist`).

```typescript
import { defineConfig } from "vite";
// @ts-expect-error type error without @types/node package
import process from "node:process";

const host = process.env.TAURI_DEV_HOST;

export default defineConfig(() => ({
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
    host: host || false,
    hmr: host ? { protocol: "ws", host, port: 1421 } : undefined,
    watch: { ignored: ["**/src-tauri/**"] },
  },
  envPrefix: ["VITE_", "TAURI_ENV_*"],
  build: {
    target: process.env.TAURI_ENV_PLATFORM == "windows" ? "chrome105" : "safari13",
    minify: !process.env.TAURI_ENV_DEBUG,
    sourcemap: !!process.env.TAURI_ENV_DEBUG,
  },
}));
```

| Option | Reason |
|---|---|
| `clearScreen: false` | Keeps Rust compiler errors visible |
| `strictPort: true` | Tauri expects a fixed port; fail instead of switching |
| `TAURI_DEV_HOST` | Needed for physical mobile devices during `tauri android/ios dev` |
| `envPrefix` | Only `VITE_*` and `TAURI_ENV_*` reach `import.meta.env` — never put secrets in them, they ship in the bundle |
| `build.target` | Chromium (WebView2) on Windows, WebKit on macOS/Linux |

---

## Lint, Format and Tests

| Tool | Packages | Purpose |
|---|---|---|
| TypeScript | `typescript` (template) | `tsc` runs in `npm run build`; keep `strict` |
| ESLint | `eslint @eslint/js typescript-eslint` | Flat config, recommended rules |
| Prettier | `prettier` | Formatting |
| Vitest | `vitest jsdom` | Unit tests with IPC mocks |

```javascript
// eslint.config.mjs
// @ts-check
import js from "@eslint/js";
import { defineConfig } from "eslint/config";
import tseslint from "typescript-eslint";

export default defineConfig({
  files: ["**/*.{js,ts}"],
  extends: [js.configs.recommended, tseslint.configs.recommended],
});
```

Test wrappers with `@tauri-apps/api/mocks`; call `clearMocks()` after each test. If the test environment lacks `window.crypto.getRandomValues`, define it as the official mocking guide shows.

```typescript
import { afterEach, expect, test } from "vitest";
import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { increment } from "../src/api/counter";

afterEach(() => clearMocks());

test("increment forwards the argument", async () => {
  mockIPC((cmd, args) => (cmd === "increment" ? (args as { by: number }).by + 1 : undefined));
  await expect(increment(2)).resolves.toBe(3);
});
```

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `el.innerHTML = data` | XSS with IPC access | `textContent` / `createElement` |
| `invoke("cmd")` scattered in UI code | Untyped, renames break silently | Typed wrapper in `src/api/` |
| `listen(...)` without keeping the unlisten handle | Duplicate handlers, leaks | Await and store `UnlistenFn` |
| Secrets in `VITE_*` variables | Shipped inside the bundle | Keep secrets in Rust / OS keychain |
| `import { invoke } from "@tauri-apps/api/tauri"` | v1 path | `@tauri-apps/api/core` |

---

## Quality

```bash
npx tsc --noEmit
npx eslint .
npx prettier --check .
npx vitest run
npm run build
```

Then run the Rust checks from `TAURI.md`.

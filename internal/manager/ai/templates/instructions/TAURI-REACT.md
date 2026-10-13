# Tauri v2 Frontend — React + TypeScript + Vite

Complement for Tauri v2 apps whose webview is **React + TypeScript bundled with Vite**, as produced by the `react-ts` template of `create-tauri-app`. It **complements `TAURI.md` and must always be used together with it** — the Rust, capability, CSP and build rules live there.

---

## Project Layout

```text
src/
├── main.tsx            # createRoot + <React.StrictMode> — keep StrictMode
├── App.tsx
├── api/                # typed wrappers around invoke / Channel (only place that imports @tauri-apps/api/core)
├── hooks/              # useCommand, useTauriEvent, feature hooks
├── components/
└── features/<name>/    # components + hooks per feature, when the app grows
```

Keep the template's `vite.config.ts` Tauri options (`clearScreen: false`, `server.port` = `devUrl` port, `strictPort: true`, `TAURI_DEV_HOST`, `watch.ignored: ["**/src-tauri/**"]`); `envPrefix` and `build.target` follow the official Vite guide (`envPrefix: ["VITE_", "TAURI_ENV_*"]`, `chrome105` on Windows, `safari13` elsewhere).

---

## Invoking Commands

- Import `invoke`/`Channel` from `@tauri-apps/api/core`, `listen` from `@tauri-apps/api/event`.
- One typed wrapper per command in `src/api/`; components never call `invoke("...")` directly.
- Argument keys are camelCase unless the Rust command uses `rename_all`. Type rejections with the serialized error shape from `TAURI.md`.
- Never call `invoke` during render — only in event handlers, effects or a data hook.

```typescript
// src/api/counter.ts
import { invoke } from "@tauri-apps/api/core";

export type CommandError = { kind: "io" | "invalidInput"; message: string };

export function increment(by: number): Promise<number> {
  return invoke<number>("increment", { by });
}
```

---

## Hooks for Commands and Events

### Async commands

Expose `data`, `error`, `busy` and a stable `run` so every component handles loading and failure the same way. Pass a module-level wrapper (stable identity), not an inline arrow function.

```typescript
// src/hooks/useCommand.ts
import { useCallback, useState } from "react";
import type { CommandError } from "../api/counter";

type CommandState<T> = { data: T | null; error: CommandError | null; busy: boolean };

export function useCommand<A extends unknown[], T>(fn: (...args: A) => Promise<T>) {
  const [state, setState] = useState<CommandState<T>>({ data: null, error: null, busy: false });

  const run = useCallback(
    async (...args: A): Promise<void> => {
      setState((s) => ({ ...s, busy: true, error: null }));
      try {
        const data = await fn(...args);
        setState({ data, error: null, busy: false });
      } catch (e) {
        setState((s) => ({ ...s, error: e as CommandError, busy: false }));
      }
    },
    [fn],
  );

  return { ...state, run };
}
```

### Events — always clean up

`listen()` returns a **Promise** of the unlisten function. Register it in `useEffect` and unlisten in the cleanup; otherwise handlers pile up on every mount (and StrictMode mounts twice in development).

```typescript
// src/hooks/useTauriEvent.ts
import { useEffect, useRef } from "react";
import { listen } from "@tauri-apps/api/event";

export function useTauriEvent<T>(name: string, handler: (payload: T) => void): void {
  const handlerRef = useRef(handler);

  useEffect(() => {
    handlerRef.current = handler;
  }, [handler]);

  useEffect(() => {
    const unlistenPromise = listen<T>(name, (event) => handlerRef.current(event.payload));
    return () => {
      unlistenPromise.then((unlisten) => unlisten());
    };
  }, [name]);
}
```

For streams (progress, logs) pass a `Channel` to the command from inside an effect or handler and stop updating state after unmount.

---

## State

- Local UI state: `useState` / `useReducer`. Shared state: React Context for low-frequency values; use the state library the project already has otherwise — do not add one without asking.
- The Rust side is the source of truth for persisted data; React state is a cache of command results. Re-fetch or update it from events after a mutation instead of guessing.
- Never store secrets in React state, `localStorage` or `VITE_*` variables.

---

## Markup Safety

- Render data through JSX (`{value}` is escaped). **Never** use `dangerouslySetInnerHTML` with data from commands, events, files or user input.
- Do not build `href`/`src` from untrusted data without validating the scheme; open external links through the opener plugin.

---

## Lint, Format and Tests

| Tool | Packages | Purpose |
|---|---|---|
| TypeScript | template (`tsc` runs in `npm run build`) | Keep `strict` |
| ESLint | `eslint @eslint/js typescript-eslint eslint-plugin-react-hooks` | Rules of Hooks, exhaustive deps |
| Prettier | `prettier` | Formatting |
| Vitest | `vitest jsdom @testing-library/react` | Hook/component tests with IPC mocks |

```javascript
// eslint.config.js
import js from "@eslint/js";
import { defineConfig } from "eslint/config";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default defineConfig([
  { ignores: ["dist", "src-tauri"] },
  {
    files: ["**/*.{ts,tsx}"],
    extends: [js.configs.recommended, tseslint.configs.recommended, reactHooks.configs.flat.recommended],
  },
]);
```

Mock the backend with `@tauri-apps/api/mocks`. Unmount before `clearMocks()` and let pending unlisten calls settle, or they hit the removed mock:

```tsx
import { afterEach, expect, test } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { Counter } from "../src/Counter";

afterEach(async () => {
  cleanup();
  await new Promise((resolve) => setTimeout(resolve, 0));
  clearMocks();
});

test("shows the value returned by the command", async () => {
  mockIPC((cmd) => (cmd === "increment" ? 7 : undefined), { shouldMockEvents: true });
  render(<Counter />);
  fireEvent.click(screen.getByRole("button"));
  expect(await screen.findByText("7")).toBeTruthy();
});
```

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `listen()` in the component body | Registers on every render, never cleaned | `useEffect` + unlisten in cleanup |
| Calling `unlisten` before the promise resolves | Listener is never removed | `unlistenPromise.then((fn) => fn())` |
| Removing `<React.StrictMode>` to hide double effects | Hides missing cleanups | Fix the cleanup |
| `invoke` in render | Runs on every render | Handler, effect or `useCommand` |
| `dangerouslySetInnerHTML` with data | XSS with IPC access | JSX text |
| Inline arrow passed to `useCommand` | New `run` every render, effect loops | Module-level API wrapper |

---

## Quality

```bash
npx tsc --noEmit
npx eslint .
npx prettier --check .
npx vitest run
npm run build
```

Run only the tools the project has configured, then the Rust checks from `TAURI.md`.

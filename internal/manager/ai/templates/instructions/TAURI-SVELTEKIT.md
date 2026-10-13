# Tauri v2 Frontend — SvelteKit (SPA / static)

Complement for Tauri v2 apps whose webview is **SvelteKit + TypeScript built as a static SPA**, as produced by the `svelte-ts` template of `create-tauri-app`. It **complements `TAURI.md` and must always be used together with it** — the Rust, capability, CSP and build rules live there.

---

## SPA Mode (Mandatory)

Tauri has no Node.js server: SvelteKit runs only in the webview.

| File | Required setting |
|---|---|
| `svelte.config.js` | `adapter: adapter({ fallback: "index.html" })` from `@sveltejs/adapter-static` |
| `src/routes/+layout.ts` | `export const ssr = false;` |
| `src-tauri/tauri.conf.json` | `build.frontendDist: "../build"`; `build.devUrl` port = `server.port` in `vite.config.js` (template: `1420`) |

- Do **not** add `export const prerender = true` to the root layout: prerendered `load` functions run at build time without access to Tauri APIs. Use SPA mode without prerendering unless the user asks for SSG.
- No server-only features: no `+page.server.ts`, `+layout.server.ts`, `+server.ts`, form actions, `hooks.server.ts` or `$env/static/private` / `$env/dynamic/private`. Backend logic goes into Rust commands.
- With `ssr = false`, `window` and the Tauri API are available in components and in universal `load` functions — no `browser` checks needed for them.
- Keep the template's `vite.config.js` Tauri options (`clearScreen: false`, `strictPort: true`, `TAURI_DEV_HOST`, `watch.ignored: ["**/src-tauri/**"]`).

```javascript
// svelte.config.js
import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({ fallback: "index.html" }),
  },
};

export default config;
```

---

## Project Layout

```text
src/
├── app.html
├── lib/
│   ├── api/            # typed wrappers around invoke / listen / Channel (only place that imports @tauri-apps/api/core)
│   ├── state/          # shared reactive state in *.svelte.ts modules
│   └── components/
└── routes/
    ├── +layout.ts      # export const ssr = false;
    └── ...
```

---

## Invoking Commands

- Import `invoke`/`Channel` from `@tauri-apps/api/core`, `listen` from `@tauri-apps/api/event`.
- One typed wrapper per command in `$lib/api/`; components and state modules call the wrapper, never `invoke("...")` directly.
- Argument keys are camelCase unless the Rust command uses `rename_all`. Type the rejection value with the serialized error shape from `TAURI.md`.

```typescript
// src/lib/api/counter.ts
import { invoke } from "@tauri-apps/api/core";

export type CommandError = { kind: "io" | "invalidInput"; message: string };

export function increment(by: number): Promise<number> {
  return invoke<number>("increment", { by });
}
```

---

## State

- Use **Svelte 5 runes** (`$state`, `$derived`, `$effect`) for new code; follow the project's existing approach (runes or `svelte/store`) when extending older code.
- Shared state lives in `.svelte.ts` modules under `$lib/state/`. A module **cannot export reassigned `$state`** — export an object and mutate its properties, or export getter functions.
- The Rust side is the source of truth for persisted data; frontend state is a cache of command results.
- Track `busy` and `error` for every async command the UI waits on.

```typescript
// src/lib/state/counter.svelte.ts
import { increment, type CommandError } from "$lib/api/counter";

export const counter = $state({ value: 0, error: null as CommandError | null, busy: false });

export async function bump(by: number): Promise<void> {
  counter.busy = true;
  counter.error = null;
  try {
    counter.value = await increment(by);
  } catch (e) {
    counter.error = e as CommandError;
  } finally {
    counter.busy = false;
  }
}
```

### Events

`listen()` returns a Promise of the unlisten function. Register it in `$effect` and unlisten in the cleanup — SvelteKit's client router does not unregister listeners when a route changes.

```svelte
<script lang="ts">
  import { listen } from "@tauri-apps/api/event";

  let lastSync = $state<number | null>(null);

  $effect(() => {
    const unlistenPromise = listen<number>("sync-complete", (event) => {
      lastSync = event.payload;
    });
    return () => {
      unlistenPromise.then((unlisten) => unlisten());
    };
  });
</script>
```

---

## Markup Safety

- Render data with `{expression}` (escaped). **Never** use `{@html ...}` with data from commands, events, files or user input.
- Do not build `href`/`src` from untrusted data without validating the scheme; open external links through the opener plugin, not by navigating the webview.

---

## Lint, Format and Tests

| Tool | How to add | Purpose |
|---|---|---|
| `svelte-check` | Template (`npm run check`) | Type-checks `.svelte` and `.ts` |
| ESLint | `npx sv add eslint` | `eslint-plugin-svelte` + `typescript-eslint` |
| Prettier | `npx sv add prettier` | `prettier-plugin-svelte` |
| Vitest | `npx sv add vitest` | Unit/component tests |

Mock the backend with `@tauri-apps/api/mocks` and call `clearMocks()` after each test:

```typescript
import { afterEach, expect, test } from "vitest";
import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { counter, bump } from "$lib/state/counter.svelte";

afterEach(() => clearMocks());

test("bump stores the command result", async () => {
  mockIPC((cmd) => (cmd === "increment" ? 5 : undefined));
  await bump(1);
  expect(counter.value).toBe(5);
});
```

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `adapter-node` / `adapter-auto` | Needs a server Tauri does not have | `adapter-static` with `fallback: "index.html"` |
| `prerender = true` with Tauri calls in `load` | `load` runs at build time without Tauri APIs | SPA mode, `ssr = false` |
| `+page.server.ts`, form actions, private env | No server at runtime | Rust commands |
| `{@html data}` | XSS with IPC access | `{data}` |
| `listen()` without cleanup | Duplicate handlers after navigation | `$effect` returning the unlisten call |
| Exporting a reassigned `$state` variable | Not allowed across modules | Export an object or getters |

---

## Quality

```bash
npm run check
npx eslint .
npx prettier --check .
npx vitest run
npm run build
```

Run only the tools the project has configured, then the Rust checks from `TAURI.md`.

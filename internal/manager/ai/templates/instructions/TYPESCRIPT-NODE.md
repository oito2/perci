# TypeScript / Node.js Standard

Standard for TypeScript and Node.js backends, CLIs and libraries; frontend specifics (UI frameworks, bundlers, webview IPC) belong to their framework standard.

Last verified: 2026-11-04

---

## Scope & Precedence

- **Framework and stack standards take precedence for their specifics** (`TAURI.md` and its frontend complements, `WEBSITE.md`, `LANDING-PAGE.md`, ...): their layout, `tsconfig`, lint/test setup and Quality commands win. This file fills the gaps they leave.
- Node-only rules (process, stdout/stderr, env, `child_process`, file system) do not apply to code that runs in a browser or webview.
- Follow the project's existing package manager, module system, test runner, linter and formatter. Do not switch or add one without asking.

---

## Runtime & Package Manager

- Target only Node.js lines that receive fixes. At the date above: **26 (Active LTS)**, **24 and 22 (Maintenance LTS, 22 ends 2027-04-30)**; 20 is end-of-life. Check <https://nodejs.org/en/about/previous-releases> instead of trusting this list.
- Declare the minimum in `package.json` and pin the development version in `.nvmrc` or `.node-version` (whichever the project uses):

```json
{
  "engines": { "node": ">=22.12" },
  "packageManager": "pnpm@<exact-version>+sha512.<hash>"
}
```

- `packageManager` (with its hash) pins the package manager for Corepack. Corepack ships with Node.js **only up to 24.x**; on 25+ install it with `npm install -g corepack`, then `corepack enable`.
- Commit the lockfile (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`). CI installs with the frozen form: `npm ci`, `pnpm install --frozen-lockfile`, `yarn install --immutable`.

---

## Modules (ESM vs CommonJS)

- New code is **ESM**: `"type": "module"` in `package.json`. Use `.cts`/`.cjs` only for files that must be CommonJS.
- Relative imports in Node-run ESM include the extension (`./config.js`, or `./config.ts` when running sources directly — see Type Stripping).
- Use `node:` specifiers for built-ins (`import { readFile } from "node:fs/promises"`).
- Replace `paths`/`baseUrl` aliases with **subpath imports** (`"imports": { "#internal/*": "./src/internal/*" }`): Node resolves them natively; `baseUrl` is deprecated since TypeScript 6.0.
- `require()` of synchronous ESM (no top-level `await`) works by default on every supported LTS line, so CommonJS consumers can load ESM-only packages.

---

## TypeScript & `tsconfig`

- Current major: **TypeScript 7** (native compiler). TypeScript 6.0 changed defaults (`strict: true`, `module: esnext`, `types: []`, `rootDir` = the config's folder) and deprecated `moduleResolution: node`/`node10`, `baseUrl`, `target: es5`, `outFile`; 7.0 removes them. Set options explicitly instead of relying on defaults.
- Tools that import the compiler API may not support TypeScript 7 yet (typescript-eslint supports TypeScript `<6.1` at the date above; framework checkers such as `svelte-check` too): keep a 6.x install for them — Microsoft publishes `@typescript/typescript6` (binary `tsc6`) to run side by side. Check each tool's supported range before upgrading.

Baseline for Node code:

```json
{
  "compilerOptions": {
    "target": "es2023",
    "module": "nodenext",
    "moduleResolution": "nodenext",
    "lib": ["es2023"],
    "types": ["node"],
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "erasableSyntaxOnly": true,
    "skipLibCheck": true,
    "outDir": "dist",
    "rootDir": "src",
    "sourceMap": true,
    "declaration": true
  },
  "include": ["src"]
}
```

| Option | Rule |
|---|---|
| `module` / `moduleResolution` | `nodenext` for code Node runs or a library publishes; `module: "preserve"` + `moduleResolution: "bundler"` only when a bundler (Vite, esbuild, ...) consumes the output |
| `strict` | Always on; never turn individual strict checks off to silence errors |
| `noUncheckedIndexedAccess` | On: indexed reads are `T \| undefined` |
| `exactOptionalPropertyTypes` | On for new projects; enable in existing ones only when the fix-up is small |
| `verbatimModuleSyntax` | On: type-only imports use `import type`, so single-file transpilers and type stripping work |
| `erasableSyntaxOnly` | On when sources run under Node type stripping: forbids `enum`, runtime `namespace`, parameter properties |
| `skipLibCheck` | On: skips checking `.d.ts` of dependencies, not your own code |
| `types` | Explicit (`["node"]`); `@types/node` major matches the lowest Node line in `engines` |

---

## Node Type Stripping

- Node runs `.ts` files directly by **erasing types** (enabled by default since 22.18.0; stable since 24.12.0 and in 26). It does **not** type-check, does not read `tsconfig.json`, ignores `paths`, refuses files under `node_modules`, and rejects `.tsx`, `enum`, runtime `namespace`, parameter properties and decorators. `--no-strip-types` disables it.
- Fine for scripts, tools and tests, which use `.ts` extensions in imports with `allowImportingTsExtensions` + `noEmit` (scripts), or `rewriteRelativeImportExtensions` when `tsc` also emits JS.
- Type checking is always a separate `tsc --noEmit` step. Published packages ship compiled `.js` + `.d.ts`, never raw `.ts`.

---

## Project Layout

- Sources in `src/` (`index.ts` for a library, a thin `cli.ts`/`server.ts` entry, `config.ts` for env validation, one folder per feature); build output in git-ignored `dist/`; tests in `test/` or next to the code — follow the project.
- Entry points only parse input and wire dependencies; logic lives in modules tests import directly. No catch-all `utils`/`helpers` modules.

---

## Typing Rules

- **No `any`.** Use `unknown` at boundaries (JSON, `catch`, IPC, env, third-party callbacks) and narrow it.
- **Validate external input at runtime** with the project's schema library (zod, valibot, ...), and derive the type from the schema (`z.infer<typeof Schema>`). A type assertion (`as T`) on external data is not validation.
- Model states with **discriminated unions** and exhaustive `switch` (a `never` check in `default`), not optional-flag soup.
- Use `satisfies` to check literals without widening them; `as const` for fixed lookup tables.
- Non-null `!` and `as` only with a comment explaining why the value is guaranteed; prefer narrowing.
- Exported functions declare their return types; prefer `readonly` for data that is not mutated.

```typescript
const Config = z.object({
  PORT: z.coerce.number().int().min(1).max(65535).default(3000),
  DATABASE_URL: z.url(),
  LOG_LEVEL: z.enum(["debug", "info", "warn", "error"]).default("info"),
});
export type Config = z.infer<typeof Config>;

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  return Config.parse(env); // throws with every invalid key listed
}
```

---

## Errors

| Rule | Example |
|---|---|
| Throw `Error` subclasses, never strings or plain objects | `class NotFoundError extends Error { override name = "NotFoundError"; }` |
| Keep the cause when rethrowing | `throw new Error("read config " + path, { cause: err })` |
| Narrow in `catch` (`err` is `unknown`) | `if (err instanceof NotFoundError) { ... }` |
| Check Node error codes, not messages | `(err as NodeJS.ErrnoException).code === "ENOENT"` after `instanceof Error` |

- Never swallow an error (`catch {}`); handle it, rethrow with context, or log it once at the top level — not both.
- Every promise is awaited, returned, or explicitly handled. Long-running processes register `process.on("unhandledRejection")` / `"uncaughtException"` only to log and exit, never to continue.
- Messages shown to users or API clients carry no stack traces, secrets or internal paths.

---

## Async & Cancellation

- **No floating promises** (enforced by `@typescript-eslint/no-floating-promises`). Fire-and-forget work uses `void task().catch(log)` and has an owner.
- Anything that does I/O or may block accepts an `AbortSignal` and passes it down (`fetch`, `fs/promises`, `child_process`, `events.once`, timers from `node:timers/promises`).
- Every network call has a timeout: `AbortSignal.timeout(ms)`, combined with a caller signal via `AbortSignal.any([signal, AbortSignal.timeout(ms)])`.
- Servers stop gracefully on `SIGINT`/`SIGTERM`: stop accepting work, abort in-flight work, close resources.
- No sync I/O (`readFileSync`, `execSync`) on a server's request path.

---

## Configuration & Logging

- Configuration comes from env vars (plus flags for CLIs), parsed and validated **once at startup** into a typed object (see Typing Rules); the rest of the code receives that object, not `process.env`.
- Local `.env` files load with `node --env-file-if-exists=.env` (or `--env-file`); never commit `.env` — commit a `.env.example` with every key and no secrets.
- Use a structured logger (the project's, e.g. pino) with stable lowercase keys; never log secrets, tokens or full env.
- CLIs and stdio protocol servers write logs and diagnostics to **stderr**; stdout carries only the program's output. Remove `console.log` debugging before finishing.

---

## Security

- Fix high/critical audit findings by upgrading the affected dependency, not by silencing them.
- New dependencies need a reason (maintained, small tree, compatible license); prefer Node built-ins (`fetch`, `node:test`, `util.parseArgs`).
- No `eval`, `new Function`, `vm` on untrusted input, or string-based `setTimeout`.
- Subprocesses use `execFile`/`spawn` with an **argument array** and no `shell: true`; put `--` before user operands when the tool supports it. Never build a command string for `exec`.
- Untrusted paths: `path.resolve(base, input)`, then confirm the result stays inside `base` (`path.relative` not starting with `..` and not absolute); resolve symlinks with `fs.realpath` when the target matters.
- Files holding secrets are written with mode `0o600`.

---

## Testing

- Use the project's runner: `node:test` (built in; picks up `.ts` tests via type stripping) or **Vitest** — never both.
- Tests are deterministic: no network, no real home directory, temp dirs from `fs.mkdtemp(path.join(os.tmpdir(), "..."))` removed afterwards, fake timers (`mock.timers` / `vi.useFakeTimers()`) instead of sleeps.
- Mock at module boundaries (`mock.method`, `mock.fn`, `vi.fn`, `vi.mock`); restore every mock after each test. Prefer injecting dependencies over module mocking.
- Coverage: `vitest run --coverage`, or `node --test --experimental-test-coverage` (still experimental). Every bug fix gets a regression test.

---

## Lint & Format

- **ESLint flat config** (`eslint.config.js`) with **typescript-eslint**; use the type-checked presets (`recommendedTypeChecked` or `strictTypeChecked` with `parserOptions.projectService: true`) for Node code so rules like `no-floating-promises` and `no-misused-promises` run.
- Formatting comes from **one** tool — Prettier, or Biome when the project uses it (it may also replace ESLint).
- `// eslint-disable-next-line <rule> -- <reason>`: always name the rule and give the reason; no file-wide disables.

---

## Libraries & Publishing

- Prefer **ESM-only** packages: `require(esm)` covers CommonJS consumers on supported Node lines. Ship dual ESM + CJS only when consumers need it, and then avoid the **dual package hazard** (two copies of module state, `instanceof` failures): keep state in one format and have the other wrap it.
- Define the public surface with `exports`; `"types"` comes first in each condition and `"default"` last. Anything not exported is private.

```json
{
  "type": "module",
  "exports": {
    ".": { "types": "./dist/index.d.ts", "default": "./dist/index.js" }
  },
  "files": ["dist"]
}
```

- `files` whitelists what ships; check with `npm pack --dry-run`. Validate the package with `npx publint` and `npx @arethetypeswrong/cli --pack .`.
- Emit `.d.ts` with `declaration: true` (consider `isolatedDeclarations` for large libraries). Runtime dependencies go in `dependencies`, frameworks the host provides in `peerDependencies`, everything else in `devDependencies`.

---

## CLIs

- Expose the binary through `"bin"`; the entry file starts with `#!/usr/bin/env node` and points to compiled JS.
- Parse flags with `util.parseArgs` (or the project's parser); support `--help` and `--version`.
- Set `process.exitCode` (`0` success, `1` failure, `2` usage error) and return, instead of `process.exit()`, so pending stdout writes flush.
- Results go to stdout (`--json` when useful); prompts, progress and errors go to stderr. Honor `NO_COLOR` and non-TTY output.

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `any`, `as T` on parsed JSON | Type lies, runtime crashes | `unknown` + schema validation |
| `catch (e) {}` | Hides failures | Handle, rethrow with `cause`, or log once |
| Unawaited promise | Lost errors, races | `await`, return, or `void p.catch(...)` |
| `fetch` without a timeout | Hangs forever | `AbortSignal.timeout(ms)` |
| `exec("git " + arg)`, `shell: true` | Shell injection | `execFile("git", ["--", arg])` |
| `path.join(base, userInput)` without a check | Path traversal | Resolve, then confirm it stays in `base` |
| `process.env.X` read across the codebase | Unvalidated, untestable config | One validated config object |
| `paths`/`baseUrl` aliases at runtime | Node ignores them | Subpath `imports` (`#...`) |
| `enum` / parameter properties in stripped code | Not erasable, fails at runtime | Union of literals, `as const` objects |

---

## Quality

Run before reporting a change as done, using the project's package manager and only the tools it has configured (framework standards may add or replace commands):

```bash
npx tsc --noEmit                  # type check (per tsconfig, e.g. -p tsconfig.json)
npx eslint .                      # zero errors
npx prettier --check .            # or: npx biome check .
npm test                          # node --test, or npx vitest run
npm run build                     # when the project has a build step
npm audit --audit-level=high      # pnpm audit --audit-level high | yarn npm audit
```

Libraries, before publishing: `npm pack --dry-run`, `npx publint`, `npx @arethetypeswrong/cli --pack .`.

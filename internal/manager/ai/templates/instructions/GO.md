# Go Development Standard

Standard for Go command-line tools, MCP servers and desktop-application backends.
Apply it whenever Go code, `go.mod`, tests, lint configuration or release builds are written or reviewed. Complements: `GO-MCP.md` (MCP servers) and `WAILS.md` (Wails desktop apps).

---

## Toolchain & `go.mod`

- The `go` directive is the **minimum** Go version the module needs, and the toolchain enforces it. Raise it only when the code uses a newer language feature or standard-library API, never as routine bumping.
- Keep it on a Go release that still receives security fixes (each major release is supported until two newer majors ship).
- Before using a recent standard-library API, check that it exists in the module's `go` version (`go doc <pkg>.<Symbol>`, or `$(go env GOROOT)/api/go1.N.txt`). Examples: `os.Root`, `testing.T.Context` (1.24), `sync.WaitGroup.Go`, `testing/synctest` (1.25), `errors.AsType` (1.26).
- Commit `go.mod` and `go.sum`. Never commit `replace` directives that point to local paths.
- Pin development tools in `go.mod` with `go get -tool <pkg>@<version>` and run them with `go tool <name>`, or pin their version in CI — never `@latest` in CI.

---

## Project Layout

```text
<module>/
├── cmd/<name>/main.go     # thin entry point: flags, wiring, exit code
├── internal/<pkg>/        # all application code, one responsibility per package
├── internal/version/      # Version variable set at link time
├── testdata/              # fixtures and golden files (ignored by the go tool)
├── go.mod
└── go.sum
```

- Put code under `internal/` unless it is a deliberate public library; do not create a `pkg/` directory for an application.
- `main` only parses arguments, builds dependencies and calls into `internal/`; business logic in `main` cannot be tested.
- Package names are short, lowercase, singular, and describe what the package provides (`config`, `store`), never `util`, `common` or `helpers`.
- Platform-specific code goes in `_linux.go` / `_windows.go` / `_darwin.go` files (or `//go:build` constraints), with the same function signatures in each.

### Entry point

`main` delegates to a `run` function that returns an exit code, so deferred cleanup always runs and the logic is testable:

```go
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := parseFlags(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2 // usage error
	}
	if err := app.Run(ctx, cfg, stdout); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
```

Only `main` calls `os.Exit` or `log.Fatal`; every other function returns an error.

---

## Error Handling

| Rule | Example |
|---|---|
| Wrap with context using `%w` | `fmt.Errorf("read config %s: %w", path, err)` |
| Messages are lowercase, without trailing punctuation | `"open store: %w"`, not `"Failed to open store."` |
| Sentinel errors are exported `Err…` variables | `var ErrNotFound = errors.New("not found")` |
| Error types end in `Error` and use pointer receivers | `type ValidationError struct{ Field string }` |
| Match with `errors.Is` / `errors.As` (or `errors.AsType`, Go 1.26+) | `if errors.Is(err, fs.ErrNotExist) {` |
| Combine independent failures with `errors.Join` | cleanup errors after a primary error |

- Never compare errors with `==` or match on `err.Error()` text.
- Handle an error **once**: either return it (wrapped) or log it — not both.
- Check the error from `Close` on files that were **written**; a failed close can mean lost data.
- Use `%v` instead of `%w` when the underlying error is an implementation detail callers must not depend on.
- `panic` only for programmer errors (impossible states, invalid static configuration at startup), never for input or I/O failures.

---

## Context & Cancellation

- Any function that does I/O, runs a subprocess or may block takes `ctx context.Context` as its **first** parameter and passes it down.
- Never store a `context.Context` in a struct field; never pass `nil` (use `context.TODO()` while wiring is incomplete).
- `context.Background()` belongs in `main`, tests and top-level goroutines only.
- Every `context.WithTimeout` / `WithCancel` is followed by `defer cancel()`.
- Long loops check `ctx.Err()` between iterations; blocking `select` statements include `case <-ctx.Done():`.
- Use `context.WithoutCancel(ctx)` only for work that must finish after the request ends (e.g. persisting a result), and give it its own timeout.

---

## Concurrency

- Every goroutine has an owner that knows **how it stops** (context cancellation or a closed channel) and **waits for it** (`sync.WaitGroup`, `errgroup.Group`). A goroutine that can block forever on a channel send is a leak.
- Prefer `golang.org/x/sync/errgroup` (`errgroup.WithContext`, `SetLimit`) for fan-out work that can fail; use `wg.Go(func() { ... })` (Go 1.25+) for fire-and-wait tasks.
- Document which mutex guards which fields; keep critical sections short and never call out to unknown code (callbacks, I/O) while holding a lock.
- The sender closes a channel, never the receiver.
- Run tests with `-race` always. Never synchronize tests with `time.Sleep`; use channels, `t.Context()` or `testing/synctest` (Go 1.25+).

---

## Logging & Output

- Use `log/slog`. Create one logger in `main` and pass it down (constructor argument or struct field); avoid `slog.SetDefault` in libraries.
- Logs go to **stderr**. stdout is reserved for the program's actual output (CLI results, or the protocol stream of a stdio server).

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
logger.Info("server started", "addr", addr, "version", version.Version)
```

- Use structured key/value attributes with stable lowercase keys (`"path"`, `"error"`), not values formatted into the message.
- Never log secrets, tokens, full environment dumps or file contents.
- User-facing CLI messages (results, prompts, progress) are written to the injected `stdout`/`stderr` writers, not through the logger.

---

## Configuration

- Precedence: command-line flags > environment variables > config file > built-in defaults. Document every key in the README/reference.
- Resolve config locations with `os.UserConfigDir()`, `os.UserCacheDir()` and `os.UserHomeDir()` — never hard-code `/home/<user>` or build paths from `$HOME` by hand.
- Load, validate and normalize config once at startup; return a typed struct with all values checked. Unknown keys are reported, not silently ignored.
- Files that may hold credentials are created with mode `0o600` inside a `0o700` directory.

---

## Running External Commands (`os/exec`)

```go
cmd := exec.CommandContext(ctx, gitPath, "-C", repoDir, "log", "--format=%H", "--", userPath)
cmd.WaitDelay = 5 * time.Second
var stderr bytes.Buffer
cmd.Stderr = &stderr
out, err := cmd.Output()
if err != nil {
	return fmt.Errorf("git log: %w: %s", err, strings.TrimSpace(stderr.String()))
}
```

- Pass arguments as separate strings. **Never** build a command line for `sh -c` / `bash -c` from variables; if a shell is unavoidable, pass values as positional parameters (`sh -c 'cmd "$1"' sh "$value"`), never interpolated.
- Put `--` before user-supplied operands when the tool supports it, so a value starting with `-` is not parsed as a flag.
- Always use `exec.CommandContext` so cancellation kills the child; set `WaitDelay` when the child may leave pipes open.
- Privileged commands (`sudo`, `pkexec`, system binaries run as root) use **absolute paths**, never a `PATH` lookup.
- Resolve tool paths once with `exec.LookPath` and report a clear "missing dependency" error.
- Include the command name and trimmed stderr in the returned error.
- Set `cmd.Env` explicitly (based on `os.Environ()`) only when changing it; never forward secrets to children that do not need them.

---

## File System

### Atomic writes

Write to a temporary file **in the destination directory**, then rename (rename is only atomic within one filesystem):

```go
func writeFileAtomic(path string, data []byte, perm fs.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

### Permissions and paths

| Content | Mode |
|---|---|
| Secrets, tokens, private config | `0o600` (directory `0o700`) |
| Regular data and config | `0o644` (directory `0o755`) |
| Executables | `0o755` |

- Write octal modes with the `0o` prefix.
- Paths from users, config or network input are untrusted: clean them, resolve symlinks when the target matters, and confine access to a base directory with `os.OpenRoot` (Go 1.24+) instead of string-prefix checks.
- Build paths with `filepath.Join`, never string concatenation.

---

## Testing

- Tests live next to the code (`foo_test.go`). Use **table-driven tests** with `t.Run` subtests and descriptive case names.
- Call `t.Helper()` in helpers; use `t.Fatalf` when the test cannot continue, `t.Errorf` otherwise. Messages use the `got X, want Y` form.
- Use `t.TempDir()` for every file the test touches and `t.Setenv` for environment changes.
- **Never touch the real home directory or user config.** Point `HOME`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME` and `XDG_CACHE_HOME` at `t.TempDir()` in any test that reaches code resolving user paths. (`t.Setenv` cannot be combined with `t.Parallel`.)
- Use `t.Context()` (Go 1.24+) for contexts that end with the test.
- No network, no real system changes, no dependence on installed tools: inject interfaces or fake binaries written to `t.TempDir()`.
- Golden files live under `testdata/` and are regenerated with an explicit flag:

```go
var update = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output mismatch for %s:\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}
```

Regenerate with `go test ./path/to/pkg -run TestName -update`, then review the diff before committing.

- Every bug fix gets a regression test that fails without the fix.

---

## Lint & Format

- `gofmt` (or `goimports`) output is mandatory; never hand-format.
- `go vet ./...` and `golangci-lint run` must report zero issues.
- `//nolint` requires the linter name and a reason: `//nolint:gosec // path is validated by resolvePath`.

Baseline `.golangci.yml` (golangci-lint v2 format — verify with `golangci-lint config verify`):

```yaml
version: "2"

linters:
  # standard = errcheck, govet, ineffassign, staticcheck, unused
  default: standard
  enable:
    - bodyclose
    - errorlint
    - gosec
    - misspell
    - nilerr
    - noctx
    - nolintlint
    - sloglint
    - unconvert
    - usetesting
  settings:
    gosec:
      excludes:
        - G104 # unchecked errors are errcheck's job
  exclusions:
    presets:
      - std-error-handling
    rules:
      - path: _test\.go
        linters:
          - gosec

formatters:
  enable:
    - gofmt
    - goimports
```

- `gosec` G204 (subprocess launched with a variable) fires on every non-constant `exec.Command`; after checking the call follows [Running External Commands](#running-external-commands-osexec), suppress it on that line with a reason.
- golangci-lint must be built with a Go version **at least** as new as the module's `go` directive, or it fails with type-check errors in the standard library. Pin its version in CI.
- Optionally run `go fix -diff ./...` (Go 1.26+) to see modernization suggestions; apply them only as a separate, reviewed change.

---

## Dependencies & Security

- Prefer the standard library. Each new dependency needs a reason: maintained, widely used, small transitive tree, compatible license.
- Run `go mod tidy` after changing imports; CI fails on `go mod tidy -diff`.
- Run `govulncheck ./...` (`golang.org/x/vuln/cmd/govulncheck`, pinned version) in CI and before releases; upgrade the affected module rather than suppressing findings.
- Update dependencies deliberately (`go get <module>@<version>`, then `go mod tidy`), one logical upgrade per change, with tests run.

---

## Release Builds

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags "-s -w -X <module>/internal/version.Version=${VERSION}" \
    -o dist/<name>_linux_amd64 ./cmd/<name>
```

- `-trimpath` removes local file-system paths; `-s -w` strip the symbol table and DWARF data.
- The `-X` target must be a package-level `string` **variable** (not a constant) with a default such as `"dev"`.
- Validate the version string (e.g. `^v[0-9]+\.[0-9]+\.[0-9]+$`) before it is interpolated into `-ldflags`.
- `CGO_ENABLED=0` only for projects that do not need cgo; cgo projects (e.g. GUI toolkits) build natively per platform.
- Publish a `checksums.txt` (SHA-256) alongside the release binaries.

---

## CI

Minimum pipeline, on every push and pull request:

1. `gofmt -l .` produces no output.
2. `go vet ./...`
3. `golangci-lint run` (pinned version).
4. `go mod tidy -diff`
5. `go build ./...`
6. `go test -race ./...`
7. `govulncheck ./...`

Use least-privilege workflow permissions (`contents: read`), pin third-party actions by commit SHA, and read the Go version from `go.mod` (`go-version-file: go.mod` in `actions/setup-go`).

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `err == ErrX` or `strings.Contains(err.Error(), ...)` | Breaks on wrapped errors | `errors.Is` / `errors.As` |
| `fmt.Errorf("...: %v", err)` when callers need the cause | Loses the error chain | `%w` |
| Logging and returning the same error | Duplicate, noisy logs | Return wrapped; log once at the top |
| `exec.Command("sh", "-c", "tool " + arg)` | Shell injection | `exec.CommandContext(ctx, tool, "--", arg)` |
| `os.WriteFile` over an existing config | Truncated file on crash | Temp file + `Sync` + rename |
| Goroutine without a stop path | Leak, hidden races | Context + `WaitGroup` / `errgroup` |
| `context.Context` stored in a struct | Lifetime confusion | Pass `ctx` per call |
| `fmt.Println` debugging left in code | Pollutes stdout / protocol streams | `slog` at debug level, to stderr |
| Tests reading the real `$HOME` | Destroys user data, flaky | `t.Setenv("HOME", t.TempDir())` |
| `time.Sleep` in tests | Slow and flaky | Channels, `synctest`, explicit sync |
| `panic` / `log.Fatal` outside `main` | Skips cleanup, untestable | Return an error |
| `interface{}` everywhere | Lost type safety | Concrete types, generics, `any` only at boundaries |

---

## Quality

Run before reporting any Go change as done:

```bash
gofmt -l .                 # must print nothing
go vet ./...
golangci-lint run
go mod tidy -diff
go test -race ./...
govulncheck ./...
```

Coverage, when assessing test gaps:

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Use `go test -race -shuffle=on ./...` periodically to expose order-dependent tests.

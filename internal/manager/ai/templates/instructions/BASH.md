# Bash/Shell Development Standard

Standard for Bash scripts, CLI tools and installers: defensive code, a consistent interface, tested behavior.

Last verified: 2026-11-04

---

## Boilerplate

```bash
#!/usr/bin/env bash
# =============================================================================
# Script Name : script-name.sh
# Description : Brief description of the purpose
# Version     : 1.0.0
# =============================================================================
set -Eeuo pipefail
shopt -s inherit_errexit # bash >= 4.4

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
readonly SCRIPT_DIR

trap 'printf "\nError at %s:%d\n" "${BASH_SOURCE[0]}" "$LINENO" >&2' ERR
trap '[[ -n "${_tmpdir:-}" ]] && rm -rf -- "$_tmpdir"' EXIT

# _tmpdir=$(mktemp -d)   # uncomment when the script needs a temporary workspace

# Prints a message to stderr.
warn() { printf '%s\n' "Warning: $*" >&2; }

# Prints an error to stderr and exits with status $2 (default 1).
die() {
    printf '%s\n' "Error: $1" >&2
    exit "${2:-1}"
}

usage() {
    cat <<EOF
Usage: ${0##*/} [options] <arg>

Options:
  -o, --output FILE  Write the result to FILE
  -v, --verbose      Print progress details
  -h, --help         Show this help and exit
EOF
}

main() {
    # Logic here
    :
}

# Runs main only when executed, so tests can source the functions.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
```

- `inherit_errexit` requires **bash >= 4.4**. If the script must run on an older bash (e.g. macOS `/bin/bash` 3.2), drop it and check command substitutions explicitly.
- Interactive headers or banners call `clear` only when stdout is a terminal: `[[ -t 1 ]] && clear`.
- Use colors only when `[[ -t 1 ]]` and `NO_COLOR` is unset; always reset after a colored string.

---

## Interface Conventions

### Help and usage

- Every script with arguments accepts `-h`/`--help`: print `usage` to **stdout** and exit `0`.
- A usage error (unknown option, missing argument) prints a one-line error plus a hint (`Try '${0##*/} --help'`) to **stderr** and exits `2`.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success (also `--help`) |
| `1` | General runtime failure |
| `2` | Usage error (bad options or arguments) |
| `126` / `127` | Reserved by the shell (not executable / command not found) — never use them |
| `128+N` | Reserved for termination by signal N — never use them |

Document any additional code in `usage`.

### Output

- Results go to stdout; errors, warnings and progress go to stderr, so the output can be piped.
- Use `printf '%s\n'` for literal text and `printf '%b\n'` only when escape sequences must be expanded; never `echo` with options or escapes.

### Confirmations

`y` or `s` confirms, any other answer cancels:

```bash
printf '%s' "Continue? (y/N): "
read -r confirm
[[ ! "$confirm" =~ ^[yYsS]$ ]] && return 0
```

---

## Defensive Shell — Golden Rules

1. **Always quote variables:** `"$var"`, `"${array[@]}"`, `"$(command)"`.
2. **Local scope in functions:** use `local` for variables, `local -r` for constants.
3. **Preserve exit codes** — separate declaration (`local`, `readonly`, `export`) from assignment:
   ```bash
   get_value() {
       local result
       result=$(command_that_might_fail)
       printf '%s\n' "$result"
   }
   ```
4. **Flag protection** — terminate options with `--` before variable arguments:
   ```bash
   rm -rf -- "$path"
   grep -F -- "$search" "$file"
   ```

---

## Production Patterns

### Atomic write

The content goes to a temporary file in the destination's own directory, which is then renamed over the destination (a rename is only atomic within one filesystem, so never stage the file in `/tmp`).

```bash
# $1 = destination, $2 = file mode (default 644). Content comes from stdin.
atomic_write() {
    local dest="$1" mode="${2:-644}" tmp
    tmp=$(mktemp -- "$(dirname -- "$dest")/.tmp.XXXXXX")
    if ! cat >"$tmp" || ! chmod -- "$mode" "$tmp"; then
        rm -f -- "$tmp"
        return 1
    fi
    mv -f -- "$tmp" "$dest"
}

generate_config | atomic_write "/etc/app/config.conf"
```

### Safe iteration (NUL-delimited)

```bash
while IFS= read -r -d '' f; do
    process "$f"
done < <(find . -type f -name "*.log" -print0)
```

### Dependency check

```bash
require_cmd() {
    command -v "$1" &>/dev/null || die "Missing dependency: $1"
}
```

### Sensitive files (credentials)

Create with restricted permissions **before** writing data:

```bash
# $1 = token to store.
store_secret() {
    local secret_file="$HOME/.secrets" token="$1"
    (umask 077 && touch -- "$secret_file")
    printf 'TOKEN=%q\n' "$token" >>"$secret_file"
}
```

### Argument parsing

```bash
main() {
    local verbose=0 output=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h | --help) usage; exit 0 ;;
            -v | --verbose) verbose=1; shift ;;
            -o | --output)
                [[ $# -lt 2 ]] && die "Option $1 requires an argument" 2
                output="$2"
                shift 2
                ;;
            --) shift; break ;;
            -*) die "Unknown option: $1 (try '${0##*/} --help')" 2 ;;
            *) break ;;
        esac
    done

    # remaining positional args are now in "$@"
}
```

---

## Testing (bats-core)

- Test scripts with [bats-core](https://github.com/bats-core/bats-core); put tests in `test/*.bats`. Use the project's existing test layout if it has one.
- Source the script in `setup` (the `main` guard in the boilerplate keeps it from running) and test functions directly; use `run` to test the whole script's exit code and output.
- Work only inside `$BATS_TEST_TMPDIR`; point `HOME` and `XDG_*` variables there. Stub external commands with fake executables placed first in `PATH`.

```bash
#!/usr/bin/env bats

setup() {
    export HOME="$BATS_TEST_TMPDIR"
    source "$BATS_TEST_DIRNAME/../script-name.sh"
}

@test "unknown option exits with usage error" {
    run "$BATS_TEST_DIRNAME/../script-name.sh" --bogus
    [ "$status" -eq 2 ]
    [[ "$output" == *"Unknown option"* ]]
}
```

---

## Anti-Patterns

| Anti-pattern | Why |
|---|---|
| `echo` for colorized or escaped output | Not portable; use `printf '%b\n'` |
| Unquoted `$var` in commands or conditions | Word splitting and glob expansion cause silent bugs |
| Parsing `ls` output | Breaks on filenames with spaces or newlines; use `find -print0` |
| `cd dir; do_something` without a guard | `cd` failure goes undetected; use `cd dir && ...` or `\|\| die` |
| `eval` with user-supplied input | Arbitrary code execution |
| `which cmd` to detect commands | Not POSIX; use `command -v cmd` |
| Missing `local` in functions | Leaks variables into the global scope |
| Declaring and assigning in one step with command substitution | Loses the exit code |
| Unconditional `clear` or colors | Breaks pipes, logs and CI output; check `[[ -t 1 ]]` |

---

## Quality

All scripts pass these with zero findings before a task is done:

```bash
shellcheck --severity=warning --shell=bash path/to/*.sh
shfmt -d -i 4 -ci path/to/*.sh     # prints a diff when formatting is off; fix with -w
bats test/                         # when the project has bats tests
```

Suppress a ShellCheck finding only inline, with the code and a reason (`# shellcheck disable=SC1091 # sourced at runtime`).

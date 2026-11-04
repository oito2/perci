# GitHub Actions (CI/CD) Standard

Workflow mechanics and security for `.github/workflows/`, composite actions and Dependabot; **which checks to run** comes from the language standards in `.instructions/` (e.g. `GO.md` → CI, `TAURI.md` → CI Basics, `FLUTTER.md`).

Last verified: 2026-11-04

---

## Scope and Agent Limits

- The agent **writes workflow files** only. It never pushes, tags, re-runs or dispatches workflows (`AGENTS.md` → Git Operations): it hands the user the exact commands (`git push origin v1.2.0`, `gh workflow run release.yml`).
- Repository settings (rulesets, required checks, environments, secrets, OIDC trust) are changed by the user; the agent lists the exact values in its report.
- Versions below were checked on the date above. Check each action's current release before writing a `uses:` line; never copy a version or SHA from memory.

---

## Layout and Naming

- One workflow per purpose, lowercase hyphenated file names: `ci.yml` (push/PR checks), `release.yml` (tag builds); reusable workflows (`on: workflow_call`) and local composite actions (`.github/actions/<name>/action.yml`) only when logic is shared.
- Every workflow and job has a `name:`. Job names are the status-check names used by branch protection — keep them stable.
- Scripts longer than ~15 lines move to a repository script (see `BASH.md`). Set `defaults.run.shell: bash` so steps run with `-eo pipefail`.

---

## Triggers

```yaml
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:
```

- CI: `pull_request` + `push` to the default branch. Releases: `push: tags: ["v*"]` (SemVer tags, `PROJECT-DOCUMENTATION.md` → Releases).
- `paths` and `paths-ignore` cannot be combined for one event. A workflow skipped by a path filter leaves its required check **pending** and blocks merging — never path-filter required checks.
- **`pull_request_target` / `workflow_run`** run with secrets and a write token even for fork PRs. Avoid them; use `pull_request` (forks get a read-only token, no secrets). If unavoidable, **never check out, build or install the PR's code**, and treat artifacts from other runs as untrusted.

---

## Permissions and `GITHUB_TOKEN`

```yaml
permissions:
  contents: read        # workflow default
jobs:
  release:
    permissions:
      contents: write   # widened only where needed
```

- Always set workflow-level `permissions:` (`contents: read`, or `{}`) and widen per job. Once any permission is listed, unlisted ones are `none`. Never `write-all`.
- Prefer `${{ github.token }}` over PATs; a PAT/App token only for what it cannot do, narrowly scoped.
- `actions/checkout` persists the token by default: set `persist-credentials: false` unless the job pushes.

---

## Pinning Actions

```yaml
- uses: actions/checkout@<40-char-commit-sha> # v7.0.0
```

- Pin **every** action, `actions/*` included, to a **full commit SHA** with the version as a comment; tags can be moved. Resolve it from the action's own repository: `git ls-remote --tags https://github.com/actions/checkout 'v7.0.0*'` (annotated tag → the `^{}` line). Docker actions pin an `@sha256:` digest.
- Read a new action's `action.yml` and source first; prefer GitHub, vendor or widely used maintained actions.
- If the repository uses GitHub's workflow lockfile (`.github/workflows/actions.lock`, `gh actions-lock`), keep it in sync after every `uses:` change.
- Dependabot keeps pinned SHAs and their comments current (its *alerts* skip SHA-pinned actions, so version updates are required):

```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: "/"
    schedule:
      interval: weekly
    groups:
      actions:
        patterns: ["*"]
```

---

## Concurrency, Timeouts, Matrix

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
jobs:
  test:
    runs-on: ${{ matrix.os }}
    timeout-minutes: 20
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
```

- Every workflow has a `concurrency` group; cancel superseded PR runs, **never** release or deploy runs.
- Every job sets `timeout-minutes` (default 360, the hosted-runner maximum) to a few times its normal duration.
- `fail-fast` defaults to `true`; use `false` for test matrices. Matrix only the platforms/versions the project supports (max 256 jobs).

---

## Toolchains and Caching

- Read versions from project files: `go-version-file: go.mod` (`actions/setup-go`), `node-version-file: .nvmrc` or `package.json` (`actions/setup-node`), `python-version-file`, `rust-toolchain.toml`.
- Prefer built-in caches: `setup-go` caches by default; `setup-node` caches npm automatically when `package.json` declares `packageManager`, else `cache: npm|pnpm|yarn`. `actions/cache` only for the rest, keyed on a lockfile hash.
- Caches are unverified: never cache secrets; release builds do not restore build-output caches. CI tools are installed at pinned versions, never `@latest`.

---

## Secrets, Environments and OIDC

- Pass secrets to the one step that needs them via step `env:`; never workflow-level `env:`, never in `run:` text, logs, artifacts or caches.
- One secret per value (no JSON/YAML blobs — redaction matches exact strings); mask derived values with `echo "::add-mask::$VALUE"`.
- Publishing/deploy jobs use an `environment:`; tell the user to configure **required reviewers**, allowed branches/tags, and environment-scoped secrets.
- Use **OIDC** (`id-token: write` + the provider's official login action) instead of long-lived cloud or registry keys.
- Reusable workflows get secrets explicitly; `secrets: inherit` only within the same repository.

---

## Script Injection

`${{ }}` is substituted before the shell runs, so untrusted values become code: issue/PR titles and bodies, comments, commit messages, `github.head_ref`, branch and label names, author names/e-mails, `workflow_dispatch` inputs.

```yaml
# Wrong
- run: echo "${{ github.event.pull_request.title }}"
# Right
- env:
    PR_TITLE: ${{ github.event.pull_request.title }}
  run: printf '%s\n' "$PR_TITLE"
```

- `run:` scripts and `actions/github-script` read values from `env` only; the same applies to `${{ inputs.* }}` in composite actions.

---

## Artifacts, Reusable Workflows, Composite Actions

- `actions/upload-artifact`: set `retention-days` (short, e.g. 7, for CI; default 90) and `if-no-files-found: error`. Names are unique per run (`dist-${{ matrix.os }}`); download with `pattern:` + `merge-multiple: true`. Keep `include-hidden-files: false`.
- Reusable workflow (`workflow_call`) shares whole jobs; composite action shares steps. Type every `inputs` entry; composite `run` steps declare `shell:`. Cross-repository reusable workflows are SHA-pinned; a called workflow can only reduce the caller's permissions.

---

## Release Workflows

```yaml
permissions:
  contents: write
  id-token: write
  attestations: write
steps:
  - run: sha256sum dist/* > dist/SHA256SUMS
  - uses: actions/attest@<40-char-commit-sha> # v4.x.y
    with:
      subject-checksums: dist/SHA256SUMS
  - env:
      GH_TOKEN: ${{ github.token }}
      TAG: ${{ github.ref_name }}
    run: gh release create "$TAG" --verify-tag --title "$TAG" --notes-file release-notes.md dist/*
```

- Artifact names and checksums follow `PROJECT-DOCUMENTATION.md` → Releases and the language standard (e.g. `GO.md` → Release Builds); validate the tag format before using it in build flags.
- Attest with `actions/attest`: since v4, `actions/attest-build-provenance` is a wrapper over it (keep existing uses, write new ones with `actions/attest`). Private repositories need a plan that supports attestations.
- Document user-side verification: `gh attestation verify <file> -R <owner>/<repo>` and `sha256sum -c SHA256SUMS`.

---

## Branch Protection

Tell the user to create a ruleset (*Settings → Rules → Rulesets*) on the default branch: require a pull request, require the CI job names as **status checks** (list them exactly), block force pushes.

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `uses: owner/action@v1` / `@main` | Movable ref, supply-chain risk | Commit SHA + version comment + Dependabot |
| No `permissions:` or `write-all` | Token can push and alter releases | `contents: read`, widened per job |
| `${{ github.event.* }}` in `run:` | Script injection | Quoted `env:` variable |
| `pull_request_target` checking out PR code | Untrusted code with secrets | `pull_request` |
| Long-lived cloud keys as secrets | Persistent access when leaked | OIDC |
| No `timeout-minutes` | Hung job runs 6 hours | Explicit per-job timeout |
| `cancel-in-progress` on releases | Half-published release | Cancel PR runs only |
| Hard-coded tool versions | Drift from the project | `*-version-file` inputs |
| Path filters on required checks | Merges blocked by pending checks | Unfiltered required workflow |

---

## Quality

Run from the repository root before reporting workflow changes as done:

```bash
# Syntax, expressions and shellcheck on run: scripts
# install: go install github.com/rhysd/actionlint/cmd/actionlint@<version> | brew install actionlint
actionlint

# Security audit (injection, permissions, unpinned actions, dangerous triggers)
# install: pipx install zizmor | brew install zizmor | cargo install --locked zizmor
zizmor --offline .github/
GH_TOKEN="$(gh auth token)" zizmor .github/    # adds online audits; read-only

# YAML well-formedness
python3 -c 'import sys, yaml; [yaml.safe_load(open(f)) for f in sys.argv[1:]]' \
    .github/workflows/*.y*ml .github/dependabot.yml

# uses: lines not pinned to a SHA, local path or digest (must print nothing)
grep -rnE '^\s*-?\s*uses:' .github/ | grep -vE 'uses:\s*(\./|docker://[^ ]+@sha256:|[^@ ]+@[0-9a-f]{40}\b)'
```

- [ ] `actionlint` and `zizmor` clean (or each remaining finding explained to the user).
- [ ] Workflow-level `permissions:`, per-job `timeout-minutes`, a `concurrency` group in every workflow.
- [ ] Settings the user must change (required checks, environments, secrets, OIDC trust) listed with exact values.

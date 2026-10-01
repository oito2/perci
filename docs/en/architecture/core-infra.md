🌐 [Português](../../pt-br/architecture/core-infra.md) | **English** | 🏠 [Index](../index.md)

---

# Core Infrastructure

Cross-cutting packages every other layer depends on.

## `internal/config`

`Load()`/`Update()` for `~/.perci/config.yaml` — `Update` (load, mutate, save under one lock) is the only write path; the save itself writes atomically through `internal/fsutil.WriteFileAtomic` (temp file in the same directory, `chmod 0600`, `fsync`, then `os.Rename`) and keeps `~/.perci` itself `0700` — the file holds the MariaDB credentials. `Load` returns an empty `Config` (not an error) when the file doesn't exist yet and never fills a field in: an empty `workspace_path` means "not chosen", and `Config.Workspace()` resolves the folder (`~/workspace` when unset) for whoever needs it. `ExpandPath` resolves a leading `~`/`~/` against the user's home directory — used anywhere a config value is a filesystem path. See the [Configuration reference](../reference/configuration.md) for every key.

## `internal/distro`

`Detect()` (Debian/Fedora family) and `DetectDE()` (desktop environment) are the **only** sanctioned way to branch on distro anywhere in the codebase — domain packages never implement their own detection. Arch is not a supported family.

## `internal/executor`

The single point through which every privileged command is escalated. `sudo` is resolved from the fixed path `/usr/bin/sudo` (not `$PATH`-resolved), and environment variables are propagated through it via `sudo env KEY=VALUE ...` rather than relying on `sudo`'s own (inconsistent, distro-dependent) env-passthrough configuration.

`PrivilegedInstall`/`PrivilegedInstallSteps` install a file the unprivileged user downloaded (and verified) into a root-owned location without trusting it after the password prompt: root copies it into a `0600` staged file it owns, re-verifies the SHA-256 on that copy, then sets the final mode and renames it over the destination as `root:root` — one batch, one prompt. Used by the self-update and the appimagetool install; `install.sh` does the same in shell.

In a `RunSudoSequence` batch, a `Soft` step that fails prints its warning and the batch goes on (unchanged). Adding `SoftReport` keeps that, but makes the batch end with `ErrCompletedWithWarnings` when any such step failed — used by the Flatpak apps and font batches, which are made only of soft steps and used to report success even when every item failed.

`CommandAvailable`/`Which` resolve a command on `$PATH` in-process (`exec.LookPath`), never through the external `which` binary; tests replace `Executor.LookPath` to fake what's installed. `RunConcurrent(ctx, items, limit, fn)` stops starting items once `ctx` is done and turns a panic in `fn` into a returned error.

## `internal/fsutil`

`WriteFileAtomic(path, data, mode)`: temp file in the same directory, `chmod` to `mode`, `fsync`, rename — the final mode always applies, even over an existing file (`os.WriteFile` only applies it on creation). Used for `config.yaml`, the Docker stack export, the exported container logs and generated `.gitignore` files.

`VerifyFile(path, hash, want)`: the one checksum check for downloaded artifacts (self-update, Go, Flutter's Android tools, appimagetool) — hex, case-insensitive.

## `internal/selfupdate`

Self-update (`Run`) checks GitHub Releases, downloads, verifies, and replaces the running binary. When the binary's directory is user-writable, the swap is an atomic same-directory rename; otherwise it goes through `executor.PrivilegedInstall`. Every redirect of an asset download is re-validated against the trusted hosts, the binary download is bounded by `Run`'s 5-minute context rather than a per-request timeout, and it's `fsync`ed before the swap. Self-uninstall (`Uninstall`) removes the binary, the `perci` alias, the application menu entry (`.desktop` + hicolor icon set, in one privileged `rm`) and, optionally, `~/.perci`. `InstallMenuIcons` reinstalls the hicolor icon set (embedded by the `packaging` package) in the color chosen in **Home → Configurações → Ícone do Aplicativo**, in one privileged batch — the PNGs reach root as an in-memory tar archive on stdin, never as files in a user-writable temp directory.

## `internal/shellrc`

Idempotent helpers for editing shell rc files (`~/.bashrc`): `AppendIfMissing`, `RemoveEntry`, `Rewrite` (preserves file permissions), `Dedup` (returns a new deduplicated slice — does not mutate its input in place). Used by every package that needs to persist a PATH export (see [Dev environment](./dev-environment.md)). Edits go through symlinks to their target (a `~/.bashrc` managed by stow/chezmoi stays a symlink), read-modify-write happens under a per-file lock, entries are single lines only (multi-line blocks are appended line by line), and an empty comment never matches blank lines on removal.

## `internal/version`

Holds the version string, injected at build time via `-ldflags "-X .../internal/version.Version=vX.Y.Z"`. Backs `prci version`/`--version`.

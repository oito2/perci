// Copyright (C) 2026  oito2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package golang

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/shellrc"
	"github.com/oito2/perci/internal/ui"
)

const (
	goInstallDir = "/usr/local/go"
	// pathEntry is quoted so it also works in fish's config.fish.
	pathEntry = `export PATH="$PATH:/usr/local/go/bin"`
	// legacyPathEntry is the unquoted variant of pathEntry; install
	// replaces it and uninstall removes it.
	legacyPathEntry = "export PATH=$PATH:/usr/local/go/bin"
)

// Variables so they can be pointed at a test server (goVersionAPI,
// downloadBase) or a fake toolchain (goBinary).
var (
	goVersionAPI = "https://go.dev/dl/?mode=json"
	downloadBase = "https://dl.google.com/go"
	goBinary     = goInstallDir + "/bin/go"
)

type goFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	SHA256   string `json:"sha256"`
	Kind     string `json:"kind"`
}

type goRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []goFile `json:"files"`
}

// InstalledVersion returns whether Go is present at the standard install path
// and its version string (e.g. "go1.26.3").
func InstalledVersion(ctx context.Context, exe *executor.Executor) (bool, string) {
	out, err := exe.Output(ctx, executor.Options{}, goBinary, "version")
	if err != nil {
		return false, ""
	}
	// Output: "go version go1.26.3 linux/amd64" — anything else (e.g. a
	// DryRun placeholder) isn't a Go toolchain.
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) >= 3 && parts[0] == "go" && parts[1] == "version" && strings.HasPrefix(parts[2], "go") {
		return true, parts[2]
	}
	return false, ""
}

// fetchReleases queries goVersionAPI and returns the decoded release list.
// LatestRelease and fetchTarballChecksum both build on it.
func fetchReleases(ctx context.Context) ([]goRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, goVersionAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("criar requisição: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("consultar API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var releases []goRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decodificar resposta: %w", err)
	}
	return releases, nil
}

// tarballChecksum returns the sha256 for the archive matching
// version/goos/arch within releases (already fetched by the caller).
func tarballChecksum(releases []goRelease, version, goos, arch string) (string, error) {
	for _, r := range releases {
		if r.Version != version {
			continue
		}
		for _, f := range r.Files {
			if f.OS == goos && f.Arch == arch && f.Kind == "archive" {
				return f.SHA256, nil
			}
		}
	}
	return "", fmt.Errorf("checksum não encontrado para %s %s/%s", version, goos, arch)
}

// LatestRelease fetches the latest stable Go release from go.dev and
// returns its version together with the SHA-256 checksum of the tarball for
// the current platform. Passing checksumSHA256 into Install avoids a second
// request to goVersionAPI.
func LatestRelease(ctx context.Context) (version, checksumSHA256 string, err error) {
	releases, err := fetchReleases(ctx)
	if err != nil {
		return "", "", err
	}
	for _, r := range releases {
		if r.Stable {
			version = r.Version
			break
		}
	}
	if version == "" {
		return "", "", fmt.Errorf("nenhuma versão estável encontrada na API")
	}
	checksumSHA256, err = tarballChecksum(releases, version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", err
	}
	return version, checksumSHA256, nil
}

// Install downloads the tarball for version and installs it to /usr/local/go.
// Extraction happens to a staging directory first so that a failed extraction
// never leaves the system without a working Go installation. checksumSHA256
// is the expected sha256 of the platform's archive — pass the value
// LatestRelease already returned to avoid re-fetching the release list; pass
// "" to have Install look it up itself (e.g. when installing a fixed
// fallback version after a LatestRelease failure).
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, version, checksumSHA256 string) error {
	tmp, err := os.MkdirTemp("", "perci-go-*")
	if err != nil {
		return fmt.Errorf("criar diretório temporário: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	dest, checksumSHA256, err := download(ctx, exe, stdout, tmp, version, checksumSHA256)
	if err != nil {
		return err
	}
	return installTarball(ctx, exe, stdout, dest, checksumSHA256)
}

// download fetches version's tarball for this platform into dir and checks
// its sha256 (looked up when checksumSHA256 is ""), returning the file's
// path and the checksum it was verified against.
func download(ctx context.Context, exe *executor.Executor, stdout io.Writer, dir, version, checksumSHA256 string) (dest, checksum string, err error) {
	tarball := fmt.Sprintf("%s.%s-%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	url := downloadBase + "/" + tarball
	dest = filepath.Join(dir, tarball)

	ui.Info(stdout, "Baixando "+url+"...")
	if err := exe.Run(ctx,
		executor.Options{Stdout: stdout, Stderr: stdout},
		"curl", "-fSL", "--connect-timeout", "10", "--max-time", "600", "--progress-bar", "-o", dest, url,
	); err != nil {
		return "", "", fmt.Errorf("baixar tarball: %w", err)
	}

	ui.Info(stdout, "Verificando integridade do tarball...")
	if checksumSHA256 == "" {
		checksumSHA256, err = fetchTarballChecksum(ctx, version, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return "", "", fmt.Errorf("obter checksum: %w", err)
		}
	}
	if err := fsutil.VerifyFile(dest, sha256.New(), checksumSHA256); err != nil {
		return "", "", fmt.Errorf("verificação de integridade falhou: %w", err)
	}
	return dest, checksumSHA256, nil
}

// installTarball replaces goInstallDir with the contents of the verified
// tarball at dest, running the whole privileged part as one batch so the
// password is asked once. See installScript.
func installTarball(ctx context.Context, exe *executor.Executor, stdout io.Writer, dest, checksumSHA256 string) error {
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, []executor.PrivilegedStep{{
		Announce: "Extraindo e instalando em " + goInstallDir + "...",
		Name:     "sh",
		Args:     []string{"-c", installScript, "sh", goInstallDir, dest, checksumSHA256},
	}})
}

// installScript is installTarball's privileged batch. $1 = install dir,
// $2 = downloaded tarball (user-owned), $3 = its expected sha256.
//
// The tarball is first copied to a root-owned file and its checksum
// checked again there: the user-owned download could have been swapped
// after the Go-side verification. Extraction goes to "$1-staging"; only
// once it succeeded the live install is renamed to "$1-old" (a fast
// rename, not a recursive delete) and staging moved into place — if that
// last move fails, "$1-old" is moved back, so there is never a window
// without a toolchain at $1.
const installScript = `set -e
staging="$1-staging"; old="$1-old"; copy="$1-staging.tar.gz"
rm -rf -- "$staging" "$old" "$copy"
install -m 0600 -T -- "$2" "$copy"
printf '%s  %s\n' "$3" "$copy" | sha256sum -c --status - || { rm -f -- "$copy"; echo "o tarball do Go foi alterado após a verificação" >&2; exit 1; }
mkdir -p -- "$staging"
tar --strip-components=1 -C "$staging" -xzf "$copy" || { rm -rf -- "$staging" "$copy"; exit 1; }
rm -f -- "$copy"
if [ -e "$1" ]; then mv -- "$1" "$old" || { rm -rf -- "$staging"; exit 1; }; fi
mv -- "$staging" "$1" || { if [ -e "$old" ]; then mv -- "$old" "$1"; fi; rm -rf -- "$staging"; exit 1; }
rm -rf -- "$old"`

// fetchTarballChecksum queries the same go.dev/dl API LatestRelease uses and
// returns the sha256 for the archive matching version/goos/arch. Install
// calls it when the caller did not provide the checksum.
func fetchTarballChecksum(ctx context.Context, version, goos, arch string) (string, error) {
	releases, err := fetchReleases(ctx)
	if err != nil {
		return "", err
	}
	return tarballChecksum(releases, version, goos, arch)
}

// Uninstall removes the Go installation and its PATH entry in ~/.bashrc.
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Removendo Go...")
	if err := exe.Run(ctx,
		executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout},
		"rm", "-rf", "--", goInstallDir,
	); err != nil {
		ui.Err(stdout, "Falha ao remover Go: "+err.Error())
		return err
	}

	RemovePathFromBashrc(stdout)

	ui.Success(stdout, "Go removido com sucesso.")
	return nil
}

// RemovePathFromBashrc removes the "# Go" comment and PATH export line.
func RemovePathFromBashrc(stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	candidates := shellrc.Dedup([]string{filepath.Join(home, ".bashrc"), shellrc.File(home)})
	for _, entry := range []string{pathEntry, legacyPathEntry} {
		for _, res := range shellrc.RemoveEntry(candidates, "# Go", entry) {
			if res.Err != nil {
				ui.Warning(stdout, "Não foi possível atualizar "+res.Path+": "+res.Err.Error())
				continue
			}
			ui.Info(stdout, "PATH removido de "+res.Path)
		}
	}
}

// EnsurePathInBashrc adds /usr/local/go/bin to PATH in the current shell's RC file.
func EnsurePathInBashrc(stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		ui.Warning(stdout, "Não foi possível localizar o diretório home.")
		return
	}

	rc := shellrc.File(home)
	// Replaces the legacy unquoted line, which breaks PATH under fish.
	if hasLine(rc, legacyPathEntry) {
		shellrc.RemoveEntry([]string{rc}, "# Go", legacyPathEntry)
	}
	appended, err := shellrc.AppendIfMissing(rc, "# Go", pathEntry)
	if err != nil {
		ui.Warning(stdout, "Não foi possível atualizar "+rc+": "+err.Error())
		return
	}
	if appended {
		ui.Info(stdout, "PATH atualizado em "+rc)
	}
}

// hasLine reports whether path contains line as a whole line.
func hasLine(path, line string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return slices.Contains(strings.Split(string(data), "\n"), line)
}

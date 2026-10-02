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

package selfupdate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
	"github.com/oito2/perci/internal/version"
)

const githubRepo = "oito2/perci"
const checksumsAssetName = "checksums.txt"

// httpClient is used for the GitHub API request. Without an explicit
// timeout, a stalled connection (slow server, dead network) would hang Run()
// forever — the ctx it receives only cancels on SIGINT/SIGTERM. Tests replace
// it with an httptest.Server's own client to trust that server's self-signed
// TLS certificate; downloadClient reuses its Transport for the same reason.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// maxBinarySize caps the downloaded binary; anything larger is refused
// explicitly instead of being truncated into a confusing checksum mismatch.
const maxBinarySize = 256 << 20

// downloadClient returns the client for release assets (checksums.txt and
// the binary). Every redirect hop is re-validated with validateDownloadURL —
// GitHub's browser_download_url redirects to its asset CDN, and without this
// only the first URL would ever be checked. timeout 0 means no overall
// deadline: the binary download is bounded by Run's context instead, since
// http.Client.Timeout also covers reading the body and a ~25 MB binary on a
// slow link easily takes longer than any fixed request timeout.
func downloadClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: httpClient.Transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("redirecionamentos demais ao baixar %s", via[0].URL)
			}
			return validateDownloadURL(req.URL.String())
		},
	}
}

// apiBaseURL is the GitHub API root. Tests override it to point at a local
// httptest.Server instead of the real GitHub API.
var apiBaseURL = "https://api.github.com"

// trustedDownloadHost/trustedDownloadHostSuffix are the hosts
// validateDownloadURL accepts for binary/checksum downloads. Tests override
// them to accept the local httptest.Server's host:port.
var (
	trustedDownloadHost       = "github.com"
	trustedDownloadHostSuffix = ".githubusercontent.com"
)

// executablePath locates the running binary — os.Executable, replaced by
// tests with a file in a temp dir.
var executablePath = os.Executable

// Release holds information about a GitHub release.
type Release struct {
	Version        string
	DownloadURL    string
	ChecksumSHA256 string
}

// Run checks for a newer version and applies the update if one is available.
func Run(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ui.PrintHeader(stdout, "Atualizar PERCI // O GENIAL", distro.DisplayName())
	ui.Info(stdout, "Versão atual: "+version.Version)

	rel, err := LatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("verificar atualizações: %w", err)
	}

	ui.Info(stdout, "Versão disponível: "+rel.Version)

	if !IsNewer(rel.Version, version.Version) {
		ui.Success(stdout, "Você já possui a versão mais recente.")
		return nil
	}

	ui.Info(stdout, fmt.Sprintf("Baixando %s (%s/%s)...", rel.Version, runtime.GOOS, runtime.GOARCH))
	if err := Apply(ctx, exe, stdout, rel); err != nil {
		return fmt.Errorf("atualizar binário: %w", err)
	}

	ui.Success(stdout, fmt.Sprintf("Perci atualizado para %s com sucesso. Reinicie o programa.", rel.Version))
	return nil
}

// LatestRelease queries the GitHub Releases API and returns the latest release.
func LatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBaseURL, githubRepo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "perci/"+version.Version)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API retornou status %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decodificar resposta: %w", err)
	}

	v := strings.TrimPrefix(payload.TagName, "v")
	assetName := fmt.Sprintf("prci-linux-%s", runtime.GOARCH)

	var downloadURL, checksumsURL string
	for _, a := range payload.Assets {
		switch a.Name {
		case assetName:
			downloadURL = a.BrowserDownloadURL
		case checksumsAssetName:
			checksumsURL = a.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("asset '%s' não encontrado na release %s", assetName, payload.TagName)
	}
	if checksumsURL == "" {
		return nil, fmt.Errorf("'%s' não encontrado na release %s — atualização recusada por segurança (sem checksum para verificar o binário)", checksumsAssetName, payload.TagName)
	}
	if err := validateDownloadURL(downloadURL); err != nil {
		return nil, err
	}
	if err := validateDownloadURL(checksumsURL); err != nil {
		return nil, err
	}

	checksum, err := fetchChecksum(ctx, checksumsURL, assetName)
	if err != nil {
		return nil, fmt.Errorf("obter checksum de %s: %w", assetName, err)
	}

	return &Release{Version: "v" + v, DownloadURL: downloadURL, ChecksumSHA256: checksum}, nil
}

// validateDownloadURL rejects anything that isn't an https URL on github.com
// or a *.githubusercontent.com asset CDN host. downloadURL/checksumsURL come
// straight from the GitHub API response — this is a cheap extra layer of
// defense in case that response is ever tampered with or the parsing logic
// changes, on top of the checksum verification in Apply.
func validateDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL de download inválida: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("URL de download deve usar https, obtido %q", u.Scheme)
	}
	if u.Host != trustedDownloadHost && !strings.HasSuffix(u.Host, trustedDownloadHostSuffix) {
		return fmt.Errorf("host de download não confiável: %q", u.Host)
	}
	return nil
}

// fetchChecksum downloads a "sha256sum"-style checksums file from checksumsURL
// and returns the hash for the line matching assetName.
func fetchChecksum(ctx context.Context, checksumsURL, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := downloadClient(30 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download retornou status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("nenhuma entrada para '%s' em %s", assetName, checksumsAssetName)
}

// Apply downloads the new binary and replaces the current one.
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, rel *Release) (err error) {
	currentExe, err := executablePath()
	if err != nil {
		return fmt.Errorf("localizar binário atual: %w", err)
	}

	// Download next to the current binary when possible so the final swap is
	// a same-filesystem rename. The system temp dir is often a separate
	// mount (e.g. tmpfs), where os.Rename fails with EXDEV, a distinct error
	// from permission denied. sameFS tracks which directory tmp actually
	// landed in so the swap logic below knows whether a plain rename can
	// even work.
	sameFS := true
	tmp, err := os.CreateTemp(filepath.Dir(currentExe), "perci-*.new")
	if err != nil {
		sameFS = false
		tmp, err = os.CreateTemp("", "perci-*.new")
		if err != nil {
			return fmt.Errorf("criar arquivo temporário: %w", err)
		}
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// tmp is kept open and passed straight to downloadFile instead of being
	// closed and reopened by tmpPath — reopening by path would leave a window
	// where tmpPath could be deleted and recreated as a symlink to an
	// arbitrary target (TOCTOU), which downloadFile would then follow and
	// write into, and the later os.Rename would move that symlink
	// itself into place as currentExe.
	ui.Info(stdout, "Baixando para "+tmpPath+"...")
	downloadErr := downloadFile(ctx, rel.DownloadURL, tmp)
	closeErr := tmp.Close()
	if downloadErr != nil {
		return fmt.Errorf("baixar: %w", downloadErr)
	}
	if closeErr != nil {
		return fmt.Errorf("fechar arquivo temporário: %w", closeErr)
	}

	ui.Info(stdout, "Verificando integridade do binário...")
	if err := verifyChecksum(tmpPath, rel.ChecksumSHA256); err != nil {
		return fmt.Errorf("verificação de integridade falhou: %w", err)
	}

	// currentExe's directory is writable by the user (tmp was created right
	// next to it): the swap needs no privilege, a same-filesystem rename is
	// atomic, and the file never passes through root.
	if sameFS {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
		if err := os.Rename(tmpPath, currentExe); err == nil {
			return nil
		} else if !os.IsPermission(err) {
			return fmt.Errorf("substituir binário: %w", err)
		}
		ui.Warning(stdout, "Permissão negada ao substituir o binário. Tentando com privilégios de administrador...")
	}

	// Privileged swap: root copies tmpPath into a root-owned staged file,
	// re-verifies the checksum on that copy and renames it over currentExe
	// (executor.PrivilegedInstallSteps) — never a plain `mv` of a file the
	// user can still rewrite while the password dialog is open.
	if err := exe.PrivilegedInstall(ctx,
		executor.Options{Stdout: stdout, Stderr: stdout},
		tmpPath, currentExe, strings.ToLower(strings.TrimSpace(rel.ChecksumSHA256)), "0755",
	); err != nil {
		return fmt.Errorf("substituir binário: %w", err)
	}
	return nil
}

// downloadFile writes url's body into dest, which the caller already has open
// (see the TOCTOU note in Apply — this must never reopen a file by path).
func downloadFile(ctx context.Context, url string, dest *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := downloadClient(0).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download retornou status %d", resp.StatusCode)
	}

	n, err := io.Copy(dest, io.LimitReader(resp.Body, maxBinarySize+1))
	if err != nil {
		return err
	}
	if n > maxBinarySize {
		return fmt.Errorf("arquivo maior que %d MiB — download recusado", maxBinarySize>>20)
	}
	// Flush to disk before the rename: a power loss right after it must not
	// leave a truncated binary in place.
	return dest.Sync()
}

// verifyChecksum reports an error if the SHA-256 of the file at path does not
// match expectedSHA256. This is the only thing standing between a compromised
// download (bad DNS, MITM'd CDN, tampered release asset) and replacing the
// running binary — Apply must call this before the file is ever executed.
func verifyChecksum(path, expectedSHA256 string) error {
	return fsutil.VerifyFile(path, sha256.New(), expectedSHA256)
}

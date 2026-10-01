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

package prereqs

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
	"sync"
	"time"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// Prereq describes a prerequisite managed by perci.gnl.
type Prereq struct {
	Name        string
	Description string
	ID          string
	// RemoveWarning, when set, is shown for confirmation before the item
	// is uninstalled — for a removal that takes more with it than the
	// item itself.
	RemoveWarning string
}

// Catalogue lists all prerequisites managed by perci.gnl, ordered by essentiality.
var Catalogue = []Prereq{
	{Name: "Git", ID: "git", Description: "Sistema de controle de versão distribuído"},
	{Name: "Curl", ID: "curl", Description: "Ferramenta de linha de comando para transferência de dados via URL"},
	{Name: "OpenSSL", ID: "openssl", Description: "Ferramentas e bibliotecas para criptografia SSL/TLS"},
	{Name: "Lsof", ID: "lsof", Description: "Utilitário para listagem de arquivos abertos pelo sistema"},
	{Name: "CMake", ID: "cmake", Description: "Gerador de sistemas de compilação cross-platform"},
	{Name: "Ninja", ID: "ninja", Description: "Sistema de compilação extremamente rápido focado em velocidade"},
	{Name: "Clang", ID: "clang", Description: "Compilador C/C++ baseado no LLVM"},
	{Name: "GTK3 Dev Headers", ID: "gtk3dev", Description: "Arquivos de cabeçalho e desenvolvimento para a biblioteca GTK 3 (usado, por exemplo, por apps Flutter Linux desktop)"},
	{Name: "GTK4 Dev Headers", ID: "gtk4dev", Description: "Arquivos de cabeçalho e desenvolvimento para a biblioteca GTK 4 — necessário para compilar a própria GUI do Perci (Wails v3)"},
	{Name: "WebKitGTK 6.0 Dev Headers", ID: "webkitgtk6dev", Description: "Arquivos de cabeçalho e desenvolvimento do WebKitGTK 6.0 — necessário, junto com o GTK4, para compilar a própria GUI do Perci (Wails v3)"},
	{Name: "Wails3 CLI", ID: "wails3", Description: "CLI do Wails v3, instalado via 'go install' — só é necessário para quem for contribuir com a GUI do Perci e precisar regenerar os bindings do frontend (make generate-bindings)"},
	{Name: "Libsecret Tools", ID: "libsecret", Description: "Utilitário para acesso a chaves do sistema via Secret Service API"},
	{Name: "GNOME Keyring", ID: "gnome-keyring", Description: "Serviço daemon para armazenamento seguro de senhas e chaves"},
	{Name: "GitHub CLI (gh)", ID: "gh", Description: "Interface de linha de comando oficial para o GitHub"},
	{Name: "Docker Engine", ID: "docker", Description: "Motor de contêineres Docker e utilitário Docker Compose"},
	{Name: "Node.js", ID: "node", Description: "Ambiente de execução JavaScript (versão LTS instalada via nvm)",
		RemoveWarning: "Remover o Node.js apaga a pasta ~/.nvm inteira: todas as versões do Node instaladas pelo nvm e todos os pacotes npm globais delas (inclusive o Codex CLI, se instalado por aqui). As linhas do nvm no arquivo de inicialização do shell precisam ser removidas manualmente."},
	{Name: "Flatpak", ID: "flatpak", Description: "Gerenciador de pacotes e distribuição de aplicativos sandbox"},
	{Name: "Flatpak Builder", ID: "flatpak-builder", Description: "Ferramenta para compilação de aplicativos no formato Flatpak"},
	{Name: "AppStream", ID: "appstream", Description: "Utilitário para metadados de componentes AppStream, usado por Flatpak/AppImage"},
	{Name: "Libfuse2", ID: "libfuse2", Description: "Biblioteca de compatibilidade FUSE2, necessária para executar AppImages"},
	{Name: "AppImageTool", ID: "appimagetool", Description: "Utilitário de empacotamento e geração de arquivos AppImage"},
	{Name: "Mkcert", ID: "mkcert", Description: "Gera certificados TLS localmente confiáveis (usado pelo Contêiner Nginx da nova stack Docker por projeto)"},
}

// InstalledMap returns which prerequisites are currently installed.
// It checks every Catalogue entry in parallel instead of one after
// another: several isInstalled branches spawn a subprocess
// (pkg-config/dpkg/rpm/go env/...), and running them sequentially would
// add up to ~20 subprocesses, one after the other, every time the
// "Desenvolvimento :: Pré-requisitos" screen loads. The list is small and
// fixed (it doesn't scale with user input), so one goroutine per item,
// with no concurrency limit, is enough.
func InstalledMap(ctx context.Context, exe *executor.Executor) map[string]bool {
	result := make(map[string]bool, len(Catalogue))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range Catalogue {
		wg.Add(1)
		go func(p Prereq) {
			defer wg.Done()
			installed := isInstalled(ctx, exe, p.ID)
			mu.Lock()
			result[p.Name] = installed
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return result
}

func isInstalled(ctx context.Context, exe *executor.Executor, id string) bool {
	which := func(cmd string) bool { return exe.CommandAvailable(ctx, cmd) }
	switch id {
	case "git":
		return which("git")
	case "curl":
		return which("curl")
	case "openssl":
		return which("openssl")
	case "lsof":
		return which("lsof")
	case "cmake":
		return which("cmake")
	case "ninja":
		return which("ninja") || which("ninja-build")
	case "clang":
		return which("clang")
	case "gtk3dev":
		_, err := exe.Output(ctx, executor.Options{}, "pkg-config", "--exists", "gtk+-3.0")
		return err == nil
	case "gtk4dev":
		_, err := exe.Output(ctx, executor.Options{}, "pkg-config", "--exists", "gtk4")
		return err == nil
	case "webkitgtk6dev":
		_, err := exe.Output(ctx, executor.Options{}, "pkg-config", "--exists", "webkitgtk-6.0")
		return err == nil
	case "wails3":
		if which("wails3") {
			return true
		}
		gobin := goBinDir(ctx, exe)
		if gobin == "" {
			return false
		}
		_, err := os.Stat(filepath.Join(gobin, "wails3"))
		return err == nil
	case "libsecret":
		return which("secret-tool")
	case "gnome-keyring":
		return which("gnome-keyring")
	case "gh":
		return which("gh")
	case "docker":
		return which("docker")
	case "node":
		script := `export NVM_DIR="$HOME/.nvm"; [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"; command -v node && command -v npm`
		_, err := exe.Output(ctx, executor.Options{}, "bash", "-c", script)
		return err == nil
	case "flatpak":
		return which("flatpak")
	case "flatpak-builder":
		return which("flatpak-builder")
	case "appstream":
		return which("appstreamcli")
	case "libfuse2":
		if distro.Detect() == distro.Fedora {
			_, err := exe.Output(ctx, executor.Options{}, "rpm", "-q", "fuse-libs")
			return err == nil
		}
		_, err := exe.Output(ctx, executor.Options{}, "dpkg", "-s", "libfuse2t64")
		return err == nil
	case "appimagetool":
		return which("appimagetool")
	case "mkcert":
		return which("mkcert")
	}
	return false
}

// InstallOne installs a single prerequisite.
func InstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, p Prereq) error {
	return installFor(ctx, exe, stdout, p, distro.Detect())
}

// installFor is InstallOne for an explicit distro family.
func installFor(ctx context.Context, exe *executor.Executor, stdout io.Writer, p Prereq, family string) error {
	opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
	switch p.ID {
	case "git", "curl", "openssl", "lsof", "cmake", "clang", "gnome-keyring", "flatpak", "flatpak-builder", "appstream":
		return distro.InstallPkgs(ctx, exe, stdout, family, p.ID)
	case "ninja":
		return distro.InstallPkgs(ctx, exe, stdout, family, "ninja-build")
	case "gtk3dev":
		pkg := "libgtk-3-dev"
		if family == distro.Fedora {
			pkg = "gtk3-devel"
		}
		return distro.InstallPkgs(ctx, exe, stdout, family, pkg)
	case "gtk4dev":
		pkg := "libgtk-4-dev"
		if family == distro.Fedora {
			pkg = "gtk4-devel"
		}
		return distro.InstallPkgs(ctx, exe, stdout, family, pkg)
	case "webkitgtk6dev":
		pkg := "libwebkitgtk-6.0-dev"
		if family == distro.Fedora {
			pkg = "webkitgtk6.0-devel"
		}
		return distro.InstallPkgs(ctx, exe, stdout, family, pkg)
	case "wails3":
		return installWails3(ctx, exe, stdout)
	case "libsecret":
		pkgs := []string{"libsecret-1-0", "libsecret-tools"}
		if family == distro.Fedora {
			pkgs = []string{"libsecret", "libsecret-devel"}
		}
		return distro.InstallPkgs(ctx, exe, stdout, family, pkgs...)
	case "libfuse2":
		return distro.InstallPkgs(ctx, exe, stdout, family, libfuse2Pkg(family))
	case "gh":
		return installGH(ctx, exe, stdout, family, opts)
	case "docker":
		return installDocker(ctx, exe, stdout, family)
	case "node":
		return installNode(ctx, exe, stdout)
	case "appimagetool":
		return installAppImageTool(ctx, exe, stdout, opts)
	case "mkcert":
		return distro.InstallPkgs(ctx, exe, stdout, family, mkcertPkgs(family)...)
	}
	return fmt.Errorf("instalador desconhecido para %s", p.Name)
}

// UninstallOne uninstalls a single prerequisite.
func UninstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, p Prereq) error {
	return uninstallFor(ctx, exe, stdout, p, distro.Detect())
}

// uninstallFor is UninstallOne for an explicit distro family.
func uninstallFor(ctx context.Context, exe *executor.Executor, stdout io.Writer, p Prereq, family string) error {
	opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
	switch p.ID {
	case "git", "curl", "openssl", "lsof", "cmake", "clang", "gnome-keyring", "flatpak", "flatpak-builder", "appstream":
		return removePkgs(ctx, exe, opts, family, p.ID)
	case "ninja":
		return removePkgs(ctx, exe, opts, family, "ninja-build")
	case "gtk3dev":
		pkg := "libgtk-3-dev"
		if family == distro.Fedora {
			pkg = "gtk3-devel"
		}
		return removePkgs(ctx, exe, opts, family, pkg)
	case "gtk4dev":
		pkg := "libgtk-4-dev"
		if family == distro.Fedora {
			pkg = "gtk4-devel"
		}
		return removePkgs(ctx, exe, opts, family, pkg)
	case "webkitgtk6dev":
		pkg := "libwebkitgtk-6.0-dev"
		if family == distro.Fedora {
			pkg = "webkitgtk6.0-devel"
		}
		return removePkgs(ctx, exe, opts, family, pkg)
	case "wails3":
		return uninstallWails3(ctx, exe)
	case "libsecret":
		pkgs := []string{"libsecret-1-0", "libsecret-tools"}
		if family == distro.Fedora {
			pkgs = []string{"libsecret", "libsecret-devel"}
		}
		return removePkgs(ctx, exe, opts, family, pkgs...)
	case "libfuse2":
		return removePkgs(ctx, exe, opts, family, libfuse2Pkg(family))
	case "gh":
		return uninstallGH(ctx, exe, stdout, family, opts)
	case "docker":
		return removePkgs(ctx, exe, opts, family, dockerPkgs(family)...)
	case "node":
		return uninstallNode(stdout)
	case "appimagetool":
		_ = exe.Run(ctx, opts, "rm", "-f", "--", appImageToolDest)
		return nil
	case "mkcert":
		return removePkgs(ctx, exe, opts, family, mkcertPkgs(family)...)
	}
	return fmt.Errorf("desinstalador desconhecido para %s", p.Name)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func removePkgs(ctx context.Context, exe *executor.Executor, opts executor.Options, family string, pkgs ...string) error {
	switch family {
	case distro.Fedora:
		return exe.Run(ctx, opts, "dnf", append([]string{"remove", "-y", "--"}, pkgs...)...)
	case distro.Debian:
		return exe.Run(ctx, opts, "apt-get", append([]string{"purge", "-y", "--"}, pkgs...)...)
	default:
		return fmt.Errorf("unsupported distro family: %s", family)
	}
}

func libfuse2Pkg(family string) string {
	if family == distro.Fedora {
		return "fuse-libs"
	}
	return "libfuse2t64"
}

// mkcertPkgs returns mkcert plus the NSS tools package it needs to install
// its local CA into Firefox's trust store (mkcert falls back to silently
// skipping Firefox support without it) — package name differs per distro
// family, confirmed against packages.debian.org / packages.ubuntu.com
// ("libnss3-tools") and Fedora's package repos ("nss-tools").
func mkcertPkgs(family string) []string {
	if family == distro.Fedora {
		return []string{"mkcert", "nss-tools"}
	}
	return []string{"mkcert", "libnss3-tools"}
}

func dockerPkgs(family string) []string {
	switch family {
	case distro.Fedora:
		// Fedora's own packaging (checked against Fedora 44): there's no
		// "docker"/"docker-buildx-plugin" package — those names are Docker
		// Inc.'s own repository's.
		return []string{"moby-engine", "docker-compose", "docker-buildx"}
	default:
		return []string{"docker.io", "docker-compose-v2", "docker-buildx"}
	}
}

// ── GitHub CLI ────────────────────────────────────────────────────────────────

func installGH(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string, opts executor.Options) error {
	ui.Info(stdout, "Instalando GitHub CLI...")
	switch family {
	case distro.Debian:
		script := `set -Eeuo pipefail
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list
apt-get update -q
apt-get install -y -- gh`
		return exe.Run(ctx, opts, "bash", "-c", script)
	case distro.Fedora:
		// gh is in Fedora's own repositories (checked against Fedora 44) —
		// no extra repo needed, and the old `dnf config-manager --add-repo`
		// syntax doesn't exist in dnf5 (Fedora 41+).
		return exe.Run(ctx, opts, "dnf", "install", "-y", "--", "gh")
	default:
		return fmt.Errorf("unsupported distro family for GitHub CLI: %s", family)
	}
}

func uninstallGH(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string, opts executor.Options) error {
	_ = stdout
	switch family {
	case distro.Debian:
		script := `set -Eeuo pipefail
apt-get purge -y -- gh
rm -f /etc/apt/sources.list.d/github-cli.list /usr/share/keyrings/githubcli-archive-keyring.gpg
apt-get update -q`
		return exe.Run(ctx, opts, "bash", "-c", script)
	case distro.Fedora:
		return exe.Run(ctx, opts, "dnf", "remove", "-y", "--", "gh")
	default:
		return fmt.Errorf("unsupported distro family for GitHub CLI: %s", family)
	}
}

// ── Docker ────────────────────────────────────────────────────────────────────

// installDocker installs the engine, enables the service and adds the user
// to the docker group as ONE privileged batch — one password prompt, not
// one per command (pkexec has no session cache). The group membership is
// checked beforehand, unprivileged.
func installDocker(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	ui.Info(stdout, "Instalando Docker via gerenciador de pacotes...")
	var steps []executor.PrivilegedStep
	switch family {
	case distro.Debian:
		steps = []executor.PrivilegedStep{
			{Name: "apt-get", Args: []string{"update", "-q"}},
			{Name: "apt-get", Args: append([]string{"install", "-y", "--"}, dockerPkgs(family)...)},
		}
	case distro.Fedora:
		steps = []executor.PrivilegedStep{{Name: "dnf", Args: append([]string{"install", "-y", "--"}, dockerPkgs(family)...)}}
	default:
		return fmt.Errorf("unsupported distro family for Docker: %s", family)
	}
	steps = append(steps, executor.PrivilegedStep{
		Announce: "Ativando o serviço docker...",
		Soft:     true, WarnMessage: "Não foi possível ativar o serviço docker.",
		Name: "systemctl", Args: []string{"enable", "--now", "docker"},
	})

	user := executor.CurrentUser()
	addToGroup := false
	if user != "" {
		out, _ := exe.Output(ctx, executor.Options{}, "groups", "--", user)
		addToGroup = !slices.Contains(strings.Fields(out), "docker")
	}
	if addToGroup {
		steps = append(steps, executor.PrivilegedStep{
			Announce: "Adicionando " + user + " ao grupo docker...",
			Soft:     true, WarnMessage: "Não foi possível adicionar " + user + " ao grupo docker.",
			Name: "usermod", Args: []string{"-aG", "docker", "--", user},
		})
	}

	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return err
	}
	if addToGroup {
		ui.Warning(stdout, "Reinicie a sessão para aplicar as permissões do grupo docker.")
	}
	if family == distro.Fedora {
		ui.Warning(stdout, "Fedora: se volumes não funcionarem, execute:\n  sudo setsebool -P container_manage_cgroup on")
	}
	return nil
}

// ── Node.js ───────────────────────────────────────────────────────────────────

func installNode(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	checkScript := `export NVM_DIR="$HOME/.nvm"; [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"; command -v node && command -v npm`
	if _, e := exe.Output(ctx, executor.Options{}, "bash", "-c", checkScript); e == nil {
		ui.Info(stdout, "Node.js já disponível.")
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("obter diretório home: %w", err)
	}
	_, statErr := os.Stat(filepath.Join(home, ".nvm", "nvm.sh"))
	freshNVM := statErr != nil

	// -u (nounset) is deliberately not set here: nvm.sh itself references
	// variables that may be unset, and sourcing it under `set -u` is a
	// well-known way to break nvm's own install/use logic.
	script := `set -Eeo pipefail
export NVM_DIR="$HOME/.nvm"
if [ ! -s "$NVM_DIR/nvm.sh" ]; then
    curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/HEAD/install.sh | bash
    export NVM_DIR="$HOME/.nvm"
fi
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
nvm install --lts
nvm use --lts
`
	ui.Info(stdout, "Instalando Node.js LTS via nvm...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "bash", "-c", script); err != nil {
		return fmt.Errorf("instalar Node.js: %w", err)
	}

	localbin.EnsureInPath(stdout)

	if freshNVM {
		ui.Warning(stdout, "Reinicie o terminal para ativar o nvm e o Node.js.")
	}
	return nil
}

func uninstallNode(stdout io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("obter diretório home: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(home, ".nvm")); err != nil {
		return fmt.Errorf("remover ~/.nvm: %w", err)
	}
	ui.Warning(stdout, "Node.js removido. Remova manualmente as linhas NVM_DIR do ~/.bashrc.")
	return nil
}

// ── AppImageTool ──────────────────────────────────────────────────────────────

const appImageToolDest = "/usr/local/bin/appimagetool"

// appImageToolVersion is a real, versioned release tag from
// github.com/AppImage/appimagetool — the actively maintained tool (not the
// old github.com/AppImage/AppImageKit repo this used to point at, whose
// "continuous" release stopped receiving new builds in 2023 and is now
// marked "obsolete" upstream). A versioned tag also lets the download be
// checked against the release's own published checksum (see
// appImageToolChecksum) — "continuous" is re-tagged onto new commits over
// time, so there would be no stable checksum to pin it against.
const appImageToolVersion = "1.9.1"

// githubAPIBase/githubDownloadBase are GitHub's API and download roots —
// variables so tests point them at a local server.
var (
	githubAPIBase      = "https://api.github.com"
	githubDownloadBase = "https://github.com"
)

func appImageToolArch() string {
	if runtime.GOARCH == "arm64" {
		return "aarch64"
	}
	return "x86_64"
}

// appImageToolChecksum fetches the GitHub release's own asset digest for
// assetName — GitHub's API has published a SHA-256 "digest" per release
// asset directly since 2024, so no separate checksums file is needed. This
// queries the API (an independent source from the asset download itself),
// same reasoning as internal/dev/golang's fetchTarballChecksum.
func appImageToolChecksum(ctx context.Context, assetName string) (string, error) {
	apiURL := githubAPIBase + "/repos/AppImage/appimagetool/releases/tags/" + appImageToolVersion
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("criar requisição: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("consultar GitHub API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API retornou status %d", resp.StatusCode)
	}

	var payload struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decodificar resposta: %w", err)
	}
	for _, a := range payload.Assets {
		if a.Name != assetName {
			continue
		}
		sum, ok := strings.CutPrefix(a.Digest, "sha256:")
		if !ok {
			return "", fmt.Errorf("formato de digest inesperado para %s: %q", assetName, a.Digest)
		}
		return sum, nil
	}
	return "", fmt.Errorf("asset %s não encontrado na release %s", assetName, appImageToolVersion)
}

// installAppImageTool downloads appimagetool to an unprivileged temp file
// and verifies its checksum before anything runs elevated. The final step
// is executor.PrivilegedInstall: root copies the file into a root-owned
// staged copy, re-verifies the checksum on it and renames it into
// /usr/local/bin as root:root 0755 — a plain `mv` would keep the user as
// owner (and mode 0600) and trust a file the user can still rewrite while
// the password dialog is open.
func installAppImageTool(ctx context.Context, exe *executor.Executor, stdout io.Writer, opts executor.Options) error {
	assetName := "appimagetool-" + appImageToolArch() + ".AppImage"
	toolURL := githubDownloadBase + "/AppImage/appimagetool/releases/download/" + appImageToolVersion + "/" + assetName

	ui.Info(stdout, "Obtendo checksum do appimagetool "+appImageToolVersion+"...")
	checksum, err := appImageToolChecksum(ctx, assetName)
	if err != nil {
		return fmt.Errorf("obter checksum do appimagetool: %w", err)
	}

	tmp, err := os.CreateTemp("", "appimagetool-*.AppImage")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	ui.Info(stdout, "Baixando appimagetool...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "curl", "-fsSL", "--connect-timeout", "10", "--max-time", "600", "-o", tmpPath, "--", toolURL); err != nil {
		return fmt.Errorf("baixar appimagetool: %w", err)
	}

	ui.Info(stdout, "Verificando integridade do appimagetool...")
	if err := fsutil.VerifyFile(tmpPath, sha256.New(), checksum); err != nil {
		return fmt.Errorf("verificação de integridade do appimagetool falhou: %w", err)
	}

	if err := exe.PrivilegedInstall(ctx, opts, tmpPath, appImageToolDest, checksum, "0755"); err != nil {
		return fmt.Errorf("instalar appimagetool: %w", err)
	}
	return nil
}

// ── Wails3 CLI ────────────────────────────────────────────────────────────────

// wails3Version is pinned to match the version this repo's own go.mod
// requires (github.com/wailsapp/wails/v3) — same version cited in
// cmd/prci-gui/README.md/CONTRIBUTING.md's "go install" instructions.
// Update together if the project's own pinned Wails version changes.
const wails3Version = "v3.0.0-beta.22"

// goBinDir resolves where `go install` puts binaries — $GOBIN if set,
// otherwise $GOPATH/bin (matching `go help install`'s own resolution
// order) — needed because a freshly-installed Go tool isn't necessarily on
// PATH yet, the same reason installNode above sources nvm.sh before
// checking for node/npm instead of trusting a plain `which`.
func goBinDir(ctx context.Context, exe *executor.Executor) string {
	if out, err := exe.Output(ctx, executor.Options{}, "go", "env", "GOBIN"); err == nil {
		if dir := strings.TrimSpace(out); dir != "" {
			return dir
		}
	}
	if out, err := exe.Output(ctx, executor.Options{}, "go", "env", "GOPATH"); err == nil {
		if dir := strings.TrimSpace(out); dir != "" {
			return filepath.Join(dir, "bin")
		}
	}
	return ""
}

// installWails3 requires Go to already be installed ("Desenvolvimento ::
// Linguagens e SDKs") — no sudo involved, `go install` writes under the
// user's own GOPATH/GOBIN, same reasoning as installNode not using
// RequiresSudo.
func installWails3(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if !exe.CommandAvailable(ctx, "go") {
		return fmt.Errorf("go não encontrado — instale primeiro em Desenvolvimento :: Linguagens e SDKs")
	}
	ui.Info(stdout, "Instalando wails3 "+wails3Version+" via go install...")
	pkg := "github.com/wailsapp/wails/v3/cmd/wails3@" + wails3Version
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "go", "install", pkg); err != nil {
		return fmt.Errorf("instalar wails3: %w", err)
	}
	if !exe.CommandAvailable(ctx, "wails3") {
		ui.Warning(stdout, "wails3 instalado, mas não encontrado no PATH — adicione o diretório de binários do Go (`go env GOPATH`/bin, ou `go env GOBIN`) ao seu ~/.bashrc.")
	}
	return nil
}

func uninstallWails3(ctx context.Context, exe *executor.Executor) error {
	gobin := goBinDir(ctx, exe)
	if gobin == "" {
		return fmt.Errorf("não foi possível localizar o diretório de binários do Go")
	}
	if err := os.Remove(filepath.Join(gobin, "wails3")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remover wails3: %w", err)
	}
	return nil
}

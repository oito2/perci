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

// Package antigravityide manages the Google Antigravity IDE. Install and
// Update take a local .tar.gz path chosen by the user. Only the commands
// that need root run with RequiresSudo; the rest run as the current user.
package antigravityide

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

const (
	installDir       = "/opt/antigravity"
	defaultBinName   = "antigravity"
	sandboxPermWant  = "4755"
	iconExtractedRel = "icon.png"
)

// excludedSuffixes lists the extensions findBinary ignores when looking for
// the app binary.
var excludedSuffixes = []string{".so", ".json", ".png", ".pak", ".dll", ".dat", ".bin", ".desktop"}

// validBinName restricts findBinary's candidates to plain filenames, so a
// tarball entry name containing newlines or other bytes can never end up
// inside the .desktop file writeDesktopEntry generates (Exec=<path> %U).
var validBinName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func desktopFile(home string) string {
	return filepath.Join(home, ".local", "share", "applications", "antigravity.desktop")
}

func iconFile(home string) string {
	return filepath.Join(home, ".local", "share", "icons", "hicolor", "512x512", "apps", "antigravity.png")
}

// Installed reports whether Antigravity IDE is currently installed, using a
// plain os.Stat on the install directory.
func Installed() bool {
	info, err := os.Stat(installDir)
	return err == nil && info.IsDir()
}

// Install performs a first install; it fails if installDir already exists
// (use Update to replace an existing installation).
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, tarballPath string) error {
	return doInstallOrUpdate(ctx, exe, stdout, tarballPath, false)
}

// Update replaces an existing installation with the contents of a new
// tarball.
func Update(ctx context.Context, exe *executor.Executor, stdout io.Writer, tarballPath string) error {
	return doInstallOrUpdate(ctx, exe, stdout, tarballPath, true)
}

func doInstallOrUpdate(ctx context.Context, exe *executor.Executor, stdout io.Writer, tarballPath string, update bool) error {
	if _, err := os.Stat(tarballPath); err != nil {
		return fmt.Errorf("arquivo não encontrado: %s", tarballPath)
	}
	if !update && Installed() {
		return fmt.Errorf("%s já existe — use Atualizar em vez de Instalar", installDir)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("obter diretório home: %w", err)
	}

	workdir, err := os.MkdirTemp("", "antigravity-extract-*")
	if err != nil {
		return fmt.Errorf("criar diretório temporário: %w", err)
	}
	defer func() { _ = os.RemoveAll(workdir) }()

	userOpts := executor.Options{Stdout: stdout, Stderr: stdout}

	ui.Info(stdout, "Extraindo "+tarballPath+"...")
	if err := exe.Run(ctx, userOpts, "tar", "-xzf", tarballPath, "-C", workdir); err != nil {
		return fmt.Errorf("extrair tarball: %w", err)
	}

	candidateDir, binName, err := findBinary(workdir)
	if err != nil {
		return err
	}
	if err := exe.Run(ctx, userOpts, "chmod", "+x", filepath.Join(candidateDir, binName)); err != nil {
		ui.Warning(stdout, "Não foi possível marcar o binário como executável: "+err.Error())
	}

	if candidateDir == workdir {
		return fmt.Errorf("o tarball não tem uma pasta raiz com o app — formato inesperado")
	}
	sandboxSum, err := fileSHA256(filepath.Join(candidateDir, "chrome-sandbox"))
	if err != nil {
		return fmt.Errorf("chrome-sandbox não encontrado no tarball: %w", err)
	}

	// One privileged batch (one pkexec prompt) that never trusts the
	// extracted tree past the password dialog: the tree is still writable by
	// the user while the dialog is open, so root copies it into a root-owned
	// staging dir, strips any setuid/setgid bit that came with the archive,
	// re-verifies chrome-sandbox against the checksum taken above before
	// making it setuid root, and only then swaps it in — keeping the previous
	// install until the new one is fully in place.
	sandbox := filepath.Join(installDir, "chrome-sandbox")
	steps := []executor.PrivilegedStep{{
		Announce: "Instalando em " + installDir + "...",
		Name:     "sh",
		Args:     []string{"-c", installScript, "sh", installDir, candidateDir, sandboxSum, sandboxPermWant},
	}}
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return fmt.Errorf("instalar: %w", err)
	}

	if err := verifySandboxPerms(ctx, exe, sandbox); err != nil {
		ui.Warning(stdout, err.Error())
	}

	if iconPath, err := extractIcon(ctx, exe, stdout, userOpts, workdir); err != nil {
		ui.Warning(stdout, "Não foi possível extrair o ícone do app — o app funcionará, mas sem ícone customizado: "+err.Error())
	} else {
		installIcon(stdout, home, iconPath)
	}

	if err := writeDesktopEntry(home, binName); err != nil {
		ui.Warning(stdout, "Falha ao criar a entrada de menu: "+err.Error())
	} else {
		ui.Info(stdout, "Entrada de menu criada em "+desktopFile(home))
	}

	refreshDesktopDB(ctx, exe, stdout, home)

	ui.Success(stdout, "Antigravity IDE instalado em "+installDir+". Procure por 'Antigravity' no menu de aplicativos.")
	return nil
}

// installScript is doInstallOrUpdate's privileged batch. $1 = installDir,
// $2 = extracted app dir (user-owned), $3 = expected chrome-sandbox SHA-256,
// $4 = chrome-sandbox mode. Positional parameters keep every value out of
// the script text.
const installScript = `set -e
new="$1.new"; old="$1.old"
rm -rf -- "$new" "$old"
cp -a --no-preserve=ownership -- "$2" "$new"
chown -R root:root -- "$new"
chmod -R u-s,g-s -- "$new"
printf '%s  %s\n' "$3" "$new/chrome-sandbox" | sha256sum -c --status - || { rm -rf -- "$new"; echo "chrome-sandbox foi alterado após a verificação" >&2; exit 1; }
chmod "$4" -- "$new/chrome-sandbox"
if [ -e "$1" ]; then mv -- "$1" "$old"; fi
mv -- "$new" "$1"
rm -rf -- "$old"`

// fileSHA256 returns the hex SHA-256 of the regular file at path.
func fileSHA256(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s não é um arquivo comum", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findBinary finds files whose name starts with "antigravity"
// (case-insensitive) up to 3 levels under workdir, excluding known
// non-binary extensions. When more than one candidate matches, prefers the
// shallowest one that has a "chrome-sandbox" sibling (the real Electron app
// root), falling back to the first candidate found.
func findBinary(workdir string) (dir, name string, err error) {
	var candidates []string
	walkErr := filepath.WalkDir(workdir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: unreadable entries are skipped
		}
		rel, relErr := filepath.Rel(workdir, path)
		if relErr != nil {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if d.IsDir() {
			if depth >= 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if depth > 3 {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasPrefix(lower, "antigravity") {
			return nil
		}
		if !validBinName.MatchString(d.Name()) {
			return nil // name outside the expected charset — discard the candidate
		}
		for _, suf := range excludedSuffixes {
			if strings.HasSuffix(lower, suf) {
				return nil
			}
		}
		candidates = append(candidates, path)
		return nil
	})
	if walkErr != nil {
		return "", "", fmt.Errorf("percorrer arquivos extraídos: %w", walkErr)
	}
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("não encontrei nenhum binário começando com 'antigravity' dentro do tarball")
	}
	if len(candidates) == 1 {
		return filepath.Dir(candidates[0]), filepath.Base(candidates[0]), nil
	}

	best, bestDepth := "", -1
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if _, err := os.Stat(filepath.Join(dir, "chrome-sandbox")); err != nil {
			continue
		}
		rel, _ := filepath.Rel(workdir, c)
		depth := strings.Count(rel, string(filepath.Separator))
		if best == "" || depth < bestDepth {
			best, bestDepth = c, depth
		}
	}
	if best == "" {
		best = candidates[0]
	}
	return filepath.Dir(best), filepath.Base(best), nil
}

// verifySandboxPerms checks that chrome-sandbox has the setuid bit (4755,
// applied by the RunSudoSequence batch in doInstallOrUpdate), without which
// Electron's sandboxed renderer process refuses to start. Read-only, no
// privilege needed.
func verifySandboxPerms(ctx context.Context, exe *executor.Executor, sandbox string) error {
	perms, err := exe.Output(ctx, executor.Options{}, "stat", "-c", "%a", sandbox)
	if err != nil {
		return fmt.Errorf("verificar permissões do chrome-sandbox: %w", err)
	}
	if strings.TrimSpace(perms) != sandboxPermWant {
		return fmt.Errorf("falha ao aplicar permissões setuid no chrome-sandbox")
	}
	return nil
}

// extractIcon extracts the app's icon from its Electron app.asar via `npx
// @electron/asar`, pinned to 3.4.1 so the version never floats. Best-effort:
// npx unavailable (no Node.js/nvm) is a warning, not a failure.
func extractIcon(ctx context.Context, exe *executor.Executor, stdout io.Writer, opts executor.Options, workdir string) (string, error) {
	dest := filepath.Join(workdir, iconExtractedRel)
	script := `
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" >/dev/null 2>&1
command -v npx >/dev/null 2>&1 || exit 3
cd "$1" || exit 1
rm -f "$2"
npx --yes @electron/asar@3.4.1 extract-file "$3/resources/app.asar" "$2" 2>/dev/null || true
[ -f "$2" ]
`
	if err := exe.Run(ctx, opts, "bash", "-c", script, "--", workdir, iconExtractedRel, installDir); err != nil {
		return "", fmt.Errorf("npx indisponível ou extração falhou")
	}
	if _, err := os.Stat(dest); err != nil {
		return "", fmt.Errorf("app.asar não continha /icon.png")
	}
	return dest, nil
}

func installIcon(stdout io.Writer, home, extractedIconPath string) {
	dest := iconFile(home)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		ui.Warning(stdout, "Falha ao criar diretório de ícones: "+err.Error())
		return
	}
	data, err := os.ReadFile(extractedIconPath)
	if err != nil {
		ui.Warning(stdout, "Falha ao ler ícone extraído: "+err.Error())
		return
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		ui.Warning(stdout, "Falha ao instalar ícone: "+err.Error())
		return
	}
	ui.Info(stdout, "Ícone instalado em "+dest)
}

// writeDesktopEntry writes the application menu entry for binName.
func writeDesktopEntry(home, binName string) error {
	content := fmt.Sprintf(`[Desktop Entry]
Name=Antigravity
Comment=Google Antigravity IDE
Exec=%s %%U
Terminal=false
Type=Application
Icon=antigravity
Categories=Development;IDE;
StartupNotify=true
StartupWMClass=Antigravity
MimeType=x-scheme-handler/antigravity;
`, filepath.Join(installDir, binName))

	path := desktopFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o755)
}

// refreshDesktopDB refreshes the desktop and icon caches. Entirely
// best-effort: a missing update-desktop-database/gtk-update-icon-cache/
// desktop-file-validate is not fatal.
func refreshDesktopDB(ctx context.Context, exe *executor.Executor, stdout io.Writer, home string) {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	_ = exe.Run(ctx, opts, "update-desktop-database", filepath.Join(home, ".local", "share", "applications"))
	_ = exe.Run(ctx, opts, "gtk-update-icon-cache", "-t", filepath.Join(home, ".local", "share", "icons", "hicolor"))
	if err := exe.Run(ctx, opts, "desktop-file-validate", desktopFile(home)); err != nil {
		ui.Warning(stdout, "desktop-file-validate encontrou problemas no .desktop: "+err.Error())
	}
}

// Uninstall removes the installation, the desktop entry and the icon.
// User config and cache (~/.config/Antigravity, ~/.cache/Antigravity,
// ~/.config/antigravity-updater) are left in place.
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	sudo := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}

	ui.Info(stdout, "Removendo "+installDir+"...")
	if err := exe.Run(ctx, sudo, "rm", "-rf", "--", installDir); err != nil {
		return fmt.Errorf("remover %s: %w", installDir, err)
	}

	if home, err := os.UserHomeDir(); err == nil {
		_ = os.Remove(desktopFile(home))
		_ = os.Remove(iconFile(home))
		refreshDesktopDB(ctx, exe, stdout, home)
	}

	ui.Success(stdout, "Antigravity IDE desinstalado.")
	return nil
}

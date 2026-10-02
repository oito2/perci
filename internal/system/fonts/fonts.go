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

package fonts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/sets"
	"github.com/oito2/perci/internal/ui"
)

// Font describes an installable font.
type Font struct {
	Name string
	// Check is a font family name, matched exactly against the families
	// fc-list reports (so "Noto Sans" does not match "Noto Sans CJK JP").
	Check string
	// AptPkg/DnfPkg are the distribution packages (Debian family / Fedora)
	// that provide the font; both empty when it's installed via download.
	AptPkg string
	DnfPkg string
	URL    string // download URL (.zip or .tar.xz); used for fonts without a package
	// ChecksumsURL, when set, points to a "sha256  filename" checksums file
	// (sha256sum format) used to verify the file downloaded from URL before
	// it's extracted.
	ChecksumsURL string
	// ChecksumSHA256 is an alternative to ChecksumsURL: a pinned SHA-256
	// of the download. Only consulted when ChecksumsURL is empty.
	ChecksumSHA256 string
	RemoveGlob     string // glob pattern for .ttf files to delete from ~/.local/share/fonts/
}

// Catalogue lists all fonts managed by perci.gnl.
var Catalogue = []Font{
	{
		Name:           "JetBrains Mono",
		Check:          "JetBrains Mono",
		URL:            "https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip",
		ChecksumSHA256: "6f6376c6ed2960ea8a963cd7387ec9d76e3f629125bc33d1fdcd7eb7012f7bbf",
		RemoveGlob:     "JetBrainsMono-*.ttf",
	},
	{
		Name:         "JetBrains Mono NF",
		Check:        "JetBrainsMono Nerd Font",
		URL:          "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/JetBrainsMono.tar.xz",
		ChecksumsURL: "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/SHA-256.txt",
		RemoveGlob:   "JetBrainsMonoNerd*.ttf",
	},
	// Fonts installed from distribution packages.
	{Name: "Carlito", Check: "Carlito", AptPkg: "fonts-crosextra-carlito", DnfPkg: "google-carlito-fonts"},
	{Name: "Caladea", Check: "Caladea", AptPkg: "fonts-crosextra-caladea", DnfPkg: "google-crosextra-caladea-fonts"},
	{Name: "Noto", Check: "Noto Sans", AptPkg: "fonts-noto", DnfPkg: "google-noto-sans-fonts"},
	{Name: "Noto CJK", Check: "Noto Sans CJK JP", AptPkg: "fonts-noto-cjk", DnfPkg: "google-noto-sans-cjk-fonts"},
	{Name: "Noto Color Emoji", Check: "Noto Color Emoji", AptPkg: "fonts-noto-color-emoji", DnfPkg: "google-noto-color-emoji-fonts"},
}

// packaged reports whether f comes from a distribution package (on any
// family) rather than a download.
func (f Font) packaged() bool { return f.AptPkg != "" || f.DnfPkg != "" }

// pkgFor returns f's package on family, or "" when it has none there.
func (f Font) pkgFor(family string) string {
	switch family {
	case distro.Debian:
		return f.AptPkg
	case distro.Fedora:
		return f.DnfPkg
	}
	return ""
}

// InstalledNames returns a set of font names that are currently installed.
func InstalledNames(ctx context.Context, exe *executor.Executor) map[string]bool {
	out, err := exe.Output(ctx, executor.Options{}, "fc-list", "--format", "%{family}\n")
	if err != nil {
		return map[string]bool{}
	}
	return installedFrom(out)
}

// installedFrom maps fc-list's family output (one font per line, its
// family names comma-separated) to the Catalogue fonts present in it.
func installedFrom(fcFamilies string) map[string]bool {
	families := map[string]bool{}
	for _, line := range strings.Split(fcFamilies, "\n") {
		for _, name := range strings.Split(line, ",") {
			if name = strings.TrimSpace(name); name != "" {
				families[name] = true
			}
		}
	}
	result := make(map[string]bool, len(Catalogue))
	for _, f := range Catalogue {
		if families[f.Check] {
			result[f.Name] = true
		}
	}
	return result
}

// urlFontJob is one URL-downloaded font queued for parallel install/remove.
// Packaged fonts are not queued here; they go through a single
// RunSudoSequence call.
type urlFontJob struct {
	font   Font
	remove bool // false = install
}

// Apply installs fonts listed in toInstall and removes those in toRemove,
// emitting one ui.Step per font processed.
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, toInstall, toRemove []string) error {
	installSet := sets.Of(toInstall)
	removeSet := sets.Of(toRemove)
	family := distro.Detect()

	total := len(toInstall) + len(toRemove)
	step := 0

	// Every packaged font (install or remove) is batched into ONE sudo/
	// pkexec authentication via RunSudoSequence below. URL-downloaded
	// fonts (installFromURL) need no root (they land in
	// ~/.local/share/fonts); they are queued into urlJobs and run
	// concurrently below.
	var privSteps []executor.PrivilegedStep
	var urlJobs []urlFontJob

	for _, f := range Catalogue {
		switch {
		case installSet[f.Name]:
			step++
			ui.Step(stdout, step, total, "Instalando "+f.Name+"...")
			if f.packaged() {
				if s, ok := pkgFontStep(stdout, f, family, false); ok {
					privSteps = append(privSteps, s)
				}
				continue
			}
			urlJobs = append(urlJobs, urlFontJob{font: f})
		case removeSet[f.Name]:
			step++
			ui.Step(stdout, step, total, "Removendo "+f.Name+"...")
			if f.packaged() {
				if s, ok := pkgFontStep(stdout, f, family, true); ok {
					privSteps = append(privSteps, s)
				}
				continue
			}
			urlJobs = append(urlJobs, urlFontJob{font: f, remove: true})
		}
	}

	// Failures stay inline warnings (one font doesn't stop the others), but
	// are counted so Apply reports them instead of plain success.
	var failedMu sync.Mutex
	var failed []string

	if len(urlJobs) > 0 {
		syncOut := executor.NewSyncWriter(stdout)
		runErr := executor.RunConcurrent(ctx, urlJobs, 4, func(j urlFontJob) {
			var err error
			verb := "instalar"
			if j.remove {
				verb = "remover"
				err = remove(ctx, exe, syncOut, j.font)
			} else {
				err = install(ctx, exe, syncOut, j.font)
			}
			if err != nil {
				ui.Warning(syncOut, fmt.Sprintf("Falha ao %s %s: %v", verb, j.font.Name, err))
				failedMu.Lock()
				failed = append(failed, j.font.Name)
				failedMu.Unlock()
			}
		})
		if runErr != nil {
			ui.Warning(stdout, "Execução paralela interrompida: "+runErr.Error())
			failed = append(failed, "(interrompido)")
		}
	}

	var batchErr error
	if len(privSteps) > 0 {
		if batchErr = exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, privSteps); batchErr != nil &&
			!errors.Is(batchErr, executor.ErrCompletedWithWarnings) {
			ui.Warning(stdout, "Falha no lote de pacotes de fontes: "+batchErr.Error())
		}
	}

	ui.Info(stdout, "Atualizando cache de fontes...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "fc-cache", "-f"); err != nil {
		return err
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d fonte(s) com falha: %s", len(failed), strings.Join(failed, ", "))
	}
	return batchErr
}

// pkgFontStep builds the RunSudoSequence step installing (or removing) the
// package that provides f on family; ok is false — after a warning — when
// f has no package there. Removal on Fedora uses rpm -e, which refuses
// when another package requires the font, instead of dnf remove, which
// would take those dependents out with it.
func pkgFontStep(stdout io.Writer, f Font, family string, remove bool) (executor.PrivilegedStep, bool) {
	pkg := f.pkgFor(family)
	if pkg == "" {
		ui.Warning(stdout, f.Name+" não tem pacote conhecido nesta distribuição. Instale manualmente.")
		return executor.PrivilegedStep{}, false
	}
	verb, warn := "Instalando", "Falha ao instalar "+f.Name+"."
	if remove {
		verb, warn = "Removendo", "Falha ao remover "+f.Name+"."
	}
	s := executor.PrivilegedStep{
		Announce: verb + " " + f.Name + " (" + pkg + ")...",
		Soft:     true, SoftReport: true, WarnMessage: warn,
	}
	switch {
	case family == distro.Debian && !remove:
		s.Name, s.Args = "apt-get", []string{"install", "-y", "--", pkg}
	case family == distro.Debian:
		s.Name, s.Args = "apt-get", []string{"purge", "-y", "--", pkg}
	case !remove:
		s.Name, s.Args = "dnf", []string{"install", "-y", "--", pkg}
	default:
		s.Name, s.Args = "rpm", []string{"-e", "--", pkg}
	}
	return s, true
}

// install downloads f. Packaged fonts are handled by Apply's batch, not
// here. The step announcement is Apply's ui.Step call, so nothing is
// printed here.
func install(ctx context.Context, exe *executor.Executor, stdout io.Writer, f Font) error {
	if f.URL == "" {
		return fmt.Errorf("fonte %q: URL de download não configurada", f.Name)
	}
	return installFromURL(ctx, exe, stdout, f)
}

// remove deletes a downloaded font's files. Packaged fonts are handled by
// Apply's batch, and the step announcement is Apply's ui.Step call.
func remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, f Font) error {
	// Pass the glob via env variable so that shell metacharacters in future
	// catalogue entries cannot break or inject into the find command.
	return exe.Run(ctx,
		executor.Options{
			Stdout: stdout,
			Stderr: stdout,
			Env:    []string{"PERCI_GLOB=" + f.RemoveGlob},
		},
		"bash", "-c",
		`find "$HOME/.local/share/fonts" -name "$PERCI_GLOB" -delete 2>/dev/null; true`,
	)
}

// archiveKind decides the download's archive filename and its extraction
// command based on url's extension.
func archiveKind(url string) (archiveName, extractCmd string) {
	if strings.HasSuffix(url, ".tar.xz") {
		return filepath.Base(strings.TrimSuffix(url, ".tar.xz")) + ".tar.xz", requireTool("xz") + `tar -xJf "$TMP/$PERCI_ARCHIVE" -C "$TMP"`
	}
	return filepath.Base(strings.TrimSuffix(url, ".zip")) + ".zip", requireTool("unzip") + `unzip -q "$TMP/$PERCI_ARCHIVE" -d "$TMP"`
}

// requireTool is a script line failing with a clear message when tool is
// missing, instead of a bare "command not found" after the download.
func requireTool(tool string) string {
	return `command -v ` + tool + ` >/dev/null || { echo "` + tool + ` não encontrado — instale o pacote ` + tool + ` e tente de novo" >&2; exit 1; }
`
}

// installFromURL downloads and installs a font from a .zip or .tar.xz URL,
// verifying it against f.ChecksumsURL (or f.ChecksumSHA256, when the release
// has no separate checksums file) first when one is set. The URL and
// archive name reach the script through Env, never interpolated into the
// script text, so their content is never parsed by bash.
func installFromURL(ctx context.Context, exe *executor.Executor, stdout io.Writer, f Font) error {
	script, env := downloadScript(f)
	return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout, Env: env}, "bash", "-c", script)
}

// downloadScript is installFromURL's script and environment.
func downloadScript(f Font) (script string, env []string) {
	fontDir := "$HOME/.local/share/fonts"
	archiveName, extractCmd := archiveKind(f.URL)

	env = []string{
		"PERCI_URL=" + f.URL,
		"PERCI_ARCHIVE=" + archiveName,
		"PERCI_CHECKSUMS_URL=" + f.ChecksumsURL,
		"PERCI_EXPECTED_SHA256=" + f.ChecksumSHA256,
	}

	var checksumCmd string
	switch {
	case f.ChecksumsURL != "":
		// awk, not grep: an exact filename match that exits 0 when nothing
		// matches, so the "not found" message below is reached.
		checksumCmd = `
EXPECTED_SUM=$(curl -fsSL --connect-timeout 10 --max-time 600 "$PERCI_CHECKSUMS_URL" | awk -v f="$PERCI_ARCHIVE" '$2 == f { print $1 }')
if [ -z "$EXPECTED_SUM" ]; then
	echo "checksum de $PERCI_ARCHIVE não encontrado em $PERCI_CHECKSUMS_URL" >&2
	exit 1
fi
ACTUAL_SUM=$(sha256sum "$TMP/$PERCI_ARCHIVE" | cut -d ' ' -f1)
if [ "$EXPECTED_SUM" != "$ACTUAL_SUM" ]; then
	echo "checksum de $PERCI_ARCHIVE não confere: esperado $EXPECTED_SUM, obtido $ACTUAL_SUM" >&2
	exit 1
fi
`
	case f.ChecksumSHA256 != "":
		// No checksums file to fetch: verify against the SHA-256 pinned
		// in Font.ChecksumSHA256.
		checksumCmd = `
ACTUAL_SUM=$(sha256sum "$TMP/$PERCI_ARCHIVE" | cut -d ' ' -f1)
if [ "$PERCI_EXPECTED_SHA256" != "$ACTUAL_SUM" ]; then
	echo "checksum de $PERCI_ARCHIVE não confere: esperado $PERCI_EXPECTED_SHA256, obtido $ACTUAL_SUM" >&2
	exit 1
fi
`
	}

	script = fmt.Sprintf(`
set -Eeuo pipefail
mkdir -p "%s"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
curl -fsSL --connect-timeout 10 --max-time 600 "$PERCI_URL" -o "$TMP/$PERCI_ARCHIVE"
%s%s
find "$TMP" -maxdepth 3 -name "*.ttf" -exec cp -- {} "%s/" \;
`, fontDir, checksumCmd, extractCmd, fontDir)
	return script, env
}

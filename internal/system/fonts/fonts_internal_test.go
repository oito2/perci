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
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

func TestArchiveKind(t *testing.T) {
	cases := []struct {
		name              string
		url               string
		wantArchive       string
		wantExtractSubstr string
	}{
		{
			name:              "tar.xz",
			url:               "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/JetBrainsMono.tar.xz",
			wantArchive:       "JetBrainsMono.tar.xz",
			wantExtractSubstr: "tar -xJf",
		},
		{
			name:              "zip",
			url:               "https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip",
			wantArchive:       "JetBrainsMono-2.304.zip",
			wantExtractSubstr: "unzip -q",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archive, extractCmd := archiveKind(tc.url)
			if archive != tc.wantArchive {
				t.Errorf("archiveKind(%q) archive = %q, want %q", tc.url, archive, tc.wantArchive)
			}
			if !strings.Contains(extractCmd, tc.wantExtractSubstr) {
				t.Errorf("archiveKind(%q) extractCmd = %q, want it to contain %q", tc.url, extractCmd, tc.wantExtractSubstr)
			}
		})
	}
}

// Families are matched exactly: a CJK family must not count as plain
// "Noto Sans", and every name in a comma-separated list is considered.
func TestInstalledFrom(t *testing.T) {
	out := "Noto Sans CJK JP,Noto Sans CJK JP Bold\nJetBrainsMono Nerd Font,JetBrainsMono NF\n Carlito \n"
	got := installedFrom(out)
	for _, name := range []string{"Noto CJK", "JetBrains Mono NF", "Carlito"} {
		if !got[name] {
			t.Errorf("%s should be detected", name)
		}
	}
	for _, name := range []string{"Noto", "JetBrains Mono", "Caladea"} {
		if got[name] {
			t.Errorf("%s should not be detected", name)
		}
	}
}

func TestPkgFontStep(t *testing.T) {
	var carlito Font
	for _, f := range Catalogue {
		if f.Name == "Carlito" {
			carlito = f
		}
	}
	cases := []struct {
		family string
		remove bool
		want   string
	}{
		{"debian", false, "apt-get " + strings.Join(distro.AptArgs("install", "-y", "--", "fonts-crosextra-carlito"), " ")},
		{"debian", true, "dpkg --purge -- fonts-crosextra-carlito"},
		{"fedora", false, "dnf install -y -- google-carlito-fonts"},
		{"fedora", true, "rpm -e -- google-carlito-fonts"},
	}
	for _, tc := range cases {
		s, ok := pkgFontStep(io.Discard, carlito, tc.family, tc.remove)
		if got := s.Name + " " + strings.Join(s.Args, " "); !ok || got != tc.want {
			t.Errorf("%s remove=%v: got %q (ok=%v), want %q", tc.family, tc.remove, got, ok, tc.want)
		}
	}
	if _, ok := pkgFontStep(io.Discard, carlito, "arch", false); ok {
		t.Error("a family without a package must be skipped")
	}
}

// fontServer serves name → content over plain HTTP.
func fontServer(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func zipWith(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(content)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runDownload(t *testing.T, f Font) (string, error) {
	t.Helper()
	for _, tool := range []string{"bash", "curl", "unzip", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available")
		}
	}
	home := t.TempDir()
	script, env := downloadScript(f)
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), append(env, "HOME="+home)...)
	out, err := cmd.CombinedOutput()
	return home, fmtErr(out, err)
}

func fmtErr(out []byte, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// The real download script: a pinned SHA-256 that matches installs the
// .ttf files into ~/.local/share/fonts; a mismatch installs nothing.
func TestDownloadScript_PinnedChecksum(t *testing.T) {
	archive := zipWith(t, "fonts/Test-Regular.ttf", []byte("ttf"))
	srv := fontServer(t, map[string][]byte{"Test.zip": archive})
	sum := sha256.Sum256(archive)

	home, err := runDownload(t, Font{Name: "T", Dir: "test-font", URL: srv.URL + "/Test.zip", ChecksumSHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(home, ".local", "share", "fonts", "perci", "test-font", "Test-Regular.ttf")); err != nil || string(b) != "ttf" {
		t.Errorf("font not installed: %q, %v", b, err)
	}

	home, err = runDownload(t, Font{Name: "T", Dir: "test-font", URL: srv.URL + "/Test.zip", ChecksumSHA256: strings.Repeat("0", 64)})
	if err == nil || !strings.Contains(err.Error(), "não confere") {
		t.Fatalf("err = %v, want a checksum mismatch", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".local", "share", "fonts", "perci")); !os.IsNotExist(statErr) {
		t.Error("a font failing its checksum was installed")
	}
}

// With a checksums file, an archive missing from it is reported by name
// (the script still reaches its "not found" message under pipefail).
func TestDownloadScript_ChecksumsFile(t *testing.T) {
	archive := zipWith(t, "Test-Regular.ttf", []byte("ttf"))
	sum := sha256.Sum256(archive)
	srv := fontServer(t, map[string][]byte{
		"Test.zip":    archive,
		"SHA-256.txt": []byte(hex.EncodeToString(sum[:]) + "  Test.zip\n"),
		"other.txt":   []byte(hex.EncodeToString(sum[:]) + "  Other.zip\n"),
	})

	if _, err := runDownload(t, Font{Name: "T", Dir: "t", URL: srv.URL + "/Test.zip", ChecksumsURL: srv.URL + "/SHA-256.txt"}); err != nil {
		t.Fatalf("listed archive refused: %v", err)
	}
	_, err := runDownload(t, Font{Name: "T", Dir: "t", URL: srv.URL + "/Test.zip", ChecksumsURL: srv.URL + "/other.txt"})
	if err == nil || !strings.Contains(err.Error(), "não encontrado") {
		t.Errorf("err = %v, want 'checksum ... não encontrado'", err)
	}
}

// An archive without any .ttf fails the install and leaves no folder behind.
func TestDownloadScript_NoTTFFails(t *testing.T) {
	archive := zipWith(t, "readme.txt", []byte("no fonts"))
	srv := fontServer(t, map[string][]byte{"Empty.zip": archive})
	sum := sha256.Sum256(archive)
	home, err := runDownload(t, Font{Name: "T", Dir: "t", URL: srv.URL + "/Empty.zip", ChecksumSHA256: hex.EncodeToString(sum[:])})
	if err == nil || !strings.Contains(err.Error(), "nenhuma fonte .ttf") {
		t.Fatalf("err = %v, want 'nenhuma fonte .ttf encontrada'", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".local", "share", "fonts", "perci")); !os.IsNotExist(statErr) {
		t.Error("an empty archive left a font folder behind")
	}
}

// Removing a downloaded font deletes its folder (whatever file names it
// holds) and the loose files an older version installed, and nothing else.
func TestRemove_DeletesFolderAndLegacyFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fonts := filepath.Join(home, ".local", "share", "fonts")
	files := map[string]bool{ // path → must be removed
		"perci/jetbrains-mono/JetBrainsMono[wght].ttf":        true,
		"perci/jetbrains-mono/JetBrainsMonoNL-Regular.ttf":    true,
		"JetBrainsMono-Bold.ttf":                              true,
		"perci/jetbrains-mono-nf/JetBrainsMonoNerdFont-R.ttf": false,
		"JetBrainsMonoNerdFont-Old.ttf":                       false,
		"Other.ttf":                                           false,
	}
	for rel := range files {
		p := filepath.Join(fonts, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var jb Font
	for _, f := range Catalogue {
		if f.Name == "JetBrains Mono" {
			jb = f
		}
	}
	if err := remove(context.Background(), executor.New(io.Discard, io.Discard), io.Discard, jb); err != nil {
		t.Fatal(err)
	}
	for rel, gone := range files {
		_, err := os.Stat(filepath.Join(fonts, rel))
		if gone != os.IsNotExist(err) {
			t.Errorf("%s: removed=%v, want %v", rel, os.IsNotExist(err), gone)
		}
	}

	if err := remove(context.Background(), executor.New(io.Discard, io.Discard), io.Discard, Font{Name: "X"}); err == nil {
		t.Error("a font without a folder name must be refused")
	}
}

// Every downloaded font has its own folder and a pinned checksum.
func TestCatalogue_DownloadedFontsAreIsolatedAndPinned(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Catalogue {
		if f.packaged() {
			continue
		}
		if f.Dir == "" || seen[f.Dir] || strings.ContainsAny(f.Dir, "/.") {
			t.Errorf("%s: invalid or duplicate Dir %q", f.Name, f.Dir)
		}
		seen[f.Dir] = true
		if f.ChecksumSHA256 == "" && f.ChecksumsURL == "" {
			t.Errorf("%s has no checksum", f.Name)
		}
	}
	for _, f := range Catalogue {
		if f.Name == "JetBrains Mono NF" && f.ChecksumSHA256 != "ef552a3e638f25125c6ad4c51176a6adcdce295ab1d2ffacf0db060caf8c1582" {
			t.Errorf("the Nerd Font checksum must be pinned, got %q", f.ChecksumSHA256)
		}
	}
}

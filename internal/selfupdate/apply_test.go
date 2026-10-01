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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/version"
)

// withExecutable makes executablePath return a fake binary (content
// "old") in dir for the duration of the test.
func withExecutable(t *testing.T, dir string) string {
	t.Helper()
	exe := filepath.Join(dir, "prci")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := executablePath
	executablePath = func() (string, error) { return exe, nil }
	t.Cleanup(func() { executablePath = orig })
	return exe
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// serveBinary serves body at /prci and returns the release pointing at it.
func serveBinary(t *testing.T, body []byte, checksum string) *Release {
	t.Helper()
	srv := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prci" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
	return &Release{Version: "v9.9.9", DownloadURL: srv.URL + "/prci", ChecksumSHA256: checksum}
}

func dryRunExecutor(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

// No leftover download next to the binary, whatever the outcome.
func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(dir, "perci-*.new"))
	if len(matches) > 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

// A writable binary directory: the new binary is renamed into place, 0755,
// with no privilege escalation.
func TestApply_ReplacesBinaryInWritableDir(t *testing.T) {
	dir := t.TempDir()
	exe := withExecutable(t, dir)
	body := []byte("new-binary")
	rel := serveBinary(t, body, sha256Hex(body))

	var buf bytes.Buffer
	if err := Apply(context.Background(), dryRunExecutor(&buf), &buf, rel); err != nil {
		t.Fatalf("Apply: %v\n%s", err, buf.String())
	}
	got, err := os.ReadFile(exe)
	if err != nil || string(got) != "new-binary" {
		t.Fatalf("binary = %q, %v", got, err)
	}
	if info, _ := os.Stat(exe); info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
	if strings.Contains(buf.String(), "pkexec") || strings.Contains(buf.String(), "sudo") {
		t.Errorf("no escalation expected:\n%s", buf.String())
	}
	assertNoTempFiles(t, dir)
}

// A checksum mismatch aborts before anything is replaced.
func TestApply_WrongChecksumKeepsBinary(t *testing.T) {
	dir := t.TempDir()
	exe := withExecutable(t, dir)
	rel := serveBinary(t, []byte("tampered"), sha256Hex([]byte("expected")))

	var buf bytes.Buffer
	err := Apply(context.Background(), dryRunExecutor(&buf), &buf, rel)
	if err == nil || !strings.Contains(err.Error(), "integridade") {
		t.Fatalf("err = %v, want an integrity failure", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("binary changed to %q", got)
	}
	assertNoTempFiles(t, dir)
}

// A download error (HTTP 404) aborts too.
func TestApply_DownloadErrorKeepsBinary(t *testing.T) {
	dir := t.TempDir()
	exe := withExecutable(t, dir)
	rel := serveBinary(t, nil, "00")
	rel.DownloadURL += "-missing"

	var buf bytes.Buffer
	if err := Apply(context.Background(), dryRunExecutor(&buf), &buf, rel); err == nil {
		t.Fatal("expected a download error")
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("binary changed to %q", got)
	}
}

// A read-only binary directory (e.g. /usr/local/bin): the download goes to
// the system temp dir and the swap is ONE privileged install that
// re-verifies the checksum as root.
func TestApply_ReadOnlyDirUsesOnePrivilegedInstall(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	exe := withExecutable(t, dir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	body := []byte("new-binary")
	sum := sha256Hex(body)
	rel := serveBinary(t, body, sum)

	var buf bytes.Buffer
	if err := Apply(context.Background(), dryRunExecutor(&buf), &buf, rel); err != nil {
		t.Fatalf("Apply: %v\n%s", err, buf.String())
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("want exactly one pkexec call, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, sum) || !strings.Contains(out, exe) {
		t.Errorf("privileged install must carry the checksum and the destination:\n%s", out)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("dry-run must not touch the binary, got %q", got)
	}
}

// Run applies only a strictly newer release.
func TestRun_OnlyNewerVersionsApply(t *testing.T) {
	origVersion := version.Version
	t.Cleanup(func() { version.Version = origVersion })

	for _, tc := range []struct {
		current string
		applied bool
	}{
		{"1.2.3", false}, // same version
		{"2.0.0", false}, // local build newer than the release
		{"1.0.0", true},
	} {
		t.Run(tc.current, func(t *testing.T) {
			version.Version = tc.current
			dir := t.TempDir()
			exe := withExecutable(t, dir)
			assetName := fmt.Sprintf("prci-linux-%s", runtime.GOARCH)
			body := []byte("new-binary")
			var srvURL string
			srv := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/oito2/perci/releases/latest":
					fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
						assetName, srvURL+"/"+assetName, srvURL+"/checksums.txt")
				case "/checksums.txt":
					fmt.Fprintf(w, "%s  %s\n", sha256Hex(body), assetName)
				case "/" + assetName:
					_, _ = w.Write(body)
				default:
					http.NotFound(w, r)
				}
			})
			srvURL = srv.URL

			var buf bytes.Buffer
			if err := Run(context.Background(), dryRunExecutor(&buf), &buf); err != nil {
				t.Fatalf("Run: %v\n%s", err, buf.String())
			}
			got, _ := os.ReadFile(exe)
			if applied := string(got) == "new-binary"; applied != tc.applied {
				t.Errorf("applied = %v, want %v\n%s", applied, tc.applied, buf.String())
			}
		})
	}
}

// Uninstall removes a user-owned binary and its 'perci' alias without
// escalating, removes ~/.perci only when asked, and reports ErrUninstalled.
func TestUninstall_UserOwnedBinary(t *testing.T) {
	for _, removeConfig := range []bool{false, true} {
		t.Run(fmt.Sprint("removeConfig=", removeConfig), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			cfgDir := filepath.Join(home, ".perci")
			if err := os.MkdirAll(cfgDir, 0o700); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			exe := withExecutable(t, dir)
			alias := filepath.Join(dir, "perci")
			if err := os.Symlink(exe, alias); err != nil {
				t.Fatal(err)
			}

			// Menu files: one the user can delete, and two in a read-only
			// directory that need root.
			userEntry := filepath.Join(t.TempDir(), "perci.desktop")
			sysDir := t.TempDir()
			sysIcons := []string{filepath.Join(sysDir, "a.png"), filepath.Join(sysDir, "b.png")}
			for _, p := range append([]string{userEntry}, sysIcons...) {
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(sysDir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(sysDir, 0o755) })
			origMenu := uninstallMenuPaths
			uninstallMenuPaths = func() []string { return append([]string{userEntry}, sysIcons...) }
			t.Cleanup(func() { uninstallMenuPaths = origMenu })

			var buf bytes.Buffer
			err := Uninstall(context.Background(), dryRunExecutor(&buf), &buf, removeConfig)
			if !errors.Is(err, ErrUninstalled) {
				t.Fatalf("err = %v, want ErrUninstalled", err)
			}
			for _, p := range []string{exe, alias} {
				if _, statErr := os.Lstat(p); !os.IsNotExist(statErr) {
					t.Errorf("%s still exists", p)
				}
			}
			if _, statErr := os.Stat(cfgDir); os.IsNotExist(statErr) != removeConfig {
				t.Errorf("~/.perci removed = %v, want %v", os.IsNotExist(statErr), removeConfig)
			}
			if _, statErr := os.Stat(userEntry); !os.IsNotExist(statErr) {
				t.Error("a user-removable menu file must be removed directly")
			}
			// The root-owned files go in ONE privileged batch — never one
			// prompt per file (skipped as root, which needs no batch).
			if os.Geteuid() != 0 {
				out := buf.String()
				if n := strings.Count(out, "pkexec"); n != 1 || !strings.Contains(out, sysIcons[0]) || !strings.Contains(out, sysIcons[1]) {
					t.Errorf("want one pkexec call removing both icons, got %d:\n%s", n, out)
				}
			}
		})
	}
}

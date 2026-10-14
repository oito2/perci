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

package appstack

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// flatpakHome creates a home with the NSS databases of a Flatpak Zen,
// Firefox and Chrome, plus a cache folder that holds no profile.
func flatpakHome(t *testing.T) (home string, dbs map[string]string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	app := filepath.Join(home, ".var", "app")
	dbs = map[string]string{
		"app.zen_browser.zen": filepath.Join(app, "app.zen_browser.zen", ".zen", "abc.Default (release)"),
		"org.mozilla.firefox": filepath.Join(app, "org.mozilla.firefox", ".mozilla", "firefox", "xyz.default"),
		"com.google.Chrome":   filepath.Join(app, "com.google.Chrome", ".pki", "nssdb"),
	}
	for _, dir := range dbs {
		writeFile(t, filepath.Join(dir, "cert9.db"), "", 0o600)
	}
	writeFile(t, filepath.Join(app, "app.zen_browser.zen", "cache", "x", "data"), "", 0o600)
	return home, dbs
}

func TestFlatpakNSSDirs(t *testing.T) {
	home, dbs := flatpakHome(t)
	got := flatpakNSSDirs(home)
	var want []string
	for _, dir := range dbs {
		want = append(want, dir)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("dirs = %v\nwant %v", got, want)
	}
	if id := flatpakAppID(home, dbs["app.zen_browser.zen"]); id != "app.zen_browser.zen" {
		t.Errorf("app id = %q", id)
	}
}

// The CA is added, trusted for websites, to every database that lacks it;
// a database that already has it is left alone.
func TestTrustCAInFlatpakBrowsers(t *testing.T) {
	_, dbs := flatpakHome(t)
	caroot := t.TempDir()
	writeFile(t, filepath.Join(caroot, "rootCA.pem"), "CA", 0o644)
	log := filepath.Join(t.TempDir(), "certutil.log")
	stubBin(t, map[string]string{
		"mkcert": "echo '" + caroot + "'",
		// -L finds the CA only in Chrome's database.
		"certutil": `case "$1" in -L) case "$3" in *nssdb) exit 0;; esac; exit 1;; esac; printf '%s\n' "$*" >> '` + log + `'`,
	})
	var buf bytes.Buffer
	trustCAInFlatpakBrowsers(context.Background(), executor.New(&buf, &buf), &buf)

	data, _ := os.ReadFile(log)
	calls := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(calls) != 2 {
		t.Fatalf("certutil -A calls = %q, want 2 (Zen and Firefox)\n%s", calls, buf.String())
	}
	for _, id := range []string{"app.zen_browser.zen", "org.mozilla.firefox"} {
		want := "-A -d sql:" + dbs[id] + " -t C,, -n " + mkcertCANickname + " -i " + filepath.Join(caroot, "rootCA.pem")
		if !slices.Contains(calls, want) {
			t.Errorf("missing %q in %q", want, calls)
		}
	}
	if out := buf.String(); !strings.Contains(out, "app.zen_browser.zen, org.mozilla.firefox") || strings.Contains(out, "com.google.Chrome") {
		t.Errorf("report:\n%s", out)
	}
}

// Without certutil nothing runs and the user is told how to fix it; with no
// Flatpak browser nothing is said at all.
func TestTrustCAInFlatpakBrowsers_WithoutCertutil(t *testing.T) {
	flatpakHome(t)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	var buf bytes.Buffer
	trustCAInFlatpakBrowsers(context.Background(), executor.New(&buf, &buf), &buf)
	if !strings.Contains(buf.String(), "certutil não está instalado") {
		t.Errorf("no warning:\n%s", buf.String())
	}

	t.Setenv("HOME", t.TempDir())
	buf.Reset()
	trustCAInFlatpakBrowsers(context.Background(), executor.New(&buf, &buf), &buf)
	if buf.Len() != 0 {
		t.Errorf("output without Flatpak browsers:\n%s", buf.String())
	}
}

// Creating and recreating Nginx registers the CA in the Flatpak browsers.
func TestCreateAndRecreateNginx_TrustFlatpakBrowsers(t *testing.T) {
	flatpakHome(t)
	fakeStack(t, stackOpts{})
	caroot := t.TempDir()
	writeFile(t, filepath.Join(caroot, "rootCA.pem"), "CA", 0o644)
	log := filepath.Join(t.TempDir(), "certutil.log")
	stubBin(t, map[string]string{
		"mkcert":   `[ "$1" = -CAROOT ] && { echo '` + caroot + `'; exit 0; }; while [ $# -gt 0 ]; do case "$1" in -cert-file|-key-file) : > "$2"; shift 2 ;; *) shift ;; esac; done`,
		"certutil": `case "$1" in -L) exit 1;; esac; printf '%s\n' "$*" >> '` + log + `'`,
	})
	exe, buf := testExe()
	for name, run := range map[string]func() error{
		"create":   func() error { _, err := CreateNginx(context.Background(), exe, nil, buf); return err },
		"recreate": func() error { _, err := RecreateNginx(context.Background(), exe, nil, buf); return err },
	} {
		_ = os.Remove(log)
		if err := run(); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, buf.String())
		}
		if data, _ := os.ReadFile(log); strings.Count(string(data), "-A -d sql:") != 3 {
			t.Errorf("%s: certutil calls:\n%s", name, data)
		}
	}
}

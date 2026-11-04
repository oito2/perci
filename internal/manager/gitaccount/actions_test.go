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

package gitaccount

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// fakeSSHServer puts on PATH an ssh that serves git's request from
// repositories under the returned folder: "git@host:owner/repo.git" is
// root/owner/repo.git.
func fakeSSHServer(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	fakeBin(t, map[string]string{"ssh": `for a; do last=$a; done
cd "` + root + `" && eval "git ${last#git-}"`})
	return root
}

func originURL(t *testing.T, dir string) string {
	t.Helper()
	out, err := quietExecutor().Output(t.Context(), executor.Options{}, "git", "-C", dir, "remote", "get-url", "origin")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestFixURL(t *testing.T) {
	_, bare, acc := syncFixture(t)
	server := fakeSSHServer(t)
	if err := os.MkdirAll(filepath.Join(server, "oito2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(bare, filepath.Join(server, "oito2", "perci.git")); err != nil {
		t.Fatal(err)
	}

	perci := filepath.Join(acc.Folder, "perci")
	git(t, acc.Folder, "init", "-q", perci)
	git(t, perci, "remote", "add", "origin", "https://oito2:ghp_secret@github.com/oito2/perci.git")
	gone := filepath.Join(acc.Folder, "gone")
	git(t, acc.Folder, "init", "-q", gone)
	git(t, gone, "remote", "add", "origin", "https://github.com/oito2/gone.git")

	sts := states(t, acc, perci, gone)
	if sts[0].SSHURL != "git@github.com:oito2/perci.git" {
		t.Fatalf("SSHURL = %q", sts[0].SSHURL)
	}

	var out bytes.Buffer
	if err := FixURLs(t.Context(), quietExecutor(), &out, acc, sts); err == nil || !strings.Contains(err.Error(), "1 URL") {
		t.Errorf("FixURLs = %v, want one failure\n%s", err, out.String())
	}
	if got := originURL(t, perci); got != "git@github.com:oito2/perci.git" {
		t.Errorf("perci origin = %q", got)
	}
	if got := originURL(t, gone); got != "https://github.com/oito2/gone.git" {
		t.Errorf("an unreachable repository's origin changed to %q", got)
	}
	if strings.Contains(out.String(), "ghp_secret") {
		t.Errorf("the token reached the output:\n%s", out.String())
	}

	if err := FixURL(t.Context(), quietExecutor(), &out, acc, states(t, acc, perci)[0]); err == nil {
		t.Error("FixURL on an SSH origin succeeded")
	}
	if err := FixURLs(t.Context(), quietExecutor(), &out, acc, states(t, acc, perci)); err == nil {
		t.Error("FixURLs without HTTPS repositories succeeded")
	}
}

func TestSetRemote(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	acc := validAccount(t, home)
	draft := filepath.Join(acc.Folder, "draft")
	git(t, acc.Folder, "init", "-q", draft)
	st := states(t, acc, draft)[0]

	for _, bad := range []string{"", "../x", "a b", "-x/..", "x;rm"} {
		if _, err := SetRemote(t.Context(), quietExecutor(), acc, st, bad); err == nil {
			t.Errorf("SetRemote(%q) succeeded", bad)
		}
	}
	url, err := SetRemote(t.Context(), quietExecutor(), acc, st, "draft.git")
	if err != nil || url != "git@github.com:oito2/draft.git" || originURL(t, draft) != url {
		t.Errorf("SetRemote = %q, %v", url, err)
	}
	if _, err := SetRemote(t.Context(), quietExecutor(), acc, states(t, acc, draft)[0], "other"); err == nil {
		t.Error("SetRemote replaced an existing origin")
	}

	acc.SSHPort = 2222
	acc.Host = "git.example.com"
	other := filepath.Join(acc.Folder, "other")
	git(t, acc.Folder, "init", "-q", other)
	if url, _ := SetRemote(t.Context(), quietExecutor(), acc, states(t, acc, other)[0], "grp/app"); url != "ssh://git@git.example.com:2222/oito2/grp/app.git" {
		t.Errorf("SetRemote on port 2222 = %q", url)
	}
}

func TestResetIdentity(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	acc := validAccount(t, home)
	dir := filepath.Join(acc.Folder, "r")
	git(t, acc.Folder, "init", "-q", dir)
	git(t, dir, "config", "user.name", "Outro")
	git(t, dir, "config", "user.email", "outro@example.com")
	if !states(t, acc, dir)[0].IdentityDiverges {
		t.Fatal("identity does not diverge before the reset")
	}
	for i := 0; i < 2; i++ { // the second run finds nothing to remove
		if err := ResetIdentity(t.Context(), quietExecutor(), states(t, acc, dir)[0]); err != nil {
			t.Fatalf("ResetIdentity #%d: %v", i+1, err)
		}
	}
	if st := states(t, acc, dir)[0]; st.IdentityDiverges || st.LocalName != "" || st.LocalEmail != "" {
		t.Errorf("after ResetIdentity: %+v", st)
	}
}

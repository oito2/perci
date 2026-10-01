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

package repo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestIsEmptyDir(t *testing.T) {
	empty := t.TempDir()
	withHidden := t.TempDir()
	if err := os.WriteFile(filepath.Join(withHidden, ".keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dir  string
		want bool
	}{
		{"empty", empty, true},
		{"hidden file only", withHidden, false},
		{"missing", filepath.Join(empty, "missing"), true},
	}
	for _, tc := range cases {
		got, err := IsEmptyDir(tc.dir)
		if err != nil {
			t.Fatalf("%s: IsEmptyDir: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: IsEmptyDir = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Clone must refuse a non-empty folder before running git at all.
func TestClone_RefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	exe := executor.New(&out, &out)
	exe.DryRun = true

	err := Clone(context.Background(), exe, io.Discard, "https://example.com/x.git", dir, "n", "e@x")
	if err == nil || !strings.Contains(err.Error(), "não está vazia") {
		t.Fatalf("expected a non-empty folder error, got %v", err)
	}
	if strings.Contains(out.String(), "clone") {
		t.Errorf("git clone should not have run:\n%s", out.String())
	}
}

func TestCloneTargetName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/org/repo.git":  "repo",
		"https://github.com/org/repo/":     "repo",
		"git@github.com:org/repo.git":      "repo",
		"git@github.com:repo.git":          "repo",
		"ssh://git@host:2222/org/repo.git": "repo",
	}
	for url, want := range cases {
		if got := cloneTargetName(url); got != want {
			t.Errorf("cloneTargetName(%q) = %q, want %q", url, got, want)
		}
	}
}

// Only one of name/email is refused before anything is cloned; both empty
// is fine (the global identity applies).
func TestClone_IdentityPairCheckedFirst(t *testing.T) {
	var out strings.Builder
	exe := executor.New(&out, &out)
	exe.DryRun = true
	dir := filepath.Join(t.TempDir(), "new")

	if err := Clone(context.Background(), exe, io.Discard, "https://example.com/x.git", dir, "Nome", ""); err == nil {
		t.Fatal("name without e-mail must be refused")
	}
	if strings.Contains(out.String(), "clone") {
		t.Errorf("nothing may run when refused:\n%s", out.String())
	}
	if err := Clone(context.Background(), exe, io.Discard, "https://example.com/x.git", dir, "", ""); err != nil {
		t.Errorf("empty identity is optional: %v", err)
	}
	if strings.Contains(out.String(), "user.name") {
		t.Errorf("no identity should be applied when none was given:\n%s", out.String())
	}
}

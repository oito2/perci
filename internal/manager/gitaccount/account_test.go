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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"OITO2", "oito2"},
		{"Kadu Velasco", "kadu-velasco"},
		{"  Carlos__Velasco!! ", "carlos-velasco"},
		{"Conta Ação", "conta-a-o"},
		{"!!!", ""},
	}
	for _, tc := range cases {
		if got := Slug(tc.in); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize(config.GitAccount{
		Name:     "  Kadu Velasco ",
		Provider: ProviderGitLab,
		Host:     "",
		Username: " kadu ",
		Folder:   "/srv/repos/kadu/",
		SSHKey:   "/home/u/.ssh/../.ssh/k",
	})
	if got.Name != "Kadu Velasco" || got.ID != "kadu-velasco" || got.Host != "gitlab.com" ||
		got.Username != "kadu" || got.Folder != "/srv/repos/kadu" || got.SSHKey != "/home/u/.ssh/k" {
		t.Errorf("Normalize = %+v", got)
	}

	kept := Normalize(config.GitAccount{ID: "old-id", Name: "New Name", Provider: ProviderGitHub, Host: "GIT.Example.COM"})
	if kept.ID != "old-id" || kept.Host != "git.example.com" {
		t.Errorf("Normalize kept ID/host = %q/%q, want old-id/git.example.com", kept.ID, kept.Host)
	}
}

func TestValidate(t *testing.T) {
	home := isolate(t)
	base := validAccount(t, home)

	other := validAccount(t, home)
	other.ID, other.Name = "kadu", "Kadu"
	other.Folder = filepath.Join(home, "repos", "kadu")
	if err := os.MkdirAll(other.Folder, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "link-to-oito2")
	if err := os.Symlink(base.Folder, link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		edit   func(a *config.GitAccount)
		others []config.GitAccount
		want   string // substring of the error; "" means valid
	}{
		{"valid", func(*config.GitAccount) {}, []config.GitAccount{other}, ""},
		{"valid self-hosted with port", func(a *config.GitAccount) { a.Host, a.SSHPort = "git.example.com", 2222 }, nil, ""},
		{"empty name", func(a *config.GitAccount) { a.Name = "" }, nil, "nome da conta"},
		{"long name", func(a *config.GitAccount) { a.Name = strings.Repeat("a", 41) }, nil, "no máximo"},
		{"name without letters", func(a *config.GitAccount) { a.Name, a.ID = "!!!", "" }, nil, "letra ou número"},
		{"unknown provider", func(a *config.GitAccount) { a.Provider = "bitbucket" }, nil, "provedor"},
		{"bad host", func(a *config.GitAccount) { a.Host = "git hub" }, nil, "host inválido"},
		{"host without dot", func(a *config.GitAccount) { a.Host = "localhost" }, nil, "host inválido"},
		{"bad port", func(a *config.GitAccount) { a.SSHPort = 70000 }, nil, "porta"},
		{"bad username", func(a *config.GitAccount) { a.Username = "-oito2" }, nil, "usuário inválido"},
		{"empty git name", func(a *config.GitAccount) { a.GitName = "" }, nil, "commits"},
		{"multiline git name", func(a *config.GitAccount) { a.GitName = "a\nb" }, nil, "commits"},
		{"bad email", func(a *config.GitAccount) { a.GitEmail = "oito2" }, nil, "e-mail"},
		{"relative folder", func(a *config.GitAccount) { a.Folder = "repos" }, nil, "pasta da conta"},
		{"missing folder", func(a *config.GitAccount) { a.Folder = filepath.Join(home, "missing") }, nil, "pasta da conta"},
		{"folder is a file", func(a *config.GitAccount) { a.Folder = file }, nil, "não é uma pasta"},
		{"same name, other case", func(*config.GitAccount) {}, []config.GitAccount{{ID: "x", Name: "oito2"}}, "já existe"},
		{"same id", func(*config.GitAccount) {}, []config.GitAccount{{ID: "oito2", Name: "Other"}}, "já existe"},
		{"same folder", func(a *config.GitAccount) { a.Folder = other.Folder }, []config.GitAccount{other}, "sobrepõe"},
		{"folder inside another", func(a *config.GitAccount) {
			a.Folder = filepath.Join(other.Folder, "sub")
			_ = os.MkdirAll(a.Folder, 0o755)
		}, []config.GitAccount{other}, "sobrepõe"},
		{"folder containing another", func(a *config.GitAccount) { a.Folder = filepath.Join(home, "repos") }, []config.GitAccount{other}, "sobrepõe"},
		{"symlink to another's folder", func(a *config.GitAccount) { a.Folder = link }, []config.GitAccount{{ID: "y", Name: "Y", Folder: base.Folder}}, "sobrepõe"},
		{"sibling with common prefix", func(a *config.GitAccount) {
			a.Folder = other.Folder + "-2"
			_ = os.MkdirAll(a.Folder, 0o755)
		}, []config.GitAccount{other}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := base
			tc.edit(&acc)
			err := Validate(acc, tc.others)
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("Validate: %v, want nil", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("Validate: %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestSSHURL(t *testing.T) {
	acc := config.GitAccount{Host: "github.com"}
	if got := SSHURL(acc, "oito2", "perci"); got != "git@github.com:oito2/perci.git" {
		t.Errorf("SSHURL = %q", got)
	}
	acc = config.GitAccount{Host: "git.example.com", SSHPort: 2222}
	if got := SSHURL(acc, "carlos", "app"); got != "ssh://git@git.example.com:2222/carlos/app.git" {
		t.Errorf("SSHURL with port = %q", got)
	}
	acc.SSHPort = 22
	if got := SSHURL(acc, "carlos", "app"); got != "git@git.example.com:carlos/app.git" {
		t.Errorf("SSHURL with port 22 = %q", got)
	}
}

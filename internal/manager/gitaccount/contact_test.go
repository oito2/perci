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
	"reflect"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestContactEmail(t *testing.T) {
	acc := config.GitAccount{
		ContactEmail: "account@example.org",
		RepoSettings: []config.RepoSettings{{Path: "labs/perci", ContactEmail: "perci@example.org"}, {Path: ".", ContactEmail: "root@example.org"}},
	}
	for _, tc := range []struct{ path, effective, own string }{
		{"labs/perci", "perci@example.org", "perci@example.org"},
		{"labs/perci/", "perci@example.org", "perci@example.org"},
		{".", "root@example.org", "root@example.org"},
		{"web", "account@example.org", ""},
	} {
		effective, own := ContactEmail(acc, tc.path)
		if effective != tc.effective || own != tc.own {
			t.Errorf("ContactEmail(%q) = %q, %q, want %q, %q", tc.path, effective, own, tc.effective, tc.own)
		}
	}
}

func TestSetContactEmail(t *testing.T) {
	home := isolate(t)
	acc := validAccount(t, home)
	saveAccounts(t, acc)

	steps := []struct {
		path, email string
	}{
		{"", " account@example.org "},
		{"labs/perci", "perci@example.org"},
		{"web", "web@example.org"},
		{"labs/perci", "perci2@example.org"},
		{"web", ""},
	}
	for _, s := range steps {
		if err := SetContactEmail(acc.ID, s.path, s.email); err != nil {
			t.Fatalf("SetContactEmail(%q, %q): %v", s.path, s.email, err)
		}
	}
	got := loadAccounts(t)[0]
	want := []config.RepoSettings{{Path: "labs/perci", ContactEmail: "perci2@example.org"}}
	if got.ContactEmail != "account@example.org" || !reflect.DeepEqual(got.RepoSettings, want) {
		t.Errorf("account = %q %+v", got.ContactEmail, got.RepoSettings)
	}

	if err := SetContactEmail(acc.ID, "labs/perci", ""); err != nil {
		t.Fatal(err)
	}
	if err := SetContactEmail(acc.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := loadAccounts(t)[0]; got.ContactEmail != "" || len(got.RepoSettings) != 0 {
		t.Errorf("cleared account = %q %+v", got.ContactEmail, got.RepoSettings)
	}

	for name, call := range map[string]func() error{
		"invalid e-mail":  func() error { return SetContactEmail(acc.ID, "", "no-at-sign") },
		"escaping path":   func() error { return SetContactEmail(acc.ID, "../other", "a@example.org") },
		"absolute path":   func() error { return SetContactEmail(acc.ID, "/etc", "a@example.org") },
		"unknown account": func() error { return SetContactEmail("missing", "", "a@example.org") },
	} {
		if err := call(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestWebURL(t *testing.T) {
	acc := config.GitAccount{Provider: ProviderGitLab, Host: "gitlab.com", Username: "kadu"}
	for remote, want := range map[string]string{
		"git@gitlab.com:kadu/app.git":            "https://gitlab.com/kadu/app",
		"ssh://git@gitlab.com:2222/kadu/app.git": "https://gitlab.com/kadu/app",
		"https://gitlab.com/kadu/group/app.git":  "https://gitlab.com/kadu/group/app",
		"https://user:tok@gitlab.com/kadu/app":   "https://gitlab.com/kadu/app",
		"git@gitlab.com:other/app.git":           "",
		"git@github.com:kadu/app.git":            "",
		"":                                       "",
	} {
		if got := WebURL(acc, remote); got != want {
			t.Errorf("WebURL(%q) = %q, want %q", remote, got, want)
		}
	}
}

// Contact e-mails travel in the export file, which is version 2 only
// when it holds one.
func TestTransfer_ContactEmails(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	acc := validAccount(t, home)
	initRepo(t, filepath.Join(acc.Folder, "labs", "perci"), "git@github.com:oito2/perci.git")
	initRepo(t, filepath.Join(acc.Folder, "web"), "git@github.com:oito2/web.git")

	file, _, err := Exportable(t.Context(), quietExecutor(), []config.GitAccount{acc})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "repos.yaml")
	if err := WriteTransfer(path, file); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); file.Version != 1 || !strings.HasPrefix(string(data), "version: 1\n") || strings.Contains(string(data), "contact_email") {
		t.Errorf("export without e-mails:\n%s", data)
	}

	acc.ContactEmail = "account@example.org"
	if file, _, err := Exportable(t.Context(), quietExecutor(), []config.GitAccount{acc}); err != nil || file.Version != 2 {
		t.Errorf("export with only the account e-mail: version %d, %v", file.Version, err)
	}
	acc.RepoSettings = []config.RepoSettings{{Path: "web", ContactEmail: "web@example.org"}}
	file, _, err = Exportable(t.Context(), quietExecutor(), []config.GitAccount{acc})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteTransfer(path, file); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTransfer(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || got.Accounts[0].ContactEmail != "account@example.org" ||
		got.Accounts[0].Repos[0].ContactEmail != "" || got.Accounts[0].Repos[1].ContactEmail != "web@example.org" {
		t.Errorf("export with e-mails = %+v", got)
	}

	for name, edit := range map[string]func(*TransferFile){
		"invalid account e-mail": func(f *TransferFile) { f.Accounts[0].ContactEmail = "nope" },
		"invalid repo e-mail":    func(f *TransferFile) { f.Accounts[0].Repos[0].ContactEmail = "a@b" },
		"padded e-mail":          func(f *TransferFile) { f.Accounts[0].ContactEmail = " a@example.org" },
	} {
		f := sampleFile(acc.Folder)
		edit(&f)
		if err := f.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}

// Importing never replaces a contact e-mail already set here.
func TestImport_ContactEmails(t *testing.T) {
	requireTool(t, "git")
	requireTool(t, "ssh-keygen")
	home := isolate(t)
	existing := validAccount(t, home)
	existing.RepoSettings = []config.RepoSettings{{Path: "web", ContactEmail: "mine@example.org"}}
	saveAccounts(t, existing)

	file := sampleFile(filepath.Join(home, "elsewhere"))
	file.Accounts[0].ContactEmail = "file@example.org"
	file.Accounts[0].Repos[0].ContactEmail = "perci@example.org"
	file.Accounts[0].Repos[1].ContactEmail = "file-web@example.org"
	file.Accounts = append(file.Accounts, TransferAccount{
		Name: "Kadu", Provider: ProviderGitLab, Host: "gitlab.com", Username: "kadu",
		GitName: "Kadu", GitEmail: "kadu@example.com", Folder: filepath.Join(home, "kadu"),
		ContactEmail: "kadu@example.org",
		Repos:        []TransferRepo{{Path: "a", URL: "git@gitlab.com:kadu/a.git", ContactEmail: "a@example.org"}},
	})
	picks := []TransferPick{{Account: "OITO2", Repos: []string{"labs/perci", "web"}}, {Account: "Kadu", Repos: []string{"a"}}}
	var out bytes.Buffer
	if err := Import(t.Context(), quietExecutor(), &out, file, picks, nil); err != nil {
		t.Fatalf("Import: %v\n%s", err, out.String())
	}

	accounts := loadAccounts(t)
	merged, created := accounts[0], accounts[1]
	wantMerged := []config.RepoSettings{{Path: "web", ContactEmail: "mine@example.org"}, {Path: "labs/perci", ContactEmail: "perci@example.org"}}
	if merged.ContactEmail != "file@example.org" || !reflect.DeepEqual(merged.RepoSettings, wantMerged) {
		t.Errorf("merged account = %q %+v", merged.ContactEmail, merged.RepoSettings)
	}
	wantCreated := []config.RepoSettings{{Path: "a", ContactEmail: "a@example.org"}}
	if created.ContactEmail != "kadu@example.org" || !reflect.DeepEqual(created.RepoSettings, wantCreated) {
		t.Errorf("created account = %q %+v", created.ContactEmail, created.RepoSettings)
	}

	// A second import keeps the account e-mail set by the first one.
	file.Accounts[0].ContactEmail = "newer@example.org"
	if err := Import(t.Context(), quietExecutor(), &out, file, picks[:1], nil); err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if got := loadAccounts(t)[0].ContactEmail; got != "file@example.org" {
		t.Errorf("account e-mail replaced by %q", got)
	}
}

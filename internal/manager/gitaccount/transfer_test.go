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
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// initRepo creates a git repository at dir with origin url ("" for none).
func initRepo(t *testing.T, dir, url string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	if url != "" {
		git(t, dir, "remote", "add", "origin", url)
	}
}

// saveAccounts writes accounts to the (isolated) config.yaml.
func saveAccounts(t *testing.T, accounts ...config.GitAccount) {
	t.Helper()
	if err := config.Update(func(c *config.Config) error {
		c.GitAccounts = accounts
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func loadAccounts(t *testing.T) []config.GitAccount {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.GitAccounts
}

// sampleFile is a valid export file with one account and two repositories.
func sampleFile(folder string) TransferFile {
	return TransferFile{Version: TransferVersion, Accounts: []TransferAccount{{
		Name: "OITO2", Provider: ProviderGitHub, Host: "github.com", Username: "oito2",
		GitName: "Oito Dois", GitEmail: "oito2@example.com", Folder: folder,
		Repos: []TransferRepo{
			{Path: "labs/perci", URL: "git@github.com:oito2/perci.git"},
			{Path: "web", URL: "git@github.com:oito2/web.git"},
		},
	}}}
}

func TestExportRoundTrip(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	acc := validAccount(t, home)
	acc.Pending = []config.PendingRepo{{Path: "apps/later", URL: "git@github.com:oito2/later.git"}}
	initRepo(t, filepath.Join(acc.Folder, "labs", "perci"), "git@github.com:oito2/perci.git")
	initRepo(t, filepath.Join(acc.Folder, "web"), "https://oito2:ghp_secret@github.com/oito2/web.git")
	initRepo(t, filepath.Join(acc.Folder, "draft"), "")
	initRepo(t, filepath.Join(acc.Folder, "lib"), "git@github.com:someone/lib.git")

	file, skipped, err := Exportable(t.Context(), quietExecutor(), []config.GitAccount{acc})
	if err != nil {
		t.Fatal(err)
	}
	want := []TransferRepo{
		{Path: "labs/perci", URL: "git@github.com:oito2/perci.git"},
		{Path: "web", URL: "git@github.com:oito2/web.git"},
		{Path: "apps/later", URL: "git@github.com:oito2/later.git"},
	}
	if len(file.Accounts) != 1 || !reflect.DeepEqual(file.Accounts[0].Repos, want) {
		t.Fatalf("Exportable = %+v", file.Accounts)
	}
	if !reflect.DeepEqual(skipped, []string{"OITO2: draft"}) {
		t.Errorf("skipped = %v", skipped)
	}

	path := filepath.Join(home, "repos.yaml")
	if err := WriteTransfer(path, file); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, secret := range []string{"ghp_secret", "perci_oito2", "has_token"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("the export file contains %q:\n%s", secret, data)
		}
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("export file mode = %v", info.Mode().Perm())
	}
	got, err := LoadTransfer(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, file) {
		t.Errorf("LoadTransfer = %+v, want %+v", got, file)
	}

	if err := WriteTransfer(path, TransferFile{Version: TransferVersion}); !errors.Is(err, ErrNothingToExport) {
		t.Errorf("WriteTransfer of no account = %v", err)
	}
}

func TestTransferValidate(t *testing.T) {
	if err := sampleFile("/srv/repos").Validate(); err != nil {
		t.Fatalf("valid file: %v", err)
	}
	cases := map[string]func(f *TransferFile){
		"no version":     func(f *TransferFile) { f.Version = 0 },
		"newer version":  func(f *TransferFile) { f.Version = TransferVersion + 1 },
		"bad e-mail":     func(f *TransferFile) { f.Accounts[0].GitEmail = "x" },
		"relative":       func(f *TransferFile) { f.Accounts[0].Folder = "repos" },
		"bad provider":   func(f *TransferFile) { f.Accounts[0].Provider = "bitbucket" },
		"escaping path":  func(f *TransferFile) { f.Accounts[0].Repos[0].Path = "../x" },
		"account folder": func(f *TransferFile) { f.Accounts[0].Repos[0].Path = "." },
		"unclean path":   func(f *TransferFile) { f.Accounts[0].Repos[0].Path = "labs//perci" },
		"absolute path":  func(f *TransferFile) { f.Accounts[0].Repos[0].Path = "/tmp/x" },
		"repeated path":  func(f *TransferFile) { f.Accounts[0].Repos[1].Path = "labs/perci" },
		"https url":      func(f *TransferFile) { f.Accounts[0].Repos[0].URL = "https://github.com/oito2/perci.git" },
		"other owner":    func(f *TransferFile) { f.Accounts[0].Repos[0].URL = "git@github.com:someone/perci.git" },
		"option url":     func(f *TransferFile) { f.Accounts[0].Repos[0].URL = "-oProxyCommand=x" },
		"repeated name": func(f *TransferFile) {
			o := f.Accounts[0]
			o.Username, o.Folder = "other", "/srv/other"
			f.Accounts = append(f.Accounts, o)
		},
		"repeated user": func(f *TransferFile) {
			o := f.Accounts[0]
			o.Name, o.Folder = "Outra", "/srv/other"
			f.Accounts = append(f.Accounts, o)
		},
		"nested folders": func(f *TransferFile) {
			o := f.Accounts[0]
			o.Name, o.Username, o.Folder = "Outra", "other", "/srv/repos/other"
			f.Accounts = append(f.Accounts, o)
		},
	}
	for name, mutate := range cases {
		f := sampleFile("/srv/repos")
		mutate(&f)
		if err := f.Validate(); err == nil {
			t.Errorf("%s: Validate accepted the file", name)
		}
	}
}

func TestLoadTransfer_Refuses(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"docker.yaml":  "workspace_path: /srv/w\ndocker:\n  nginx_created: true\n",
		"empty.yaml":   "",
		"invalid.yaml": "version: 1\naccounts:\n  - name: X\n",
	} {
		path := filepath.Join(dir, name)
		writeFile(t, path, content)
		_, err := LoadTransfer(path)
		if err == nil {
			t.Errorf("LoadTransfer(%s) succeeded", name)
		} else if name == "docker.yaml" && !strings.Contains(err.Error(), "não é um arquivo de repositórios") {
			t.Errorf("LoadTransfer of a Docker export = %v", err)
		}
	}
}

func TestTransferFilter(t *testing.T) {
	f := sampleFile("/srv/repos")
	part, err := f.Filter([]TransferPick{{Account: "OITO2", Repos: []string{"web"}}})
	if err != nil || len(part.Accounts) != 1 || len(part.Accounts[0].Repos) != 1 || part.Accounts[0].Repos[0].Path != "web" {
		t.Errorf("Filter = %+v, %v", part, err)
	}
	if part, err := f.Filter(nil); err != nil || len(part.Accounts) != 0 {
		t.Errorf("Filter(nil) = %+v, %v", part, err)
	}
	for _, bad := range [][]TransferPick{{{Account: "Outra"}}, {{Account: "OITO2", Repos: []string{"nope"}}}} {
		if _, err := f.Filter(bad); err == nil {
			t.Errorf("Filter(%+v) succeeded", bad)
		}
	}
}

func TestImport(t *testing.T) {
	requireTool(t, "git")
	requireTool(t, "ssh-keygen")
	home := isolate(t)
	existing := validAccount(t, home)
	existing.Name = "Minha"
	existing.ID = Slug(existing.Name)
	existing.Pending = []config.PendingRepo{{Path: "old", URL: "git@github.com:oito2/old.git"}}
	saveAccounts(t, existing)
	initRepo(t, filepath.Join(existing.Folder, "labs", "perci"), "https://github.com/OITO2/perci")
	if err := os.MkdirAll(filepath.Join(existing.Folder, "full"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existing.Folder, "full", "f"), "x")
	// A pending repository stays pending even when its folder got content.
	if err := os.MkdirAll(filepath.Join(existing.Folder, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existing.Folder, "old", "f"), "x")

	file := sampleFile(filepath.Join(home, "elsewhere"))
	file.Accounts[0].Repos = append(file.Accounts[0].Repos,
		TransferRepo{Path: "old", URL: "git@github.com:oito2/old.git"},
		TransferRepo{Path: "full", URL: "git@github.com:oito2/full.git"})
	file.Accounts = append(file.Accounts, TransferAccount{
		Name: "Kadu", Provider: ProviderGitLab, Host: "gitlab.com", Username: "kadu",
		GitName: "Kadu", GitEmail: "kadu@example.com", Folder: filepath.Join(home, "from-file"),
		Repos: []TransferRepo{{Path: "a", URL: "git@gitlab.com:kadu/a.git"}, {Path: "g/b", URL: "git@gitlab.com:kadu/g/b.git"}},
	}, TransferAccount{
		Name: "Minha", Provider: ProviderGitHub, Host: "github.com", Username: "outra",
		GitName: "Outra", GitEmail: "outra@example.com", Folder: filepath.Join(home, "outra"),
	})

	kaduFolder := filepath.Join(home, "picked", "kadu")
	folders := map[string]string{"Kadu": kaduFolder, "OITO2": filepath.Join(home, "ignored")}
	plans := PlanImport(t.Context(), quietExecutor(), file, loadAccounts(t), folders)
	states := map[string]string{}
	for _, st := range plans[0].States {
		states[st.Path] = st.State
	}
	wantStates := map[string]string{"labs/perci": ImportPresent, "web": ImportNew, "old": ImportPending, "full": ImportConflict}
	if plans[0].ExistingID != existing.ID || plans[0].Folder != existing.Folder || !reflect.DeepEqual(states, wantStates) {
		t.Errorf("plan of the existing user = %+v", plans[0])
	}
	if plans[1].ExistingID != "" || plans[1].Folder != kaduFolder || plans[1].Reason != "" {
		t.Errorf("plan of the new account = %+v", plans[1])
	}
	if !strings.Contains(plans[2].Reason, "já existe uma conta") {
		t.Errorf("plan of the clashing name = %+v", plans[2])
	}

	picks := []TransferPick{
		{Account: "OITO2", Repos: []string{"labs/perci", "web", "old", "full"}},
		{Account: "Kadu", Repos: []string{"a", "g/b"}},
		{Account: "Minha"},
	}
	var out bytes.Buffer
	if err := Import(t.Context(), quietExecutor(), &out, file, picks, folders); !errors.Is(err, executor.ErrCompletedWithWarnings) {
		t.Fatalf("Import = %v, want warnings\n%s", err, out.String())
	}
	accounts := loadAccounts(t)
	if len(accounts) != 2 {
		t.Fatalf("accounts after import: %+v", accounts)
	}
	wantPending := []config.PendingRepo{{Path: "old", URL: "git@github.com:oito2/old.git"}, {Path: "web", URL: "git@github.com:oito2/web.git"}}
	if !reflect.DeepEqual(accounts[0].Pending, wantPending) || accounts[0].Name != "Minha" || accounts[0].Folder != existing.Folder {
		t.Errorf("merged account = %+v", accounts[0])
	}
	kadu := accounts[1]
	if kadu.ID != "kadu" || kadu.Folder != kaduFolder || !kadu.KeyGenerated || len(kadu.Pending) != 2 {
		t.Errorf("new account = %+v", kadu)
	}
	if err := CheckKey(kadu.SSHKey); err != nil {
		t.Errorf("the new account's key: %v", err)
	}
	pub, _ := PublicKey(kadu.SSHKey)
	if !strings.Contains(out.String(), pub) {
		t.Errorf("the output does not show the public key:\n%s", out.String())
	}
	if out, err := quietExecutor().Output(t.Context(), executor.Options{}, "git", "config", "--global", "--get-regexp", "includeif"); err != nil || !strings.Contains(out, kaduFolder) {
		t.Errorf("includeIf of the new account: %q, %v", out, err)
	}

	// A second import merges into both accounts and adds nothing new.
	if err := Import(t.Context(), quietExecutor(), &out, file, picks[:2], folders); !errors.Is(err, executor.ErrCompletedWithWarnings) {
		t.Fatalf("second Import = %v", err)
	}
	again := loadAccounts(t)
	if !reflect.DeepEqual(again[0].Pending, accounts[0].Pending) || !reflect.DeepEqual(again[1], kadu) {
		t.Errorf("a second import changed the accounts: %+v", again)
	}

	if err := Import(t.Context(), quietExecutor(), &out, file, nil, nil); err == nil {
		t.Error("Import without picks succeeded")
	}
}

func TestImport_ReusesExistingKey(t *testing.T) {
	requireTool(t, "git")
	requireTool(t, "ssh-keygen")
	home := isolate(t)
	key, err := DefaultKeyPath("oito2")
	if err != nil {
		t.Fatal(err)
	}
	if err := GenerateKey(t.Context(), quietExecutor(), Askpass{}, key, "old", ""); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(key)
	file := sampleFile(filepath.Join(home, "repos", "oito2"))
	var out bytes.Buffer
	if err := Import(t.Context(), quietExecutor(), &out, file, []TransferPick{{Account: "OITO2"}}, nil); err != nil {
		t.Fatalf("Import: %v\n%s", err, out.String())
	}
	after, _ := os.ReadFile(key)
	acc := loadAccounts(t)[0]
	if !bytes.Equal(before, after) || acc.SSHKey != key || acc.KeyGenerated || len(acc.Pending) != 0 {
		t.Errorf("account = %+v, key replaced: %v", acc, !bytes.Equal(before, after))
	}
}

func TestClonePending(t *testing.T) {
	_, bare, acc := syncFixture(t)
	server := fakeSSHServer(t)
	if err := os.MkdirAll(filepath.Join(server, "oito2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(bare, filepath.Join(server, "oito2", "perci.git")); err != nil {
		t.Fatal(err)
	}
	acc.Pending = []config.PendingRepo{
		{Path: "labs/perci", URL: "git@github.com:oito2/perci.git"},
		{Path: "apps/missing", URL: "git@github.com:oito2/missing.git"},
		{Path: "bad", URL: "https://github.com/oito2/bad.git"},
	}
	saveAccounts(t, acc)

	listed, err := ListRepos(t.Context(), quietExecutor(), acc)
	if err != nil || len(listed.Pending) != 3 {
		t.Fatalf("pending before the clone: %+v, %v", listed.Pending, err)
	}

	var out bytes.Buffer
	if err := ClonePending(t.Context(), quietExecutor(), &out, acc); err == nil || !strings.Contains(err.Error(), "2 repositório") {
		t.Errorf("ClonePending = %v, want two failures\n%s", err, out.String())
	}
	if got := originURL(t, filepath.Join(acc.Folder, "labs", "perci")); got != "git@github.com:oito2/perci.git" {
		t.Errorf("cloned origin = %q", got)
	}
	acc = loadAccounts(t)[0]
	if len(acc.Pending) != 2 || acc.Pending[0].Path != "apps/missing" {
		t.Errorf("pending after the clone: %+v", acc.Pending)
	}
	if listed, _ := ListRepos(t.Context(), quietExecutor(), acc); len(listed.Pending) != 2 || len(listed.Repos) != 1 {
		t.Errorf("ListRepos after the clone: %+v", listed)
	}

	if err := DiscardPending(acc.ID, "apps/missing"); err != nil {
		t.Fatal(err)
	}
	if err := DiscardPending(acc.ID, "apps/missing"); err == nil {
		t.Error("discarding a repository that is not pending succeeded")
	}
	if p := loadAccounts(t)[0].Pending; len(p) != 1 || p[0].Path != "bad" {
		t.Errorf("pending after discard: %+v", p)
	}
	if err := ClonePending(t.Context(), quietExecutor(), &out, config.GitAccount{}); err == nil {
		t.Error("ClonePending without pending repositories succeeded")
	}
}

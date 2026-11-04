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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/manager/gitaccount"
)

// isolateGitAccounts points HOME, the XDG folders and the global git
// configuration at a temporary folder, disconnects the SSH agent and puts
// fake ssh-add/secret-tool first on PATH, so no test reaches the user's
// real files, agent or keyring. secret-tool keeps one file per account in
// the returned keyring folder ("fail" there makes "store" fail). Real git
// and ssh-keygen are used; the test is skipped without them.
func isolateGitAccounts(t *testing.T) (home, keyring string) {
	t.Helper()
	for _, tool := range []string{"git", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available")
		}
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("SSH_AGENT_PID", "")

	keyring = t.TempDir()
	bin := t.TempDir()
	scripts := map[string]string{
		"ssh-add": "exit 2",
		"secret-tool": `store="` + keyring + `"
case "$1" in
store) [ -f "$store/fail" ] && exit 1; eval "acc=\${$#}"; cat > "$store/$acc";;
lookup) [ -f "$store/$5" ] || exit 1; cat "$store/$5";;
clear) [ -f "$store/$5" ] || exit 1; rm "$store/$5";;
esac`,
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return home, keyring
}

// approvedFolder creates home/rel and approves it, as the folder picker
// does.
func approvedFolder(t *testing.T, home, rel string) string {
	t.Helper()
	dir := filepath.Join(home, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	approved, _ := approvePath(dir, nil)
	return approved
}

func newReposService() *RepositoriesService {
	return &RepositoriesService{serviceBase: serviceBase{exe: executor.New(nil, nil)}}
}

func globalGitconfig(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func oito2Input(folder string) GitAccountInput {
	return GitAccountInput{
		Name: "OITO2", Provider: "github", Username: "oito2",
		GitName: "Oito Dois", GitEmail: "oito2@example.com",
		Folder: folder, KeyMode: keyModeGenerate,
	}
}

func TestGitAccounts_CreateEditRemove(t *testing.T) {
	home, keyring := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")

	in := oito2Input(folder)
	in.Token = "ghp_secret"
	v, err := svc.SaveGitAccount(ctx, in)
	if err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	key := filepath.Join(home, ".ssh", "perci_oito2")
	if v.ID != "oito2" || v.SSHKey != key || !v.KeyGenerated || v.KeyState != keyStateReady ||
		!strings.HasPrefix(v.PublicKey, "ssh-ed25519 ") || !v.PinnedHost || !v.HostTrusted || !v.HasToken ||
		v.KeysURL != "https://github.com/settings/keys" {
		t.Errorf("created account view: %+v", v)
	}
	if token, _ := os.ReadFile(filepath.Join(keyring, "oito2")); strings.TrimSpace(string(token)) != "ghp_secret" {
		t.Errorf("keyring holds %q", token)
	}
	gc := globalGitconfig(t, home)
	if !strings.Contains(gc, `[includeIf "gitdir:`+folder+`/"]`) {
		t.Errorf("~/.gitconfig has no includeIf for %s:\n%s", folder, gc)
	}
	cats := (&HomeService{}).GetCategories()
	if repos := cats[4]; repos.Label != "Repositórios" || len(repos.Items) != 3 || repos.Items[2].AccountID != "oito2" {
		t.Errorf("Repositórios category: %+v", cats[4])
	}

	// Edit: renamed, moved to another folder, same key; the ID stays.
	other := approvedFolder(t, home, "repos/oito2-labs")
	edit := oito2Input(other)
	edit.ID, edit.Name, edit.KeyMode = "oito2", "OITO2 Labs", keyModeKeep
	v, err = svc.SaveGitAccount(ctx, edit)
	if err != nil {
		t.Fatalf("SaveGitAccount (edit): %v", err)
	}
	if v.ID != "oito2" || v.Name != "OITO2 Labs" || v.SSHKey != key || !v.KeyGenerated || !v.HasToken {
		t.Errorf("edited account view: %+v", v)
	}
	gc = globalGitconfig(t, home)
	if strings.Contains(gc, "gitdir:"+folder+"/") || !strings.Contains(gc, "gitdir:"+other+"/") {
		t.Errorf("includeIf not moved to the new folder:\n%s", gc)
	}

	if err := svc.RemoveGitAccount(ctx, "oito2"); err != nil {
		t.Fatalf("RemoveGitAccount: %v", err)
	}
	if gc := globalGitconfig(t, home); strings.Contains(gc, "includeIf") {
		t.Errorf("includeIf left after removal:\n%s", gc)
	}
	if _, err := os.Stat(key); err != nil {
		t.Errorf("the SSH key must stay on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(keyring, "oito2")); !os.IsNotExist(err) {
		t.Errorf("the token must leave the keyring: %v", err)
	}
	if list, err := svc.ListGitAccounts(ctx); err != nil || len(list) != 0 {
		t.Errorf("ListGitAccounts after removal = %+v, %v", list, err)
	}
}

func TestGitAccounts_SaveRefusals(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()

	notPicked := filepath.Join(home, "repos", "typed")
	if err := os.MkdirAll(notPicked, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveGitAccount(ctx, oito2Input(notPicked)); err == nil {
		t.Error("saved an account whose folder was not picked")
	}

	folder := approvedFolder(t, home, "repos/oito2")
	keep := oito2Input(folder)
	keep.KeyMode = keyModeKeep
	if _, err := svc.SaveGitAccount(ctx, keep); err == nil {
		t.Error("saved a new account without a key")
	}
	existing := oito2Input(folder)
	existing.KeyMode, existing.KeyPath = keyModeExisting, filepath.Join(home, ".ssh", "id_ed25519")
	if _, err := svc.SaveGitAccount(ctx, existing); err == nil {
		t.Error("saved an account with a key that was not picked")
	}

	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	inside := approvedFolder(t, home, "repos/oito2/sub")
	clash := oito2Input(inside)
	clash.Name = "Outra"
	if _, err := svc.SaveGitAccount(ctx, clash); err == nil {
		t.Error("saved an account inside another account's folder")
	}
	if _, err := svc.SaveGitAccount(ctx, GitAccountInput{ID: "missing", KeyMode: keyModeKeep}); err == nil {
		t.Error("edited an account that does not exist")
	}
}

// A key generated by a save that fails afterwards is removed, so the next
// attempt can generate it again.
func TestGitAccounts_FailedSaveRemovesGeneratedKey(t *testing.T) {
	home, keyring := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")
	if err := os.WriteFile(filepath.Join(keyring, "fail"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	in := oito2Input(folder)
	in.Token = "ghp_secret"
	if _, err := svc.SaveGitAccount(ctx, in); err == nil {
		t.Fatal("SaveGitAccount succeeded with a failing keyring")
	}
	key := filepath.Join(home, ".ssh", "perci_oito2")
	for _, p := range []string{key, key + ".pub"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s left behind: %v", p, err)
		}
	}
	if cfg, err := config.Load(); err != nil || len(cfg.GitAccounts) != 0 {
		t.Errorf("config after a failed save: %+v, %v", cfg, err)
	}

	if err := os.Remove(filepath.Join(keyring, "fail")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveGitAccount(ctx, in); err != nil {
		t.Errorf("retry: %v", err)
	}
}

// A self-hosted server is untrusted until its scanned keys are confirmed,
// and only keys of a scan can be confirmed.
func TestGitAccounts_SelfHostedServerKeys(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()
	in := oito2Input(approvedFolder(t, home, "repos/oito2"))
	in.Provider, in.Host, in.SSHPort = "gitlab", "git.example.com", 2222
	v, err := svc.SaveGitAccount(ctx, in)
	if err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	if v.PinnedHost || v.HostTrusted || v.SSHPort != 2222 || v.KeysURL != "https://git.example.com/-/user_settings/ssh_keys" {
		t.Errorf("self-hosted view: %+v", v)
	}
	if err := svc.TrustGitAccountHostKeys("oito2"); err == nil {
		t.Error("trusted host keys without a scan")
	}
	if _, err := svc.TestGitAccountConnection(ctx, "oito2"); err == nil || !strings.Contains(err.Error(), "confirme a chave do servidor") {
		t.Errorf("TestGitAccountConnection with an untrusted server: %v", err)
	}
}

func TestSidebarCategories_RepositoriesAfterDocker(t *testing.T) {
	cats := sidebarCategories([]config.GitAccount{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}})
	if len(cats) != len(categories)+1 {
		t.Fatalf("got %d categories, want %d", len(cats), len(categories)+1)
	}
	var labels []string
	for _, c := range cats {
		labels = append(labels, c.Label)
	}
	if got := strings.Join(labels, ","); got != "Home,Linux,Desenvolvimento,Docker,Repositórios,Dev Tools" {
		t.Errorf("categories: %s", got)
	}
	items := cats[4].Items
	if len(items) != 4 || items[0].ActionID != "git-accounts" || items[1].ActionID != "git-transfer" ||
		items[2].ActionID != "git-account" || items[2].AccountID != "a" || items[3].Title != "B" {
		t.Errorf("Repositórios items: %+v", items)
	}
	if got := sidebarCategories(nil)[4].Items; len(got) != 2 {
		t.Errorf("without accounts: %+v", got)
	}
}

func TestGitAccounts_ListRepos(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")
	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	for _, args := range [][]string{
		{"init", "-q", filepath.Join(folder, "labs", "perci")},
		{"-C", filepath.Join(folder, "labs", "perci"), "remote", "add", "origin", "git@github.com:oito2/perci.git"},
		{"init", "-q", filepath.Join(folder, "third")},
		{"-C", filepath.Join(folder, "third"), "remote", "add", "origin", "git@github.com:someone/lib.git"},
	} {
		if err := svc.exe.Run(ctx, executor.Options{}, "git", args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	res, err := svc.ListGitAccountRepos(ctx, "oito2")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Repos) != 1 || res.Repos[0].Path != "labs/perci" || res.Repos[0].RemoteClass != "ok" ||
		len(res.Hidden) != 1 || res.Hidden[0].Path != "third" {
		t.Errorf("ListGitAccountRepos = %+v", res)
	}
	if _, err := svc.ListGitAccountRepos(ctx, "missing"); err == nil {
		t.Error("listed the repositories of an unknown account")
	}
}

// Verificar/Baixar refuse at once an account that can't reach its server,
// and run only over the account's listed repositories.
func TestGitAccounts_VerifyAndPullGuards(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	b, rec := newTestBase(t)
	b.exe = executor.New(nil, nil)
	svc := &RepositoriesService{serviceBase: *b}
	ctx := t.Context()

	self := oito2Input(approvedFolder(t, home, "repos/self"))
	self.Name, self.Host = "Self", "git.example.com"
	if _, err := svc.SaveGitAccount(ctx, self); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	if err := svc.VerifyGitAccountRepos("self", ""); err == nil || !strings.Contains(err.Error(), "confirme a chave do servidor") {
		t.Errorf("VerifyGitAccountRepos with an untrusted server = %v", err)
	}
	if err := svc.PullGitAccountRepos("missing", ""); err == nil {
		t.Error("PullGitAccountRepos of an unknown account started")
	}

	folder := approvedFolder(t, home, "repos/oito2")
	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	if err := svc.VerifyGitAccountRepos("oito2", filepath.Join(home, "repos", "self")); err != nil {
		t.Fatalf("VerifyGitAccountRepos: %v", err)
	}
	logs, done := rec.wait(t)
	if done["ok"] != false || !strings.Contains(done["error"].(string), "não é um repositório listado") {
		t.Errorf("action-done = %v (logs %q)", done, logs)
	}
}

func TestGitAccounts_RepoActions(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	b, rec := newTestBase(t)
	b.exe = executor.New(nil, nil)
	svc := &RepositoriesService{serviceBase: *b}
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")
	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	draft := filepath.Join(folder, "draft")
	for _, args := range [][]string{
		{"init", "-q", draft},
		{"-C", draft, "config", "user.email", "other@example.com"},
	} {
		if err := svc.exe.Run(ctx, executor.Options{}, "git", args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	if _, err := svc.SetGitAccountRepoRemote(ctx, "oito2", "", "draft"); err == nil {
		t.Error("SetGitAccountRepoRemote without a repository succeeded")
	}
	if _, err := svc.SetGitAccountRepoRemote(ctx, "oito2", home, "draft"); err == nil {
		t.Error("SetGitAccountRepoRemote outside the account succeeded")
	}
	url, err := svc.SetGitAccountRepoRemote(ctx, "oito2", draft, "draft")
	if err != nil || url != "git@github.com:oito2/draft.git" {
		t.Errorf("SetGitAccountRepoRemote = %q, %v", url, err)
	}
	if err := svc.ResetGitAccountRepoIdentity(ctx, "oito2", draft); err != nil {
		t.Errorf("ResetGitAccountRepoIdentity: %v", err)
	}
	res, err := svc.ListGitAccountRepos(ctx, "oito2")
	if err != nil || len(res.Repos) != 1 || res.Repos[0].RemoteClass != "ok" || res.Repos[0].IdentityDiverges {
		t.Errorf("after the actions: %+v, %v", res, err)
	}

	if err := svc.GenerateGitAccountRepoGitignore("oito2", draft); err != nil {
		t.Fatalf("GenerateGitAccountRepoGitignore: %v", err)
	}
	if _, done := rec.wait(t); done["ok"] != true {
		t.Errorf("gitignore action-done = %v", done)
	}
	if _, err := os.Stat(filepath.Join(draft, ".gitignore")); err != nil {
		t.Errorf(".gitignore not created: %v", err)
	}
}

func TestGitAccounts_NewRepository(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")
	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}

	dest, err := svc.InitGitAccountRepo(ctx, "oito2", "apps/draft", "draft")
	if err != nil || dest != filepath.Join(folder, "apps", "draft") {
		t.Fatalf("InitGitAccountRepo = %q, %v", dest, err)
	}
	if _, err := svc.InitGitAccountRepo(ctx, "oito2", "apps/draft", ""); err == nil {
		t.Error("InitGitAccountRepo into an existing repository succeeded")
	}
	if _, err := svc.InitGitAccountRepo(ctx, "oito2", "../escape", ""); err == nil {
		t.Error("InitGitAccountRepo outside the account folder succeeded")
	}
	res, err := svc.ListGitAccountRepos(ctx, "oito2")
	if err != nil || len(res.Repos) != 1 || res.Repos[0].Remote != "git@github.com:oito2/draft.git" {
		t.Errorf("after InitGitAccountRepo: %+v, %v", res, err)
	}

	// Refused before any action starts: no network is reached.
	if _, err := svc.CheckGitAccountClone(ctx, "oito2", "git@github.com:someone/lib.git", "apps/lib"); err == nil {
		t.Error("CheckGitAccountClone accepted a foreign repository")
	}
	if dest, err := svc.CheckGitAccountClone(ctx, "oito2", "perci", "labs/perci"); err != nil || dest != filepath.Join(folder, "labs", "perci") {
		t.Errorf("CheckGitAccountClone = %q, %v", dest, err)
	}
	for _, c := range [][2]string{{"git@github.com:someone/lib.git", "apps/lib"}, {"perci", "apps/draft"}, {"perci", ""}} {
		if err := svc.CloneGitAccountRepo("oito2", c[0], c[1]); err == nil {
			t.Errorf("CloneGitAccountRepo(%q, %q) started", c[0], c[1])
		}
	}
}

// Gerenciar Repositórios: export, preview with a picked folder, import
// into a new account, and the pending repositories' guards.
func TestGitAccounts_Transfer(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	b, rec := newTestBase(t)
	b.exe = executor.New(nil, nil)
	svc := &RepositoriesService{serviceBase: *b}
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")
	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	perci := filepath.Join(folder, "labs", "perci")
	for _, args := range [][]string{
		{"init", "-q", perci},
		{"-C", perci, "remote", "add", "origin", "https://oito2:ghp_secret@github.com/oito2/perci.git"},
		{"init", "-q", filepath.Join(folder, "draft")},
	} {
		if err := svc.exe.Run(ctx, executor.Options{}, "git", args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	items, err := svc.GetGitRepoExportItems(ctx)
	if err != nil || len(items.Accounts) != 1 || len(items.Accounts[0].Repos) != 1 ||
		items.Accounts[0].Repos[0].URL != "git@github.com:oito2/perci.git" || len(items.Skipped) != 1 {
		t.Fatalf("GetGitRepoExportItems = %+v, %v", items, err)
	}
	picks := []gitaccount.TransferPick{{Account: "OITO2", Repos: []string{"labs/perci"}}}
	file := filepath.Join(home, "repos.yaml")
	if err := svc.ExportGitRepos(ctx, file, picks); err == nil {
		t.Error("ExportGitRepos to a path not picked in the dialog succeeded")
	}
	approved, _ := approvePath(file, nil)
	if err := svc.ExportGitRepos(ctx, approved, picks); err != nil {
		t.Fatalf("ExportGitRepos: %v", err)
	}

	if err := svc.RemoveGitAccount(ctx, "oito2"); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(home, "new", "oito2")
	if _, err := svc.PreviewGitRepoImport(ctx, approved, map[string]string{"OITO2": other}); err == nil {
		t.Error("PreviewGitRepoImport with a folder not picked in the dialog succeeded")
	}
	picked := approvedFolder(t, home, "new/oito2")
	folders := map[string]string{"OITO2": picked}
	plans, err := svc.PreviewGitRepoImport(ctx, approved, folders)
	if err != nil || len(plans) != 1 || plans[0].Folder != picked || plans[0].Reason != "" || plans[0].States[0].State != gitaccount.ImportNew {
		t.Fatalf("PreviewGitRepoImport = %+v, %v", plans, err)
	}
	if err := svc.ImportGitRepos(approved, picks, folders); err != nil {
		t.Fatalf("ImportGitRepos: %v", err)
	}
	if logs, done := rec.wait(t); done["ok"] != true {
		t.Fatalf("import action-done = %v (logs %q)", done, logs)
	}
	res, err := svc.ListGitAccountRepos(ctx, "oito2")
	if err != nil || len(res.Pending) != 1 || res.Pending[0].Path != "labs/perci" {
		t.Fatalf("pending after the import: %+v, %v", res, err)
	}

	// The connection test runs before any clone: a fake ssh refuses the key.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte("#!/bin/sh\necho 'git@github.com: Permission denied (publickey).' >&2\nexit 255\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := svc.CloneGitAccountPending("oito2"); err == nil || !strings.Contains(err.Error(), "cadastre a chave pública") {
		t.Errorf("CloneGitAccountPending with a refused key = %v", err)
	}
	if err := svc.DiscardGitAccountPending("oito2", "labs/perci"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloneGitAccountPending("oito2"); err == nil || !strings.Contains(err.Error(), "nenhum repositório pendente") {
		t.Errorf("CloneGitAccountPending without pending repositories = %v", err)
	}
}

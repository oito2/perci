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

// Bound methods for the "Repositórios" category — a thin layer over
// internal/manager/gitaccount.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/manager/gitaccount"
	managergitignore "github.com/oito2/perci/internal/manager/gitignore"
	managerrepo "github.com/oito2/perci/internal/manager/repo"
)

// RepositoriesService binds the "Repositórios" category: Configurar Conta
// and the per-account screens.
type RepositoriesService struct {
	serviceBase
	// askpass answers the passphrase prompts of ssh-keygen and ssh-add.
	askpass gitaccount.Askpass

	// scanned holds, per account ID, the host keys of the last
	// ScanGitAccountHostKeys — the only keys TrustGitAccountHostKeys
	// accepts. Guarded by mu.
	mu      sync.Mutex
	scanned map[string][]gitaccount.HostKey
}

// Key states reported by GitAccountView.KeyState.
const (
	keyStateMissing  = "missing"  // the key file or its .pub is gone
	keyStateReady    = "ready"    // no passphrase
	keyStateUnlocked = "unlocked" // passphrase, and loaded in the SSH agent
	keyStateLocked   = "locked"   // passphrase, not in the SSH agent
	keyStateNoAgent  = "no-agent" // passphrase, and no SSH agent running
)

// GitAccountView is one account as the Configurar Conta screen shows it:
// the stored fields plus the state of its key and host.
type GitAccountView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Host         string `json:"host"`
	SSHPort      int    `json:"sshPort"`
	Username     string `json:"username"`
	GitName      string `json:"gitName"`
	GitEmail     string `json:"gitEmail"`
	Folder       string `json:"folder"`
	SSHKey       string `json:"sshKey"`
	KeyGenerated bool   `json:"keyGenerated"`
	HasToken     bool   `json:"hasToken"`
	KeyState     string `json:"keyState"`
	PublicKey    string `json:"publicKey"`
	PinnedHost   bool   `json:"pinnedHost"`
	HostTrusted  bool   `json:"hostTrusted"`
	// KeysURL is the provider page where the public key is registered.
	KeysURL string `json:"keysUrl"`
}

// GitAccountInput is the Configurar Conta form. ID is empty for a new
// account. KeyMode is "generate" (new key at ~/.ssh/perci_<id>, with the
// optional Passphrase), "existing" (KeyPath, picked in the dialog) or ""
// to keep the current key of an existing account. A non-empty Token is
// stored in the keyring; ClearToken removes the stored one.
type GitAccountInput struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Host       string `json:"host"`
	SSHPort    int    `json:"sshPort"`
	Username   string `json:"username"`
	GitName    string `json:"gitName"`
	GitEmail   string `json:"gitEmail"`
	Folder     string `json:"folder"`
	KeyMode    string `json:"keyMode"`
	KeyPath    string `json:"keyPath"`
	Passphrase string `json:"passphrase"`
	Token      string `json:"token"`
	ClearToken bool   `json:"clearToken"`
}

// Key modes of GitAccountInput.
const (
	keyModeKeep     = ""
	keyModeGenerate = "generate"
	keyModeExisting = "existing"
)

// ListGitAccounts returns every registered account with its key and host
// state.
func (s *RepositoriesService) ListGitAccounts(ctx context.Context) ([]GitAccountView, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	views := make([]GitAccountView, 0, len(cfg.GitAccounts))
	for _, acc := range cfg.GitAccounts {
		views = append(views, s.accountView(ctx, acc))
	}
	return views, nil
}

// GetGitAccount returns the account id with its key and host state.
func (s *RepositoriesService) GetGitAccount(ctx context.Context, id string) (GitAccountView, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return GitAccountView{}, err
	}
	return s.accountView(ctx, acc), nil
}

// accountView adds the key and host state to acc.
func (s *RepositoriesService) accountView(ctx context.Context, acc config.GitAccount) GitAccountView {
	v := GitAccountView{
		ID: acc.ID, Name: acc.Name, Provider: acc.Provider, Host: acc.Host,
		SSHPort: acc.SSHPort, Username: acc.Username, GitName: acc.GitName,
		GitEmail: acc.GitEmail, Folder: acc.Folder, SSHKey: acc.SSHKey,
		KeyGenerated: acc.KeyGenerated, HasToken: acc.HasToken,
		PinnedHost: gitaccount.IsPinnedHost(acc),
		KeysURL:    gitaccount.KeysURL(acc),
	}
	v.HostTrusted, _ = gitaccount.HostTrusted(acc)
	v.KeyState = s.keyState(ctx, acc.SSHKey)
	if v.KeyState != keyStateMissing {
		v.PublicKey, _ = gitaccount.PublicKey(acc.SSHKey)
	}
	return v
}

// keyState classifies the key at path (see the keyState* constants).
func (s *RepositoriesService) keyState(ctx context.Context, path string) string {
	if gitaccount.CheckKey(path) != nil {
		return keyStateMissing
	}
	if !gitaccount.KeyEncrypted(ctx, s.exe, path) {
		return keyStateReady
	}
	loaded, err := gitaccount.AgentHasKey(ctx, s.exe, path)
	switch {
	case errors.Is(err, gitaccount.ErrNoAgent):
		return keyStateNoAgent
	case loaded:
		return keyStateUnlocked
	}
	return keyStateLocked
}

// findGitAccount returns the registered account id.
func findGitAccount(id string) (config.GitAccount, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.GitAccount{}, err
	}
	for _, acc := range cfg.GitAccounts {
		if acc.ID == id {
			return acc, nil
		}
	}
	return config.GitAccount{}, fmt.Errorf("conta não encontrada: %q", id)
}

// PickGitAccountFolder opens the native folder picker for an account's
// root folder.
func (s *RepositoriesService) PickGitAccountFolder() (string, error) {
	return s.pickFolder("Selecionar pasta da conta")
}

// PickGitAccountKey opens the native file picker, in ~/.ssh, for an
// existing private key.
func (s *RepositoriesService) PickGitAccountKey() (string, error) {
	dir := ""
	if home, err := os.UserHomeDir(); err == nil {
		dir = filepath.Join(home, ".ssh")
	}
	return approvePath(s.wailsApp.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:           "Selecionar chave SSH privada",
		CanChooseFiles:  true,
		ShowHiddenFiles: true,
		Directory:       dir,
	}).PromptForSingleSelection())
}

// SaveGitAccount creates the account (in.ID empty) or updates the account
// in.ID: validates the form, sets up the SSH key, trusts the built-in host
// keys of GitHub/GitLab, stores or removes the token, writes the account's
// gitconfig and includeIf rule and saves config.yaml. Holds the action
// lock, so it never overlaps a running action.
func (s *RepositoriesService) SaveGitAccount(ctx context.Context, in GitAccountInput) (GitAccountView, error) {
	var view GitAccountView
	err := runExclusive(func() error {
		acc, err := s.saveGitAccount(ctx, in)
		if err != nil {
			return err
		}
		view = s.accountView(ctx, acc)
		return nil
	})
	return view, err
}

func (s *RepositoriesService) saveGitAccount(ctx context.Context, in GitAccountInput) (config.GitAccount, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.GitAccount{}, err
	}
	var prev *config.GitAccount
	others := make([]config.GitAccount, 0, len(cfg.GitAccounts))
	for i := range cfg.GitAccounts {
		if in.ID != "" && cfg.GitAccounts[i].ID == in.ID {
			prev = &cfg.GitAccounts[i]
			continue
		}
		others = append(others, cfg.GitAccounts[i])
	}
	if in.ID != "" && prev == nil {
		return config.GitAccount{}, fmt.Errorf("conta não encontrada: %q", in.ID)
	}

	acc := config.GitAccount{
		ID: in.ID, Name: in.Name, Provider: in.Provider, Host: in.Host,
		SSHPort: in.SSHPort, Username: in.Username, GitName: in.GitName,
		GitEmail: in.GitEmail, Folder: in.Folder,
	}
	if prev != nil {
		acc.SSHKey, acc.KeyGenerated = prev.SSHKey, prev.KeyGenerated
		acc.HasToken, acc.Pending = prev.HasToken, prev.Pending
	}
	acc = gitaccount.Normalize(acc)
	if acc.SSHPort == 22 {
		acc.SSHPort = 0
	}

	// A new folder must come from the picker; the current one is kept.
	if prev == nil || filepath.Clean(prev.Folder) != acc.Folder {
		if err := requireApprovedPath(acc.Folder); err != nil {
			return config.GitAccount{}, errors.New("escolha a pasta da conta no seletor")
		}
	}
	if err := gitaccount.Validate(acc, others); err != nil {
		return config.GitAccount{}, err
	}

	generated, err := s.setupKey(ctx, &acc, prev, in)
	if err == nil {
		err = s.finishSave(ctx, acc, prev, in)
	}
	if err != nil {
		// A key generated by this call is never used once the save fails:
		// removed, so the next attempt can generate it again.
		if generated {
			_ = os.Remove(acc.SSHKey)
			_ = os.Remove(acc.SSHKey + ".pub")
		}
		return config.GitAccount{}, err
	}
	if generated && in.Passphrase != "" {
		// Best effort: without an agent the key stays locked and the
		// screen offers to unlock it.
		_ = gitaccount.UnlockKey(ctx, s.exe, s.askpass, acc.SSHKey, in.Passphrase)
	}
	return findGitAccount(acc.ID)
}

// finishSave runs the steps of SaveGitAccount after the key is set up:
// built-in host keys, token, gitconfig + includeIf rule, config.yaml.
func (s *RepositoriesService) finishSave(ctx context.Context, acc config.GitAccount, prev *config.GitAccount, in GitAccountInput) error {
	if err := gitaccount.TrustPinnedHostKeys(acc); err != nil {
		return err
	}
	if err := s.setupToken(ctx, &acc, in); err != nil {
		return err
	}
	if err := gitaccount.Apply(ctx, s.exe, io.Discard, prev, acc); err != nil {
		return err
	}
	return config.Update(func(c *config.Config) error {
		for i := range c.GitAccounts {
			if c.GitAccounts[i].ID == acc.ID {
				c.GitAccounts[i] = acc
				return nil
			}
		}
		c.GitAccounts = append(c.GitAccounts, acc)
		return nil
	})
}

// setupKey applies in.KeyMode to acc: generates a new key, takes a picked
// key, or keeps the current one. It reports whether it generated the key.
func (s *RepositoriesService) setupKey(ctx context.Context, acc *config.GitAccount, prev *config.GitAccount, in GitAccountInput) (bool, error) {
	generated := false
	switch in.KeyMode {
	case keyModeGenerate:
		path, err := gitaccount.DefaultKeyPath(acc.ID)
		if err != nil {
			return false, err
		}
		if err := gitaccount.GenerateKey(ctx, s.exe, s.askpass, path, acc.GitEmail, in.Passphrase); err != nil {
			return false, err
		}
		acc.SSHKey, acc.KeyGenerated = path, true
		generated = true
	case keyModeExisting:
		path := filepath.Clean(in.KeyPath)
		if prev == nil || path != prev.SSHKey {
			if err := requireApprovedPath(in.KeyPath); err != nil {
				return false, errors.New("escolha a chave SSH no seletor")
			}
		}
		acc.SSHKey, acc.KeyGenerated = path, prev != nil && path == prev.SSHKey && prev.KeyGenerated
	case keyModeKeep:
		if prev == nil {
			return false, errors.New("gere uma chave SSH ou escolha uma existente")
		}
	default:
		return false, fmt.Errorf("modo de chave desconhecido: %q", in.KeyMode)
	}
	return generated, gitaccount.CheckKey(acc.SSHKey)
}

// setupToken stores in.Token in the keyring or, with in.ClearToken,
// removes the stored one, and records on acc whether one is stored.
func (s *RepositoriesService) setupToken(ctx context.Context, acc *config.GitAccount, in GitAccountInput) error {
	switch {
	case in.Token != "":
		if err := gitaccount.StoreToken(ctx, s.exe, *acc, in.Token); err != nil {
			return err
		}
		acc.HasToken = true
	case in.ClearToken && acc.HasToken:
		if err := gitaccount.DeleteToken(ctx, s.exe, acc.ID); err != nil {
			return err
		}
		acc.HasToken = false
	}
	return nil
}

// RemoveGitAccount forgets the account id: removes its includeIf rule,
// gitconfig file and token. Its repositories and SSH key stay on disk.
func (s *RepositoriesService) RemoveGitAccount(ctx context.Context, id string) error {
	return runExclusive(func() error {
		acc, err := findGitAccount(id)
		if err != nil {
			return err
		}
		if err := gitaccount.Remove(ctx, s.exe, io.Discard, acc); err != nil {
			return err
		}
		if acc.HasToken {
			if err := gitaccount.DeleteToken(ctx, s.exe, acc.ID); err != nil {
				return fmt.Errorf("remover o token do chaveiro: %w", err)
			}
		}
		return config.Update(func(c *config.Config) error {
			kept := c.GitAccounts[:0]
			for _, a := range c.GitAccounts {
				if a.ID != id {
					kept = append(kept, a)
				}
			}
			c.GitAccounts = kept
			return nil
		})
	})
}

// ScanGitAccountHostKeys asks the server of account id for its host keys,
// for the user to confirm with TrustGitAccountHostKeys.
func (s *RepositoriesService) ScanGitAccountHostKeys(ctx context.Context, id string) ([]gitaccount.HostKey, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return nil, err
	}
	keys, err := gitaccount.ScanHostKeys(ctx, s.exe, acc)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.scanned == nil {
		s.scanned = map[string][]gitaccount.HostKey{}
	}
	s.scanned[id] = keys
	s.mu.Unlock()
	return keys, nil
}

// TrustGitAccountHostKeys trusts the host keys of the last
// ScanGitAccountHostKeys of account id.
func (s *RepositoriesService) TrustGitAccountHostKeys(id string) error {
	acc, err := findGitAccount(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	keys := s.scanned[id]
	delete(s.scanned, id)
	s.mu.Unlock()
	if len(keys) == 0 {
		return errors.New("verifique a chave do servidor antes de confirmar")
	}
	return gitaccount.TrustHostKeys(acc, keys)
}

// UnlockGitAccountKey loads the key of account id into the session's SSH
// agent with passphrase.
func (s *RepositoriesService) UnlockGitAccountKey(ctx context.Context, id, passphrase string) error {
	acc, err := findGitAccount(id)
	if err != nil {
		return err
	}
	if passphrase == "" {
		return errors.New("informe a senha da chave")
	}
	return gitaccount.UnlockKey(ctx, s.exe, s.askpass, acc.SSHKey, passphrase)
}

// TestGitAccountConnection authenticates to the server of account id and
// returns the user name it greets.
func (s *RepositoriesService) TestGitAccountConnection(ctx context.Context, id string) (string, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return "", err
	}
	if err := s.requireReady(ctx, acc); err != nil {
		return "", err
	}
	return gitaccount.TestConnection(ctx, s.exe, acc)
}

// requireReady fails when acc can't authenticate to its server: its
// server key is not confirmed, or its SSH key is missing or locked.
func (s *RepositoriesService) requireReady(ctx context.Context, acc config.GitAccount) error {
	if trusted, err := gitaccount.HostTrusted(acc); err != nil {
		return err
	} else if !trusted {
		return fmt.Errorf("confirme a chave do servidor %s em Configurar Conta", acc.Host)
	}
	switch s.keyState(ctx, acc.SSHKey) {
	case keyStateMissing:
		return fmt.Errorf("a chave %s não foi encontrada", acc.SSHKey)
	case keyStateLocked:
		return errors.New("a chave está bloqueada — desbloqueie-a em Configurar Conta")
	case keyStateNoAgent:
		return errors.New("a chave tem senha e não há agente SSH nesta sessão para desbloqueá-la")
	}
	return nil
}

// VerifyGitAccountRepos fetches the listed repositories of account id —
// all of them, or only the one at dir — on the Execução tab.
func (s *RepositoriesService) VerifyGitAccountRepos(id, dir string) error {
	return s.runRepoAction(id, dir, gitaccount.Fetch)
}

// PullGitAccountRepos brings the listed repositories of account id — all
// of them, or only the one at dir — up to date, fast-forward only, on the
// Execução tab.
func (s *RepositoriesService) PullGitAccountRepos(id, dir string) error {
	return s.runRepoAction(id, dir, gitaccount.Pull)
}

// repoAction is Fetch or Pull.
type repoAction func(ctx context.Context, exe *executor.Executor, stdout io.Writer, acc config.GitAccount, repos []gitaccount.RepoState) error

// runRepoAction checks that account id can reach its server, then runs
// action as a long-running action over the repositories SelectRepos
// picks for dir.
func (s *RepositoriesService) runRepoAction(id, dir string, action repoAction) error {
	ctx := context.Background()
	acc, err := findGitAccount(id)
	if err != nil {
		return err
	}
	if err := s.requireReady(ctx, acc); err != nil {
		return err
	}
	return s.runAction(func(stdout io.Writer) error {
		repos, err := gitaccount.SelectRepos(ctx, s.exe, acc, dir)
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			return errors.New("nenhum repositório para processar")
		}
		return action(ctx, s.exe, stdout, acc, repos)
	})
}

// ListGitAccountRepos lists the repositories under the folder of account
// id with their local state — no network access. Repositories of another
// owner or host come back in Hidden.
func (s *RepositoriesService) ListGitAccountRepos(ctx context.Context, id string) (gitaccount.AccountRepos, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return gitaccount.AccountRepos{}, err
	}
	return gitaccount.ListRepos(ctx, s.exe, acc)
}

// accountRepo returns account id and its listed repository at dir.
func (s *RepositoriesService) accountRepo(ctx context.Context, id, dir string) (config.GitAccount, gitaccount.RepoState, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return acc, gitaccount.RepoState{}, err
	}
	if dir == "" {
		return acc, gitaccount.RepoState{}, errors.New("nenhum repositório escolhido")
	}
	repos, err := gitaccount.SelectRepos(ctx, s.exe, acc, dir)
	if err != nil {
		return acc, gitaccount.RepoState{}, err
	}
	return acc, repos[0], nil
}

// FixGitAccountRepoURLs replaces the HTTP(S) origin of the repository at
// dir — or, with dir empty, of every listed repository that has one —
// with its SSH URL, on the Execução tab.
func (s *RepositoriesService) FixGitAccountRepoURLs(id, dir string) error {
	return s.runRepoAction(id, dir, gitaccount.FixURLs)
}

// SetGitAccountRepoRemote adds origin, the SSH URL of name under the
// account's user, to the repository at dir, which has none. Returns the
// URL.
func (s *RepositoriesService) SetGitAccountRepoRemote(ctx context.Context, id, dir, name string) (string, error) {
	var url string
	err := runExclusive(func() error {
		acc, st, err := s.accountRepo(ctx, id, dir)
		if err != nil {
			return err
		}
		url, err = gitaccount.SetRemote(ctx, s.exe, acc, st, name)
		return err
	})
	return url, err
}

// ResetGitAccountRepoIdentity removes the user.name/user.email the
// repository at dir sets for itself, so it uses the account's.
func (s *RepositoriesService) ResetGitAccountRepoIdentity(ctx context.Context, id, dir string) error {
	return runExclusive(func() error {
		_, st, err := s.accountRepo(ctx, id, dir)
		if err != nil {
			return err
		}
		return gitaccount.ResetIdentity(ctx, s.exe, st)
	})
}

// GenerateGitAccountRepoGitignore creates the .gitignore of the repository
// at dir, or merges the missing patterns into it, on the Execução tab.
func (s *RepositoriesService) GenerateGitAccountRepoGitignore(id, dir string) error {
	ctx := context.Background()
	_, st, err := s.accountRepo(ctx, id, dir)
	if err != nil {
		return err
	}
	return s.runAction(func(stdout io.Writer) error {
		_, err := managergitignore.Generate(stdout, st.Dir)
		return err
	})
}

// CreateGitAccountRepoConduct writes CODE_OF_CONDUCT.md and its Portuguese
// mirror into the repository at dir, replacing existing copies, with
// contactEmail for reports, on the Execução tab.
func (s *RepositoriesService) CreateGitAccountRepoConduct(id, dir, contactEmail string) error {
	ctx := context.Background()
	_, st, err := s.accountRepo(ctx, id, dir)
	if err != nil {
		return err
	}
	return s.runAction(func(stdout io.Writer) error {
		return managerrepo.CreateConduct(stdout, st.Dir, contactEmail, true)
	})
}

// RemoteRepoList is the account's repositories on its provider, for
// Novo Repositório.
type RemoteRepoList struct {
	Repos []gitaccount.RemoteRepo `json:"repos"`
	// WithToken says the list came with the account's token, so it
	// includes the private repositories.
	WithToken bool `json:"withToken"`
	// Note explains why a saved token could not be used.
	Note string `json:"note,omitempty"`
}

// ListGitAccountRemoteRepos lists the repositories account id owns on its
// provider, marking the ones already cloned under its folder.
func (s *RepositoriesService) ListGitAccountRemoteRepos(ctx context.Context, id string) (RemoteRepoList, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return RemoteRepoList{}, err
	}
	var list RemoteRepoList
	token := ""
	if acc.HasToken {
		token, err = gitaccount.LookupToken(ctx, s.exe, acc.ID)
		switch {
		case err != nil:
			list.Note = "o token salvo não pôde ser lido do chaveiro (" + err.Error() + "); só os repositórios públicos aparecem"
		case token == "":
			list.Note = "o token não está mais no chaveiro; só os repositórios públicos aparecem — salve-o de novo em Configurar Conta"
		}
	}
	list.Repos, err = gitaccount.ListRemoteRepos(ctx, gitaccount.APIBase(acc), acc, token)
	if err != nil {
		return RemoteRepoList{}, err
	}
	list.WithToken = token != ""
	if local, err := gitaccount.ListRepos(ctx, s.exe, acc); err == nil {
		gitaccount.MarkCloned(acc, list.Repos, local.Repos)
	}
	return list, nil
}

// CheckGitAccountClone validates a clone of source into rel without
// starting it — the same checks CloneGitAccountRepo makes, so the form can
// show them before the run. Returns the destination folder.
func (s *RepositoriesService) CheckGitAccountClone(ctx context.Context, id, source, rel string) (string, error) {
	acc, _, dest, err := s.cloneTarget(id, source, rel)
	if err != nil {
		return "", err
	}
	return dest, s.requireReady(ctx, acc)
}

// cloneTarget resolves account id, the SSH URL of source and the
// destination folder rel.
func (s *RepositoriesService) cloneTarget(id, source, rel string) (config.GitAccount, string, string, error) {
	acc, err := findGitAccount(id)
	if err != nil {
		return acc, "", "", err
	}
	url, err := gitaccount.CloneURL(acc, source)
	if err != nil {
		return acc, "", "", err
	}
	dest, err := gitaccount.DestDir(acc, rel)
	return acc, url, dest, err
}

// CloneGitAccountRepo clones source — a repository name or a URL of one of
// the account's repositories — into rel, a new folder relative to the
// account folder, on the Execução tab.
func (s *RepositoriesService) CloneGitAccountRepo(id, source, rel string) error {
	ctx := context.Background()
	acc, url, dest, err := s.cloneTarget(id, source, rel)
	if err != nil {
		return err
	}
	if err := s.requireReady(ctx, acc); err != nil {
		return err
	}
	return s.runAction(func(stdout io.Writer) error {
		return gitaccount.Clone(ctx, s.exe, stdout, acc, url, dest)
	})
}

// InitGitAccountRepo runs `git init` in rel, a new folder relative to the
// account folder, adding origin for remoteName when it is not empty.
// Returns the new repository's folder.
func (s *RepositoriesService) InitGitAccountRepo(ctx context.Context, id, rel, remoteName string) (string, error) {
	var dest string
	err := runExclusive(func() error {
		acc, err := findGitAccount(id)
		if err != nil {
			return err
		}
		if dest, err = gitaccount.DestDir(acc, rel); err != nil {
			return err
		}
		return gitaccount.Init(ctx, s.exe, acc, dest, remoteName)
	})
	return dest, err
}

// GitIdentity is a commit name and e-mail pair.
type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetGlobalGitIdentity reports the global git identity (empty fields mean
// it was never set) — pre-fills the "Identidade padrão" tab.
func (s *RepositoriesService) GetGlobalGitIdentity() GitIdentity {
	name, email := managerrepo.GetGlobalIdentity(context.Background(), s.exe)
	return GitIdentity{Name: name, Email: email}
}

// ApplyGlobalGitIdentity sets the global git user.name/user.email and, when
// none is configured yet, credential.helper (managerrepo.ConfigureGlobal),
// on the Execução tab.
func (s *RepositoriesService) ApplyGlobalGitIdentity(name, email string) error {
	return s.runAction(func(stdout io.Writer) error {
		return managerrepo.ConfigureGlobal(context.Background(), s.exe, stdout, name, email)
	})
}

// ── Gerenciar Repositórios ────────────────────────────────────────────────

// RepoExportItems is the Exportar checklist: every account with the
// repositories it can export, and the repositories left out because they
// have no remote ("<account>: <path>").
type RepoExportItems struct {
	Accounts []gitaccount.TransferAccount `json:"accounts"`
	Skipped  []string                     `json:"skipped"`
}

// GetGitRepoExportItems lists what the registered accounts can export.
func (s *RepositoriesService) GetGitRepoExportItems(ctx context.Context) (RepoExportItems, error) {
	cfg, err := config.Load()
	if err != nil {
		return RepoExportItems{}, err
	}
	file, skipped, err := gitaccount.Exportable(ctx, s.exe, cfg.GitAccounts)
	if err != nil {
		return RepoExportItems{}, err
	}
	return RepoExportItems{Accounts: file.Accounts, Skipped: skipped}, nil
}

// PickGitRepoExportPath opens the native "Salvar Como" dialog for the
// repositories export file.
func (s *RepositoriesService) PickGitRepoExportPath() (string, error) {
	return approvePath(s.wailsApp.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Exportar repositórios",
		Filename: "perci-repositorios.yaml",
		Filters:  []application.FileFilter{{DisplayName: "YAML (*.yaml)", Pattern: "*.yaml;*.yml"}},
	}).PromptForSingleSelection())
}

// ExportGitRepos writes the picked accounts and repositories to path (only
// a path chosen in PickGitRepoExportPath).
func (s *RepositoriesService) ExportGitRepos(ctx context.Context, path string, picks []gitaccount.TransferPick) error {
	if err := requireApprovedPath(path); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	file, _, err := gitaccount.Exportable(ctx, s.exe, cfg.GitAccounts)
	if err != nil {
		return err
	}
	part, err := file.Filter(picks)
	if err != nil {
		return err
	}
	return gitaccount.WriteTransfer(path, part)
}

// PickGitRepoImportPath opens the native "Selecionar arquivo" dialog for a
// repositories export file.
func (s *RepositoriesService) PickGitRepoImportPath() (string, error) {
	return approvePath(s.wailsApp.Dialog.OpenFile().
		SetTitle("Selecionar arquivo de repositórios").
		AddFilter("YAML (*.yaml)", "*.yaml;*.yml").
		PromptForSingleSelection())
}

// loadRepoImport reads the export file at path (only a path chosen in
// PickGitRepoImportPath) and checks folders, the account folders that
// replace the file's: each one must come from the folder picker.
func loadRepoImport(path string, folders map[string]string) (gitaccount.TransferFile, error) {
	if err := requireApprovedPath(path); err != nil {
		return gitaccount.TransferFile{}, err
	}
	file, err := gitaccount.LoadTransfer(path)
	if err != nil {
		return gitaccount.TransferFile{}, err
	}
	for name, folder := range folders {
		if err := requireApprovedPath(folder); err != nil {
			return gitaccount.TransferFile{}, fmt.Errorf("escolha a pasta da conta %s no seletor", name)
		}
	}
	return file, nil
}

// PreviewGitRepoImport reads the export file at path and checks each of its
// accounts and repositories against this machine, with folders replacing
// the folders of new accounts — without applying anything: the Importar
// checklist.
func (s *RepositoriesService) PreviewGitRepoImport(ctx context.Context, path string, folders map[string]string) ([]gitaccount.ImportAccount, error) {
	file, err := loadRepoImport(path, folders)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return gitaccount.PlanImport(ctx, s.exe, file, cfg.GitAccounts, folders), nil
}

// ImportGitRepos imports the picked accounts and repositories of the
// export file at path (gitaccount.Import), on the Execução tab: new
// accounts get a folder, an SSH key and their includeIf rule; the
// repositories become pending on each account's screen.
func (s *RepositoriesService) ImportGitRepos(path string, picks []gitaccount.TransferPick, folders map[string]string) error {
	return s.runAction(func(stdout io.Writer) error {
		file, err := loadRepoImport(path, folders)
		if err != nil {
			return err
		}
		return gitaccount.Import(context.Background(), s.exe, stdout, file, picks, folders)
	})
}

// CloneGitAccountPending clones the pending repositories of account id,
// on the Execução tab, once the account authenticates to its server.
func (s *RepositoriesService) CloneGitAccountPending(id string) error {
	ctx := context.Background()
	acc, err := findGitAccount(id)
	if err != nil {
		return err
	}
	if len(acc.Pending) == 0 {
		return errors.New("nenhum repositório pendente")
	}
	if err := s.requireReady(ctx, acc); err != nil {
		return err
	}
	if _, err := gitaccount.TestConnection(ctx, s.exe, acc); err != nil {
		return fmt.Errorf("teste de conexão falhou: %w", err)
	}
	return s.runAction(func(stdout io.Writer) error {
		return gitaccount.ClonePending(ctx, s.exe, stdout, acc)
	})
}

// DiscardGitAccountPending removes the pending repository at path from
// account id without cloning it.
func (s *RepositoriesService) DiscardGitAccountPending(id, path string) error {
	return runExclusive(func() error {
		return gitaccount.DiscardPending(id, path)
	})
}

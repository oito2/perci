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

// Bound methods for "Dev Tools :: Repositórios" — a thin layer over
// internal/manager/repo and internal/manager/gitignore.
// Init/CreateConduct/Generate take a `dir` parameter: "" means the
// shell's current folder; the GUI, which has no "current directory" of
// its own, passes whichever folder the user picked.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	managergitignore "github.com/oito2/perci/internal/manager/gitignore"
	managerrepo "github.com/oito2/perci/internal/manager/repo"
)

// DevToolsService binds the "Dev Tools" category: Repositórios, IA:
// Contextos, IA: SKILLs, IA: MCPs — one struct, with one group of
// methods per sub-screen.
type DevToolsService struct {
	serviceBase
}

// GitIdentity is a name+email pair — used for both a global and a local
// (per-folder) git identity.
type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetGlobalGitIdentity reports the currently configured global git identity
// (empty fields mean it was never set) — pre-fills "Identidade Global" and
// the "Nome"/"E-mail" defaults everywhere else in this screen.
func (t *DevToolsService) GetGlobalGitIdentity() GitIdentity {
	name, email := managerrepo.GetGlobalIdentity(context.Background(), t.exe)
	return GitIdentity{Name: name, Email: email}
}

// ApplyGlobalGitIdentity sets the global git user.name/user.email/
// credential.helper (managerrepo.ConfigureGlobal) — shows the Execução tab
// like every other mutating action in this screen.
func (t *DevToolsService) ApplyGlobalGitIdentity(name, email string) error {
	return t.runAction(func(stdout io.Writer) error {
		return managerrepo.ConfigureGlobal(context.Background(), t.exe, stdout, name, email)
	})
}

// PickRepoWorkingFolder opens the native folder picker for the
// "Repositórios" sub-tab's "pasta de trabalho" — CanCreateDirectories lets
// the user create a brand-new empty folder from inside the dialog (the
// common case for "clonar/iniciar algo novo aqui").
func (t *DevToolsService) PickRepoWorkingFolder() (string, error) {
	return t.pickFolder("Selecionar pasta de trabalho")
}

// RepoFolderInfo is one remembered repo folder card — Name is always
// derived from Path (its last segment), never stored separately.
type RepoFolderInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetRepoFolders lists every remembered repo folder (config.yaml's
// RepoFolders) for the "Repositórios" tab's cards. Prunes entries whose
// folder no longer exists first (config.PruneMissingRepoFolders) — a
// failure there is non-fatal (the stale entry just stays around one more
// read) so it doesn't block listing whatever's left.
func (t *DevToolsService) GetRepoFolders() []RepoFolderInfo {
	_ = config.PruneMissingRepoFolders()

	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	infos := make([]RepoFolderInfo, len(cfg.RepoFolders))
	for i, p := range cfg.RepoFolders {
		infos[i] = RepoFolderInfo{Name: filepath.Base(p), Path: p}
	}
	return infos
}

// AddRepoFolder appends path to the remembered repo folder list — a no-op
// (not an error) if it's already there. Always reloads the config first,
// same reasoning as SetTheme/SetWorkspacePath: don't clobber other fields
// another process may have changed while the GUI was open.
func (t *DevToolsService) AddRepoFolder(path string) error {
	if err := requireFolder(path); err != nil {
		return err
	}
	return config.Update(func(cfg *config.Config) error {
		for _, p := range cfg.RepoFolders {
			if p == path {
				return nil
			}
		}
		cfg.RepoFolders = append(cfg.RepoFolders, path)
		return nil
	})
}

// RemoveRepoFolder drops path from the remembered repo folder list — only
// the card goes away, nothing on disk is touched. A no-op if it isn't
// there. Same reload-first reasoning as AddRepoFolder.
func (t *DevToolsService) RemoveRepoFolder(path string) error {
	return config.Update(func(cfg *config.Config) error {
		kept := make([]string, 0, len(cfg.RepoFolders))
		for _, p := range cfg.RepoFolders {
			if p != path {
				kept = append(kept, p)
			}
		}
		cfg.RepoFolders = kept
		return nil
	})
}

// RepoFolderState is what the "Clonar"/"Iniciar Repositório"/"Identidade
// Local" sub-tabs need to decide between showing their form or an
// "already done" message. IsGitRepo+HasRemote: has .git plus an "origin"
// remote configured → treated as "already cloned"; has .git with no
// remote → "already initialized". IsEmpty gates "Clonar", which only
// clones into an empty folder.
type RepoFolderState struct {
	IsEmpty    bool   `json:"isEmpty"`
	IsGitRepo  bool   `json:"isGitRepo"`
	HasRemote  bool   `json:"hasRemote"`
	RemoteURL  string `json:"remoteUrl"`
	LocalName  string `json:"localName"`
	LocalEmail string `json:"localEmail"`
}

// GetRepoFolderState inspects path (the picked working folder).
func (t *DevToolsService) GetRepoFolderState(path string) (RepoFolderState, error) {
	var state RepoFolderState

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return state, fmt.Errorf("pasta inválida: %s", path)
	}

	if state.IsEmpty, err = managerrepo.IsEmptyDir(path); err != nil {
		return state, fmt.Errorf("ler pasta: %w", err)
	}

	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return state, nil // not a git repository yet — the rest stays zero-value
	}
	state.IsGitRepo = true

	ctx := context.Background()
	if url, err := t.exe.Output(ctx, executor.Options{}, "git", "-C", path, "remote", "get-url", "origin"); err == nil {
		if u := strings.TrimSpace(url); u != "" {
			state.HasRemote = true
			state.RemoteURL = u
		}
	}

	name, email := managerrepo.GetCurrentLocalIdentity(ctx, t.exe, path)
	state.LocalName = name
	state.LocalEmail = email
	return state, nil
}

// CloneRepo clones url directly into folder (the already-selected working
// folder must be empty — there's no separate subfolder field) and applies
// name/email as its local identity.
func (t *DevToolsService) CloneRepo(url, folder, name, email string) error {
	if err := requireFolder(folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		return managerrepo.Clone(context.Background(), t.exe, stdout, url, folder, name, email)
	})
}

// InitRepoAt runs "git init" in folder (created if it doesn't exist yet)
// and, when name/email are given, applies them as its local identity.
func (t *DevToolsService) InitRepoAt(folder, name, email string) error {
	if err := requireFolder(folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		return managerrepo.Init(context.Background(), t.exe, stdout, folder, name, email)
	})
}

// ApplyLocalGitIdentityAt sets folder's local user.name/user.email/
// credential.helper.
func (t *DevToolsService) ApplyLocalGitIdentityAt(folder, name, email string) error {
	if err := requireFolder(folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		return managerrepo.ApplyLocalIdentityAt(context.Background(), t.exe, stdout, name, email, folder)
	})
}

// GenerateGitignoreAt creates folder/.gitignore, or merges the missing
// patterns into an existing one (managergitignore.Generate).
func (t *DevToolsService) GenerateGitignoreAt(folder string) error {
	if err := requireFolder(folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		_, err := managergitignore.Generate(stdout, folder)
		return err
	})
}

// CreateConductAt writes folder/CODE_OF_CONDUCT.md and its Portuguese
// mirror folder/docs/pt-br/codigo-de-conduta.md, overwriting any existing
// copies, with contactEmail as the address for reporting violations (the
// frontend pre-fills it with the folder's Git e-mail).
func (t *DevToolsService) CreateConductAt(folder, contactEmail string) error {
	if err := requireFolder(folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		return managerrepo.CreateConduct(stdout, folder, contactEmail, true)
	})
}

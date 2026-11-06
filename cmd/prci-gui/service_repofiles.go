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
	"context"
	"path/filepath"
	"time"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/manager/gitaccount"
	"github.com/oito2/perci/internal/manager/repofiles"
)

// RepoFilesView is the Arquivos tab of a repository: the files table and
// the values the files are generated with.
type RepoFilesView struct {
	Rows     []repofiles.Row     `json:"rows"`
	Licenses []repofiles.License `json:"licenses"`
	// License is the license the LICENSE row is compared against.
	License string `json:"license"`
	// Holder and Year are the suggested copyright line of the licenses
	// that have one: the account's git name and the current year.
	Holder string `json:"holder"`
	Year   int    `json:"year"`
	// ContactEmail is the address the files use: RepoEmail when set,
	// otherwise AccountEmail.
	ContactEmail string `json:"contactEmail"`
	AccountEmail string `json:"accountEmail"`
	RepoEmail    string `json:"repoEmail"`
	// WebURL is the repository page the links point to; empty without an
	// origin of the account.
	WebURL string `json:"webUrl"`
}

// RepoFileChoice holds the LICENSE choices of the Arquivos tab.
type RepoFileChoice struct {
	License string `json:"license"`
	Holder  string `json:"holder"`
	Year    int    `json:"year"`
}

// repoFileOptions returns the generation options of the repository st
// of acc, without the license choices.
func repoFileOptions(acc config.GitAccount, st gitaccount.RepoState) repofiles.Options {
	email, _ := gitaccount.ContactEmail(acc, st.Path)
	return repofiles.Options{
		Provider:     acc.Provider,
		Project:      filepath.Base(st.Dir),
		WebURL:       gitaccount.WebURL(acc, st.Remote),
		ContactEmail: email,
	}
}

// GetGitAccountRepoFiles returns the Arquivos tab of the repository at
// dir of account id. license picks the license the LICENSE row is
// compared against; empty means the one found in LICENSE, or the default
// license when none is recognized.
func (s *RepositoriesService) GetGitAccountRepoFiles(ctx context.Context, id, dir, license string) (RepoFilesView, error) {
	acc, st, err := s.accountRepo(ctx, id, dir)
	if err != nil {
		return RepoFilesView{}, err
	}
	opts := repoFileOptions(acc, st)
	opts.License = license
	if license == "" {
		opts.License = repofiles.DefaultLicense
	}
	rows, err := repofiles.Rows(st.Dir, opts)
	if err != nil {
		return RepoFilesView{}, err
	}
	if license == "" {
		for _, r := range rows {
			if r.ID == repofiles.RowLicense && r.License != "" && r.License != opts.License {
				opts.License = r.License
				if rows, err = repofiles.Rows(st.Dir, opts); err != nil {
					return RepoFilesView{}, err
				}
				break
			}
		}
	}
	_, own := gitaccount.ContactEmail(acc, st.Path)
	return RepoFilesView{
		Rows: rows, Licenses: repofiles.Licenses, License: opts.License,
		Holder: acc.GitName, Year: time.Now().Year(),
		ContactEmail: opts.ContactEmail, AccountEmail: acc.ContactEmail, RepoEmail: own,
		WebURL: opts.WebURL,
	}, nil
}

// ApplyGitAccountRepoFile creates or updates the files of row (a
// repofiles row ID) in the repository at dir of account id, with the
// account's contact e-mail and the LICENSE choices, and returns a short
// summary. Holds the action lock.
func (s *RepositoriesService) ApplyGitAccountRepoFile(ctx context.Context, id, dir, row string, choice RepoFileChoice) (string, error) {
	var msg string
	err := runExclusive(func() error {
		acc, st, err := s.accountRepo(ctx, id, dir)
		if err != nil {
			return err
		}
		opts := repoFileOptions(acc, st)
		opts.License, opts.Holder, opts.Year = choice.License, choice.Holder, choice.Year
		msg, err = repofiles.Apply(st.Dir, row, opts)
		return err
	})
	return msg, err
}

// RemoveGitAccountRepoFile deletes the files of row (a repofiles row ID)
// from the repository at dir of account id. Holds the action lock.
func (s *RepositoriesService) RemoveGitAccountRepoFile(ctx context.Context, id, dir, row string) error {
	return runExclusive(func() error {
		acc, st, err := s.accountRepo(ctx, id, dir)
		if err != nil {
			return err
		}
		return repofiles.Remove(st.Dir, row, repoFileOptions(acc, st))
	})
}

// SetGitAccountContactEmail saves the contact e-mail of account id: the
// account's own when dir is empty, otherwise the override of the
// repository at dir. An empty email clears the value. Holds the action
// lock.
func (s *RepositoriesService) SetGitAccountContactEmail(ctx context.Context, id, dir, email string) error {
	return runExclusive(func() error {
		if dir == "" {
			return gitaccount.SetContactEmail(id, "", email)
		}
		_, st, err := s.accountRepo(ctx, id, dir)
		if err != nil {
			return err
		}
		return gitaccount.SetContactEmail(id, st.Path, email)
	})
}

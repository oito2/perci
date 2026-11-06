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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/manager/repofiles"
)

// ContactEmail returns the contact e-mail of the repository at path
// (relative to acc's folder) and the repository's own override: the
// effective address is the override when set, otherwise the account's.
func ContactEmail(acc config.GitAccount, path string) (effective, own string) {
	path = settingsPath(path)
	for _, rs := range acc.RepoSettings {
		if rs.Path == path && rs.ContactEmail != "" {
			return rs.ContactEmail, rs.ContactEmail
		}
	}
	return acc.ContactEmail, ""
}

// SetContactEmail saves the contact e-mail of account id: the account's
// own when path is "", otherwise the override of the repository at path
// (relative to the account folder). An empty email clears the value; any
// other must be a valid address.
func SetContactEmail(id, path, email string) error {
	email = strings.TrimSpace(email)
	if email != "" {
		checked, err := repofiles.CheckContactEmail(email)
		if err != nil {
			return err
		}
		email = checked
	}
	if path != "" {
		path = settingsPath(path)
		if err := checkSettingsPath(path); err != nil {
			return err
		}
	}
	return config.Update(func(c *config.Config) error {
		for i := range c.GitAccounts {
			if c.GitAccounts[i].ID != id {
				continue
			}
			acc := &c.GitAccounts[i]
			if path == "" {
				acc.ContactEmail = email
				return nil
			}
			acc.RepoSettings = setRepoEmail(acc.RepoSettings, path, email)
			return nil
		}
		return fmt.Errorf("conta não encontrada: %q", id)
	})
}

// setRepoEmail returns settings with the contact e-mail of path set to
// email, dropping the entry of path when it is left empty.
func setRepoEmail(settings []config.RepoSettings, path, email string) []config.RepoSettings {
	out := make([]config.RepoSettings, 0, len(settings)+1)
	found := false
	for _, rs := range settings {
		if rs.Path == path {
			found = true
			rs.ContactEmail = email
		}
		if rs.ContactEmail != "" {
			out = append(out, rs)
		}
	}
	if !found && email != "" {
		out = append(out, config.RepoSettings{Path: path, ContactEmail: email})
	}
	return out
}

// settingsPath returns path cleaned and written with "/".
func settingsPath(path string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}

// checkSettingsPath accepts "." or a clean relative path below the
// account folder.
func checkSettingsPath(path string) error {
	if path == "." {
		return nil
	}
	return checkRepoPath(path)
}

// WebURL returns the repository page (https://<host>/<owner>/<repo>) of
// remote, an origin of acc (SSH or HTTP(S), the account's host and user),
// or "" for any other remote.
func WebURL(acc config.GitAccount, remote string) string {
	switch ClassifyRemote(acc, remote) {
	case RemoteOK, RemoteHTTPS:
	default:
		return ""
	}
	r, _ := ParseRemote(remote)
	return "https://" + r.Host + "/" + r.Owner + "/" + r.Repo
}

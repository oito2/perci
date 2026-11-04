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

// Package gitaccount manages Git hosting accounts: each account owns a
// folder, and every repository under it uses the account's commit identity
// and SSH key through an includeIf rule in the global git configuration.
package gitaccount

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/config"
)

// Supported providers.
const (
	ProviderGitHub = "github"
	ProviderGitLab = "gitlab"
)

// DefaultHost returns the public host of provider, or "" for an unknown
// provider.
func DefaultHost(provider string) string {
	switch provider {
	case ProviderGitHub:
		return "github.com"
	case ProviderGitLab:
		return "gitlab.com"
	}
	return ""
}

// KeysURL is the page of acc's provider where SSH keys are registered.
func KeysURL(acc config.GitAccount) string {
	switch acc.Provider {
	case ProviderGitHub:
		return "https://" + acc.Host + "/settings/keys"
	case ProviderGitLab:
		return "https://" + acc.Host + "/-/user_settings/ssh_keys"
	}
	return ""
}

var (
	slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)
	hostPattern    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	// usernamePattern accepts GitHub and GitLab user names: letters,
	// digits, ".", "_" and "-", starting and ending with a letter or digit.
	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)
	emailPattern    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// maxNameLength bounds the display name shown in the menu.
const maxNameLength = 40

// Slug derives an account ID from name: lowercase ASCII letters and digits
// separated by single "-".
func Slug(name string) string {
	return strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// Normalize trims every text field, lowercases the host, fills in the
// provider's default host when Host is empty, derives ID from Name when
// ID is empty and cleans the folder and key paths.
func Normalize(acc config.GitAccount) config.GitAccount {
	acc.Name = strings.TrimSpace(acc.Name)
	acc.Provider = strings.TrimSpace(acc.Provider)
	acc.Host = strings.ToLower(strings.TrimSpace(acc.Host))
	if acc.Host == "" {
		acc.Host = DefaultHost(acc.Provider)
	}
	acc.Username = strings.TrimSpace(acc.Username)
	acc.GitName = strings.TrimSpace(acc.GitName)
	acc.GitEmail = strings.TrimSpace(acc.GitEmail)
	if acc.ID == "" {
		acc.ID = Slug(acc.Name)
	}
	if acc.Folder != "" {
		acc.Folder = filepath.Clean(acc.Folder)
	}
	if acc.SSHKey != "" {
		acc.SSHKey = filepath.Clean(acc.SSHKey)
	}
	return acc
}

// Validate checks a normalized account. others holds every other
// registered account (not acc itself): the name, ID and folder must not
// clash with any of them. The SSH key is checked by CheckKey.
func Validate(acc config.GitAccount, others []config.GitAccount) error {
	if err := validateFields(acc); err != nil {
		return err
	}
	if err := checkFolder(acc.Folder); err != nil {
		return err
	}
	return checkClash(acc, others)
}

// validateFields checks every field of a normalized account except the
// folder and the SSH key.
func validateFields(acc config.GitAccount) error {
	if acc.Name == "" {
		return errors.New("informe o nome da conta")
	}
	if len([]rune(acc.Name)) > maxNameLength {
		return fmt.Errorf("o nome da conta pode ter no máximo %d caracteres", maxNameLength)
	}
	if acc.ID == "" || acc.ID != Slug(acc.ID) {
		return fmt.Errorf("o nome %q precisa ter ao menos uma letra ou número", acc.Name)
	}
	if DefaultHost(acc.Provider) == "" {
		return fmt.Errorf("provedor desconhecido: %q", acc.Provider)
	}
	if !hostPattern.MatchString(acc.Host) {
		return fmt.Errorf("host inválido: %q", acc.Host)
	}
	if acc.SSHPort < 0 || acc.SSHPort > 65535 {
		return fmt.Errorf("porta SSH inválida: %d", acc.SSHPort)
	}
	if !usernamePattern.MatchString(acc.Username) {
		return fmt.Errorf("usuário inválido: %q", acc.Username)
	}
	if acc.GitName == "" || strings.ContainsAny(acc.GitName, "\n\r") {
		return errors.New("informe o nome usado nos commits")
	}
	if !emailPattern.MatchString(acc.GitEmail) {
		return fmt.Errorf("e-mail inválido: %q", acc.GitEmail)
	}
	return nil
}

// checkClash fails when acc's name, ID or folder clashes with one of
// others.
func checkClash(acc config.GitAccount, others []config.GitAccount) error {
	for _, o := range others {
		if o.ID == acc.ID || strings.EqualFold(o.Name, acc.Name) {
			return fmt.Errorf("já existe uma conta chamada %q", o.Name)
		}
		if foldersOverlap(o.Folder, acc.Folder) {
			return fmt.Errorf("a pasta %s se sobrepõe à pasta da conta %q (%s)", acc.Folder, o.Name, o.Folder)
		}
	}
	return nil
}

// checkFolder requires an absolute path to an existing directory.
func checkFolder(folder string) error {
	if folder == "" || !filepath.IsAbs(folder) {
		return errors.New("escolha a pasta da conta")
	}
	info, err := os.Stat(folder)
	if err != nil {
		return fmt.Errorf("pasta da conta: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s não é uma pasta", folder)
	}
	return nil
}

// foldersOverlap reports whether a and b are the same folder or one
// contains the other, comparing both the paths as given and with symlinks
// resolved.
func foldersOverlap(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if within(a, b) || within(b, a) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return false
	}
	return within(ra, rb) || within(rb, ra)
}

// within reports whether path is dir itself or lies below it.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

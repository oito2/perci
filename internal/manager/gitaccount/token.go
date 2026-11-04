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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// tokenService is the "service" attribute of every API token perci
// stores in the system keyring; the "account" attribute is the account ID.
const tokenService = "perci-git"

// ErrNoKeyring means secret-tool, the keyring client, is not installed.
var ErrNoKeyring = errors.New("secret-tool não encontrado — instale o Libsecret Tools em Pré-requisitos")

// StoreToken saves the API token of acc in the system keyring, replacing
// any previous one. The token goes to secret-tool on stdin.
func StoreToken(ctx context.Context, exe *executor.Executor, acc config.GitAccount, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("informe o token")
	}
	if strings.ContainsAny(token, " \t\n\r") {
		return errors.New("o token não pode conter espaços")
	}
	if !exe.CommandAvailable(ctx, "secret-tool") {
		return ErrNoKeyring
	}
	opts := executor.Options{Stdin: strings.NewReader(token)}
	label := "perci: token da conta " + acc.Name
	if err := exe.Run(ctx, opts, "secret-tool", "store", "--label="+label, "service", tokenService, "account", acc.ID); err != nil {
		return fmt.Errorf("guardar o token no chaveiro: %w", err)
	}
	return nil
}

// LookupToken returns the API token stored for the account id, or "" when
// there is none.
func LookupToken(ctx context.Context, exe *executor.Executor, id string) (string, error) {
	if !exe.CommandAvailable(ctx, "secret-tool") {
		return "", ErrNoKeyring
	}
	out, err := exe.Output(ctx, executor.Options{}, "secret-tool", "lookup", "service", tokenService, "account", id)
	if exitCode(err) == 1 {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("ler o token do chaveiro: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// DeleteToken removes the API token stored for the account id; having
// none is not an error.
func DeleteToken(ctx context.Context, exe *executor.Executor, id string) error {
	if !exe.CommandAvailable(ctx, "secret-tool") {
		return ErrNoKeyring
	}
	err := exe.Run(ctx, executor.Options{}, "secret-tool", "clear", "service", tokenService, "account", id)
	if err != nil && exitCode(err) != 1 {
		return fmt.Errorf("remover o token do chaveiro: %w", err)
	}
	return nil
}

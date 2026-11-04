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
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// fakeSecretTool is a keyring kept in one file per account under $1 of
// the "store"/"lookup"/"clear" commands, logging every call's arguments.
func fakeSecretTool(t *testing.T) string {
	t.Helper()
	store := t.TempDir()
	fakeBin(t, map[string]string{"secret-tool": `store="` + store + `"
echo "$*" >> "$store/calls"
case "$1" in
store) eval "acc=\${$#}"; cat > "$store/$acc";;
lookup) [ -f "$store/$5" ] || exit 1; cat "$store/$5";;
clear) [ -f "$store/$5" ] || exit 1; rm "$store/$5";;
esac`})
	return store
}

func TestTokenStoreLookupDelete(t *testing.T) {
	isolate(t)
	store := fakeSecretTool(t)
	ctx := t.Context()
	exe := quietExecutor()
	acc := config.GitAccount{ID: "oito2", Name: "OITO2"}

	if got, err := LookupToken(ctx, exe, "oito2"); got != "" || err != nil {
		t.Fatalf("LookupToken before storing = %q, %v", got, err)
	}
	if err := StoreToken(ctx, exe, acc, "  ghp_secret \n"); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}
	if got, err := LookupToken(ctx, exe, "oito2"); got != "ghp_secret" || err != nil {
		t.Errorf("LookupToken = %q, %v", got, err)
	}

	calls, _ := os.ReadFile(filepath.Join(store, "calls"))
	if bytes.Contains(calls, []byte("ghp_secret")) {
		t.Errorf("the token appeared on secret-tool's command line:\n%s", calls)
	}
	if !strings.Contains(string(calls), "store --label=perci: token da conta OITO2 service perci-git account oito2") {
		t.Errorf("secret-tool store arguments:\n%s", calls)
	}

	if err := DeleteToken(ctx, exe, "oito2"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if got, _ := LookupToken(ctx, exe, "oito2"); got != "" {
		t.Errorf("LookupToken after DeleteToken = %q", got)
	}
	if err := DeleteToken(ctx, exe, "oito2"); err != nil {
		t.Errorf("DeleteToken without a token: %v", err)
	}

	for _, bad := range []string{"", "  ", "two words"} {
		if err := StoreToken(ctx, exe, acc, bad); err == nil {
			t.Errorf("StoreToken(%q) succeeded", bad)
		}
	}
}

func TestToken_NoSecretTool(t *testing.T) {
	isolate(t)
	exe := &executor.Executor{LookPath: func(string) (string, error) { return "", errors.New("not found") }}
	ctx := t.Context()
	if err := StoreToken(ctx, exe, config.GitAccount{ID: "a"}, "t"); !errors.Is(err, ErrNoKeyring) {
		t.Errorf("StoreToken = %v, want ErrNoKeyring", err)
	}
	if _, err := LookupToken(ctx, exe, "a"); !errors.Is(err, ErrNoKeyring) {
		t.Errorf("LookupToken = %v, want ErrNoKeyring", err)
	}
	if err := DeleteToken(ctx, exe, "a"); !errors.Is(err, ErrNoKeyring) {
		t.Errorf("DeleteToken = %v, want ErrNoKeyring", err)
	}
}

func TestRespondAskpass(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "counter")
	if err := os.WriteFile(counter, []byte("2"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envAskpassCounter, counter)
	t.Setenv(envAskpassSecret, "frase")
	if !IsAskpassCall() {
		t.Fatal("IsAskpassCall = false with the counter set")
	}

	for i, want := range []int{0, 0, 1} {
		var out bytes.Buffer
		if code := RespondAskpass(&out); code != want {
			t.Fatalf("call %d: exit code %d, want %d", i+1, code, want)
		}
		if want == 0 && out.String() != "frase\n" {
			t.Errorf("call %d printed %q", i+1, out.String())
		}
		if want == 1 && out.Len() != 0 {
			t.Errorf("refused call printed %q", out.String())
		}
	}

	t.Setenv(envAskpassCounter, filepath.Join(t.TempDir(), "missing"))
	if code := RespondAskpass(&bytes.Buffer{}); code != 1 {
		t.Errorf("missing counter: exit code %d, want 1", code)
	}
}

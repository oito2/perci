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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

func TestGenerateKey(t *testing.T) {
	requireTool(t, "ssh-keygen")
	home := isolate(t)
	ctx := t.Context()
	exe := quietExecutor()
	askpass := testAskpass(t)

	plain, _ := DefaultKeyPath("plain")
	if plain != filepath.Join(home, ".ssh", "perci_plain") {
		t.Fatalf("DefaultKeyPath = %q", plain)
	}
	if err := GenerateKey(ctx, exe, askpass, plain, "plain@perci", ""); err != nil {
		t.Fatalf("GenerateKey without passphrase: %v", err)
	}
	if err := CheckKey(plain); err != nil {
		t.Errorf("CheckKey: %v", err)
	}
	if KeyEncrypted(ctx, exe, plain) {
		t.Error("key without passphrase reported as encrypted")
	}
	if info, err := os.Stat(filepath.Dir(plain)); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("~/.ssh mode = %v, %v; want 0700", info.Mode().Perm(), err)
	}
	pub, err := PublicKey(plain)
	if err != nil || !strings.HasPrefix(pub, "ssh-ed25519 ") || !strings.HasSuffix(pub, " plain@perci") {
		t.Errorf("PublicKey = %q, %v", pub, err)
	}

	secret, _ := DefaultKeyPath("secret")
	if err := GenerateKey(ctx, exe, askpass, secret, "secret@perci", "frase com espaço"); err != nil {
		t.Fatalf("GenerateKey with passphrase: %v", err)
	}
	if !KeyEncrypted(ctx, exe, secret) {
		t.Error("key with passphrase reported as not encrypted")
	}
	if _, err := exe.Output(ctx, executor.Options{}, "ssh-keygen", "-y", "-P", "frase com espaço", "-f", secret); err != nil {
		t.Errorf("the passphrase given to GenerateKey does not open the key: %v", err)
	}

	if err := GenerateKey(ctx, exe, askpass, plain, "again", ""); err == nil || !strings.Contains(err.Error(), "já existe") {
		t.Errorf("GenerateKey over an existing key = %v, want refusal", err)
	}
}

func TestCheckKey(t *testing.T) {
	home := isolate(t)
	missingPub := filepath.Join(home, "k")
	if err := os.WriteFile(missingPub, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	badPub := filepath.Join(home, "bad")
	for _, f := range []string{badPub, badPub + ".pub"} {
		if err := os.WriteFile(f, []byte("not a key"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"", "relative/key", filepath.Join(home, "missing"), home, missingPub, badPub} {
		if err := CheckKey(path); err == nil {
			t.Errorf("CheckKey(%q) succeeded", path)
		}
	}
}

// writeKeyPair writes a fake private key file and the public key line of
// a pinned host key next to it, returning the private key path and the
// public key fingerprint.
func writeKeyPair(t *testing.T, dir string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, "id_test")
	pub := pinnedHostKeys["github.com"][0] + " test"
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".pub", []byte(pub+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := Fingerprint(pub)
	if err != nil {
		t.Fatal(err)
	}
	return path, fp
}

func TestAgentHasKey(t *testing.T) {
	home := isolate(t)
	path, fp := writeKeyPair(t, home)

	cases := []struct {
		name    string
		script  string
		want    bool
		wantErr error
	}{
		{"listed", `echo "256 SHA256:other x (ED25519)"; echo "256 ` + fp + ` test (ED25519)"`, true, nil},
		{"not listed", `echo "256 SHA256:other x (ED25519)"`, false, nil},
		{"no identities", `echo "The agent has no identities."; exit 1`, false, nil},
		{"no agent", `echo "Could not open a connection to your authentication agent." >&2; exit 2`, false, ErrNoAgent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, map[string]string{"ssh-add": `[ "$1" = -l ] || exit 9` + "\n" + tc.script})
			got, err := AgentHasKey(t.Context(), quietExecutor(), path)
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Errorf("AgentHasKey = %v, %v; want %v, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// The fake ssh-add asks the askpass program for the passphrase; on a
// wrong one it asks again, like the real ssh-add, and reports whether
// that second request was answered.
const fakeSSHAdd = `pw=$("$SSH_ASKPASS" "Enter passphrase") || exit 1
if [ "$pw" = "certa" ]; then echo "Identity added: $2"; exit 0; fi
if "$SSH_ASKPASS" "Bad passphrase, try again" >/dev/null; then echo "asked again"; else echo "gave up"; fi
exit 1`

func TestUnlockKey(t *testing.T) {
	home := isolate(t)
	path, _ := writeKeyPair(t, home)
	fakeBin(t, map[string]string{"ssh-add": fakeSSHAdd})
	ctx := t.Context()
	exe := quietExecutor()
	askpass := testAskpass(t)

	if err := UnlockKey(ctx, exe, askpass, path, "certa"); err != nil {
		t.Errorf("UnlockKey with the right passphrase: %v", err)
	}
	err := UnlockKey(ctx, exe, askpass, path, "errada")
	if err == nil || !strings.Contains(err.Error(), "gave up") {
		t.Errorf("UnlockKey with a wrong passphrase = %v, want a failure after a single answer", err)
	}
	if err := UnlockKey(ctx, exe, Askpass{}, path, "certa"); err == nil {
		t.Error("UnlockKey without an askpass program succeeded")
	}

	fakeBin(t, map[string]string{"ssh-add": "exit 2"})
	if err := UnlockKey(ctx, exe, askpass, path, "certa"); !errors.Is(err, ErrNoAgent) {
		t.Errorf("UnlockKey without an agent = %v, want ErrNoAgent", err)
	}
}

func TestTestConnection(t *testing.T) {
	home := isolate(t)
	acc := validAccount(t, home)
	acc.SSHKey, _ = writeKeyPair(t, home)

	cases := []struct {
		name     string
		script   string
		wantUser string
		wantErr  string
	}{
		{"github greeting", `echo "Hi oito2! You've successfully authenticated, but GitHub does not provide shell access." >&2; exit 1`, "oito2", ""},
		{"gitlab greeting", `echo "Welcome to GitLab, @OITO2!"; exit 0`, "OITO2", ""},
		{"other user", `echo "Hi kaduvelasco! You've successfully authenticated, but GitHub does not provide shell access." >&2; exit 1`, "kaduvelasco", "pertence ao usuário kaduvelasco"},
		{"key refused", `echo "git@github.com: Permission denied (publickey)." >&2; exit 255`, "", "cadastre a chave pública"},
		{"host key unknown", `echo "Host key verification failed." >&2; exit 255`, "", "ainda não foi confirmada"},
		{"other failure", `echo "ssh: connect to host github.com port 22: Connection refused" >&2; exit 255`, "", "Connection refused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The fake checks the options every account connection must use.
			fakeBin(t, map[string]string{"ssh": `case "$*" in *"BatchMode=yes"*"-p 22"*"-i ` + acc.SSHKey + `"*"IdentitiesOnly=yes"*"UserKnownHostsFile="*"-- git@github.com") ;; *) echo "unexpected args: $*"; exit 99;; esac
` + tc.script})
			user, err := TestConnection(t.Context(), quietExecutor(), acc)
			if user != tc.wantUser {
				t.Errorf("user = %q, want %q", user, tc.wantUser)
			}
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("TestConnection: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Errorf("TestConnection = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestTestConnection_CustomPort(t *testing.T) {
	home := isolate(t)
	acc := config.GitAccount{Host: "git.example.com", SSHPort: 2222, Username: "carlos"}
	acc.SSHKey, _ = writeKeyPair(t, home)
	fakeBin(t, map[string]string{"ssh": `case "$*" in *"-p 2222"*"-- git@git.example.com") echo "Welcome to GitLab, @carlos!";; *) echo "unexpected args: $*"; exit 99;; esac`})
	if _, err := TestConnection(t.Context(), quietExecutor(), acc); err != nil {
		t.Errorf("TestConnection on port 2222: %v", err)
	}
}

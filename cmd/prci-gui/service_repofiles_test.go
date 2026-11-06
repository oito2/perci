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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/manager/repofiles"
)

func repoFileStates(v RepoFilesView) map[string]repofiles.State {
	out := map[string]repofiles.State{}
	for _, r := range v.Rows {
		out[r.ID] = r.State
	}
	return out
}

func TestGitAccounts_RepoFiles(t *testing.T) {
	home, _ := isolateGitAccounts(t)
	svc := newReposService()
	ctx := t.Context()
	folder := approvedFolder(t, home, "repos/oito2")
	if _, err := svc.SaveGitAccount(ctx, oito2Input(folder)); err != nil {
		t.Fatalf("SaveGitAccount: %v", err)
	}
	perci := filepath.Join(folder, "labs", "perci")
	for _, args := range [][]string{
		{"init", "-q", perci},
		{"-C", perci, "remote", "add", "origin", "git@github.com:oito2/perci.git"},
	} {
		if err := svc.exe.Run(ctx, executor.Options{}, "git", args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	v, err := svc.GetGitAccountRepoFiles(ctx, "oito2", perci, "")
	if err != nil {
		t.Fatalf("GetGitAccountRepoFiles: %v", err)
	}
	if v.License != repofiles.DefaultLicense || v.Holder != "Oito Dois" || v.Year != time.Now().Year() ||
		v.ContactEmail != "" || v.WebURL != "https://github.com/oito2/perci" || len(v.Licenses) != len(repofiles.Licenses) {
		t.Errorf("first view = %+v", v)
	}
	for id, st := range repoFileStates(v) {
		if want := map[bool]repofiles.State{true: repofiles.StateDisabled, false: repofiles.StateMissing}[id == repofiles.RowFunding]; st != want {
			t.Errorf("%s: state %s, want %s", id, st, want)
		}
	}
	if _, err := svc.ApplyGitAccountRepoFile(ctx, "oito2", perci, repofiles.RowConduct, RepoFileChoice{}); err == nil {
		t.Error("the code of conduct was generated without a contact e-mail")
	}

	// The account e-mail is the default; a repository override wins.
	if err := svc.SetGitAccountContactEmail(ctx, "oito2", "", "account@example.org"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetGitAccountContactEmail(ctx, "oito2", perci, "perci@example.org"); err != nil {
		t.Fatal(err)
	}
	v, err = svc.GetGitAccountRepoFiles(ctx, "oito2", perci, "")
	if err != nil || v.ContactEmail != "perci@example.org" || v.AccountEmail != "account@example.org" || v.RepoEmail != "perci@example.org" {
		t.Errorf("e-mails = %+v, %v", v, err)
	}

	for _, row := range []string{repofiles.RowConduct, repofiles.RowSupport, repofiles.RowGitignore} {
		if _, err := svc.ApplyGitAccountRepoFile(ctx, "oito2", perci, row, RepoFileChoice{}); err != nil {
			t.Fatalf("ApplyGitAccountRepoFile(%s): %v", row, err)
		}
	}
	support, _ := os.ReadFile(filepath.Join(perci, "SUPPORT.md"))
	if !strings.Contains(string(support), "https://github.com/oito2/perci/issues") || !strings.Contains(string(support), "perci@example.org") ||
		!strings.Contains(string(support), "help with perci.") {
		t.Errorf("SUPPORT.md:\n%s", support)
	}
	msg, err := svc.ApplyGitAccountRepoFile(ctx, "oito2", perci, repofiles.RowLicense,
		RepoFileChoice{License: "MIT", Holder: "Oito Dois", Year: 2026})
	if err != nil || msg == "" {
		t.Fatalf("ApplyGitAccountRepoFile(license) = %q, %v", msg, err)
	}

	// Without a choice, LICENSE is compared against the license it holds.
	v, err = svc.GetGitAccountRepoFiles(ctx, "oito2", perci, "")
	states := repoFileStates(v)
	if err != nil || v.License != "MIT" || states[repofiles.RowLicense] != repofiles.StateCurrent ||
		states[repofiles.RowConduct] != repofiles.StateCurrent || states[repofiles.RowSupport] != repofiles.StateCurrent {
		t.Errorf("after Apply: license %q states %v, %v", v.License, states, err)
	}
	v, _ = svc.GetGitAccountRepoFiles(ctx, "oito2", perci, "GPL-3.0")
	if repoFileStates(v)[repofiles.RowLicense] != repofiles.StateDiffers {
		t.Errorf("MIT compared against GPL-3.0: %v", repoFileStates(v))
	}

	// Dropping the override falls back to the account e-mail, so the
	// generated files no longer match.
	if err := svc.SetGitAccountContactEmail(ctx, "oito2", perci, ""); err != nil {
		t.Fatal(err)
	}
	v, _ = svc.GetGitAccountRepoFiles(ctx, "oito2", perci, "")
	if v.ContactEmail != "account@example.org" || repoFileStates(v)[repofiles.RowConduct] != repofiles.StateDiffers {
		t.Errorf("after dropping the override: %q %v", v.ContactEmail, repoFileStates(v))
	}

	if err := svc.RemoveGitAccountRepoFile(ctx, "oito2", perci, repofiles.RowSupport); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(perci, "SUPPORT.md")); !os.IsNotExist(err) {
		t.Errorf("SUPPORT.md left after Remove: %v", err)
	}

	// Editing the account keeps its contact e-mail.
	edit := oito2Input(folder)
	edit.ID, edit.KeyMode = "oito2", keyModeKeep
	if _, err := svc.SaveGitAccount(ctx, edit); err != nil {
		t.Fatal(err)
	}
	if v, _ := svc.GetGitAccountRepoFiles(ctx, "oito2", perci, ""); v.AccountEmail != "account@example.org" {
		t.Errorf("account e-mail after editing the account: %q", v.AccountEmail)
	}

	for name, call := range map[string]func() error{
		"outside the account": func() error {
			_, err := svc.GetGitAccountRepoFiles(ctx, "oito2", home, "")
			return err
		},
		"unknown account": func() error { return svc.SetGitAccountContactEmail(ctx, "missing", "", "a@example.org") },
		"invalid e-mail":  func() error { return svc.SetGitAccountContactEmail(ctx, "oito2", perci, "nope") },
		"unknown row": func() error {
			_, err := svc.ApplyGitAccountRepoFile(ctx, "oito2", perci, "readme", RepoFileChoice{})
			return err
		},
		"unknown license": func() error {
			_, err := svc.GetGitAccountRepoFiles(ctx, "oito2", perci, "WTFPL")
			return err
		},
		"remove outside the account": func() error {
			return svc.RemoveGitAccountRepoFile(ctx, "oito2", home, repofiles.RowConduct)
		},
		"funding": func() error {
			_, err := svc.ApplyGitAccountRepoFile(ctx, "oito2", perci, repofiles.RowFunding, RepoFileChoice{})
			return err
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	// A running action blocks the file actions.
	runMu.Lock()
	_, applyErr := svc.ApplyGitAccountRepoFile(ctx, "oito2", perci, repofiles.RowPRTemplate, RepoFileChoice{})
	removeErr := svc.RemoveGitAccountRepoFile(ctx, "oito2", perci, repofiles.RowConduct)
	emailErr := svc.SetGitAccountContactEmail(ctx, "oito2", "", "x@example.org")
	runMu.Unlock()
	if applyErr == nil || removeErr == nil || emailErr == nil {
		t.Errorf("file actions ran while another action held the lock: %v / %v / %v", applyErr, removeErr, emailErr)
	}
	if _, err := os.Stat(filepath.Join(perci, "CODE_OF_CONDUCT.md")); err != nil {
		t.Errorf("CODE_OF_CONDUCT.md removed while the lock was held: %v", err)
	}
}

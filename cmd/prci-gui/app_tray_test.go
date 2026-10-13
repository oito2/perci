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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/appstack"
	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/selfupdate"
)

// TestWriteAutostartDesktopFile writes the XDG autostart .desktop entry,
// checks it has the fields a hidden-on-login autostart entry needs
// (NoDisplay, Exec), then removes it and confirms it's gone.
func TestWriteAutostartDesktopFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatalf("writeAutostartDesktopFile: %v", err)
	}

	path := filepath.Join(home, ".config", "autostart", "perci.desktop")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler arquivo gerado: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "[Desktop Entry]") {
		t.Error("arquivo .desktop malformado — deveria começar com [Desktop Entry]")
	}
	if !strings.Contains(content, "NoDisplay=true") {
		t.Error("autostart deve ter NoDisplay=true (não deve aparecer no menu de apps, só rodar oculto no login)")
	}
	if !strings.Contains(content, "Exec=") {
		t.Error("arquivo .desktop sem linha Exec=")
	}

	if err := removeAutostartDesktopFile(); err != nil {
		t.Fatalf("removeAutostartDesktopFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("arquivo deveria ter sido removido")
	}
}

func TestRemoveAutostartDesktopFile_NeverEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := removeAutostartDesktopFile(); err != nil {
		t.Errorf("remover um autostart que nunca existiu não deveria ser erro: %v", err)
	}
}

// TestOpenCompactWindow_UnknownTab confirms that openCompactWindow silently
// ignores an unknown tab (compactTabs[tab] == false) instead of creating a
// window — doesn't depend on any real Wails window/app, it only exercises
// the early return that happens before any Wails API call.
func TestOpenCompactWindow_UnknownTab(t *testing.T) {
	a := &TrayService{}
	// Must not try to use a.wailsApp (nil) for an invalid tab — doing so
	// would panic (nil pointer dereference) instead of simply returning.
	a.openCompactWindow("aba-que-nao-existe")
	if a.compactWindow != nil {
		t.Error("openCompactWindow com aba inválida não deveria criar nenhuma janela")
	}
}

func TestDesktopExecQuote(t *testing.T) {
	cases := map[string]string{
		"/usr/local/bin/prci":  `"/usr/local/bin/prci"`,
		"/home/u/My Apps/prci": `"/home/u/My Apps/prci"`,
		`/opt/a"b`:             `"/opt/a\\"b"`,
		"/opt/$HOME/`x`":       "\"/opt/\\\\$HOME/\\\\`x\\\\`\"",
		`/opt/back\slash`:      `"/opt/back\\\\slash"`,
		"/opt/100%":            `"/opt/100%%"`,
	}
	for in, want := range cases {
		got, err := desktopExecQuote(in)
		if err != nil || got != want {
			t.Errorf("desktopExecQuote(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := desktopExecQuote("/opt/a\nb"); err == nil {
		t.Error("a newline can't be represented in a .desktop value and must be refused")
	}
}

// The autostart entry starts Perci hidden in the tray.
func TestWriteAutostartDesktopFile_StartsHidden(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatal(err)
	}
	path, _ := autostartDesktopPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Exec=") {
			if !strings.HasPrefix(line, `Exec="`) || !strings.HasSuffix(line, `" `+hiddenFlag) {
				t.Errorf("Exec line = %q, want a quoted binary followed by %s", line, hiddenFlag)
			}
			return
		}
	}
	t.Fatal("no Exec= line")
}

// TestHandleSecondInstance confirms that a second launch focuses the running
// instance's main window, except for the hidden autostart launch.
func TestHandleSecondInstance(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"/usr/local/bin/prci"}, true},
		{[]string{"/usr/local/bin/prci", "--other"}, true},
		{[]string{"/usr/local/bin/prci", hiddenFlag}, false},
		{[]string{hiddenFlag}, true},
		{nil, true},
	}
	for _, c := range cases {
		focused := false
		handleSecondInstance(c.args, func() { focused = true })
		if focused != c.want {
			t.Errorf("handleSecondInstance(%q): focused = %v, want %v", c.args, focused, c.want)
		}
	}
}

// newTestTray returns a TrayService whose events are recorded.
func newTestTray() (*TrayService, *recorder) {
	rec := &recorder{done: make(chan struct{})}
	t := &TrayService{}
	t.emitFn = func(name string, data any) {
		rec.mu.Lock()
		rec.events = append(rec.events, recordedEvent{name, data})
		rec.mu.Unlock()
	}
	return t, rec
}

func (r *recorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		out = append(out, e.name)
	}
	return out
}

// Quitting is refused, and announced to the frontend, only while an action
// holds the action lock.
func TestQuitBlocked(t *testing.T) {
	tray, rec := newTestTray()
	if tray.quitBlocked() {
		t.Fatal("idle: quit must be allowed")
	}
	if len(rec.names()) != 0 {
		t.Errorf("idle: unexpected events %v", rec.names())
	}

	runMu.Lock()
	blocked := tray.quitBlocked()
	runMu.Unlock()
	if !blocked {
		t.Fatal("an action is running: quit must be refused")
	}
	if got := rec.names(); len(got) != 1 || got[0] != "quit-blocked" {
		t.Errorf("events = %v, want [quit-blocked]", got)
	}
	if !runMu.TryLock() {
		t.Fatal("quitBlocked left the action lock held")
	}
	runMu.Unlock()
}

// A quit that was decided on purpose (finished uninstall) is never refused,
// even though the uninstall itself still holds the action lock.
func TestQuitBlocked_AllowedWhenQuitting(t *testing.T) {
	quitAllowed.Store(true)
	defer quitAllowed.Store(false)
	tray, _ := newTestTray()
	runMu.Lock()
	defer runMu.Unlock()
	if tray.quitBlocked() {
		t.Error("quit must not be refused once it was allowed")
	}
}

// Closing the main window: hides into the tray when it's enabled; cancels
// while an action runs; quits when idle with the tray off.
func TestMainCloseDecision(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tray, rec := newTestTray()

	if cancel, hide := tray.mainCloseDecision(); cancel || hide {
		t.Errorf("idle, tray off: cancel=%v hide=%v, want both false", cancel, hide)
	}

	runMu.Lock()
	cancel, hide := tray.mainCloseDecision()
	runMu.Unlock()
	if !cancel || hide {
		t.Errorf("running, tray off: cancel=%v hide=%v, want cancel only", cancel, hide)
	}
	if got := rec.names(); len(got) != 1 || got[0] != "quit-blocked" {
		t.Errorf("events = %v, want [quit-blocked]", got)
	}

	if err := config.Update(func(c *config.Config) error { c.TrayEnabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if cancel, hide := tray.mainCloseDecision(); !cancel || !hide {
		t.Errorf("tray on: cancel=%v hide=%v, want both true", cancel, hide)
	}
}

// Turning the tray off persists the setting and removes the autostart
// entry (the tray icon itself is a Wails object, absent here).
func TestSetTrayEnabled_Off(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.Update(func(c *config.Config) error { c.TrayEnabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatal(err)
	}
	tray, _ := newTestTray()
	if err := tray.SetTrayEnabled(false); err != nil {
		t.Fatalf("SetTrayEnabled(false): %v", err)
	}
	if tray.GetTrayEnabled() {
		t.Error("TrayEnabled still true in config")
	}
	path, _ := autostartDesktopPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("autostart entry should be gone")
	}
}

func TestAppIcons(t *testing.T) {
	pink := &config.Config{AppIcon: "pink"}
	if w, tr := appIcons(pink); &w[0] != &iconPink[0] || &tr[0] != &iconPinkTray[0] {
		t.Error("pink config must select the pink window and tray icons")
	}
	for _, cfg := range []*config.Config{nil, {}, {AppIcon: "blue"}} {
		if w, tr := appIcons(cfg); &w[0] != &iconBlue[0] || &tr[0] != &iconBlueTray[0] {
			t.Errorf("cfg %+v must select the blue icons", cfg)
		}
	}
}

// newUninstallHome prepares a temp HOME with a registered Nginx (so a
// backup has something to export) and replaces the uninstall step with a
// recorder that never touches the real binary.
func newUninstallHome(t *testing.T, onUninstall func() error) (*HomeService, *int) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	calls := 0
	prev := uninstallPerci
	uninstallPerci = func(_ context.Context, _ *executor.Executor, _ io.Writer, _ bool) error {
		calls++
		return onUninstall()
	}
	t.Cleanup(func() { uninstallPerci = prev })
	exe := executor.New(nil, nil)
	exe.DryRun = true
	return &HomeService{serviceBase: serviceBase{exe: exe}}, &calls
}

func registerNginx(t *testing.T) {
	t.Helper()
	if err := config.Update(func(c *config.Config) error { c.Docker.NginxCreated = true; return nil }); err != nil {
		t.Fatal(err)
	}
}

// A backup that fails stops the uninstall before anything is removed.
func TestSelfUninstall_FailedBackupAborts(t *testing.T) {
	h, calls := newUninstallHome(t, func() error { return selfupdate.ErrUninstalled })
	registerNginx(t)
	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The destination's parent is a regular file: the export cannot succeed.
	err := h.selfUninstall(io.Discard, true, true, filepath.Join(blocker, "backup.yaml"))
	if err == nil || !strings.Contains(err.Error(), "nada foi removido") {
		t.Fatalf("err = %v, want the backup failure", err)
	}
	if errors.Is(err, selfupdate.ErrUninstalled) || *calls != 0 {
		t.Errorf("uninstall ran after a failed backup (calls=%d, err=%v)", *calls, err)
	}
	if path, _ := autostartDesktopPath(); !fileExists(path) {
		t.Error("the autostart entry was removed despite the failed backup")
	}
	if cfg, err := config.Load(); err != nil || !cfg.Docker.NginxCreated {
		t.Error("the configuration was altered despite the failed backup")
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// With a working backup, the file exists before the uninstall step runs,
// and the autostart entry goes away after it succeeds.
func TestSelfUninstall_BackupBeforeUninstall(t *testing.T) {
	backup := filepath.Join(t.TempDir(), "backup.yaml")
	backedUp := false
	h, calls := newUninstallHome(t, func() error {
		backedUp = fileExists(backup)
		return selfupdate.ErrUninstalled
	})
	registerNginx(t)
	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatal(err)
	}
	err := h.selfUninstall(io.Discard, false, false, backup)
	if !errors.Is(err, selfupdate.ErrUninstalled) || *calls != 1 {
		t.Fatalf("err=%v calls=%d, want ErrUninstalled after one uninstall", err, *calls)
	}
	if !backedUp {
		t.Error("the uninstall ran before the backup was written")
	}
	if path, _ := autostartDesktopPath(); fileExists(path) {
		t.Error("the autostart entry should be removed after the uninstall")
	}
}

// "Nothing to export" is only a warning: the uninstall goes on.
func TestSelfUninstall_NothingToExportIsWarning(t *testing.T) {
	h, calls := newUninstallHome(t, func() error { return selfupdate.ErrUninstalled })
	backup := filepath.Join(t.TempDir(), "backup.yaml")
	var out strings.Builder
	err := h.selfUninstall(&out, false, false, backup)
	if !errors.Is(err, selfupdate.ErrUninstalled) || *calls != 1 {
		t.Fatalf("err=%v calls=%d, want the uninstall to proceed", err, *calls)
	}
	if !strings.Contains(out.String(), appstack.ErrNothingToExport.Error()) {
		t.Errorf("no warning in the output: %q", out.String())
	}
	if fileExists(backup) {
		t.Error("no backup file expected")
	}
}

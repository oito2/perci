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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/executor"
)

// serviceBase is embedded by value in every domain service struct
// (HomeService/LinuxService/DevSetupService/DockerService/DevToolsService/
// TrayService) — one application.Service per domain, each sharing the
// plumbing common to all of them (wailsApp/exe, pickFolder, runAction,
// eventWriter) instead of duplicating it. pickFolder/runAction stay
// unexported so they're never bindable by `wails3 generate bindings` even
// though they're promoted onto every embedding struct.
type serviceBase struct {
	wailsApp *application.App
	exe      *executor.Executor
	// emitFn replaces wailsApp.Event.Emit when set — tests only.
	emitFn func(name string, data any)
}

// emit sends a Wails event to every window.
func (b *serviceBase) emit(name string, data any) {
	if b.emitFn != nil {
		b.emitFn(name, data)
		return
	}
	b.wailsApp.Event.Emit(name, data)
}

// approvedPaths holds every path the user picked in a native dialog during
// this session (all Pick* methods go through approvePath). Methods that
// write or read files at a path coming from the frontend accept only these
// (requireApprovedPath): the frontend is a webview, and a bound method that
// writes arbitrary content to any path it's handed would turn anything
// running there into a write-anywhere primitive (~/.bashrc, autostart
// entries...). Package-level, not per service: each service embeds its own
// serviceBase by value.
var approvedPaths = struct {
	sync.Mutex
	set map[string]struct{}
}{set: map[string]struct{}{}}

// approvePath records a dialog's result (empty = cancelled) and passes it
// through unchanged.
func approvePath(path string, err error) (string, error) {
	if err == nil && path != "" {
		approvedPaths.Lock()
		approvedPaths.set[filepath.Clean(path)] = struct{}{}
		approvedPaths.Unlock()
	}
	return path, err
}

// requireApprovedPath rejects any path that didn't come from a native
// dialog in this session.
func requireApprovedPath(path string) error {
	if path == "" {
		return fmt.Errorf("nenhum arquivo selecionado")
	}
	approvedPaths.Lock()
	_, ok := approvedPaths.set[filepath.Clean(path)]
	approvedPaths.Unlock()
	if !ok {
		return fmt.Errorf("caminho não escolhido no seletor de arquivos: %s", path)
	}
	return nil
}

// requireFolder validates a project folder coming from the frontend before
// anything acts on it: an absolute path to an existing directory. The
// domain functions treat "" as "the current directory" — in a GUI that's
// wherever Perci was launched from ($HOME, or / from autostart), so an
// empty value used to write CODE_OF_CONDUCT.md, .agents/ or a git repo
// there.
func requireFolder(p string) error {
	if !filepath.IsAbs(p) {
		return fmt.Errorf("selecione uma pasta válida (recebido: %q)", p)
	}
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("pasta inválida: %s", p)
	}
	return nil
}

// requireFolderUnlessGlobal is requireFolder for actions with a Global/Local
// scope — the folder only matters for Local.
func requireFolderUnlessGlobal(global bool, p string) error {
	if global {
		return nil
	}
	return requireFolder(p)
}

// pickFolder opens the native "choose a folder" dialog with the options
// every Pick*Folder screen-specific method wants — CanCreateDirectories so
// the user can create a brand-new empty folder from inside the dialog (the
// common case for "clone/init/install something new here"). Only the title
// differs between callers.
func (b *serviceBase) pickFolder(title string) (string, error) {
	return approvePath(b.wailsApp.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                title,
		CanChooseDirectories: true,
		CanChooseFiles:       false,
		CanCreateDirectories: true,
	}).PromptForSingleSelection())
}

// eventWriter connects the io.Writer convention every domain function
// already uses (ui.Info/Warning/Success/Step, real command output via
// internal/executor) to the Execução tab's terminal panel: every Write()
// becomes a "log-line" event. Safe for concurrent use (a command's stdout
// and stderr are copied from different goroutines), and an incomplete
// UTF-8 sequence at the end of a Write is held back until the next one —
// emitted on its own it became "�" in the terminal. Flush emits whatever
// is left once the action ends.
type eventWriter struct {
	emit func(string)
	mu   sync.Mutex
	buf  []byte
}

func (b *serviceBase) newEventWriter() *eventWriter {
	return &eventWriter{emit: func(s string) { b.emit("log-line", s) }}
}

func (w *eventWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	n := completeUTF8Prefix(w.buf)
	if n > 0 {
		w.emit(string(w.buf[:n]))
		w.buf = append(w.buf[:0], w.buf[n:]...)
	}
	return len(p), nil
}

// Flush emits any held-back bytes, even an invalid trailing sequence.
func (w *eventWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = w.buf[:0]
	}
}

// completeUTF8Prefix returns the length of b without a trailing rune that
// is still missing bytes. Anything else — including invalid bytes — is
// passed through, so a non-UTF-8 stream is never held back.
func completeUTF8Prefix(b []byte) int {
	n := len(b)
	for i := 1; i < utf8.UTFMax && i <= n; i++ {
		if utf8.RuneStart(b[n-i]) {
			if !utf8.FullRune(b[n-i:]) {
				return n - i
			}
			break
		}
	}
	return n
}

// runMu serializes long-running actions across every service and window
// (decided with the user on 2026-09-29): package-level because each service
// embeds its own serviceBase, and the tray's compact window runs actions
// through the same services. Two actions at once would interleave their
// output in the terminal, race on the package manager's lock and on the
// Nginx config, and prompt for the password twice.
var runMu sync.Mutex

// errActionRunning is returned (synchronously, to the one caller) when
// another action is still in progress — never announced through
// "action-done", which every window receives.
var errActionRunning = errors.New("outra ação já está em execução — aguarde ela terminar")

// runAction runs fn on its own goroutine (the "Executar" button click must
// not block the UI), with its output going to the terminal via eventWriter,
// and notifies the frontend when it finishes via the "action-done" event.
// Only one action runs at a time (runMu); a panic inside fn is turned into
// a failed action instead of taking the whole process down — possibly in
// the middle of a privileged package-manager run — and "action-done" is
// always emitted.
func (b *serviceBase) runAction(fn func(stdout io.Writer) error) error {
	if !runMu.TryLock() {
		return errActionRunning
	}
	go func() {
		defer runMu.Unlock()
		w := b.newEventWriter()
		err := runRecovered(func() error { return fn(w) })
		w.Flush()
		// The action may have installed a tool into ~/.local/bin or through
		// nvm: refreshed before action-done, whose handlers re-check what's
		// installed.
		localbin.RefreshPath(context.Background(), b.exe)
		b.emitDone(err)
	}()
	return nil
}

// runRecovered calls fn, converting a panic into an error.
func runRecovered(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("erro interno: %v", r)
		}
	}()
	return fn()
}

// emitDone sends the "action-done" event the frontend's run lifecycle waits
// for (js/core.js).
func (b *serviceBase) emitDone(err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	b.emit("action-done", map[string]interface{}{
		"ok":    err == nil,
		"error": msg,
	})
}

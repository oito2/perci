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
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/oito2/perci/internal/executor"
)

// A second action while one is running is refused synchronously, to the
// caller only — fn never runs and nothing is emitted.
func TestRunAction_RefusesWhileAnotherRuns(t *testing.T) {
	runMu.Lock()
	defer runMu.Unlock()

	ran := false
	err := (&serviceBase{}).runAction(func(io.Writer) error { ran = true; return nil })
	if !errors.Is(err, errActionRunning) {
		t.Fatalf("runAction = %v, want errActionRunning", err)
	}
	if ran {
		t.Error("fn must not run while another action holds the lock")
	}
}

// An action started from the "action-done" event of the previous one is
// accepted: the lock is released before action-done is emitted.
func TestRunAction_LockReleasedBeforeDone(t *testing.T) {
	exe := executor.New(nil, nil)
	exe.DryRun = true
	result := make(chan error, 1)
	secondDone := make(chan struct{})
	var mu sync.Mutex
	dones := 0
	var b *serviceBase
	b = &serviceBase{exe: exe, emitFn: func(name string, _ any) {
		if name != "action-done" {
			return
		}
		mu.Lock()
		dones++
		n := dones
		mu.Unlock()
		if n == 1 {
			result <- b.runAction(func(io.Writer) error { return nil })
		} else {
			close(secondDone)
		}
	}}
	if err := b.runAction(func(io.Writer) error { return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("action started on action-done was refused: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("action-done never emitted")
	}
	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the second action never finished")
	}
}

// runExclusive is refused while an action runs and releases the lock after.
func TestRunExclusive(t *testing.T) {
	runMu.Lock()
	if err := runExclusive(func() error { return nil }); !errors.Is(err, errActionRunning) {
		t.Errorf("while locked: %v, want errActionRunning", err)
	}
	runMu.Unlock()
	ran := false
	if err := runExclusive(func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("unlocked: err=%v ran=%v", err, ran)
	}
	if !runMu.TryLock() {
		t.Fatal("lock still held after runExclusive")
	}
	runMu.Unlock()
}

func TestRunRecovered_TurnsPanicIntoError(t *testing.T) {
	err := runRecovered(func() error { panic("boom") })
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("runRecovered = %v, want an error mentioning the panic", err)
	}
	want := errors.New("plain")
	if got := runRecovered(func() error { return want }); got != want {
		t.Errorf("runRecovered should pass errors through, got %v", got)
	}
}

func TestRequireFolder(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/file"
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "relative/dir", dir + "/missing", file} {
		if requireFolder(bad) == nil {
			t.Errorf("requireFolder(%q) should fail", bad)
		}
	}
	if err := requireFolder(dir); err != nil {
		t.Errorf("an existing absolute directory must pass: %v", err)
	}
}

// A multibyte rune split across two Writes reaches the terminal whole;
// Flush emits what's left, the valid part first.
func TestEventWriter_KeepsRunesWhole(t *testing.T) {
	var got []string
	w := &eventWriter{interval: time.Hour, emit: func(s string) { got = append(got, s) }}
	b := []byte("ação")
	_, _ = w.Write(b[:2]) // "a" + first byte of "ç": goes out at once, minus the partial rune
	_, _ = w.Write(b[2:])
	_, _ = w.Write([]byte{0xE2, 0x82}) // incomplete "€", held back
	w.Flush()

	if len(got) != 3 || got[0] != "a" || got[1] != "ção" || got[2] != "\xE2\x82" {
		t.Errorf("emitted %q", got)
	}
	for _, s := range got[:2] {
		if !utf8.ValidString(s) {
			t.Errorf("%q is not valid UTF-8", s)
		}
	}
}

// Writes inside one interval are batched into a single event, sent by the
// timer; no byte is lost or reordered.
func TestEventWriter_BatchesByTime(t *testing.T) {
	var mu sync.Mutex
	var got []string
	emitted := make(chan struct{}, 8)
	w := &eventWriter{interval: 40 * time.Millisecond, emit: func(s string) {
		mu.Lock()
		got = append(got, s)
		mu.Unlock()
		emitted <- struct{}{}
	}}
	for _, part := range []string{"um\n", "dois\n", "três\n"} {
		_, _ = w.Write([]byte(part))
	}
	<-emitted // the first write, sent immediately
	select {
	case <-emitted: // the other two, in one event
	case <-time.After(2 * time.Second):
		t.Fatal("the timer never emitted the batched output")
	}
	w.Flush()
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0] != "um\n" || got[1] != "dois\ntrês\n" {
		t.Errorf("emitted %q", got)
	}
}

// Pending output over the size limit is emitted at once, and a batched
// multibyte rune is never split by the batch boundary.
func TestEventWriter_FlushesBySize(t *testing.T) {
	var got []string
	w := &eventWriter{interval: time.Hour, emit: func(s string) { got = append(got, s) }}
	_, _ = w.Write([]byte("x")) // first write: immediate
	big := strings.Repeat("é", logBatchMax/2+1)
	_, _ = w.Write([]byte(big[:len(big)-1])) // ends in half a rune, at the limit
	if len(got) != 2 {
		t.Fatalf("size limit did not trigger an emit: %d events", len(got))
	}
	if !utf8.ValidString(got[1]) {
		t.Error("the size-triggered event splits a rune")
	}
	_, _ = w.Write([]byte(big[len(big)-1:]))
	w.Flush()
	if joined := strings.Join(got, ""); joined != "x"+big {
		t.Errorf("output differs from input (%d vs %d bytes)", len(joined), len("x"+big))
	}
}

// Flush stops the timer: nothing is emitted after it returns.
func TestEventWriter_FlushStopsTimer(t *testing.T) {
	var mu sync.Mutex
	count := 0
	w := &eventWriter{interval: 20 * time.Millisecond, emit: func(string) { mu.Lock(); count++; mu.Unlock() }}
	_, _ = w.Write([]byte("a"))
	_, _ = w.Write([]byte("b"))
	w.Flush()
	mu.Lock()
	after := count
	mu.Unlock()
	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != after || after != 2 {
		t.Errorf("events: %d after Flush, %d later; want 2 and unchanged", after, count)
	}
}

// recorder collects the events a serviceBase emits; done is closed on
// "action-done".
type recorder struct {
	mu     sync.Mutex
	events []recordedEvent
	done   chan struct{}
}

type recordedEvent struct {
	name string
	data any
}

func newTestBase(t *testing.T) (*serviceBase, *recorder) {
	t.Helper()
	rec := &recorder{done: make(chan struct{})}
	exe := executor.New(nil, nil)
	exe.DryRun = true
	b := &serviceBase{exe: exe, emitFn: func(name string, data any) {
		rec.mu.Lock()
		rec.events = append(rec.events, recordedEvent{name, data})
		rec.mu.Unlock()
		if name == "action-done" {
			close(rec.done)
		}
	}}
	return b, rec
}

func (r *recorder) wait(t *testing.T) (logs string, done map[string]interface{}) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("action-done never emitted")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		switch e.name {
		case "log-line":
			logs += e.data.(string)
		case "action-done":
			done = e.data.(map[string]interface{})
		}
	}
	return logs, done
}

// runAction streams fn's output, then reports its result in action-done
// (a panic included), and releases the lock for the next action.
func TestRunAction_EmitsLogsAndDone(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fn     func(io.Writer) error
		ok     bool
		errSub string
	}{
		{"success", func(w io.Writer) error { _, _ = io.WriteString(w, "olá\n"); return nil }, true, ""},
		{"failure", func(w io.Writer) error { _, _ = io.WriteString(w, "olá\n"); return errors.New("falhou") }, false, "falhou"},
		{"panic", func(w io.Writer) error { _, _ = io.WriteString(w, "olá\n"); panic("boom") }, false, "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, rec := newTestBase(t)
			if err := b.runAction(tc.fn); err != nil {
				t.Fatalf("runAction: %v", err)
			}
			logs, done := rec.wait(t)
			if logs != "olá\n" {
				t.Errorf("logs = %q", logs)
			}
			if done["ok"] != tc.ok || !strings.Contains(done["error"].(string), tc.errSub) {
				t.Errorf("action-done = %v", done)
			}
			// The lock is free again once action-done was sent.
			deadline := time.Now().Add(time.Second)
			for !runMu.TryLock() {
				if time.Now().After(deadline) {
					t.Fatal("runMu still held after action-done")
				}
				time.Sleep(time.Millisecond)
			}
			runMu.Unlock()
		})
	}
}

// fakeItem is a checklist entry for runChecklist tests.
type fakeItem struct{ id string }

// runChecklist applies only the difference between the selection and what
// is installed, and does nothing (still reporting success) when there's
// none.
func TestRunChecklist_AppliesDiff(t *testing.T) {
	for _, tc := range []struct {
		name                string
		selected            []string
		wantApply           bool
		wantInstall, wantRm []string
	}{
		{"diff", []string{"a", "c"}, true, []string{"c"}, []string{"b"}},
		{"no change", []string{"a", "b"}, false, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, rec := newTestBase(t)
			var gotInstall, gotRemove []string
			applied := false
			c := checklistCatalog[fakeItem]{
				items:     []fakeItem{{"a"}, {"b"}, {"c"}},
				idOf:      func(f fakeItem) string { return f.id },
				labelOf:   func(f fakeItem) string { return f.id },
				installed: func() map[string]bool { return map[string]bool{"a": true, "b": true} },
				apply: func(_ io.Writer, toInstall, toRemove []string) error {
					applied, gotInstall, gotRemove = true, toInstall, toRemove
					return nil
				},
			}
			if err := runChecklist(b, c, tc.selected); err != nil {
				t.Fatal(err)
			}
			_, done := rec.wait(t)
			if done["ok"] != true {
				t.Errorf("action-done = %v", done)
			}
			if applied != tc.wantApply || !slices.Equal(gotInstall, tc.wantInstall) || !slices.Equal(gotRemove, tc.wantRm) {
				t.Errorf("applied=%v install=%v remove=%v", applied, gotInstall, gotRemove)
			}
		})
	}
}

// getChecklistInfo carries each item's installed state, description and
// removal warning.
func TestGetChecklistInfo(t *testing.T) {
	c := checklistCatalog[fakeItem]{
		items:     []fakeItem{{"a"}, {"b"}},
		idOf:      func(f fakeItem) string { return f.id },
		labelOf:   func(f fakeItem) string { return "L" + f.id },
		descOf:    func(f fakeItem) string { return "D" + f.id },
		warnOf:    func(f fakeItem) string { return map[string]string{"b": "cuidado"}[f.id] },
		installed: func() map[string]bool { return map[string]bool{"b": true} },
	}
	got := getChecklistInfo(c)
	want := []MultiSelectItemInfo{
		{ID: "a", Label: "La", Description: "Da"},
		{ID: "b", Label: "Lb", Description: "Db", Installed: true, RemoveWarning: "cuidado"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v", got)
	}
}

// Only an approved, existing, regular .tar.gz file is accepted.
func TestValidateTarballPath(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "a.tar.gz")
	zip := filepath.Join(dir, "a.zip")
	unapproved := filepath.Join(dir, "b.tar.gz")
	subdir := filepath.Join(dir, "d.tar.gz")
	for _, p := range []string{ok, zip, unapproved} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{ok, zip, subdir, filepath.Join(dir, "missing.tar.gz")} {
		_, _ = approvePath(p, nil)
	}

	if err := validateTarballPath(ok); err != nil {
		t.Errorf("valid tarball refused: %v", err)
	}
	for _, bad := range []string{zip, subdir, unapproved, filepath.Join(dir, "missing.tar.gz"), ""} {
		if validateTarballPath(bad) == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

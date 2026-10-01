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

// Package shellrc provides shared helpers for editing the user's shell
// startup file (~/.bashrc, ~/.zshrc, or fish's config.fish) — used by every
// internal/dev/* package that needs to add or remove a PATH export line.
package shellrc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oito2/perci/internal/fsutil"
)

// rcFileLocks serializes Rewrite/AppendIfMissing/RemoveEntry calls against
// the same rc file path — internal/dev/* packages call these from GUI
// goroutines that can run concurrently (e.g. installing two tools at once
// that both touch ~/.bashrc), and without this two such calls could each
// read the file, compute their own edit, and have the last one to rename
// silently drop the other's change.
var rcFileLocks sync.Map // map[string]*sync.Mutex

func lockFor(path string) *sync.Mutex {
	v, _ := rcFileLocks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// File returns the primary shell RC file for the current user's shell.
func File(home string) string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return filepath.Join(home, ".bashrc")
	}
}

// Dedup returns a NEW slice with ss's duplicates removed, preserving order;
// ss itself is left untouched. (An earlier version reused ss's backing
// array via ss[:0], which is fine for every current caller — none keep the
// original slice afterward — but is a trap for a future one that does.)
func Dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// realPath resolves path through symlinks when it exists — rc files are
// often symlinks managed by stow/chezmoi/yadm, and renaming a temp file over
// the link itself would replace it with a regular file and silently detach
// the user's dotfiles repository. A path that doesn't exist yet is returned
// as is.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// rewriteLocked atomically replaces path (already resolved through
// symlinks, see realPath) with lines joined by newlines, preserving the
// file's permissions (0o644 for a new file). The caller holds path's lock.
func rewriteLocked(path string, lines []string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return fsutil.WriteFileAtomic(path, []byte(strings.Join(lines, "\n")), mode)
}

// AppendIfMissing appends entry (preceded by comment, when non-empty) to
// path if entry is not already present in path's content, creating path
// (and a leading blank line before the new block) if necessary. Reports
// whether it actually wrote anything.
func AppendIfMissing(path, comment, entry string) (appended bool, err error) {
	// comment/entry end up verbatim in the user's shell startup file and are
	// executed on every new shell session — a caller building either from
	// data that could contain a newline (e.g. a value influenced by external
	// input) must not be able to inject extra shell commands this way.
	if strings.ContainsAny(comment, "\n\r") || strings.ContainsAny(entry, "\n\r") {
		return false, fmt.Errorf("shellrc: comment/entry não podem conter quebras de linha")
	}

	path = realPath(path)
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()

	// Matched as a whole line, not a substring — RemoveEntry below matches
	// whole lines too, and using a different criterion here would let entry
	// be judged "already present" by appearing inside some unrelated line,
	// while never matching RemoveEntry's stricter check when it's time to
	// clean it back up.
	data, _ := os.ReadFile(path) // missing file == no match yet, same as an empty one
	for _, l := range strings.Split(string(data), "\n") {
		if l == entry {
			return false, nil
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	block := entry + "\n"
	if comment != "" {
		block = comment + "\n" + block
	}
	if _, err := fmt.Fprint(f, "\n"+block); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveResult is one candidate rc file's outcome from RemoveEntry.
type RemoveResult struct {
	Path    string
	Changed bool
	Err     error
}

// RemoveEntry strips the comment and entry lines (when present, matched as
// whole lines) from each rc file in candidates, rewriting atomically. An
// empty comment matches nothing — it used to match, and delete, every blank
// line of the file. Files that don't exist, or don't contain entry/comment,
// are skipped and omitted from the returned results — only files actually
// touched (successfully or not) are reported, so callers can loop over the
// result to log outcomes however fits their UI. Read, filter and rewrite
// happen under the file's lock, so a concurrent AppendIfMissing can't be
// lost in between.
func RemoveEntry(candidates []string, comment, entry string) []RemoveResult {
	var results []RemoveResult
	for _, rc := range candidates {
		if r, touched := removeFrom(realPath(rc), comment, entry); touched {
			r.Path = rc
			results = append(results, r)
		}
	}
	return results
}

func removeFrom(path, comment, entry string) (RemoveResult, bool) {
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return RemoveResult{}, false
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	changed := false
	for _, l := range lines {
		if l == entry || (comment != "" && l == comment) {
			changed = true
			continue
		}
		out = append(out, l)
	}
	if !changed {
		return RemoveResult{}, false
	}
	err = rewriteLocked(path, out)
	return RemoveResult{Changed: err == nil, Err: err}, true
}

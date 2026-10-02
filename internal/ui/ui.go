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

package ui

import (
	"fmt"
	"io"
	"sync"

	"github.com/oito2/perci/internal/version"
)

// ── palette ──────────────────────────────────────────────────────────────────
//
// A single fixed palette for the messages domain functions write to the
// GUI's terminal panel — the GUI's own theme (GUITheme, daisyUI) is
// unrelated to it.
const (
	colorPrimary = "153;102;255" // #9966FF — dividers, titles, steps
	colorSuccess = "0;255;136"   // #00FF88
	colorErr     = "255;68;102"  // #FF4466
	colorWarning = "255;170;0"   // #FFAA00
	colorMuted   = "102;102;102" // #666666
	colorAccent  = "255;153;255" // #FF99FF — only PrintHeader's "◈ "
)

// ansiColor wraps text in a bold 24-bit ANSI foreground color escape.
func ansiColor(rgb, text string) string {
	return "\x1b[1m\x1b[38;2;" + rgb + "m" + text + "\x1b[0m"
}

// ── PrintHeader ───────────────────────────────────────────────────────────────

// PrintHeader prints a single-line header: brand identity, version and
// distroLabel (the caller's — e.g. distro.DisplayName(); empty to omit)
// on the left, the action title on the right. It doesn't clear the
// terminal: the GUI already starts each run on an empty one.
func PrintHeader(w io.Writer, title, distroLabel string) {
	left := ansiColor(colorAccent, "◈ ") +
		ansiColor(colorMuted, "perci") +
		ansiColor(colorPrimary, ".gnl") +
		ansiColor(colorMuted, "  │  ") +
		ansiColor(colorMuted, version.Version)
	if distroLabel != "" {
		left += ansiColor(colorMuted, "  │  ") + ansiColor(colorPrimary, distroLabel)
	}
	_, _ = fmt.Fprintln(w, left+ansiColor(colorMuted, "  │  ")+ansiColor(colorMuted, title))
}

// ── messages ─────────────────────────────────────────────────────────────────

// Info prints a primary-colored line for general messages.
func Info(w io.Writer, text string) {
	fmt.Fprintln(w, ansiColor(colorPrimary, text))
}

// Err prints an error-colored line.
func Err(w io.Writer, text string) {
	fmt.Fprintln(w, ansiColor(colorErr, text))
}

// Warning prints a warning-colored line.
func Warning(w io.Writer, text string) {
	fmt.Fprintln(w, ansiColor(colorWarning, text))
}

// Success prints a success-colored line.
func Success(w io.Writer, text string) {
	fmt.Fprintln(w, ansiColor(colorSuccess, text))
}

// ── Step ─────────────────────────────────────────────────────────────────────

// stepHook, when set, is notified on every Step call in addition to the
// line Step always writes to w. A GUI layer can register one to forward
// steps as events; internal/ui itself has no Wails dependency.
var (
	stepHookMu sync.Mutex
	stepHook   func(index, total int, label string)
)

// SetStepHook registers fn to be called on every Step, alongside the
// written line. Pass nil to clear it.
func SetStepHook(fn func(index, total int, label string)) {
	stepHookMu.Lock()
	defer stepHookMu.Unlock()
	stepHook = fn
}

// Step announces progress through a named step of index out of total
// (1-based) — the mechanism used for determinate progress in the GUI.
// Domain functions with multiple meaningful phases (ex.
// internal/system/update.Run) call this once per phase, computing total
// upfront from what will actually run on this machine (never a padded
// worst case — a genuinely atomic action counts as a single step, not a
// placeholder for several).
func Step(w io.Writer, index, total int, label string) {
	fmt.Fprintln(w, ansiColor(colorPrimary, fmt.Sprintf("[%d/%d] %s", index, total, label)))

	stepHookMu.Lock()
	fn := stepHook
	stepHookMu.Unlock()
	if fn != nil {
		fn(index, total, label)
	}
}

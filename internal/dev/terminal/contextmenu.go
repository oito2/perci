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

package terminal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// SyncContextMenuEntries makes the "Abrir no <terminal>" entries match
// installed (terminal name → installed, as returned by InstalledMap): added
// for every installed terminal, removed for the others. Idempotent.
func SyncContextMenuEntries(installed map[string]bool, stdout io.Writer) {
	for _, t := range Catalogue {
		if installed[t.Name] {
			AddContextMenuEntries(t, stdout)
		} else {
			RemoveContextMenuEntries(t, stdout)
		}
	}
}

// AddContextMenuEntries creates "Open here" entries for Nautilus, Nemo and Dolphin.
func AddContextMenuEntries(t Terminal, stdout io.Writer) {
	if t.DirFlag == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		ui.Warning(stdout, "Menu de contexto: falha ao obter diretório home: "+err.Error())
		return
	}
	ui.Info(stdout, "Integrando "+t.Name+" ao menu de contexto dos gerenciadores de arquivos...")
	writeContextMenuFile(nautilusScriptPath(t, home), nautilusScript(t), 0o755, "Nautilus", stdout)
	writeContextMenuFile(nemoScriptPath(t, home), nemoScript(t), 0o755, "Nemo", stdout)
	writeContextMenuFile(dolphinMenuPath(t, home), dolphinMenu(t), 0o644, "Dolphin", stdout)
}

// RemoveContextMenuEntries removes "Open here" entries for all file managers.
func RemoveContextMenuEntries(t Terminal, stdout io.Writer) {
	if t.DirFlag == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for _, p := range []string{
		nautilusScriptPath(t, home),
		nemoScriptPath(t, home),
		dolphinMenuPath(t, home),
	} {
		_ = os.Remove(p)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeContextMenuFile(path, content string, perm os.FileMode, label string, stdout io.Writer) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		ui.Warning(stdout, label+": falha ao criar diretório: "+err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		ui.Warning(stdout, label+": falha ao escrever arquivo: "+err.Error())
	}
}

func execCmd(t Terminal) string {
	if t.MenuExec != "" {
		return t.MenuExec
	}
	return t.Cmd
}

func menuSlug(t Terminal) string {
	return strings.ToLower(strings.ReplaceAll(t.Name, " ", "-"))
}

// shellescapeWords quotes each whitespace-separated word in s individually
// via executor.ShellQuote — the project's single, exported shell-escaping
// mechanism, instead of reimplementing the same single-quote-wrapping
// algorithm by hand.
func shellescapeWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = executor.ShellQuote(w)
	}
	return strings.Join(words, " ")
}

// ── Nautilus ─────────────────────────────────────────────────────────────────

func nautilusScriptPath(t Terminal, home string) string {
	return filepath.Join(home, ".local", "share", "nautilus", "scripts", "Abrir no "+t.Name)
}

func nautilusScript(t Terminal) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
URI="${NAUTILUS_SCRIPT_CURRENT_URI:-${NEMO_SCRIPT_CURRENT_URI:-}}"
DIR=$(python3 -c "import sys,urllib.parse; u=sys.argv[1]; print(urllib.parse.unquote(u[7:] if u.startswith('file://') else u))" "$URI" 2>/dev/null)
[[ -z "$DIR" ]] && DIR="$HOME"
%s %s "$DIR"
`, shellescapeWords(execCmd(t)), shellescapeWords(t.DirFlag))
}

// ── Nemo ─────────────────────────────────────────────────────────────────────

func nemoScriptPath(t Terminal, home string) string {
	return filepath.Join(home, ".local", "share", "nemo", "scripts", "Abrir no "+t.Name)
}

func nemoScript(t Terminal) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
URI="${NEMO_SCRIPT_CURRENT_URI:-${NAUTILUS_SCRIPT_CURRENT_URI:-}}"
DIR=$(python3 -c "import sys,urllib.parse; u=sys.argv[1]; print(urllib.parse.unquote(u[7:] if u.startswith('file://') else u))" "$URI" 2>/dev/null)
[[ -z "$DIR" ]] && DIR="$HOME"
%s %s "$DIR"
`, shellescapeWords(execCmd(t)), shellescapeWords(t.DirFlag))
}

// ── Dolphin ───────────────────────────────────────────────────────────────────

func dolphinMenuPath(t Terminal, home string) string {
	return filepath.Join(home, ".local", "share", "kio", "servicemenus", "perci-"+menuSlug(t)+".desktop")
}

func dolphinMenu(t Terminal) string {
	actionID := "perci-open-" + menuSlug(t)
	return fmt.Sprintf(`[Desktop Entry]
Type=Service
ServiceTypes=KonqPopupMenu/Plugin
MimeType=inode/directory;
Actions=%s;

[Desktop Action %s]
Name=Abrir no %s
Icon=utilities-terminal
Exec=%s %s %%f
`, actionID, actionID, t.Name, shellescapeWords(execCmd(t)), shellescapeWords(t.DirFlag))
}

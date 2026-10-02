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

// Package mcpservers registers MCP servers with three AI CLIs: Claude
// Code, Codex and Antigravity.
//
// Each agent has its own mechanism:
//   - Claude Code: `claude mcp add/remove` (generic servers) and
//     `claude plugin install/uninstall/update` (Dart + Flutter, distributed
//     as a marketplace plugin), both with `--scope user|local`.
//   - Codex: `codex mcp add/remove` and `codex plugin add/remove`, only at
//     Global scope; Local scope is skipped.
//   - Antigravity: reads, merges and writes its `mcp_config.json` directly
//     (Global: ~/.gemini/config/mcp_config.json; Local:
//     .agents/mcp_config.json), preserving any other entries.
//
// Each catalog server has its own install logic; there is no single
// generic function covering every entry.
package mcpservers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// ParamKind describes the extra input, if any, Install needs before running.
type ParamKind string

const (
	ParamNone      ParamKind = ""
	ParamDirectory ParamKind = "directory"
	ParamFile      ParamKind = "file"
)

// Server describes one catalog entry.
type Server struct {
	Slug string
	Name string

	// Param/ParamLabel: some servers need an extra value before Install
	// can run (Filesystem: an allowed directory; SQLite: a .sqlite file
	// path).
	Param      ParamKind
	ParamLabel string

	// Manual: true means there is no command to automate, so
	// Install/Remove/Update don't apply. ManualURL/ManualNote are shown
	// instead of the usual buttons.
	Manual     bool
	ManualURL  string
	ManualNote string
}

// Catalogue lists every MCP server offered.
var Catalogue = []Server{
	{Slug: "dart-flutter", Name: "Dart + Flutter"},
	{
		Slug: "filesystem", Name: "Filesystem MCP (@modelcontextprotocol/server-filesystem)",
		Param: ParamDirectory, ParamLabel: "Pasta permitida",
	},
	{
		Slug: "sqlite", Name: "SQLite",
		Param: ParamFile, ParamLabel: "Caminho do arquivo .sqlite",
	},
	{
		Slug: "godot-studio", Name: "Godot Studio",
		Manual:    true,
		ManualURL: "https://github.com/hi-godot/godot-ai",
		ManualNote: "Baixe uma versão publicada em GitHub Releases e extraia em <projeto>/addons/godot_ai/ " +
			"(não sobrescreva uma árvore antiga). Abra o Godot a partir da pasta do projeto e, no dock do " +
			"Godot AI, clique em \"Configure\" — o comando de conexão real (porta, versão, etc.) só existe " +
			"com o editor rodando, então o Perci não consegue automatizar esta instalação.",
	},
}

// filesystemServerPkg/sqliteServerPkg pin the exact package versions the
// agents launch, instead of whatever npx/uvx resolves as latest.
const (
	filesystemServerPkg = "@modelcontextprotocol/server-filesystem@2026.8.31"
	sqliteServerPkg     = "mcp-server-sqlite@2025.4.25"
)

// BySlug resolves a catalog entry by its Slug.
func BySlug(slug string) (Server, bool) {
	for _, s := range Catalogue {
		if s.Slug == slug {
			return s, true
		}
	}
	return Server{}, false
}

// ── shared scope handling (Claude Code + cwd-based execution) ───────────

// scopedOptions sets Dir to folder when global is false, since Claude
// Code's local scope is tied to the directory the command runs from.
func scopedOptions(stdout io.Writer, global bool, folder string) executor.Options {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	if !global {
		opts.Dir = folder
	}
	return opts
}

func claudeScope(global bool) string {
	if global {
		return "user"
	}
	return "local"
}

// ── Antigravity: direct mcp_config.json editing ──────────────────────────

func antigravityConfigPath(global bool, folder string) (string, error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("obter diretório home: %w", err)
		}
		return filepath.Join(home, ".gemini", "config", "mcp_config.json"), nil
	}
	return filepath.Join(folder, ".agents", "mcp_config.json"), nil
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsear %s: %w", path, err)
	}
	return doc, nil
}

// writeJSONObject replaces path atomically, keeping its current
// permissions (0600 for a new file) and without escaping <, > and & the
// way json.Marshal does by default.
func writeJSONObject(path string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", filepath.Dir(path), err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := fsutil.WriteFileAtomic(path, buf.Bytes(), mode); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	return nil
}

// mcpServersOf returns doc's "mcpServers" object, creating it when absent;
// a value of any other type is an error rather than silently replaced.
func mcpServersOf(doc map[string]any, path string) (map[string]any, error) {
	raw, present := doc["mcpServers"]
	if !present || raw == nil {
		servers := map[string]any{}
		doc["mcpServers"] = servers
		return servers, nil
	}
	servers, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: \"mcpServers\" não é um objeto — corrija o arquivo manualmente", path)
	}
	return servers, nil
}

func antigravitySetServer(global bool, folder, name, command string, args []string) error {
	path, err := antigravityConfigPath(global, folder)
	if err != nil {
		return err
	}
	doc, err := readJSONObject(path)
	if err != nil {
		return err
	}
	servers, err := mcpServersOf(doc, path)
	if err != nil {
		return err
	}
	servers[name] = map[string]any{"command": command, "args": args}
	return writeJSONObject(path, doc)
}

func antigravityRemoveServer(global bool, folder, name string) error {
	path, err := antigravityConfigPath(global, folder)
	if err != nil {
		return err
	}
	doc, err := readJSONObject(path)
	if err != nil {
		return err
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		return nil // no file, or nothing registered there — nothing to write
	}
	if _, registered := servers[name]; !registered {
		return nil
	}
	delete(servers, name)
	return writeJSONObject(path, doc)
}

// ── per-agent results ─────────────────────────────────────────────────────

// agentResults collects one install/update attempt's outcome per agent: a
// partial success stays a warning reported inline, but when no agent
// accepted it the whole action fails.
type agentResults struct {
	stdout io.Writer
	ok     int
	errs   []error
}

func (r *agentResults) done(agent, successMsg string, err error) {
	if err != nil {
		ui.Warning(r.stdout, agent+": "+err.Error())
		r.errs = append(r.errs, fmt.Errorf("%s: %w", agent, err))
		return
	}
	r.ok++
	ui.Success(r.stdout, agent+": "+successMsg)
}

func (r *agentResults) err(what string) error {
	if r.ok > 0 || len(r.errs) == 0 {
		return nil
	}
	return fmt.Errorf("nenhum agente aceitou %s: %w", what, errors.Join(r.errs...))
}

// ── generic registration (Filesystem, SQLite) ────────────────────────────

func installGeneric(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string, global bool, folder, command string, cmdArgs []string) error {
	opts := scopedOptions(stdout, global, folder)
	res := &agentResults{stdout: stdout}

	claudeArgs := append([]string{"mcp", "add", "--scope", claudeScope(global), name, "--", command}, cmdArgs...)
	res.done("Claude Code", name+" registrado.", exe.Run(ctx, opts, "claude", claudeArgs...))

	if global {
		codexOpts := executor.Options{Stdout: stdout, Stderr: stdout}
		codexArgs := append([]string{"mcp", "add", name, "--", command}, cmdArgs...)
		res.done("Codex", name+" registrado.", exe.Run(ctx, codexOpts, "codex", codexArgs...))
	} else {
		ui.Warning(stdout, "Codex: escopo Local não é suportado de forma confirmada nesta versão — pulado (registre em Global se precisar do Codex).")
	}

	res.done("Antigravity", name+" registrado em mcp_config.json.", antigravitySetServer(global, folder, name, command, cmdArgs))
	return res.err("o registro de " + name)
}

func removeGeneric(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string, global bool, folder string) {
	opts := scopedOptions(stdout, global, folder)

	if err := exe.Run(ctx, opts, "claude", "mcp", "remove", name, "--scope", claudeScope(global)); err != nil {
		ui.Warning(stdout, "Claude Code: "+err.Error())
	} else {
		ui.Success(stdout, "Claude Code: "+name+" removido.")
	}

	if global {
		if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "codex", "mcp", "remove", name); err != nil {
			ui.Warning(stdout, "Codex: "+err.Error())
		} else {
			ui.Success(stdout, "Codex: "+name+" removido.")
		}
	}

	if err := antigravityRemoveServer(global, folder, name); err != nil {
		ui.Warning(stdout, "Antigravity: "+err.Error())
	} else {
		ui.Success(stdout, "Antigravity: "+name+" removido de mcp_config.json.")
	}
}

// ── Dart + Flutter (plugin via the flutter/agent-plugins marketplace) ──

const (
	// dartFlutterPluginRef is PLUGIN@MARKETPLACE for both CLIs.
	dartFlutterPluginRef    = "dart-flutter@dart-flutter"
	dartFlutterMarketplace  = "flutter/agent-plugins"
	dartFlutterCodexMarket  = "flutter/agent-plugins"
	antigravityNativeNotice = "Antigravity: nativo — nenhuma instalação necessária."
)

// installDartFlutter fails only when neither Claude Code nor Codex took
// the plugin (Antigravity needs nothing, it is native).
func installDartFlutter(ctx context.Context, exe *executor.Executor, stdout io.Writer, global bool, folder string) error {
	ui.Info(stdout, antigravityNativeNotice)
	res := &agentResults{stdout: stdout}

	opts := scopedOptions(stdout, global, folder)
	if err := exe.Run(ctx, opts, "claude", "plugin", "marketplace", "add", dartFlutterMarketplace); err != nil {
		ui.Warning(stdout, "Claude Code: adicionar marketplace: "+err.Error())
	}
	res.done("Claude Code", "plugin instalado.", exe.Run(ctx, opts, "claude", "plugin", "install", dartFlutterPluginRef, "--scope", claudeScope(global), "-y"))

	if global {
		codexOpts := executor.Options{Stdout: stdout, Stderr: stdout}
		if err := exe.Run(ctx, codexOpts, "codex", "plugin", "marketplace", "add", dartFlutterCodexMarket); err != nil {
			ui.Warning(stdout, "Codex: adicionar marketplace: "+err.Error())
		}
		res.done("Codex", "plugin instalado.", exe.Run(ctx, codexOpts, "codex", "plugin", "add", dartFlutterPluginRef))
	} else {
		ui.Warning(stdout, "Codex: escopo Local não é suportado de forma confirmada nesta versão — pulado.")
	}
	return res.err("o plugin dart-flutter")
}

func removeDartFlutter(ctx context.Context, exe *executor.Executor, stdout io.Writer, global bool, folder string) {
	opts := scopedOptions(stdout, global, folder)
	if err := exe.Run(ctx, opts, "claude", "plugin", "uninstall", dartFlutterPluginRef, "--scope", claudeScope(global), "-y"); err != nil {
		ui.Warning(stdout, "Claude Code: "+err.Error())
	} else {
		ui.Success(stdout, "Claude Code: plugin removido.")
	}
	if global {
		if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "codex", "plugin", "remove", dartFlutterPluginRef); err != nil {
			ui.Warning(stdout, "Codex: "+err.Error())
		} else {
			ui.Success(stdout, "Codex: plugin removido.")
		}
	}
}

func updateDartFlutter(ctx context.Context, exe *executor.Executor, stdout io.Writer, global bool, folder string) error {
	res := &agentResults{stdout: stdout}
	opts := scopedOptions(stdout, global, folder)
	res.done("Claude Code", "plugin atualizado.", exe.Run(ctx, opts, "claude", "plugin", "update", dartFlutterPluginRef, "--scope", claudeScope(global)))
	if global {
		// Codex has no plugin update, so this removes and reinstalls.
		codexOpts := executor.Options{Stdout: stdout, Stderr: stdout}
		_ = exe.Run(ctx, codexOpts, "codex", "plugin", "remove", dartFlutterPluginRef)
		res.done("Codex", "plugin reinstalado (sem update nativo confirmado).", exe.Run(ctx, codexOpts, "codex", "plugin", "add", dartFlutterPluginRef))
	}
	return res.err("a atualização do plugin dart-flutter")
}

// ── catalog dispatch ──────────────────────────────────────────────────────

// validateParam checks the extra parameter filesystem/sqlite need: an
// absolute path (a directory or a .sqlite file), which also means it can
// never start with "-" and be read as an option by npx/uvx.
func validateParam(slug, param string) error {
	switch slug {
	case "filesystem":
		if param == "" {
			return fmt.Errorf("selecione uma pasta permitida antes de instalar")
		}
	case "sqlite":
		if param == "" {
			return fmt.Errorf("informe o caminho do arquivo .sqlite antes de instalar")
		}
	default:
		return nil
	}
	if !filepath.IsAbs(param) {
		return fmt.Errorf("o parâmetro de %s deve ser um caminho absoluto (recebido: %q)", slug, param)
	}
	return nil
}

// Install registers slug with every supported agent. param is required for
// "filesystem" (an allowed directory) and "sqlite" (a .sqlite file path);
// ignored otherwise. Fails when no agent accepted the registration.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder, param string) error {
	if err := validateParam(slug, param); err != nil {
		return err
	}
	switch slug {
	case "dart-flutter":
		return installDartFlutter(ctx, exe, stdout, global, folder)
	case "filesystem":
		return installGeneric(ctx, exe, stdout, "filesystem", global, folder, "npx", []string{"-y", filesystemServerPkg, param})
	case "sqlite":
		return installGeneric(ctx, exe, stdout, "sqlite", global, folder, "uvx", []string{sqliteServerPkg, "--db-path", param})
	}
	return fmt.Errorf("instalação não suportada para %s", slug)
}

// Remove unregisters slug from every supported agent. Failures stay
// warnings: a server already missing from one agent (removed by hand) must
// still be removable from Perci's list.
func Remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder string) error {
	switch slug {
	case "dart-flutter":
		removeDartFlutter(ctx, exe, stdout, global, folder)
		return nil
	case "filesystem":
		removeGeneric(ctx, exe, stdout, "filesystem", global, folder)
		return nil
	case "sqlite":
		removeGeneric(ctx, exe, stdout, "sqlite", global, folder)
		return nil
	}
	return fmt.Errorf("remoção não suportada para %s", slug)
}

// Update refreshes slug's registration. Claude Code has a native "plugin
// update" for dart-flutter; every other case runs Remove+Install with the
// same param, validated first so a missing parameter can't remove the
// server and then fail.
func Update(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder, param string) error {
	switch slug {
	case "dart-flutter":
		return updateDartFlutter(ctx, exe, stdout, global, folder)
	case "filesystem", "sqlite":
		if err := validateParam(slug, param); err != nil {
			return err
		}
		removeGeneric(ctx, exe, stdout, slug, global, folder)
		if err := Install(ctx, exe, stdout, slug, global, folder, param); err != nil {
			return fmt.Errorf("%w: %w", ErrNotReinstalled, err)
		}
		return nil
	}
	return fmt.Errorf("atualização não suportada para %s", slug)
}

// ErrNotReinstalled means Update removed the server but no agent accepted
// the new registration — it's no longer registered anywhere.
var ErrNotReinstalled = errors.New("o MCP foi removido, mas não pôde ser registrado de novo")

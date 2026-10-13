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
	"slices"
	"strings"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/dev/nettimeout"
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

	// Note is a short description shown under the name.
	Note string

	// Param/ParamLabel: some servers need an extra value before Install
	// can run (Filesystem: an allowed directory; DBHub: its dbhub.toml).
	Param      ParamKind
	ParamLabel string

	// Retired: the entry left the catalog. It is listed only where it is
	// still registered, and can only be removed.
	Retired bool

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
		Slug: "context7", Name: "Context7",
		Note: "Documentação atualizada de bibliotecas e frameworks. As consultas são enviadas para context7.com.",
	},
	{
		Slug: "gopls", Name: "Go: gopls MCP",
		Note: "Diagnósticos, referências e vulncheck do servidor de linguagem oficial do Go. Requer o gopls no PATH " +
			"(go install golang.org/x/tools/gopls@v0.23.0); usa a pasta em que o agente é aberto como workspace.",
	},
	{
		Slug: "chrome-devtools", Name: "Chrome DevTools MCP",
		Note: "Navegação, console, rede e performance no navegador. Requer o Google Chrome; abre um perfil " +
			"temporário isolado e a telemetria de uso fica desligada.",
	},
	{
		Slug: "svelte", Name: "Svelte MCP (oficial)",
		Note: "Documentação do Svelte/SvelteKit e verificação de componentes.",
	},
	{
		Slug: "dbhub", Name: "DBHub (MySQL, MariaDB, SQLite...)",
		Note: "Bancos configurados num dbhub.toml ([[sources]] com o dsn). Para somente leitura, adicione " +
			"[[tools]] com name = \"execute_sql\" e readonly = true. A senha fica no arquivo — mantenha-o fora do git.",
		Param: ParamFile, ParamLabel: "Arquivo dbhub.toml",
	},
	{
		Slug: "sqlite", Name: "SQLite",
		Note:  "Fora do catálogo: o servidor foi arquivado e permite escrita. Use o DBHub.",
		Param: ParamFile, ParamLabel: "Caminho do arquivo .sqlite",
		Retired: true,
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

// The *ServerPkg constants pin the exact package versions the agents
// launch, instead of whatever npx/uvx resolves as latest.
const (
	filesystemServerPkg     = "@modelcontextprotocol/server-filesystem@1.0.0"
	sqliteServerPkg         = "mcp-server-sqlite@2025.4.25"
	context7ServerPkg       = "@upstash/context7-mcp@4.4.1"
	chromeDevToolsServerPkg = "chrome-devtools-mcp@1.10.1"
	svelteServerPkg         = "@sveltejs/mcp@0.1.26"
	dbhubServerPkg          = "@bytebase/dbhub@1.5.0"
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

// The agents a server is registered with, as recorded in config.yaml.
const (
	AgentClaude      = "claude"
	AgentCodex       = "codex"
	AgentAntigravity = "antigravity"
)

// AgentLabel is the name the interface shows for an agent ID.
func AgentLabel(id string) string {
	switch id {
	case AgentClaude:
		return "Claude"
	case AgentCodex:
		return "Codex"
	case AgentAntigravity:
		return "Antigravity"
	}
	return id
}

// ExpectedAgents lists the agents a registration of slug is meant to reach
// in the scope: Codex only takes Global registrations, and Antigravity
// ships Dart + Flutter natively, so it has nothing to register there.
func ExpectedAgents(slug string, global bool) []string {
	agents := []string{AgentClaude}
	if global {
		agents = append(agents, AgentCodex)
	}
	if slug != "dart-flutter" {
		agents = append(agents, AgentAntigravity)
	}
	return agents
}

// genericServer describes a server registered through the agents' plain
// command + arguments form: the launcher command, the marker found among
// the arguments of an entry Perci registered (the package name without its
// version, or the subcommand), the arguments for a given parameter, and the
// error shown when the launcher is not installed.
type genericServer struct {
	command string
	marker  string
	args    func(param string) []string
	missing string
}

const npxMissing = "npx não encontrado — instale o Node.js em Pré-requisitos antes de registrar este MCP"

// npxServer is a server launched as `npx -y <pkg> <extra...>`.
func npxServer(pkg string, extra ...string) genericServer {
	return genericServer{
		command: "npx",
		marker:  pkgBase(pkg),
		args:    func(string) []string { return append([]string{"-y", pkg}, extra...) },
		missing: npxMissing,
	}
}

// genericServers maps each catalog slug registered as a plain command to
// how it is launched.
var genericServers = map[string]genericServer{
	"filesystem": {
		command: "npx", marker: pkgBase(filesystemServerPkg),
		args:    func(dir string) []string { return []string{"-y", filesystemServerPkg, dir} },
		missing: npxMissing,
	},
	"sqlite": {
		command: "uvx", marker: pkgBase(sqliteServerPkg),
		args:    func(db string) []string { return []string{sqliteServerPkg, "--db-path", db} },
		missing: "uvx não encontrado — instale o uv (https://docs.astral.sh/uv/) antes de registrar o MCP SQLite",
	},
	"context7":        npxServer(context7ServerPkg),
	"chrome-devtools": npxServer(chromeDevToolsServerPkg, "--isolated", "--no-usage-statistics"),
	"svelte":          npxServer(svelteServerPkg),
	"dbhub": {
		command: "npx", marker: pkgBase(dbhubServerPkg),
		args:    func(cfg string) []string { return []string{"-y", dbhubServerPkg, "--config=" + cfg} },
		missing: npxMissing,
	},
	"gopls": {
		command: "gopls", marker: "mcp",
		args:    func(string) []string { return []string{"mcp"} },
		missing: "gopls não encontrado no PATH — instale com `go install golang.org/x/tools/gopls@v0.23.0` e adicione o diretório de binários do Go (`go env GOPATH`/bin) ao PATH",
	},
}

// pkgBase strips the "@version" suffix of an npm/PyPI package reference.
func pkgBase(ref string) string {
	if i := strings.LastIndex(ref, "@"); i > 0 {
		return ref[:i]
	}
	return ref
}

// createdByPerci reports whether a server entry with this command and
// arguments is the one Perci registers for slug, whatever the package
// version or the parameter: the entry's command and marker match.
func createdByPerci(slug, command string, args []string) bool {
	g, ok := genericServers[slug]
	if !ok || command != g.command {
		return false
	}
	return slices.ContainsFunc(args, func(a string) bool { return a == g.marker || strings.HasPrefix(a, g.marker+"@") })
}

// errNotCreatedByPerci means a server of that name exists but was not
// registered by Perci, so it is left alone.
var errNotCreatedByPerci = errors.New("já existe um servidor com esse nome que não foi registrado pelo Perci — foi mantido")

// requireTool fails when the launcher a generic server needs is not
// installed.
func requireTool(ctx context.Context, exe *executor.Executor, slug string) error {
	g, ok := genericServers[slug]
	if !ok || localbin.Which(ctx, exe, g.command) {
		return nil
	}
	return errors.New(g.missing)
}

// warnMissingChrome tells the user, without failing, when no Google Chrome
// binary is found for the Chrome DevTools MCP, which launches the stable
// channel.
func warnMissingChrome(ctx context.Context, exe *executor.Executor, stdout io.Writer) {
	for _, c := range []string{"google-chrome", "google-chrome-stable"} {
		if localbin.Which(ctx, exe, c) {
			return
		}
	}
	ui.Warning(stdout, "Google Chrome não encontrado — o Chrome DevTools MCP só funciona com o Chrome instalado.")
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
	// UseNumber keeps numbers exactly as written instead of rounding them
	// through float64.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsear %s: %w", path, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%s não contém um objeto JSON — corrija o arquivo manualmente", path)
	}
	return doc, nil
}

// writeJSONObject writes doc to path through fsutil.WriteThroughLink: a
// symlink at path keeps pointing at its target, no backup copy is left
// next to a file that may hold tokens, and the file's permissions are
// preserved (0600 for a new file). <, > and & are not escaped the way json.Marshal does by
// default.
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
	if err := fsutil.WriteThroughLink(path, buf.Bytes(), mode); err != nil {
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

// entryCommand reads the command and arguments of one mcpServers entry.
func entryCommand(entry any) (command string, args []string) {
	m, ok := entry.(map[string]any)
	if !ok {
		return "", nil
	}
	command, _ = m["command"].(string)
	raw, _ := m["args"].([]any)
	for _, a := range raw {
		if str, ok := a.(string); ok {
			args = append(args, str)
		}
	}
	return command, args
}

// antigravitySetServer registers the server in Antigravity's config. An
// existing entry with that name is replaced only when Perci created it.
func antigravitySetServer(slug string, global bool, folder, name, command string, args []string) error {
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
	if existing, present := servers[name]; present {
		if c, a := entryCommand(existing); !createdByPerci(slug, c, a) {
			return fmt.Errorf("%s: %w", path, errNotCreatedByPerci)
		}
	}
	servers[name] = map[string]any{"command": command, "args": args}
	return writeJSONObject(path, doc)
}

// antigravityRemoveServer removes the server from Antigravity's config,
// keeping an entry that Perci did not create.
func antigravityRemoveServer(slug string, global bool, folder, name string) error {
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
	existing, registered := servers[name]
	if !registered {
		return nil
	}
	if c, a := entryCommand(existing); !createdByPerci(slug, c, a) {
		return fmt.Errorf("%s: %w", path, errNotCreatedByPerci)
	}
	delete(servers, name)
	return writeJSONObject(path, doc)
}

// ── Claude Code / Codex: reading what is registered ─────────────────────

// claudeConfigPath is Claude Code's user state file.
func claudeConfigPath() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, ".claude.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

// claudeEntry looks name up in Claude Code's state file — the user scope's
// mcpServers when global, otherwise the project's at folder — without
// starting the server. found is false when it is not registered there.
func claudeEntry(name string, global bool, folder string) (command string, args []string, found bool, err error) {
	path, err := claudeConfigPath()
	if err != nil {
		return "", nil, false, err
	}
	doc, err := readJSONObject(path)
	if err != nil {
		return "", nil, false, err
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if !global {
		projects, _ := doc["projects"].(map[string]any)
		project, ok := projects[folder].(map[string]any)
		if !ok {
			if real, evalErr := filepath.EvalSymlinks(folder); evalErr == nil {
				project, _ = projects[real].(map[string]any)
			}
		}
		servers, _ = project["mcpServers"].(map[string]any)
	}
	entry, ok := servers[name]
	if !ok {
		return "", nil, false, nil
	}
	command, args = entryCommand(entry)
	return command, args, true, nil
}

// codexEntry looks name up through `codex mcp get`. found is false when
// Codex has no such server.
func codexEntry(ctx context.Context, exe *executor.Executor, name string) (command string, args []string, found bool) {
	out, err := exe.Output(ctx, executor.Options{}, "codex", "mcp", "get", name, "--json")
	if err != nil {
		return "", nil, false
	}
	var payload struct {
		Transport struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"transport"`
	}
	// An output that cannot be read still means the name is taken.
	_ = json.Unmarshal([]byte(out), &payload)
	return payload.Transport.Command, payload.Transport.Args, true
}

// ── per-agent results ─────────────────────────────────────────────────────

// agentResults collects one install/update attempt's outcome per agent: a
// partial success stays a warning reported inline, but when no agent
// accepted it the whole action fails.
type agentResults struct {
	stdout   io.Writer
	ok       int
	errs     []error
	accepted []string
}

// done records agent's outcome; id is its AgentClaude/AgentCodex/
// AgentAntigravity ID and label the name shown in the messages.
func (r *agentResults) done(id, label, successMsg string, err error) {
	if err != nil {
		ui.Warning(r.stdout, label+": "+err.Error())
		r.errs = append(r.errs, fmt.Errorf("%s: %w", label, err))
		return
	}
	r.ok++
	r.accepted = append(r.accepted, id)
	ui.Success(r.stdout, label+": "+successMsg)
}

func (r *agentResults) err(what string) error {
	if r.ok > 0 || len(r.errs) == 0 {
		return nil
	}
	return fmt.Errorf("nenhum agente aceitou %s: %w", what, errors.Join(r.errs...))
}

// ── generic registration ─────────────────────────────────────────────────

// installGeneric registers a generic server with every agent that applies
// and returns the agents that accepted it.
func installGeneric(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder, command string, cmdArgs []string) ([]string, error) {
	name := slug
	opts := scopedOptions(stdout, global, folder)
	res := &agentResults{stdout: stdout}

	claudeArgs := append([]string{"mcp", "add", "--scope", claudeScope(global), name, "--", command}, cmdArgs...)
	res.done(AgentClaude, "Claude Code", name+" registrado.", exe.Run(ctx, opts, "claude", claudeArgs...))

	if global {
		res.done(AgentCodex, "Codex", name+" registrado.", codexAdd(ctx, exe, stdout, slug, name, command, cmdArgs))
	} else {
		ui.Warning(stdout, "Codex: escopo Local não é suportado de forma confirmada nesta versão — pulado (registre em Global se precisar do Codex).")
	}

	res.done(AgentAntigravity, "Antigravity", name+" registrado em mcp_config.json.", antigravitySetServer(slug, global, folder, name, command, cmdArgs))
	return res.accepted, res.err("o registro de " + name)
}

// codexAdd registers the server with Codex, whose `mcp add` silently
// replaces an entry of the same name: an entry Perci did not create is
// refused.
func codexAdd(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug, name, command string, cmdArgs []string) error {
	if c, a, found := codexEntry(ctx, exe, name); found && !createdByPerci(slug, c, a) {
		return errNotCreatedByPerci
	}
	args := append([]string{"mcp", "add", name, "--", command}, cmdArgs...)
	return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "codex", args...)
}

func removeGeneric(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder string) {
	name := slug
	opts := scopedOptions(stdout, global, folder)

	switch command, args, found, err := claudeEntry(name, global, folder); {
	case err != nil:
		ui.Warning(stdout, "Claude Code: não foi possível conferir o registro: "+err.Error())
	case !found:
		ui.Info(stdout, "Claude Code: "+name+" não estava registrado.")
	case !createdByPerci(slug, command, args):
		ui.Warning(stdout, "Claude Code: "+errNotCreatedByPerci.Error()+".")
	default:
		if err := exe.Run(ctx, opts, "claude", "mcp", "remove", name, "--scope", claudeScope(global)); err != nil {
			ui.Warning(stdout, "Claude Code: "+err.Error())
		} else {
			ui.Success(stdout, "Claude Code: "+name+" removido.")
		}
	}

	if global {
		switch command, args, found := codexEntry(ctx, exe, name); {
		case !found:
			ui.Info(stdout, "Codex: "+name+" não estava registrado.")
		case !createdByPerci(slug, command, args):
			ui.Warning(stdout, "Codex: "+errNotCreatedByPerci.Error()+".")
		default:
			if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "codex", "mcp", "remove", name); err != nil {
				ui.Warning(stdout, "Codex: "+err.Error())
			} else {
				ui.Success(stdout, "Codex: "+name+" removido.")
			}
		}
	}

	if err := antigravityRemoveServer(slug, global, folder, name); err != nil {
		ui.Warning(stdout, "Antigravity: "+err.Error())
	} else {
		ui.Success(stdout, "Antigravity: "+name+" removido de mcp_config.json.")
	}
}

// ── Dart + Flutter (plugin via the flutter/agent-plugins marketplace) ──

const (
	// dartFlutterPluginRef is PLUGIN@MARKETPLACE for both CLIs.
	dartFlutterPluginRef   = "dart-flutter@dart-flutter"
	dartFlutterMarketplace = "flutter/agent-plugins"
	// dartFlutterCodexMarket is the marketplace repository at a fixed
	// commit, in the owner/repo@ref form `codex plugin marketplace add`
	// takes.
	dartFlutterCodexMarket  = "flutter/agent-plugins@0ef3972f93e2baa4156ba1cbb1e515cd53079c68"
	antigravityNativeNotice = "Antigravity: nativo — nenhuma instalação necessária."
)

// installDartFlutter fails only when neither Claude Code nor Codex took
// the plugin (Antigravity needs nothing, it is native).
func installDartFlutter(ctx context.Context, exe *executor.Executor, stdout io.Writer, global bool, folder string) ([]string, error) {
	ui.Info(stdout, antigravityNativeNotice)
	res := &agentResults{stdout: stdout}

	ctx, cancel := nettimeout.Context(ctx)
	defer cancel()
	opts := scopedOptions(stdout, global, folder)
	if err := exe.Run(ctx, opts, "claude", "plugin", "marketplace", "add", dartFlutterMarketplace); err != nil {
		ui.Warning(stdout, "Claude Code: adicionar marketplace: "+err.Error())
	}
	res.done(AgentClaude, "Claude Code", "plugin instalado.", exe.Run(ctx, opts, "claude", "plugin", "install", dartFlutterPluginRef, "--scope", claudeScope(global), "-y"))

	if global {
		codexOpts := executor.Options{Stdout: stdout, Stderr: stdout}
		if err := exe.Run(ctx, codexOpts, "codex", "plugin", "marketplace", "add", dartFlutterCodexMarket); err != nil {
			ui.Warning(stdout, "Codex: adicionar marketplace: "+err.Error())
		}
		res.done(AgentCodex, "Codex", "plugin instalado.", exe.Run(ctx, codexOpts, "codex", "plugin", "add", dartFlutterPluginRef))
	} else {
		ui.Warning(stdout, "Codex: escopo Local não é suportado de forma confirmada nesta versão — pulado.")
	}
	return res.accepted, res.err("o plugin dart-flutter")
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

func updateDartFlutter(ctx context.Context, exe *executor.Executor, stdout io.Writer, global bool, folder string) ([]string, error) {
	res := &agentResults{stdout: stdout}
	ctx, cancel := nettimeout.Context(ctx)
	defer cancel()
	opts := scopedOptions(stdout, global, folder)
	res.done(AgentClaude, "Claude Code", "plugin atualizado.", exe.Run(ctx, opts, "claude", "plugin", "update", dartFlutterPluginRef, "--scope", claudeScope(global)))
	if global {
		// Codex has no plugin update, so this removes and reinstalls.
		codexOpts := executor.Options{Stdout: stdout, Stderr: stdout}
		_ = exe.Run(ctx, codexOpts, "codex", "plugin", "remove", dartFlutterPluginRef)
		res.done(AgentCodex, "Codex", "plugin reinstalado (sem update nativo confirmado).", exe.Run(ctx, codexOpts, "codex", "plugin", "add", dartFlutterPluginRef))
	}
	return res.accepted, res.err("a atualização do plugin dart-flutter")
}

// ── catalog dispatch ──────────────────────────────────────────────────────

// validateParam checks the extra parameter a server takes (Filesystem's
// directory, DBHub's dbhub.toml): an absolute path, which also means it can
// never start with "-" and be read as an option by the launcher.
func validateParam(slug, param string) error {
	sv, ok := BySlug(slug)
	if !ok || sv.Param == ParamNone {
		return nil
	}
	if param == "" {
		return fmt.Errorf("selecione o parâmetro (%s) antes de instalar", strings.ToLower(sv.ParamLabel))
	}
	if !filepath.IsAbs(param) {
		return fmt.Errorf("o parâmetro de %s deve ser um caminho absoluto (recebido: %q)", slug, param)
	}
	return nil
}

// refuseRetired fails for an entry that left the catalog: it can still be
// removed, but not installed or updated.
func refuseRetired(slug string) error {
	if sv, ok := BySlug(slug); ok && sv.Retired {
		return fmt.Errorf("%s saiu do catálogo do Perci — só pode ser removido", sv.Name)
	}
	return nil
}

// Install registers slug with every supported agent and returns the IDs of
// the agents that accepted it. param is required for the servers that take
// one (Server.Param) and ignored otherwise. Fails when no agent accepted the
// registration, when the server's launcher is not installed, and for an
// entry that left the catalog.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder, param string) ([]string, error) {
	if err := refuseRetired(slug); err != nil {
		return nil, err
	}
	if err := validateParam(slug, param); err != nil {
		return nil, err
	}
	if err := requireTool(ctx, exe, slug); err != nil {
		return nil, err
	}
	if slug == "dart-flutter" {
		return installDartFlutter(ctx, exe, stdout, global, folder)
	}
	g, ok := genericServers[slug]
	if !ok {
		return nil, fmt.Errorf("instalação não suportada para %s", slug)
	}
	if slug == "chrome-devtools" {
		warnMissingChrome(ctx, exe, stdout)
	}
	return installGeneric(ctx, exe, stdout, slug, global, folder, g.command, g.args(param))
}

// Remove unregisters slug from every supported agent. Failures stay
// warnings: a server already missing from one agent (removed by hand) must
// still be removable from Perci's list. A server of the same name that
// Perci did not register is kept.
func Remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder string) error {
	if slug == "dart-flutter" {
		removeDartFlutter(ctx, exe, stdout, global, folder)
		return nil
	}
	if _, ok := genericServers[slug]; !ok {
		return fmt.Errorf("remoção não suportada para %s", slug)
	}
	removeGeneric(ctx, exe, stdout, slug, global, folder)
	return nil
}

// Update refreshes slug's registration and returns the agents that hold it
// afterwards. Claude Code has a native "plugin update" for dart-flutter;
// every other case runs Remove+Install with the same param, validated
// first so a missing parameter or launcher can't remove the server and
// then fail.
func Update(ctx context.Context, exe *executor.Executor, stdout io.Writer, slug string, global bool, folder, param string) ([]string, error) {
	if err := refuseRetired(slug); err != nil {
		return nil, err
	}
	if slug == "dart-flutter" {
		return updateDartFlutter(ctx, exe, stdout, global, folder)
	}
	if _, ok := genericServers[slug]; !ok {
		return nil, fmt.Errorf("atualização não suportada para %s", slug)
	}
	if err := validateParam(slug, param); err != nil {
		return nil, err
	}
	if err := requireTool(ctx, exe, slug); err != nil {
		return nil, err
	}
	removeGeneric(ctx, exe, stdout, slug, global, folder)
	agents, err := Install(ctx, exe, stdout, slug, global, folder, param)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotReinstalled, err)
	}
	return agents, nil
}

// ErrNotReinstalled means Update removed the server but no agent accepted
// the new registration — it's no longer registered anywhere.
var ErrNotReinstalled = errors.New("o MCP foi removido, mas não pôde ser registrado de novo")

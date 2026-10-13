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

// Package agentskills installs, updates and removes third-party "agent
// skills" (SKILL.md packages taught to a coding agent) by running the
// `skills` CLI through npx.
package agentskills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/dev/nettimeout"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Skill describes one installable skills.sh skill.
type Skill struct {
	Slug string // internal catalog id (config.AgentSkillInstall.Slug, DOM ids)
	Name string // display name
	Repo string // full GitHub URL of the repository
	Ref  string // full commit SHA of Repo the skill is installed from
	Arg  string // value passed to --skill; "*" installs every skill in Repo

	// Retired: the entry left the catalog. It is listed only where it is
	// still installed, and can only be removed.
	Retired bool
}

// Catalogue lists every skill offered.
var Catalogue = []Skill{
	{Slug: "godot-ui", Name: "Godot: UI", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Ref: "0a70cfc64672d512c5b5b6006b5dcb5cf90dbdbd", Arg: "godot-ui-control"},
	{Slug: "godot-master", Name: "Godot: Master", Repo: "https://github.com/thedivergentai/Game-Design-Agentic-Skills", Ref: "eca389a58929b6624d5af1e857de4b76827b8802", Arg: "godot-master"},
	{Slug: "godot-mobile", Name: "Godot: Mobile (Android/iOS)", Repo: "https://github.com/thedivergentai/Game-Design-Agentic-Skills", Ref: "eca389a58929b6624d5af1e857de4b76827b8802", Arg: "godot-platform-mobile"},
	{Slug: "godot-animations", Name: "Godot: Animations", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Ref: "0a70cfc64672d512c5b5b6006b5dcb5cf90dbdbd", Arg: "godot-animation"},
	{Slug: "godot-shaders", Name: "Godot: Shaders", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Ref: "0a70cfc64672d512c5b5b6006b5dcb5cf90dbdbd", Arg: "godot-shaders"},
	{Slug: "godot-gdscript", Name: "Godot: GDScript", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Ref: "0a70cfc64672d512c5b5b6006b5dcb5cf90dbdbd", Arg: "godot-gdscript"},
	{Slug: "godot-export", Name: "Godot: Export & Builds", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Ref: "0a70cfc64672d512c5b5b6006b5dcb5cf90dbdbd", Arg: "godot-export"},
	{Slug: "godot-patterns", Name: "Godot: Patterns", Repo: "https://github.com/wshobson/agents", Ref: "46891e7e60da0e52baf1050b7b6391b64e84c6d9", Arg: "godot-gdscript-patterns"},
	{Slug: "moodle-theme", Name: "Moodle: Theme v5+", Repo: "https://github.com/nicolasflores9/skills", Ref: "6bc130aedb0ababf8cb7cb84142ec011a68c40f1", Arg: "moodle5-theme"},
	{Slug: "moodle-external-api", Name: "Moodle: External API", Repo: "https://github.com/sickn33/agentic-awesome-skills", Ref: "128d40a8df9970b94c822b46ea7147be62665212", Arg: "moodle-external-api-development"},
	// Arg "*" installs every skill in the repo.
	{Slug: "daisyui", Name: "DaisyUI", Repo: "https://github.com/saadeghi/daisyui", Ref: "bf3bac3a397b543e55641ecb2d60a2ed961f09ec", Arg: "*"},
	{Slug: "shadcn-ui", Name: "ShaCN UI", Repo: "https://github.com/shadcn-ui/ui", Ref: "30a58d4498f270af1c1e981a7d78a15cf5740841", Arg: "shadcn"},
	{Slug: "frontend-design", Name: "Frontend Design", Repo: "https://github.com/anthropics/skills", Ref: "dbd4588f9e1033efb41dad4bef2f7947c8993d44", Arg: "frontend-design"},
	{Slug: "svelte-code-writer", Name: "Svelte: Code Writer (oficial)", Repo: "https://github.com/sveltejs/ai-tools", Ref: "6b5d0dab3c9c083387247ab20dc684573481076b", Arg: "svelte-code-writer"},
	{Slug: "svelte-best-practices", Name: "Svelte: Best Practices (oficial)", Repo: "https://github.com/sveltejs/ai-tools", Ref: "6b5d0dab3c9c083387247ab20dc684573481076b", Arg: "svelte-core-bestpractices"},
	{Slug: "react-best-practices", Name: "React: Best Practices (Vercel)", Repo: "https://github.com/vercel-labs/agent-skills", Ref: "063bee94c3f4df8453406c830b0a7df0f2860278", Arg: "vercel-react-best-practices"},
	{Slug: "vite", Name: "Vite", Repo: "https://github.com/antfu/skills", Ref: "feb0f3aba6566426f12230e7fc7a10d97c1293bb", Arg: "vite"},
	{Slug: "vitest", Name: "Vitest", Repo: "https://github.com/antfu/skills", Ref: "feb0f3aba6566426f12230e7fc7a10d97c1293bb", Arg: "vitest"},
	{Slug: "web-quality", Name: "Web: Acessibilidade, SEO e Performance", Repo: "https://github.com/addyosmani/web-quality-skills", Ref: "afa8da942115f2961fdbfa80807ea0b232ff6c00", Arg: "*"},
	{Slug: "webapp-testing", Name: "Web: Testes com Playwright", Repo: "https://github.com/anthropics/skills", Ref: "dbd4588f9e1033efb41dad4bef2f7947c8993d44", Arg: "webapp-testing"},
	{Slug: "superpowers", Name: "SuperPowers", Repo: "https://github.com/obra/superpowers", Ref: "bb92a77741419a4ab5f06e711a283343f1ada0c3", Arg: "using-superpowers"},
	{Slug: "code-review", Name: "Code Review", Repo: "https://github.com/mattpocock/skills", Ref: "49dd158d1076134a641b33efb035946536778336", Arg: "code-review"},
	{Slug: "security-review", Name: "Security Review (Sentry)", Repo: "https://github.com/getsentry/skills", Ref: "d18b7aa8ba878354e5c348310230e652f7690f9c", Arg: "security-review"},
	{Slug: "gha-hardening", Name: "GitHub Actions: Hardening", Repo: "https://github.com/github/awesome-copilot", Ref: "156736fb63e965b2f547a1460f3b5972c46e6b24", Arg: "github-actions-hardening"},
	{Slug: "go-code-style", Name: "Go: Code Style", Repo: "https://github.com/samber/cc-skills-golang", Ref: "8e899e20ff0cd4dc524af3993e4c62d8ee8c5717", Arg: "golang-code-style"},
	{Slug: "go-error-handling", Name: "Go: Error Handling", Repo: "https://github.com/samber/cc-skills-golang", Ref: "8e899e20ff0cd4dc524af3993e4c62d8ee8c5717", Arg: "golang-error-handling"},
	{Slug: "go-testing", Name: "Go: Testing", Repo: "https://github.com/samber/cc-skills-golang", Ref: "8e899e20ff0cd4dc524af3993e4c62d8ee8c5717", Arg: "golang-testing"},
	{Slug: "go-security", Name: "Go: Security", Repo: "https://github.com/samber/cc-skills-golang", Ref: "8e899e20ff0cd4dc524af3993e4c62d8ee8c5717", Arg: "golang-security"},
	{Slug: "go-lint", Name: "Go: Lint", Repo: "https://github.com/samber/cc-skills-golang", Ref: "8e899e20ff0cd4dc524af3993e4c62d8ee8c5717", Arg: "golang-lint"},
	{Slug: "go-ci", Name: "Go: Continuous Integration", Repo: "https://github.com/samber/cc-skills-golang", Ref: "8e899e20ff0cd4dc524af3993e4c62d8ee8c5717", Arg: "golang-continuous-integration"},
	{Slug: "go-golang-pro", Name: "Go: Golang Pro", Repo: "https://github.com/jeffallan/claude-skills", Ref: "1be15d8064f88fc25216442406d40add8fd23b53", Arg: "golang-pro"},
	{Slug: "go-wails", Name: "Go: Wails", Repo: "https://github.com/spf13/go-skills", Ref: "9ac6eca43161163bb21621a520a57df50d5ad464", Arg: "wails"},
	{Slug: "rust-best-practices", Name: "Rust: Best Practices", Repo: "https://github.com/apollographql/skills", Ref: "aafc8944365a766d48cc519ce8b8575f93f953a4", Arg: "rust-best-practices"},
	{Slug: "php-pro", Name: "PHP: Pro 8.3+", Repo: "https://github.com/jeffallan/claude-skills", Ref: "1be15d8064f88fc25216442406d40add8fd23b53", Arg: "php-pro"},
	{Slug: "mariadb-query-optimization", Name: "MariaDB: Query Optimization (oficial)", Repo: "https://github.com/MariaDB/skills", Ref: "7b01484d7a7475824bea86f3e628e29273d1817b", Arg: "mariadb-query-optimization"},
	{Slug: "mariadb-features", Name: "MariaDB: Features (oficial)", Repo: "https://github.com/MariaDB/skills", Ref: "7b01484d7a7475824bea86f3e628e29273d1817b", Arg: "mariadb-features"},
	{Slug: "sql-code-review", Name: "SQL: Code Review", Repo: "https://github.com/github/awesome-copilot", Ref: "156736fb63e965b2f547a1460f3b5972c46e6b24", Arg: "sql-code-review"},
	{Slug: "mcp-server-dev", Name: "MCP Server Dev", Repo: "https://github.com/anthropics/skills", Ref: "dbd4588f9e1033efb41dad4bef2f7947c8993d44", Arg: "mcp-builder"},
	{Slug: "mcp-server-dev-go", Name: "MCP Server Dev in Go", Repo: "https://github.com/github/awesome-copilot", Ref: "156736fb63e965b2f547a1460f3b5972c46e6b24", Arg: "go-mcp-server-generator"},
	{Slug: "mcp-security-audit", Name: "MCP: Security Audit", Repo: "https://github.com/github/awesome-copilot", Ref: "156736fb63e965b2f547a1460f3b5972c46e6b24", Arg: "mcp-security-audit"},
	{Slug: "dart-official", Name: "Dart: Oficial", Repo: "https://github.com/dart-lang/skills", Ref: "0b8dc544de71e5412c7f407b34767e081c32ddfa", Arg: "*"},
	{Slug: "flutter-official", Name: "Flutter: Oficial", Repo: "https://github.com/flutter/agent-plugins", Ref: "0ef3972f93e2baa4156ba1cbb1e515cd53079c68", Arg: "*"},
	// Retired entries: listed only where still installed, to be removed.
	{Slug: "godot-core-ui", Name: "Godot: Core UI Knowledge", Repo: "https://github.com/zate/cc-godot", Ref: "d9796f75c3f5626d8b66dc31a9095f9d735c2327", Arg: "godot-ui", Retired: true},
	{Slug: "moodle-plugin", Name: "Moodle: Plugin Development", Repo: "https://github.com/af1ah/moodle-plugin-skills", Ref: "b0b758c45b73a49f664b5f8658afc5d18d781c73", Arg: "moodle-plugin", Retired: true},
	{Slug: "moodle-education-expert", Name: "Moodle: Education Expert", Repo: "https://github.com/personamanagmentlayer/pcl", Ref: "79ccaa9820486121fa33aaa696ec16fa1607bf02", Arg: "education-expert", Retired: true},
	{Slug: "svg-logo-designer", Name: "SVG Logo Designer", Repo: "https://github.com/rknall/claude-skills", Ref: "ca7fbd0e07f824b119030f323da3409bc779f9bc", Arg: "SVG Logo Designer", Retired: true},
	{Slug: "php-specialist", Name: "PHP: Specialist", Repo: "https://github.com/pixel-process-ug/superkit-agents", Ref: "cc7fcb7e47bc4c50020abdf352fa5d8914d0a7c2", Arg: "php-specialist", Retired: true},
}

// BySlug resolves a catalog entry by its internal Slug.
func BySlug(slug string) (Skill, bool) {
	for _, sk := range Catalogue {
		if sk.Slug == slug {
			return sk, true
		}
	}
	return Skill{}, false
}

// targetAgents lists the AI CLIs every skill is installed for (the same
// four managed by the llm package), applied to the whole catalogue.
var targetAgents = []string{"claude-code", "opencode", "antigravity", "codex"}

// TargetAgents returns the IDs of the agents every skill is installed for.
func TargetAgents() []string { return slices.Clone(targetAgents) }

// AgentLabel is the name the interface shows for an agent ID.
func AgentLabel(id string) string {
	switch id {
	case "claude-code":
		return "Claude"
	case "opencode":
		return "OpenCode"
	case "antigravity":
		return "Antigravity"
	case "codex":
		return "Codex"
	}
	return id
}

// scopedOptions returns the executor options for the scope. Local scope
// runs in folder, since the tool has no directory flag and uses the
// process's working directory. The CLI's telemetry is always turned off.
func scopedOptions(stdout io.Writer, global bool, folder string) executor.Options {
	opts := executor.Options{Stdout: stdout, Stderr: stdout, Env: []string{"DISABLE_TELEMETRY=1"}}
	if !global {
		opts.Dir = folder
	}
	return opts
}

// skillsCLI pins the npm "skills" CLI version instead of whatever npx
// resolves as latest.
const skillsCLI = "skills@1.7.0"

// Source is the repository reference the CLI installs sk from: the
// repository URL pinned to Ref.
func (sk Skill) Source() string { return sk.Repo + "#" + sk.Ref }

// addArgs builds the `skills add` arguments for sk and the given agents.
// `-y` right after `npx` skips npx's install prompt; `--agent` and `--yes`
// skip every confirmation and agent-selection prompt, so it never waits
// for stdin.
func addArgs(sk Skill, global bool, agents []string) []string {
	args := []string{"-y", skillsCLI, "add", sk.Source(), "--skill", sk.Arg}
	for _, a := range agents {
		args = append(args, "--agent", a)
	}
	args = append(args, "--yes")
	if global {
		args = append(args, "--global")
	}
	return args
}

// Install runs `npx skills add` for sk and returns the IDs of the agents
// that received it. One run covers every target agent; when it fails, each
// agent is retried on its own so the ones that accept the skill are known.
// It fails when no agent took it, and for an entry that left the catalog.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, sk Skill, global bool, folder string) ([]string, error) {
	if sk.Retired {
		return nil, fmt.Errorf("%s saiu do catálogo do Perci — só pode ser removida", sk.Name)
	}
	ctx, cancel := nettimeout.Context(ctx)
	defer cancel()
	opts := scopedOptions(stdout, global, folder)
	firstErr := exe.Run(ctx, opts, "npx", addArgs(sk, global, targetAgents)...)
	if firstErr == nil {
		return TargetAgents(), nil
	}
	ui.Warning(stdout, "A instalação para todos os agentes falhou ("+firstErr.Error()+") — tentando um agente por vez...")
	var accepted []string
	for _, a := range targetAgents {
		if err := exe.Run(ctx, opts, "npx", addArgs(sk, global, []string{a})...); err != nil {
			ui.Warning(stdout, AgentLabel(a)+": "+err.Error())
			continue
		}
		ui.Success(stdout, AgentLabel(a)+": skill instalada.")
		accepted = append(accepted, a)
	}
	if len(accepted) == 0 {
		return nil, fmt.Errorf("nenhum agente aceitou a skill: %w", firstErr)
	}
	return accepted, nil
}

// Remove runs `npx skills remove` for sk, with the same target agents and
// scope as Install.
//
// An entry with Arg "*" is removed by the names of the skills that came
// from its repository (installedFromRepo), never with `--skill '*'`, which
// would remove every installed skill in the scope.
func Remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, sk Skill, global bool, folder string) error {
	ctx, cancel := nettimeout.Context(ctx)
	defer cancel()
	names := []string{sk.Arg}
	if sk.Arg == "*" {
		var err error
		names, err = installedFromRepo(ctx, exe, sk.Repo, global, folder)
		if err != nil {
			return fmt.Errorf("listar as skills instaladas de %s: %w", sk.Repo, err)
		}
		if len(names) == 0 {
			ui.Warning(stdout, "Nenhuma skill de "+sk.Repo+" está instalada neste escopo — nada a remover.")
			return nil
		}
	}
	args := []string{"-y", skillsCLI, "remove"}
	for _, n := range names {
		args = append(args, "--skill", n)
	}
	for _, a := range targetAgents {
		args = append(args, "--agent", a)
	}
	args = append(args, "--yes")
	if global {
		args = append(args, "--global")
	}
	return exe.Run(ctx, scopedOptions(stdout, global, folder), "npx", args...)
}

// installedSkill is the part of one `skills list --json` entry used here.
type installedSkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// installedFromRepo returns the names of the skills installed in the scope
// (global, or the project at folder) whose recorded source is repo. Both
// sides are compared through repoKey, so "owner/repo" and the full URL
// match.
func installedFromRepo(ctx context.Context, exe *executor.Executor, repo string, global bool, folder string) ([]string, error) {
	args := []string{"-y", skillsCLI, "list", "--json"}
	if global {
		args = append(args, "--global")
	}
	opts := executor.Options{Env: []string{"DISABLE_TELEMETRY=1"}}
	if !global {
		opts.Dir = folder
	}
	out, err := exe.Output(ctx, opts, "npx", args...)
	if err != nil {
		return nil, err
	}
	var skills []installedSkill
	if err := json.Unmarshal([]byte(out), &skills); err != nil {
		return nil, fmt.Errorf("saída inesperada de skills list: %w", err)
	}
	want := repoKey(repo)
	var names []string
	for _, s := range skills {
		// A name starting with "-" would be read as a flag by the CLI.
		if s.Name == "" || strings.HasPrefix(s.Name, "-") || repoKey(s.Source) != want {
			continue
		}
		names = append(names, s.Name)
	}
	return names, nil
}

// repoKey normalizes a GitHub repository reference ("owner/repo",
// "https://github.com/owner/repo", with or without ".git" or a trailing
// slash) to a lowercase "owner/repo".
func repoKey(repo string) string {
	k := strings.ToLower(strings.TrimSpace(repo))
	k = strings.TrimPrefix(k, "https://github.com/")
	k = strings.TrimSuffix(k, "/")
	return strings.TrimSuffix(k, ".git")
}

// Update reinstalls sk at its pinned commit, for the same agents as
// Install, and returns the agents that received it. `skills update` takes
// no agent list and writes to every agent folder it finds in the project,
// so the update goes through the same pinned `skills add` as the install.
func Update(ctx context.Context, exe *executor.Executor, stdout io.Writer, sk Skill, global bool, folder string) ([]string, error) {
	return Install(ctx, exe, stdout, sk, global, folder)
}

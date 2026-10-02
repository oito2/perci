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
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Skill describes one installable skills.sh skill.
type Skill struct {
	Slug string // internal catalog id (config.AgentSkillInstall.Slug, DOM ids)
	Name string // display name
	Repo string // "owner/repo" shorthand or full git URL
	Arg  string // value passed to --skill; "*" installs every skill in Repo
}

// Catalogue lists every skill offered.
var Catalogue = []Skill{
	{Slug: "godot-ui", Name: "Godot: UI", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Arg: "godot-ui-control"},
	{Slug: "godot-master", Name: "Godot: Master", Repo: "https://github.com/thedivergentai/gd-agentic-skills", Arg: "godot-master"},
	{Slug: "godot-animations", Name: "Godot: Animations", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Arg: "godot-animation"},
	{Slug: "godot-shaders", Name: "Godot: Shaders", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Arg: "godot-shaders"},
	{Slug: "godot-gdscript", Name: "Godot: GDScript", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Arg: "godot-gdscript"},
	{Slug: "godot-export", Name: "Godot: Export & Builds", Repo: "https://github.com/gamedev-skills/awesome-gamedev-agent-skills", Arg: "godot-export"},
	{Slug: "godot-patterns", Name: "Godot: Patterns", Repo: "https://github.com/wshobson/agents", Arg: "godot-gdscript-patterns"},
	{Slug: "godot-core-ui", Name: "Godot: Core UI Knowledge", Repo: "https://github.com/zate/cc-godot", Arg: "godot-ui"},
	{Slug: "moodle-theme", Name: "Moodle: Theme v5+", Repo: "https://github.com/nicolasflores9/skills", Arg: "moodle5-theme"},
	{Slug: "moodle-external-api", Name: "Moodle: External API", Repo: "https://github.com/sickn33/agentic-awesome-skills", Arg: "moodle-external-api-development"},
	{Slug: "moodle-plugin", Name: "Moodle: Plugin Development", Repo: "https://github.com/af1ah/moodle-plugin-skills", Arg: "moodle-plugin"},
	{Slug: "moodle-education-expert", Name: "Moodle: Education Expert", Repo: "https://github.com/personamanagmentlayer/pcl", Arg: "education-expert"},
	// Arg "*" installs every skill in the repo.
	{Slug: "daisyui", Name: "DaisyUI", Repo: "saadeghi/daisyui", Arg: "*"},
	{Slug: "shadcn-ui", Name: "ShaCN UI", Repo: "https://github.com/shadcn-ui/ui", Arg: "shadcn"},
	{Slug: "frontend-design", Name: "Frontend Design", Repo: "https://github.com/anthropics/skills", Arg: "frontend-design"},
	{Slug: "superpowers", Name: "SuperPowers", Repo: "https://github.com/obra/superpowers", Arg: "using-superpowers"},
	{Slug: "code-review", Name: "Code Review", Repo: "https://github.com/mattpocock/skills", Arg: "code-review"},
	{Slug: "svg-logo-designer", Name: "SVG Logo Designer", Repo: "https://github.com/rknall/claude-skills", Arg: "SVG Logo Designer"},
	{Slug: "go-code-style", Name: "Go: Code Style", Repo: "https://github.com/samber/cc-skills-golang", Arg: "golang-code-style"},
	{Slug: "go-golang-pro", Name: "Go: Golang Pro", Repo: "https://github.com/jeffallan/claude-skills", Arg: "golang-pro"},
	{Slug: "php-specialist", Name: "PHP: Specialist", Repo: "https://github.com/pixel-process-ug/superkit-agents", Arg: "php-specialist"},
	{Slug: "php-pro", Name: "PHP: Pro 8.3+", Repo: "https://github.com/jeffallan/claude-skills", Arg: "php-pro"},
	{Slug: "mcp-server-dev", Name: "MCP Server Dev", Repo: "https://github.com/anthropics/skills", Arg: "mcp-builder"},
	{Slug: "mcp-server-dev-go", Name: "MCP Server Dev in Go", Repo: "https://github.com/github/awesome-copilot", Arg: "go-mcp-server-generator"},
	{Slug: "dart-official", Name: "Dart: Oficial", Repo: "dart-lang/skills", Arg: "*"},
	{Slug: "flutter-official", Name: "Flutter: Oficial", Repo: "flutter/agent-plugins", Arg: "*"},
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

// scopedOptions returns the executor options for the scope. Local scope
// runs in folder, since the tool has no directory flag and uses the
// process's working directory.
func scopedOptions(stdout io.Writer, global bool, folder string) executor.Options {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	if !global {
		opts.Dir = folder
	}
	return opts
}

// skillsCLI pins the npm "skills" CLI version instead of whatever npx
// resolves as latest.
const skillsCLI = "skills@1.7.0"

// Install runs `npx skills add` for sk. `-y` right after `npx` skips npx's
// install prompt; `--agent` and `--yes` on `skills add` skip every
// confirmation and agent-selection prompt, so it never waits for stdin.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, sk Skill, global bool, folder string) error {
	args := []string{"-y", skillsCLI, "add", sk.Repo, "--skill", sk.Arg}
	for _, a := range targetAgents {
		args = append(args, "--agent", a)
	}
	args = append(args, "--yes")
	if global {
		args = append(args, "--global")
	}
	return exe.Run(ctx, scopedOptions(stdout, global, folder), "npx", args...)
}

// Remove runs `npx skills remove` for sk, with the same target agents and
// scope as Install.
//
// An entry with Arg "*" is removed by the names of the skills that came
// from its repository (installedFromRepo), never with `--skill '*'`, which
// would remove every installed skill in the scope.
func Remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, sk Skill, global bool, folder string) error {
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
	opts := executor.Options{}
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

// Update runs `npx skills update` for sk, passing skill names as
// positional arguments (Skill.Arg for named entries).
//
// An Arg "*" entry is updated by the names of the skills installed from
// its repository (installedFromRepo), same as Remove.
func Update(ctx context.Context, exe *executor.Executor, stdout io.Writer, sk Skill, global bool, folder string) error {
	names := []string{sk.Arg}
	if sk.Arg == "*" {
		var err error
		names, err = installedFromRepo(ctx, exe, sk.Repo, global, folder)
		if err != nil {
			return fmt.Errorf("listar as skills instaladas de %s: %w", sk.Repo, err)
		}
		if len(names) == 0 {
			ui.Warning(stdout, "Nenhuma skill de "+sk.Repo+" está instalada neste escopo — nada a atualizar.")
			return nil
		}
	}
	args := append([]string{"-y", skillsCLI, "update"}, names...)
	args = append(args, "--yes")
	if global {
		args = append(args, "--global")
	} else {
		args = append(args, "--project")
	}
	return exe.Run(ctx, scopedOptions(stdout, global, folder), "npx", args...)
}

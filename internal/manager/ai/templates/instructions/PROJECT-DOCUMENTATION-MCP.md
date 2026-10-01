# Documentation Complement — MCP Servers

## Overview

Type-specific documentation requirements for **Model Context Protocol (MCP) servers**, in any language. This complements the base standard in `.instructions/PROJECT-DOCUMENTATION.md` — apply both. Like the base, it's a standard for when documentation is created or updated, not a task to run on its own.

---

## 1. Build and Project Layout

Document how the server is built, tested, and laid out, using the project's real toolchain. Examples by language — use only the one(s) the project actually uses:

| Language | Document |
|---|---|
| Go | Module path and package layout from `go.mod` (entry point under `cmd/`, internal packages), `go build -o <bin> ./cmd/<name>`, `go test ./...`, minimum Go version |
| Node/TypeScript | `package.json` scripts (`build`, `start`, `test`), the built entry file, `npx`-based execution if published |
| Python | Dependency manager (`uv`, `pip`, `poetry`), entry point, `uvx`/`pipx` execution if published, test command |

Put the quick path in the README (Prerequisites & Quick Installation) and the full detail in `docs/en/getting-started/installation.md`.

Also document the **transport(s)** the server supports (`stdio`, Streamable HTTP, SSE), and every environment variable / CLI flag it reads, in `docs/en/reference/`.

---

## 2. Registering the Server in MCP Clients

Add a **"Client Setup"** section to the README (short, with the most common client inline) and one page per client under `docs/en/guides/clients/` (this folder becomes mandatory for MCP servers). Cover the clients below, each with global and project/workspace scope when the client supports both. Always use the real binary path/command and arguments of this project.

Client configuration mechanisms change over time — **verify each one against the client's current official documentation before writing it**, and update the page when it has changed.

### Claude Desktop
File `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/`, Windows: `%APPDATA%\Claude\`):
```json
{
  "mcpServers": {
    "{{SERVER_NAME}}": {
      "command": "/absolute/path/to/{{BINARY}}",
      "args": [],
      "env": {}
    }
  }
}
```
Restart Claude Desktop after editing.

### Claude Code
```bash
claude mcp add --scope user {{SERVER_NAME}} -- /absolute/path/to/{{BINARY}}
```
Scopes: `local` (default, current project, private), `project` (shared via a committed `.mcp.json` at the project root, same `mcpServers` JSON shape as above), `user` (all projects). Verify with `claude mcp list`.

### Codex
```bash
codex mcp add {{SERVER_NAME}} -- /absolute/path/to/{{BINARY}}
```
Or edit `~/.codex/config.toml` directly:
```toml
[mcp_servers.{{SERVER_NAME}}]
command = "/absolute/path/to/{{BINARY}}"
args = []
```

### Antigravity (IDE) and Antigravity CLI
Both share the same `mcp_config.json` (same `mcpServers` JSON shape as Claude Desktop):
- Global: `~/.gemini/config/mcp_config.json`
- Workspace: `.agents/mcp_config.json`

Servers can also be added through the IDE's MCP settings, or the CLI's interactive `/mcp` manager. Note for the CLI: at the time of writing, Antigravity CLI has a known issue where workspace-level `mcpServers` are discovered but not started — recommend the global file for CLI users, and re-check the issue status when updating the docs.

---

## 3. Tools, Resources and Prompts Reference

Document **every** capability the server exposes over the protocol, under `docs/en/reference/` (e.g. `tools.md`, `resources.md`, `prompts.md` inside `reference/` — or a single `mcp.md` for small servers). Read them from the code (tool registration, schemas), never from memory.

- **Tools:** name, description, every parameter (name, type, required/optional, default, constraints), return value/content, side effects (writes files, calls external APIs, destructive operations), errors.
- **Resources:** URI or URI template, MIME type, what the content represents, when it changes.
- **Prompts (protocol primitive):** name, description, arguments (name, required), what messages it produces.

---

## 4. `docs/{en,pt-br}/prompts.md` — Example User Prompts (mandatory)

Not to be confused with the MCP *Prompts* primitive above: this page lists example requests a **user types to their AI agent** that make it use this server. It is mandatory for MCP servers.

- Write every prompt in **natural human language**, the way a real user would ask — no tool names or JSON required from the user.
- Categorize them in sections or tables. Recommended categories (adapt, drop or add to fit the server's real capabilities):
  - `Analysis / Refactoring Prompts`
  - `Automation / Generation Prompts`
  - `Testing and Validation Prompts`
- For each prompt, describe:
  - **Name** — a short label.
  - **Expected parameters** — the information the user needs to provide in the prompt (and any prerequisite state).
  - **Example** — the prompt itself, in a blockquote or code span.
  - **Expected output** — what the agent does (which tools it triggers) and what the user gets back.
- Be exhaustive: every meaningfully distinct capability should have at least one working example, so a user never has to guess a phrasing.

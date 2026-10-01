🌐 [English](../../en/concepts/glossary.md) | **Português** | 🏠 [Índice](../index.md)

---

# Glossário

| Termo | Significado |
|---|---|
| **`prci`** | O nome do binário compilado, `cmd/prci-gui`. Executá-lo abre a janela da GUI diretamente — nenhum argumento de linha de comando é interpretado. `perci` é instalado ao lado como um symlink alias — os dois são intercambiáveis. |
| **Categoria** | Uma das cinco seções principais da barra lateral da GUI: Home, Linux, Desenvolvimento, Docker, Dev Tools. Cada uma mapeia para um valor de `MenuCategory` em `cmd/prci-gui/catalog.go`. |
| **Ação** | Uma entrada de menu dentro de uma categoria (ex. "Gerenciar Fontes"), ligada a um `MenuItem.ActionID` conectado a um método Go (ligado ao frontend da GUI) que chama uma função de domínio. |
| **Família de distro** | A linhagem Debian ou Fedora à qual uma distro pertence, detectada por `internal/distro`. A lógica de pós-instalação e gerenciador de pacotes ramifica pela família, não pelo nome específico da distro. Arch não é uma família suportada. |
| **Workspace** | A pasta raiz (`workspace_path` na config) sob a qual projetos e as pastas `localhost/*` da stack Docker de aplicativos por projeto vivem. |
| **Container Aplicativo** | O container Docker próprio de um projeto — sua própria URL `*.localhost`, versão de PHP e/ou Node, pastas `html`/`data` isoladas. A unidade que a stack Docker de aplicativos gerencia; veja a [arquitetura da stack Docker de aplicativos](../architecture/appstack.md). |
| **Instalação Moodle** | Um `Container Aplicativo` do tipo `moodle` — seu próprio container/vhost dedicado, não uma entrada de roteamento compartilhada entre várias instalações. |
| **Contexto IA** | Arquivos de contexto gerados (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.instructions/*.md`) que ensinam as convenções de um projeto-alvo a agentes de IA de código. Gerado por `internal/manager/ai`, chamado diretamente pela tela "Dev Tools :: IA: Contextos" da GUI. |
| **Skill de IA** | Um arquivo `SKILL.md` autônomo que ensina um modo de trabalho específico a um agente de codificação (ex. "Frontend Design"). Só na GUI, "Dev Tools :: IA: SKILLs" (`internal/dev/agentskills`): instala skills de terceiros do [skills.sh](https://www.skills.sh/) via `npx skills add/remove/update`, mirando Claude Code/OpenCode/Antigravity/Codex. Um conceito irmão do Contexto IA e de servidor MCP, não um submenu de nenhum dos dois. (`internal/manager/skills`/`prci skills` — skills próprias do Perci, ex. "Claude Code Mode" — foi removido em 2026-09-11 em favor deste catálogo mais amplo.) |
| **Servidor MCP** | Um servidor Model Context Protocol que dá ferramentas extras a um agente de codificação (ex. acesso a arquivos, um banco SQLite). Só na GUI, "Dev Tools :: IA: MCPs" (`internal/dev/mcpservers`): registra um servidor em Claude Code/Codex (seus próprios subcomandos de CLI `mcp`/`plugin`) e Antigravity (edição direta do `mcp_config.json`) — cada agente tem seu próprio mecanismo, ao contrário da ferramenta única `npx skills` que a Skill de IA usa. |
| **Executor** | `internal/executor.Executor`, o único ponto pelo qual todo comando privilegiado (`sudo`) roda. Pacotes de domínio nunca chamam `sudo` diretamente. |
| **DE** | Ambiente de Desktop (ex. `cinnamon`, `xfce`, `gnome`), usado junto com a distro para decidir qual entrada de menu de pós-instalação fica habilitada. |

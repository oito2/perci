🌐 [English](../../en/architecture/dev-environment.md) | **Português** | 🏠 [Índice](../index.md)

---

# Ambiente de Desenvolvimento (`internal/dev/*`)

Sustenta a categoria **Desenvolvimento** da GUI.

| Pacote | Responsabilidade |
|---|---|
| `prereqs` | 18 pacotes essenciais alternáveis individualmente (compiladores, `build-essential`, Git, curl, mkcert, etc.) mais instalações agrupadas de Docker Engine/Node.js. No Fedora, o Docker Engine é o `moby-engine` + `docker-compose` + `docker-buildx` da distribuição, e o GitHub CLI é o pacote nativo `gh`. Remover o Node.js apaga a pasta `~/.nvm` inteira (todas as versões do Node instaladas pelo nvm e os pacotes npm globais delas) — a GUI pede confirmação antes, deixando isso explícito. |
| `golang` | Instalador do SDK Go. `LatestRelease(ctx)` busca o JSON de releases uma única vez e retorna a versão e seu checksum juntos (`Install(ctx, exe, stdout, version, checksumSHA256)` — passe um checksum vazio para cair no fallback de buscá-lo sozinho). A parte privilegiada é um único lote (um pedido de senha): cópia do tarball de dono root, checksum conferido de novo, extração em `/usr/local/go-staging` e troca com rollback (`installScript`). Exports de PATH locais via `internal/shellrc`, com uma linha entre aspas que também funciona no fish (a antiga, sem aspas, é substituída/removida). |
| `flutter` | SDK Flutter via checkout `git`, exports de PATH, aceitação automática de ferramentas Android, wrapper do `flutter doctor`. O JDK vem do `openjdk-21-jdk` no Debian/Ubuntu e do Eclipse Temurin 21 (repositório assinado da Adoptium) no Fedora, cujos repositórios não trazem mais o JDK 21. Baixa o Android cmdline-tools sem verificação de checksum — uma limitação documentada e aceita (não existe checksum oficial publicado para esse artefato específico); feito staging no mesmo diretório pai do destino para evitar erros `EXDEV` de cross-filesystem no rename. |
| `ide` | Setup do VS Code, VSCodium e Zed Editor. |
| `llm` | Gerenciador unificado de Claude Code, Antigravity CLI, Codex, OpenCode. Desinstalar também remove o que os instaladores oficiais deixam além do binário: `~/.local/share/claude` (quando o `claude` é o link da instalação nativa para lá) e o `~/.opencode/bin` do OpenCode com as linhas `# opencode` do rc; as configurações do usuário ficam. |
| `mcp` | Catálogo de servidores MCP Node.js locais (`servers.yaml`, embutido em tempo de compilação — recompile para pegar edições no catálogo). |
| `terminal` | Wrappers de Kitty, Alacritty, GNOME Console, prompt Starship, integrações de menu de contexto do gerenciador de arquivos. |
| `localbin` | Helper compartilhado para garantir que `~/.local/bin` esteja exportado no `PATH` (`EnsureInPath`, construído sobre `internal/shellrc`, gravado no arquivo rc do shell atual — `fish_add_path` no fish). `RefreshPath` reconstrói o `PATH` do próprio processo da GUI (`~/.local/bin` + o Node padrão do nvm) no início e após cada ação, para que uma ferramenta instalada ali durante a sessão seja encontrada sem reiniciar o Perci. |

## Dois Catálogos, Duas Telas na GUI

`llm` e `terminal` têm cada um sua própria tela na GUI ("Aplicativos: IA" e "Aplicativos: Terminais", ambas dentro de **Desenvolvimento**) em vez de serem fundidos numa só — a separação acontece só na camada de apresentação; os catálogos subjacentes continuam em seus próprios pacotes. (`mcp`, um terceiro catálogo que uma única tela fundida da TUI também cobria antes da TUI/CLI ser removida, foi ele mesmo removido em 2026-09-11 em favor da tela separada, exclusiva da GUI, "IA: MCPs" — veja `internal/dev/mcpservers`.) Veja [Adicionando um Novo LLM, IDE ou Terminal](../contribuindo.md#adicionando-um-novo-app-de-ia-ide-ou-terminal).

### Terminais: Menu de Contexto e Starship

- **Menu de contexto** — toda execução de "Aplicativos: Terminais" termina com `terminal.SyncContextMenuEntries`, que alinha as entradas "Abrir no <terminal>" ao que está de fato instalado (`InstalledMap`, depois de atualizar o `PATH`): criadas para todo terminal instalado com `DirFlag` — inclusive os instalados fora do Perci — e removidas dos demais, mesmo quando um item da execução falhou. As entradas ficam na home do usuário, então nenhuma senha é pedida: scripts do Nautilus e do Nemo (`~/.local/share/{nautilus,nemo}/scripts/Abrir no <terminal>` — o Nautilus os mostra no submenu **Scripts**) e um service menu do Dolphin (`~/.local/share/kio/servicemenus/perci-<terminal>.desktop`).
- **Starship** — não é item da checklist; dois botões extras na mesma tela: **Aplicar Starship** (`InstallStarship`) e **Remover Starship** (`UninstallStarship`, com confirmação), que apaga `~/.local/bin/starship` e as linhas de inicialização no bash/zsh/fish, preservando `~/.config/starship.toml` como `starship.toml.perci-bak`.

## Versões Fixadas de Pacotes

As ferramentas que o Perci executa via `npx`/`uvx` têm versão fixa, nunca "a mais nova do dia":

| Usado por | Pacote | Constante |
|---|---|---|
| Dev Tools → IA: SKILLs | `skills@1.7.0` (npm, [vercel-labs/skills](https://github.com/vercel-labs/skills)) | `agentskills.skillsCLI` |
| Dev Tools → IA: MCPs (Filesystem) | `@modelcontextprotocol/server-filesystem@2026.8.31` (npm) | `mcpservers.filesystemServerPkg` |
| Dev Tools → IA: MCPs (SQLite) | `mcp-server-sqlite@2025.4.25` (PyPI, via `uvx`) | `mcpservers.sqliteServerPkg` |

Para atualizar um deles, confira a nova release (`npm view <pacote> version`, pypi.org), altere a constante e recompile. Um servidor MCP já registrado mantém a versão com que foi registrado até ser atualizado na tela.

## Gerenciamento de PATH

Todo pacote que precisa persistir um export de PATH usa os helpers compartilhados `internal/shellrc.AppendIfMissing`/`RemoveEntry` contra o `~/.bashrc` — nenhum pacote reimplementa edição de arquivo rc por conta própria. As regravações (`RemoveEntry`) são atômicas, preservam as permissões existentes do arquivo rc e mantêm um symlink de dotfiles apontando para o destino.

## Padrão EXDEV / Instalação Atômica

Todo instalador que baixa para um local temporário e depois move o resultado para o destino final faz staging desse arquivo/diretório temporário no **mesmo diretório pai** do destino (mesmo filesystem) antes de renomear — é isso que evita erros de rename cross-filesystem `EXDEV` (que antes eram confundidos com erros de permissão, escalando desnecessariamente para `sudo`). Aplicado de forma consistente em `golang` e `flutter`.

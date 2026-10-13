🌐 [English](../../en/architecture/dev-environment.md) | **Português** | 🏠 [Índice](../index.md)

---

# Ambiente de Desenvolvimento (`internal/dev/*`)

Sustenta a categoria **Desenvolvimento** da GUI.

| Pacote | Responsabilidade |
|---|---|
| `prereqs` | 23 itens essenciais (CMake, Ninja, Clang, os cabeçalhos de desenvolvimento do GTK3/GTK4/WebKitGTK 6.0, Git, curl, mkcert, `librsvg2-bin`, libsecret com os arquivos de desenvolvimento do `libsecret-1`, etc.) mais instalações agrupadas de Docker Engine/Node.js. No Fedora, o Docker Engine é o `moby-engine` + `docker-compose` + `docker-buildx` da distribuição, e o GitHub CLI é o pacote nativo `gh`. Cada item tem seus próprios Instalar/Atualizar/Desinstalar (`InstallOne`/`UpdateOne`/`UninstallOne`): atualizar só atualiza os pacotes do próprio item (`apt-get install --only-upgrade` depois de atualizar o índice, na mesma senha, ou `dnf upgrade`), leva o Node.js à última LTS pelo nvm trazendo os pacotes npm globais, e reinstala o Wails3 CLI e o AppImageTool, que têm versão fixa. `Prereq.Requires` lista o que precisa estar instalado antes (Curl para GitHub CLI, Node.js e AppImageTool; Flatpak para Flatpak Builder; Go, de Linguagens e SDKs, para o Wails3 CLI): o `Catalogue` lista esses requisitos primeiro, e `MissingRequirements` mantém o Instalar desabilitado com a dica "Requer: …" até eles estarem instalados. Desinstalar sempre pede confirmação; remover o Node.js apaga a pasta `~/.nvm` inteira (todas as versões do Node instaladas pelo nvm e os pacotes npm globais delas), e a confirmação deixa isso explícito, e também remove as linhas do nvm dos arquivos rc do shell. Remover qualquer item baseado em pacote (isso inclui os terminais e IDEs abaixo que passam pelo mesmo helper: Alacritty, GNOME Console, VS Code, VSCodium) primeiro simula a remoção e **recusa** — "também removeria …" — quando o gerenciador de pacotes levaria outros pacotes junto. O Libsecret Tools nunca remove a biblioteca compartilhada `libsecret-1-0` no Debian/Ubuntu e, no Fedora, também instala o `git-credential-libsecret`. A descrição do Docker Engine avisa que o grupo `docker` equivale a acesso de root. Os passos de rede sem privilégio (nvm, `npx`, `git`, downloads com `curl`) têm um limite de 30 minutos (`internal/dev/nettimeout`); `apt`/`dnf` nunca são interrompidos. |
| `golang` | Instalador do SDK Go. `LatestRelease(ctx)` busca o JSON de releases uma única vez e retorna a versão e seu checksum juntos (`Install(ctx, exe, stdout, version, checksumSHA256)` — passe um checksum vazio para cair no fallback de buscá-lo sozinho). A parte privilegiada é um único lote (um pedido de senha): cópia do tarball de dono root, checksum conferido de novo, extração em `/usr/local/go-staging` e troca com rollback (`installScript`). Exports de PATH locais via `internal/shellrc`, com uma linha entre aspas que também funciona no fish (a antiga, sem aspas, é substituída/removida). |
| `flutter` | SDK Flutter via checkout `git`, exports de PATH, aceitação automática de ferramentas Android, wrapper do `flutter doctor`. A instalação aceita as licenças do Android SDK em nome do usuário (`sdkmanager --licenses`); a descrição do Flutter na tela **Linguagens e SDKs** avisa isso e aponta os termos. Um `java` existente só é aceito na versão 17 ou mais nova (o que as cmdline-tools do Android exigem); senão o JDK 21 é instalado, no mesmo lote privilegiado dos pré-requisitos do Flutter. O JDK vem do `openjdk-21-jdk` no Debian/Ubuntu e do Eclipse Temurin 21 (repositório assinado da Adoptium) no Fedora, cujos repositórios não trazem mais o JDK 21. O arquivo do Android cmdline-tools (um build fixo) é conferido, antes da extração, contra o SHA-1 que o manifesto de pacotes do Google (`repository2-3.xml`) lista para ele, e passa por staging no mesmo diretório pai do destino para evitar erros `EXDEV` de cross-filesystem no rename. |
| `sdks` | A tela **Linguagens e SDKs**: Go e Flutter, cada um com seus próprios Instalar/Atualizar/Desinstalar (`InstallOne`/`UpdateOne`/`UninstallOne`), na mesma tabela de Pré-requisitos. Atualizar o Go instala a última versão estável só quando ela é mais nova que a instalada; atualizar o Flutter roda `flutter upgrade`. `SDK.Requires` aponta os itens de Pré-requisitos de que cada instalador precisa — Curl para o Go (o tarball é baixado com `curl`), Git para o Flutter (`git clone`) — e `MissingRequirements` mantém o Instalar desabilitado com a dica "Requer: …" até eles estarem instalados. Desinstalar sempre pede confirmação. |
| `ide` | Setup do VS Code, VSCodium e Zed Editor. Os instaladores do VS Code/VSCodium não adicionam uma segunda fonte apt quando já existe uma fonte `packages.microsoft.com/repos/code` (ela conflitaria no `Signed-By`); desinstalar o Zed também remove a entrada `.desktop` dele. |
| `llm` | Gerenciador unificado de Claude Code, Antigravity CLI, Codex, OpenCode. Desinstalar também remove o que os instaladores oficiais deixam além do binário: `~/.local/share/claude` (quando o `claude` é o link da instalação nativa para lá) e o `~/.opencode/bin` do OpenCode com as linhas `# opencode` do rc; as configurações do usuário ficam. O Codex é fixado (`@openai/codex@0.162.1`). |
| `terminal` | Wrappers de Kitty, Alacritty, GNOME Console, prompt Starship, integrações de menu de contexto do gerenciador de arquivos. |
| `mcpservers` | A tela **IA: MCPs**: registra servidores MCP no Claude Code, no Codex e no Antigravity (veja [`mcpservers`](#ia-mcps-e-skills)). |
| `agentskills` | A tela **IA: SKILLs**: instala skills de agentes de terceiros pelo CLI `skills` (veja [`agentskills`](#ia-mcps-e-skills)). |
| `nettimeout` | O limite de 30 minutos e as flags de timeout do `curl` compartilhados por todo comando de rede sem privilégio. |
| `localbin` | Helper compartilhado para garantir que `~/.local/bin` esteja exportado no `PATH` (`EnsureInPath`, construído sobre `internal/shellrc`, gravado no arquivo rc do shell atual — `fish_add_path` no fish). `RefreshPath` reconstrói o `PATH` do próprio processo da GUI (`~/.local/bin` + o Node padrão do nvm) no início e após cada ação, para que uma ferramenta instalada ali durante a sessão seja encontrada sem reiniciar o Perci. |

## Dois Catálogos, Duas Telas na GUI

`llm` e `terminal` têm cada um sua própria tela na GUI ("Aplicativos: IA" e "Aplicativos: Terminais", ambas dentro de **Desenvolvimento**) em vez de serem fundidos numa só — a separação acontece só na camada de apresentação; os catálogos subjacentes continuam em seus próprios pacotes. (`mcp`, um terceiro catálogo que uma única tela fundida da TUI também cobria antes da TUI/CLI ser removida, foi ele mesmo removido em 2026-09-11 em favor da tela separada, exclusiva da GUI, "IA: MCPs" — veja `internal/dev/mcpservers`.) Veja [Adicionando um Novo LLM, IDE ou Terminal](../contribuindo.md#adicionando-um-novo-app-de-ia-ide-ou-terminal).

### Terminais: Menu de Contexto e Starship

- **Menu de contexto** — toda execução de "Aplicativos: Terminais" termina com `terminal.SyncContextMenuEntries`, que alinha as entradas "Abrir no <terminal>" ao que está de fato instalado (`InstalledMap`, depois de atualizar o `PATH`): criadas para todo terminal instalado com `DirFlag` — inclusive os instalados fora do Perci — e removidas dos demais, mesmo quando um item da execução falhou. As entradas ficam na home do usuário, então nenhuma senha é pedida: scripts do Nautilus e do Nemo (`~/.local/share/{nautilus,nemo}/scripts/Abrir no <terminal>` — o Nautilus os mostra no submenu **Scripts**) e um service menu do Dolphin (`~/.local/share/kio/servicemenus/perci-<terminal>.desktop`).
- **Starship** — não é item da checklist; dois botões extras na mesma tela: **Aplicar Starship** (`InstallStarship`) e **Remover Starship** (`UninstallStarship`, com confirmação), que apaga `~/.local/bin/starship` e as linhas de inicialização no bash/zsh/fish, preservando `~/.config/starship.toml` como `starship.toml.perci-bak`.

## Versões Fixadas

Nada que o Perci baixa ou executa por um executor de pacotes é "a versão mais nova do dia". Os instaladores oficiais de Starship, Claude Code, Kitty e ferramentas parecidas são a exceção: são baixados por HTTPS com `curl` e timeouts (`internal/dev/nettimeout`) e sempre trazem a release atual.

| Usado por | O que é fixado | Onde |
|---|---|---|
| Dev Tools → IA: SKILLs | `skills@1.7.0` (npm, [vercel-labs/skills](https://github.com/vercel-labs/skills)); cada skill é instalada do seu repositório num SHA de commit completo (`https://github.com/<dono>/<repo>#<sha>`) | `agentskills.skillsCLI`, `Skill.Ref` |
| Dev Tools → IA: MCPs (Filesystem) | `@modelcontextprotocol/server-filesystem@2026.8.31` (npm) | `mcpservers.filesystemServerPkg` |
| Dev Tools → IA: MCPs (SQLite) | `mcp-server-sqlite@2025.4.25` (PyPI, via `uvx`) | `mcpservers.sqliteServerPkg` |
| Dev Tools → IA: MCPs (Dart + Flutter, Codex) | o marketplace `flutter/agent-plugins` num commit fixo | `mcpservers.dartFlutterCodexMarket` |
| Pré-requisitos → Node.js | o instalador do nvm, por tag (`v0.40.8`) | `prereqs.nvmVersion` |
| Aplicativos: IA → Codex | `@openai/codex@0.162.1` (npm) | `llm.codexPackage` |
| Linux → Fontes → JetBrains Mono NF | o arquivo da release, por versão e SHA-256 | catálogo `fonts` |

Para atualizar um deles, confira a nova release (`npm view <pacote> version`, pypi.org, o último commit do repositório), altere a constante e recompile. Um servidor MCP já registrado mantém a versão com que foi registrado até ser atualizado na tela, e **Atualizar** numa SKILL instalada a reinstala no commit fixado.

## IA: MCPs e Skills

O `mcpservers` registra um servidor por agente (Claude Code, Codex, Antigravity) e grava no `config.yaml` a quais agentes cada registro chegou (`mcp_servers[].agents`); o `agentskills` faz o mesmo com as skills (`agent_skills[].agents`). Quando algum agente falta num registro (por exemplo, um falhou enquanto outro funcionou), a tela marca a linha como **parcial: …**, listando os agentes que a aceitaram. Outras regras:

- Um servidor de mesmo nome que o Perci não registrou (encontrado no `mcp_config.json` do Antigravity ou na lista do próprio Claude Code ou Codex) nunca é sobrescrito nem removido; adicioná-lo é recusado com a mensagem "já existe um servidor com esse nome que não foi registrado pelo Perci".
- Filesystem e SQLite são recusados de cara quando `npx`/`uvx` não está disponível.
- Pastas e parâmetros de MCP (a pasta do Filesystem, o arquivo do SQLite) precisam ter sido escolhidos no diálogo nativo durante a sessão ou já estar registrados para aquele servidor; qualquer outra coisa é recusada com "pasta não escolhida no seletor". A mesma regra vale para adicionar uma pasta de repositório.
- O CLI `skills` roda com `DISABLE_TELEMETRY=1`, e a tela mostra um aviso de que skills são conteúdo de terceiros.

## Gerenciamento de PATH

Todo pacote que precisa persistir um export de PATH usa os helpers compartilhados `internal/shellrc.AppendIfMissing`/`RemoveEntry` contra o `~/.bashrc` — nenhum pacote reimplementa edição de arquivo rc por conta própria. As regravações (`RemoveEntry`) são atômicas, preservam as permissões existentes do arquivo rc e mantêm um symlink de dotfiles apontando para o destino.

## Padrão EXDEV / Instalação Atômica

Todo instalador que baixa para um local temporário e depois move o resultado para o destino final faz staging desse arquivo/diretório temporário no **mesmo diretório pai** do destino (mesmo filesystem) antes de renomear — é isso que evita erros de rename cross-filesystem `EXDEV` (que antes eram confundidos com erros de permissão, escalando desnecessariamente para `sudo`). Aplicado de forma consistente em `golang` e `flutter`.

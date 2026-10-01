🌐 [English](../../en/concepts/architecture.md) | **Português** | 🏠 [Índice](../index.md)

---

# Visão Geral da Arquitetura

Esta é a resposta de "como tudo se encaixa", para usuários e potenciais contribuintes que querem o quadro geral sem mergulhar nos detalhes de cada pacote — veja páginas como [Gerenciamento do sistema](../architecture/system-management.md) e [Ambiente de desenvolvimento](../architecture/dev-environment.md) para esse nível de detalhe.

## Camadas

O perci.gnl é uma aplicação desktop exclusivamente GUI — backend Go, frontend JS, construída com Wails — estruturada em camadas:

```
cmd/prci-gui/main.go → cmd/prci-gui (app*.go)   → pacotes de domínio → internal/executor
(abre a janela GUI)    (métodos Go ligados ao       (por categoria,       (único ponto de
                         frontend JS, cinco           veja abaixo)          escalada sudo)
                         categorias, veja abaixo)
```

- **`cmd/prci-gui/main.go`** não faz nada além de configurar e abrir a janela do Wails (`wails.Run`) — nenhuma lógica vive aqui. Os comandos `prci`/`perci` não recebem nenhum argumento de linha de comando — rodar qualquer um dos dois simplesmente abre a janela da GUI.
- **`cmd/prci-gui`** (`app.go` mais um `app_*.go` por área de tela, e `catalog.go` com as definições do menu lateral) contém os métodos Go que o Wails liga ao frontend JS — um método por ação, ligado a um `MenuItem.ActionID` dentro de uma das cinco categorias da barra lateral.
- **Pacotes de domínio** implementam o trabalho de fato, agrupados por categoria (veja abaixo). Nenhum deles contém prompts interativos (sem formulários, sem leitura de stdin) — recebem parâmetros simples e são chamados diretamente pelos métodos Go da GUI.
- **`internal/executor`** é o único ponto pelo qual todo comando privilegiado é escalado via `sudo` — nenhum pacote de domínio chama `sudo` diretamente.

O `internal/ui` (helpers de terminal estilo script: `PrintHeader`/`Info`/`Err`/`Warning`/`Success`/`Step`) ainda é usado, agora só na GUI: sua saída é transmitida ao vivo para o painel de terminal embutido da própria GUI (xterm.js) dentro da aba "Execução", usando uma única paleta de cores fixa em vez dos temas selecionáveis pelo usuário que a antiga TUI tinha.

## Componentes

| Pacote | Responsabilidade |
|---|---|
| `cmd/prci-gui` | Aplicação desktop Wails: `main.go` (configuração da janela), `app.go`/`app_*.go` (métodos Go ligados ao frontend JS), `catalog.go` (as cinco categorias da barra lateral e seus itens de menu) |
| `internal/ui` | Primitivos de terminal (`PrintHeader`, `Info`, `Err`, `Warning`, `Success`, `Step`), transmitidos para o painel de terminal embutido da GUI — sem dependência de `config`/`distro` |
| `internal/config` | Carregamento/gravação de `~/.perci/config.yaml` |
| `internal/distro` | Detecção de família Debian/Fedora + ambiente de desktop |
| `internal/executor` | Único ponto de escalada `sudo` |
| `internal/selfupdate` | Auto-atualização via GitHub Releases, auto-desinstalação |
| `internal/shellrc` | Helpers compartilhados para editar `~/.bashrc`/arquivos rc (exports de PATH, appends idempotentes) |
| `internal/system/*` | **Categoria Linux**: pós-instalação (Mint/Zorin/Ubuntu/Fedora), fontes, modelos de arquivos, apps Flatpak, atualização de sistema, Linux Toys, MegaSync |
| `internal/dev/*` | **Categoria Desenvolvimento**: pré-requisitos, SDKs Go/Flutter, IDEs, CLIs de LLM/MCP, terminais; também dá suporte às skills de IA (`agentskills`) e ao registro de servidores MCP (`mcpservers`) da **categoria Dev Tools** |
| `internal/appstack` | **Categoria Docker**: stack Docker de aplicativos por projeto — criação, ciclo de vida e roteamento Moodle de containers Nginx/MariaDB/PHP/Node/PHP+Node, exportar/importar |
| `internal/manager/*` | **Categoria Dev Tools**: contexto IA (`ai`), backup/restore de banco de dados (`db`, usado por `internal/appstack`), repositório Git (`repo`), `.gitignore` (`gitignore`) |

## Fluxo de Dados

Não há processo persistente nem banco de dados. Toda ação na GUI segue o mesmo formato: carrega a config (se necessário) → faz o trabalho, escalando via `internal/executor` quando privilegiado → transmite o resultado para o painel de terminal "Execução" → devolve o controle à barra lateral/menu. O único estado que sobrevive entre execuções é o `~/.perci/config.yaml`, mais o que cada recurso escreve explicitamente em disco (containers Docker, um arquivo `AGENTS.md`, uma fonte instalada).

## Integrações Externas

- **GitHub Releases** (`oito2/perci`) — tanto a auto-atualização quanto o instalador `install.sh` buscam daqui, verificando o `checksums.txt` antes de instalar.
- **APT / DNF / Flatpak / Snap** — gerenciadores de pacotes do sistema, invocados através do `internal/executor`.
- **Docker Engine** (`docker run`/`create`/`start`/`stop`/`rm`, sem `docker-compose`) — a stack de aplicativos Nginx/MariaDB/PHP/Node por projeto.
- **mkcert** — HTTPS local para `*.localhost`, um certificado wildcard compartilhado por todo app.
- **Diversos instaladores upstream** (Go, Flutter, servidores MCP em Node, IDEs, ferramentas de terminal) — cada um com sua própria estratégia de download/verificação; veja [Gerenciamento de Sistema](../architecture/system-management.md) e [Ambiente de Desenvolvimento](../architecture/dev-environment.md) para detalhes específicos, e o [log de decisões](../architecture/decisions.md) para riscos aceitos onde não existe checksum upstream.

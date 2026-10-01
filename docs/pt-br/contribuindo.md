# Contribuindo com o perci.gnl

🌐 **Idioma:** [English](../../CONTRIBUTING.md) · Português

Obrigado pelo interesse em contribuir! Este documento explica como configurar o ambiente de desenvolvimento, as convenções do projeto e como enviar alterações.

---

## Ambiente de Desenvolvimento

**Requisitos:**
- Go 1.26 ou superior
- Git
- Pacotes de sistema pra compilar/rodar a GUI (Wails, cgo): `libgtk-4-dev libwebkitgtk-6.0-dev` (Debian/Ubuntu; veja `cmd/prci-gui/README.md`)

**Configuração:**

```bash
git clone https://github.com/oito2/perci.git
cd perci
go mod download
```

**Comandos comuns:**

```bash
# Compilar (modo desenvolvimento)
go build ./cmd/prci-gui

# Compilar com versão injetada
go build -ldflags "-X github.com/oito2/perci/internal/version.Version=v1.0.0" -o prci ./cmd/prci-gui

# Executar diretamente
go run ./cmd/prci-gui

# Testes
go test ./...
go test -race ./...

# Análise estática
go vet ./...
golangci-lint run

# Smoke test do frontend (Node puro, sem npm install): node --check em
# todos os scripts, depois a suíte node:test de cmd/prci-gui/frontend/test
make test-frontend

# Atalhos do Makefile (já com as flags acima)
make build
make test
make lint
```

### Escrevendo Testes

- Fluxos privilegiados ou que alteram o sistema são testados com `executor.Executor{DryRun: true, UsePolicyKit: true}`, verificando o rastro do dry-run — em geral, que o fluxo pede a senha **uma vez** (`strings.Count(out, "pkexec") == 1`). Use `Executor.LookPath` para simular quais comandos estão instalados; nunca dependa do que a máquina que roda o teste tem.
- Comandos que precisam rodar de verdade (Docker, mkcert) são trocados por um script pequeno colocado primeiro no `PATH` com `t.Setenv("PATH", ...)` — veja `internal/appstack/stack_test.go` e `internal/manager/db/backup_test.go`.
- Nunca toque em caminhos reais do sistema: aponte o `HOME` para `t.TempDir()` e use os pontos de teste do pacote (`selfupdate.executablePath`, `uninstallMenuPaths`, `serviceBase.emitFn`).
- Um método bindado que recebe pasta ou arquivo do frontend precisa recusar um caminho ruim antes de fazer qualquer coisa: acrescente-o à tabela em `cmd/prci-gui/path_guards_test.go` (caminhos vazios, relativos, inexistentes e não escolhidos no seletor).
- As ações da GUI são testadas em DryRun via `newTestBase` com `isolateActions` (`HOME` temporário, `PATH` vazio) — veja `cmd/prci-gui/actions_test.go`. Em DryRun, as checagens de "está instalado?" feitas via shell sempre respondem sim, e downloads feitos pelo próprio Go (fora do executor) continuam acessando a rede: deixe esses de fora.
- A suíte do frontend (`cmd/prci-gui/frontend/test`) carrega os scripts clássicos num contexto `vm` do Node com um DOM mínimo (`harness.mjs`); chame as funções dos scripts pelo contexto devolvido e use `evalIn` para os `let` de nível superior deles.

O CI (`.github/workflows/ci.yml`) roda gofmt, go vet, golangci-lint, `go test -race` e `make test-frontend` em todo push para a `main` e em todo pull request.

---

## Estrutura do Projeto

```
perci/
├── cmd/prci-gui/       # Ponto de entrada — GUI Wails (única interface; não existe mais CLI/TUI)
│   ├── main.go
│   ├── service_base.go # Estrutura compartilhada (wailsApp/exe, pickFolder, runAction, eventWriter) embutida por cada serviço abaixo
│   ├── service_*.go    # Um application.Service por categoria da sidebar (HomeService, LinuxService, DevSetupService, DockerService, DevToolsService, TrayService), expostos ao frontend via bindings ES module gerados (frontend/dist/bindings/)
│   ├── catalog.go      # Categorias/itens da sidebar exibidos na GUI
│   └── frontend/dist/  # index.html + js/ (sem build step — daisyUI/Tailwind/xterm.js vendorizados, CSP estrita)
├── internal/
│   ├── ui/             # Primitivos do painel de terminal (PrintHeader, Info, Err, Success, Step…) — texto colorido em ANSI puro, transmitido ao vivo pro painel xterm.js da GUI
│   ├── executor/       # Único ponto de escalada sudo/pkexec, incl. RunSudoSequence (uma única autenticação pra um lote inteiro de comandos privilegiados)
│   ├── config/         # ~/.perci/config.yaml
│   ├── distro/         # Detecção de família de distro (Debian/Fedora)
│   ├── sets/           # Estruturas de conjunto
│   ├── checklist/      # Loop compartilhado de "checklist simples" usado pelas telas de checklist
│   ├── shellrc/        # Helpers para editar os arquivos rc do shell do usuário
│   ├── version/        # String de versão (injetada via -ldflags)
│   ├── selfupdate/     # Auto-atualização via GitHub Releases (com verificação de checksum) + desinstalação + ícones do menu
│   ├── system/         # Categoria "Linux" (update, fonts, apps, templates, postinstall…)
│   ├── appstack/       # Stack Docker por projeto (containers Nginx/MariaDB/PHP/Node)
│   ├── dev/            # Categoria "Desenvolvimento" (SDKs, apps de IA, IDEs, terminais, MCP)
│   └── manager/        # Categoria "Dev Tools" (contexto de IA, .gitignore, repositórios)
│       ├── ai/         # Geração de contexto de IA (CLAUDE.md, AGENTS.md, .instructions/*.md)
│       ├── db/         # Backup/restore/otimização do MariaDB
│       ├── gitignore/  # Geração de .gitignore com base no tipo de projeto detectado
│       └── repo/       # Identidade Git (global, init, clone, ident local, código de conduta)
├── packaging/          # perci.desktop + conjuntos de ícones hicolor (entrada no menu de aplicativos)
├── docs/
│   ├── en/             # Site de documentação (inglês, cânone)
│   ├── pt-br/          # Site de documentação (espelho em português)
│   └── img/            # Logos, ícones e screenshots
├── CODE_OF_CONDUCT.md  # Contributor Covenant
└── install.sh          # Instalador one-line
```

Veja [docs/pt-br/index.md](./index.md) para o site de documentação completo — este arquivo cobre só o necessário para começar a contribuir.

> **Nota histórica:** o Perci já foi uma TUI Bubble Tea mais uma CLI não-interativa (`internal/tui`, `internal/app`, `cmd/prci`), antes de virar uma GUI Wails-only (2026-09-14). `internal/theme` e `internal/prompt` foram removidos na mesma mudança — nenhum pacote de domínio depende mais de `charmbracelet/*`.

---

## Convenções de Código

- **Idioma:** Todo o código, comentários e documentação em inglês; todas as strings visíveis ao usuário em português do Brasil.
- **Tratamento de erros:** Sempre encapsule com `fmt.Errorf("contexto: %w", err)`.
- **Sem sudo direto:** Todos os comandos privilegiados passam pelo `executor.Executor` com `RequiresSudo: true` (um comando) ou `RunSudoSequence` (um lote que precisa autenticar só uma vez).
- **Sem prompt interativo em pacote de domínio:** Funções de domínio (`internal/system/*`, `internal/dev/*`, `internal/manager/*`, `internal/appstack`) recebem parâmetros simples e nunca leem stdin nem mostram formulário — toda entrada/confirmação acontece na GUI (frontend Wails + os serviços bindados de `cmd/prci-gui`, um `application.Service` por domínio: `HomeService`/`LinuxService`/`DevSetupService`/`DockerService`/`DevToolsService`/`TrayService`). O clique num botão da GUI já é a confirmação; não existe prompt "tem certeza?" dentro do código de domínio.
- **Assinatura de funções de domínio:**
  ```go
  func DoSomething(ctx context.Context, exe *executor.Executor, stdout io.Writer) error
  ```
- **Padrão de UI:** A saída do painel de terminal de uma função de domínio segue `PrintHeader → Info/Warning → Err ou Success` (transmitida ao vivo pra aba Execução da GUI via `eventWriter` de `cmd/prci-gui/service_base.go`).
- **Item oculto, não desabilitado:** Itens de menu incompatíveis com a distro/ambiente atual somem da lista por completo, nunca aparecem desabilitados nela.
- **Sem abstrações prematuras:** Implemente apenas o necessário.
- **Detecção de distro:** Use sempre `distro.Detect()` — nunca crie funções locais de detecção em pacotes de domínio. Família suportada: Debian, Fedora (Arch não é suportado).

---

## Adicionando uma Nova Ação ao Menu

1. Adicione a entrada no slice `Items` da categoria correspondente em `cmd/prci-gui/catalog.go` (`Title`, `Desc`, `ActionID`).
2. Implemente a função de domínio no pacote apropriado (parâmetros simples, sem interatividade — ver Convenções de Código acima).
3. Adicione um método Go no `service_*.go` do domínio correspondente em `cmd/prci-gui` (`HomeService`/`LinuxService`/`DevSetupService`/`DockerService`/`DevToolsService`/`TrayService` — um `application.Service` por categoria) que a chame — exposto ao frontend via os bindings ES module gerados em `frontend/dist/bindings/` (rode `make generate-bindings` depois de adicionar um método ou mudar a assinatura de um existente).
4. Monte a tela no frontend: um `<div>` container em `cmd/prci-gui/frontend/dist/index.html`, a função de render em `frontend/dist/js/screens/`, uma entrada em `screenTable()` (`js/navigation.js`) e, quando ela precisar se atualizar ao fim de uma ação, em `ACTION_DONE_HANDLERS` (`js/core.js`) — ou reaproveite um modo já existente (simple/multiselect/singleapp/etc.) quando a tela se encaixar em algum. Encadeie `.catch(failRun)` em toda chamada que inicia uma ação depois de `startExecution()`.

---

## Adicionando um Novo Aplicativo Flatpak

Edite `internal/system/apps/catalogue.go` e adicione uma entrada ao slice `Catalogue`:

```go
{Name: "Nome do App", FlatID: "com.exemplo.AppID"},
```

---

## Adicionando um Novo App de IA, IDE ou Terminal

Edite o arquivo `catalogue.go` do pacote correspondente em `internal/dev/llm/`, `internal/dev/ide/` ou `internal/dev/terminal/`, e adicione uma entrada ao slice `Catalogue`. Em seguida, implemente o fluxo de instalação/desinstalação nos respectivos `install.go` e `uninstall.go` (funções simples, sem prompt — ver Convenções de Código acima).

---

## Enviando Alterações

1. Crie um fork do repositório e uma branch para sua funcionalidade.
2. Implemente as alterações seguindo as convenções acima.
3. Execute `make lint`, `make test` e `make test-frontend` — todos devem passar (o CI roda as mesmas verificações).
4. Abra um Pull Request com uma descrição clara do que foi alterado e por quê.

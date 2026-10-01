# prci-gui

A GUI do Perci — [Wails v3](https://v3.wails.io) + JS/HTML/CSS puro (sem build
step: daisyUI, Tailwind, xterm.js e lucide vendorizados em `frontend/dist/vendor/`,
com Content-Security-Policy só da própria origem).
Desde 2026-09-14 esta é a **única** interface do Perci — a antiga TUI
(Bubble Tea, `internal/tui`) e a CLI não-interativa (`internal/app`,
binário `cmd/prci`) foram removidas por completo. `cmd/prci-gui` continua
sendo o único ponto de entrada; o binário resultante ainda se chama `prci`
(veja o `Makefile` da raiz do repositório), só que agora sempre abre a
janela gráfica, sem nenhum parsing de argumento de linha de comando.

Migrado do Wails v2 para o v3 em 2026-09-14 (motivo: v3 tem systray e
multi-janela nativos, que a feature de bandeja do sistema precisa — v2 não
tem nenhum dos dois).

## Estrutura

- `main.go` — ponto de entrada; `application.New(...)` + `Window.NewWithOptions`
  + `Run()` (a API do v3 separa criação do app, criação da janela e o loop
  principal — no v2 isso era tudo um `wails.Run(&options.App{...})` só);
  embute `frontend/dist/` inteiro via `//go:embed` (HTML/CSS/JS/bindings
  gerados). O ícone da janela (`Home :: Configurações`) é o PNG quadrado de
  512 px do conjunto hicolor em `packaging/icons/`. Com a bandeja ativa, o
  autostart abre `prci --hidden`: a janela nasce oculta e o Perci fica só
  na bandeja.
- `service_base.go` + `service_*.go` — 6 structs, um `application.Service`
  do Wails por domínio (`application.NewService(x)` para cada um, em vez
  do `Bind` do v2): `HomeService` (service_home.go), `LinuxService`
  (service_linux.go), `DevSetupService` (service_devsetup.go),
  `DockerService` (service_docker.go), `DevToolsService`
  (service_repos.go/service_agentskills.go/service_aicontext.go/
  service_mcpservers.go — um struct só, métodos espalhados em 4 arquivos)
  e `TrayService` (service_tray.go). Cada um embute `serviceBase`
  (service_base.go: `wailsApp`/`exe`/`pickFolder`/`runAction`/
  `eventWriter`, compartilhados sem duplicar código). `runAction` executa
  uma ação longa por vez no app inteiro (trava global `runMu`, inclusive a
  janela da bandeja): uma segunda ação é recusada com erro síncrono só
  para quem chamou; um panic vira ação com falha em vez de derrubar o app.
  Caminhos de arquivo vindos do frontend só são aceitos se saíram de um
  diálogo nativo da sessão (`approvePath`/`requireApprovedPath`); pastas de
  projeto passam por `requireFolder`. `service_checklist.go`
  generaliza o par `Get*Info`/`Run*` das telas "checklist simples"
  (`checklistCatalog[T]`/`getChecklistInfo`/`runChecklist`).
- `catalog.go` — as 5 categorias/itens da sidebar (Home, Linux,
  Desenvolvimento, Docker, Dev Tools).
- `frontend/dist/index.html` — o HTML e o `<style>` da página; nenhum
  script inline (a CSP só aceita scripts da própria origem —
  `csp_test.go` garante isso).
- `frontend/dist/js/` — o JavaScript, sem bundler:
  - `bootstrap.js` — o único módulo ES: importa os bindings e o runtime do
    Wails e os expõe como `App`/`Events`/`Browser`.
  - os demais são scripts clássicos (`defer`) que compartilham um único
    escopo global, carregados na ordem das tags do `index.html`: `core.js`
    (estado e ciclo de execução — `startExecution`/`activeRun`/`finishRun`/
    `failRun`, `ACTION_DONE_HANDLERS`), `navigation.js` (sidebar e
    `selectItem`, com o registry de telas `screenFor`), `widgets.js`
    (modais, escape, construtores de campos, `latestOnly`,
    `reconcileKeyedList`) e `screens/*.js` (uma tela ou família de telas
    por arquivo).
  - Uma nova tela: container `<div>` no `index.html`, render em
    `screens/`, entrada em `screenTable()` (`navigation.js`) e, se ela
    precisar se atualizar ao fim de uma ação, em `ACTION_DONE_HANDLERS`
    (`core.js`). Toda chamada que inicia uma ação após `startExecution`
    encadeia `.catch(failRun)`.
- `frontend/dist/bindings/` — JS gerado por `wails3 generate bindings`
  (`make generate-bindings`, na raiz) a partir dos métodos exportados dos
  6 serviços acima — um arquivo `.js` por serviço (`homeservice.js`,
  `linuxservice.js`, etc.), mais um `index.js` que os reexporta todos;
  substitui o global `window.go.main.App.*` que o v2 injetava
  automaticamente. `js/bootstrap.js` importa os 6 e monta o objeto `App`
  com `Object.assign` (mantém as chamadas `App.Método(...)` já existentes
  sem precisar renomear cada uma por namespace). **Versionado no disco
  como qualquer outro asset vendorizado** (mesma lógica do xterm.js) — só
  precisa ser regenerado (e recomitado) quando a assinatura de um método
  bindado muda ou um novo é adicionado; não faz parte de `make build`.

Nenhum arquivo aqui importa nada de `charmbracelet/*` — o pacote
compartilhado `internal/ui` (painéis de terminal: `PrintHeader`/`Info`/
`Err`/`Warning`/`Success`/`Step`) também não, desde a remoção da TUI.

## Dependências de sistema (Linux)

```
sudo apt install -y libgtk-4-dev libwebkitgtk-6.0-dev
```

GTK4 + WebKitGTK 6.0 é a pilha **padrão** do Wails v3 (ao contrário do v2,
não precisa de nenhuma build tag — `-tags gtk3` existe como caminho legado
pra quem ainda depende de GTK3/WebKit2GTK 4.1, mas não é o que este projeto
usa).

## Rodar em desenvolvimento

```
go build -o /tmp/prci-gui ./cmd/prci-gui
/tmp/prci-gui
```

Ou, a partir da raiz do repositório, `make build`/`make run`.

Se você mudar a assinatura de um método bindado em `App` (ou adicionar um
novo), regenere os bindings do frontend antes de testar:

```
make generate-bindings
```

Isso requer o CLI `wails3` instalado uma vez (mesma versão pinada em
`go.mod`):

```
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.22
```

## Testar sem tocar no sistema real

Toda tela nova é testada primeiro com o backend mockado — um arquivo
`mock.js` que substitui os bindings gerados (`frontend/dist/bindings/`) e
o runtime do Wails, servido via `python3 -m http.server` a partir de
`frontend/dist/`, aberto no Chrome real via a extensão `claude-in-chrome`
— antes de clicar em qualquer ação com efeito colateral real (instalar
pacotes, criar containers, etc.).

## Onde estão as decisões de design

`docs/en/architecture/decisions.md` (espelho em `docs/pt-br/architecture/decisions.md`) — o log de decisões técnicas e
riscos aceitos que não são óbvios só lendo o código. A documentação de
arquitetura completa (bilíngue) fica em `docs/en/` / `docs/pt-br/`.

🌐 [English](../../en/architecture/decisions.md) | **Português** | 🏠 [Índice](../index.md)

---

# Log de Decisões Internas

Decisões técnicas internas e riscos aceitos que não são óbvios só pela leitura do código.

## 2026-07-14 — appimagetool: continuar baixando da tag de release "continuous" do GitHub

**Contexto**: uma revisão de código apontou `internal/dev/prereqs/catalogue.go` (`installAppImageTool`) por
baixar `appimagetool-<arch>.AppImage` da tag de release `continuous` do AppImageKit sem
verificação de checksum, rodando com `RequiresSudo: true`. A correção sugerida era fixar uma
release estável e versionada, com o checksum publicado dela.

**Constatação**: verificado diretamente na API do GitHub
(`gh api repos/AppImage/AppImageKit/releases`). A última release "estável" com tag (`13`, publicada
em 2020-12-31) teve todos os assets renomeados com o prefixo `obsolete-` pelo próprio projeto
AppImageKit (ex.: `obsolete-appimagetool-x86_64.AppImage`). A tag `continuous` é o canal de
distribuição mantido de fato, e é assim há anos — não é um descuido do nosso código. Nenhuma release,
estável ou continuous, publica um `checksums.txt` ou `.sha256` por arquivo; só existem arquivos `.zsync`,
que servem para downloads incrementais/delta e não dão nenhuma garantia de integridade contra uma
origem comprometida.

**Decisão**: manter `installAppImageTool` baixando da tag `continuous` via HTTPS, sem
verificação de checksum. Fixar uma versão "estável" instalaria uma ferramenta que o próprio AppImageKit
declara obsoleta, e de qualquer forma não há nada publicado upstream para verificar o asset baixado.
É a mesma categoria de risco aceito do padrão de instalador `curl | sh` usado em outros pontos de
`internal/dev/*` (Starship, nvm, Zed, Claude Code, Kitty, Linux Toys): o transporte HTTPS é a única
garantia de integridade disponível, porque o projeto upstream não publica nada mais forte.

**Rever se**: o AppImageKit passar a publicar checksums ou assinaturas GPG para os assets de
`continuous`, ou lançar uma nova release estável com tag que não esteja marcada como obsoleta.

**Substituída (2026-09-29)**: a ferramenta passou a ter repositório próprio (`AppImage/appimagetool`),
que publica releases com tag e um digest SHA-256 por asset na API do GitHub. O `installAppImageTool`
agora fixa a versão `1.9.1`, verifica esse digest e instala o arquivo via
`executor.PrivilegedInstall` (o root reverifica o checksum numa cópia própria e instala como
`root:root` 0755). O risco aceito acima não se aplica mais.

## 2026-09-14 — arm64 retirado das releases publicadas na mudança para a GUI Wails

**Contexto**: a remoção da TUI/CLI (`internal/tui`, `internal/app`, `cmd/prci`) deixou `cmd/prci-gui`
(Wails) como o único binário a compilar e publicar. O pipeline de release antigo fazia cross-compile do
binário TUI/CLI em Go puro para `linux/amd64` e `linux/arm64` com um simples `GOARCH=arm64 go
build` — sem cgo envolvido.

**Constatação**: `cmd/prci-gui` exige cgo (bindings GTK3/WebKit2GTK via o backend Linux do Wails).
`GOARCH=arm64 go build` sozinho não faz cross-compile de um binário que depende de cgo — é preciso um
cross-toolchain arm64 (ex.: `CC=aarch64-linux-gnu-gcc`) e as versões arm64 dos pacotes `-dev` de
GTK3/WebKit2GTK instalados na máquina de build, e o `.github/workflows/release.yml`
(um runner padrão `ubuntu-latest` amd64) não provisiona nenhum dos dois.

**Decisão**: `make release`/`release.yml` agora só compilam/publicam `linux/amd64`. Montar releases
arm64 exigiria um runner arm64 nativo hospedado pelo GitHub ou uma etapa dedicada de cross-toolchain —
tratado como trabalho futuro e separado, sem bloquear esta mudança (nenhum usuário conhecido do perci
usava o build arm64 anterior; o projeto ainda não tem usuários em produção confirmados). Compilar
a partir do código-fonte diretamente numa máquina arm64 não é afetado e está documentado como alternativa
(`docs/pt-br/getting-started/installation.md`).

**Rever se**: alguém realmente precisar do perci em Linux arm64, ou os runners arm64 do GitHub Actions
ficarem disponíveis de graça para repositórios públicos.

## 2026-09-14 — GUI migrada do Wails v2 para o v3, adotando a nova stack padrão GTK4/WebKitGTK 6.0 em vez do opt-in legado GTK3

**Contexto**: uma funcionalidade de bandeja do sistema está planejada para `cmd/prci-gui`. O Wails v2 não tem
suporte nativo a systray nem a múltiplas janelas; os dois são necessários (systray para a funcionalidade em si,
múltiplas janelas porque a interação planejada na bandeja abre uma janela separada da principal).

**Constatação**: o Wails v3 (`v3.0.0-beta.22`) oferece os dois nativamente. Ele também muda a dependência de sistema
padrão no Linux de GTK3 + WebKit2GTK 4.1 para GTK4 + WebKitGTK 6.0 — uma escolha deliberada do upstream,
não um efeito colateral: GTK4/WebKitGTK 6.0 é a stack estável e padrão do v3, com GTK3/WebKit2GTK
4.1 mantidos só como opt-in legado atrás de `-tags gtk3`. O v2 exigia `-tags
desktop,production,webkit2_41` em todo build; a stack padrão do v3 não precisa de build tag nenhuma. O
mecanismo de binding do frontend também mudou: o v2 injetava automaticamente um global `window.go.main.App.*` em
tempo de execução, enquanto o v3 gera bindings ES module antecipadamente via `wails3 generate bindings`.

**Decisão**: migrar `cmd/prci-gui` para o Wails v3 na stack padrão GTK4/WebKitGTK 6.0 (não pelo caminho
legado `-tags gtk3` — sem motivo para carregar adiante a stack mais antiga e fora do padrão). As build tags
foram removidas por completo (`go build ./cmd/prci-gui`, sem `-tags`). Os bindings gerados são versionados em
`frontend/dist/bindings/`, o mesmo tratamento de vendorização já usado para o xterm.js — regenerados via
`make generate-bindings` só quando a assinatura de um método `App` bindado muda ou um novo é adicionado,
não como parte do build normal. `go build`/`go vet`/`go test`/`gofmt` passam todos após a migração.

**Rever se**: o Wails v3 sair do beta com uma mudança incompatível na API de bindings ou de systray,
ou a funcionalidade de bandeja do sistema for descartada e o suporte a múltiplas janelas/systray acabar
sem uso.

## 2026-09-29 — Frontend dividido em scripts clássicos com um escopo global compartilhado

**Contexto**: O frontend inteiro era um único `<script type="module">` inline (~2600 linhas, uma
closure) no `index.html`. A Content-Security-Policy adicionada no mesmo dia só conseguia liberá-lo por
hash, que mudava a cada edição.

**Decisão**: mover o código para `frontend/dist/js/`, cortado nos limites de seção que já existiam: o
`bootstrap.js` é o único módulo ES (importa os bindings e o runtime do Wails e expõe `App`/`Events`/
`Browser`), e os demais são scripts clássicos com `defer` que compartilham um único escopo global,
carregados em ordem. Isso mantém exatamente a semântica da closure antiga — toda tela lê e atualiza o
mesmo estado compartilhado — sem bundler (uma restrição deliberada do projeto) e sem reescrever ~2600
linhas com imports/exports explícitos. Verificado renderizando todos os itens da sidebar e a janela
compacta da bandeja num Chrome headless, antes e depois, com o runtime do Wails simulado: mesmo
conteúdo visível, nenhum erro, as mesmas chamadas ao backend. `script-src 'self'`, sem hash.

**Rever se**: o frontend adotar um passo de build, ou os globais compartilhados começarem a colidir
(uma divisão em módulos ES com exports explícitos seria o próximo passo).

## 2026-09-29 — Uma ação longa por vez, no app inteiro

**Contexto**: O `runAction` iniciava cada ação numa goroutine própria sem nenhuma serialização; só a
flag `running` do frontend, por janela, evitava sobreposições, e a janela compacta da bandeja tem a
sua. Duas ações ao mesmo tempo misturam a saída no terminal, disputam a trava do gerenciador de
pacotes e a configuração do Nginx, e pedem a senha duas vezes.

**Decisão** (escolhida pelo usuário entre uma trava global ou por domínio): um único mutex no nível
do pacote (`runMu`) para todos os services e janelas. Uma segunda ação é recusada com um erro síncrono
devolvido só a quem chamou (nunca pelo `action-done`, que toda janela recebe); um panic numa ação vira
uma ação com falha em vez de derrubar o processo.

**Rever se**: surgir uma necessidade real de rodar ações de domínios diferentes em paralelo.

## 2026-09-29 — O autostart da bandeja inicia o Perci oculto

**Contexto**: A entrada de autostart da bandeja dizia "Perci iniciado oculto na bandeja", mas a janela
principal sempre abria no login.

**Decisão** (escolha do usuário): a entrada de autostart executa `prci --hidden`; o `main.go` cria a
janela principal oculta só quando essa flag está presente **e** a bandeja está ligada (senão o app
ficaria inacessível). A entrada é regravada a cada início com a bandeja ligada, para que entradas
antigas recebam a flag.

**Rever se**: o Wails ganhar tratamento de instância única, para que abrir o Perci pelo menu com ele
na bandeja foque essa instância em vez de iniciar uma segunda.

## 2026-09-29 — O instalador do Linux Toys roda como root, pelo stdin

**Contexto**: O instalador oficial do linux.toys escala com o próprio `sudo`, que não consegue pedir
senha dentro da GUI (sem terminal) — sem credenciais em cache, a instalação simplesmente falhava.

**Decisão** (escolha do usuário, em vez de instalar o `.deb`/`.rpm` da release ou remover o item):
executar o script oficial como root via pkexec (um pedido de senha). O script chega ao root pelo
stdin (`bash -s`), nunca como caminho de arquivo, para que um processo do mesmo usuário não possa
trocá-lo durante o diálogo de senha; um stdin que não é tty também faz ele seguir o próprio caminho
não interativo. O risco aceito "sempre a versão mais nova, sem checksum" deste instalador não muda.

**Rever se**: o script deixar de funcionar como root, ou o projeto quiser instalações com checksum
verificado — a release publica `.deb`/`.rpm` com digests SHA-256.

## 2026-09-29 — Credenciais do MariaDB sobre dados existentes são recusadas, não regravadas

**Contexto**: A imagem `mariadb` só aplica `MYSQL_USER`/`MYSQL_PASSWORD`/`MYSQL_ROOT_PASSWORD` num
diretório de dados vazio. Editar ou recriar o MariaDB com outras credenciais sobre dados existentes
deixava o `config.yaml` com credenciais que o banco não conhecia.

**Decisão** (escolha do usuário, em vez de aplicar a troca com `ALTER USER`): recusar com uma
mensagem clara (manter as credenciais, alterá-las dentro do banco antes, ou remover o diretório de
dados). `data_user`/`data_pass` registram as credenciais com que o diretório de dados foi inicializado
e sobrevivem à remoção do contêiner da stack; configs anteriores a esses campos recebem um aviso, não
uma recusa.

**Rever se**: alterar as credenciais do banco pelo Perci virar uma necessidade real.

## 2026-09-29 — "Atualizar Sistema" não apaga mais logs nem remove pacotes por conta própria

**Contexto**: Toda atualização do sistema também limpava o journal (`journalctl --vacuum-time=7d`) e rodava `apt-get autoremove`/`dnf autoremove` — apagando logs que alguém pode precisar para diagnóstico e removendo pacotes que o usuário nunca pediu para remover.

**Decisão** (escolha do usuário): as duas viram caixas na tela, desmarcadas por padrão; os runtimes Flatpak sem uso seguem a opção de autoremove. A atualização em si cobre `apt`/`dnf` (só `full-upgrade` — o `upgrade` separado era redundante), Snap e Flatpak nas instalações do usuário e do sistema.

**Rever se**: os usuários marcarem as duas caixas quase sempre, tornando o opt-out o padrão melhor.

## 2026-09-29 — Remover o Node.js continua apagando o `~/.nvm`, depois de um aviso explícito

**Contexto**: O pré-requisito Node.js é instalado via nvm, e removê-lo apaga a pasta `~/.nvm` inteira — todas as versões do Node e pacotes npm globais dela, inclusive os que o Perci não instalou.

**Decisão** (escolha do usuário, em vez de remover só a LTS que o Perci instalou): continuar apagando o `~/.nvm`, mas a GUI mostra o que será perdido e pede confirmação antes de executar. O instalador do nvm continua vindo do `HEAD` do nvm.

**Rever se**: usuários mantiverem versões próprias do Node no nvm e as perderem, ou o instalador do `HEAD` do nvm quebrar.

## 2026-09-29 — Imagens base são reconstruídas quando aquilo de que foram construídas muda

**Contexto**: As imagens `perci-php*`/`perci-node*`/combo eram construídas uma vez e nunca mais, então correções no Dockerfile, no `php.ini` ou no `supervisord.conf` nunca chegavam a máquinas que já as tinham.

**Decisão** (escolha do usuário, em vez de uma ação manual de "reconstruir"): cada imagem leva um label `perci.hash` (um hash dos arquivos e build args do build). Uma imagem cujo label não bate é reconstruída automaticamente na próxima vez que um app precisar dela, e o terminal lista os contêineres que continuam na imagem antiga até serem recriados.

**Rever se**: as reconstruções ficarem lentas ou frequentes o bastante para justificar perguntar antes.

## 2026-09-29 — A autoatualização compara versões localmente como semver

**Contexto**: Qualquer diferença entre a versão em execução e a última release contava como atualização — uma build local mais nova que a release recebia a oferta de um downgrade.

**Decisão** (escolha do usuário, em vez de adicionar `golang.org/x/mod/semver`): uma comparação local pequena (MAJOR.MINOR.PATCH mais prerelease, metadados de build ignorados) em `selfupdate.IsNewer`. Versões que não são semver (ex.: uma build `dev`) mantêm o comportamento antigo: qualquer diferença conta como mais nova.

**Rever se**: as tags de release deixarem de seguir semver.

## 2026-09-29 — Fedora: o JDK do Flutter é o Eclipse Temurin 21

**Contexto**: Os repositórios do Fedora 44 só trazem o JDK 25 (`java-latest-openjdk`), e o toolchain Android/Flutter que o Perci configura tem como alvo o JDK 21. Verificado num contêiner `fedora:44`.

**Decisão** (escolha do usuário): no Fedora, instalar o `temurin-21-jdk` do repositório assinado com GPG da Adoptium (`distro.InstallFromSignedRepo`); Debian/Ubuntu continuam com o `openjdk-21-jdk` dos próprios repositórios.

**Rever se**: o Fedora voltar a ter um pacote do JDK 21, ou o toolchain passar do JDK 21.

## 2026-09-29 — Um `workspace_path` vazio significa "não escolhido", em todo lugar

**Contexto**: O `config.Load` preenchia `~/workspace` só quando o `config.yaml` não existia; um arquivo sem `workspace_path` voltava vazio. O Importar configurações adota o workspace exportado só quando o local está vazio, então adotar ou recusar dependia de alguma outra configuração já ter sido salva.

**Decisão** (escolha do usuário, em vez de sempre preencher o padrão): o `Load` nunca preenche campo nenhum. Vazio significa "ainda não escolhido", com ou sem arquivo — a tela Configurações avisa, o Importar configurações adota o workspace exportado — e `Config.Workspace()` resolve `~/workspace` para o código que precisa de uma pasta de verdade. As chaves `distro`/`de`, que nada lia, foram removidas.

**Rever se**: outra configuração precisar de um padrão aplicado na leitura.

## 2026-09-29 — Os pacotes de `npx`/`uvx` têm versão fixa

**Contexto**: A CLI do skills.sh e os servidores MCP Filesystem/SQLite rodavam como "o que o `npx`/`uvx` resolver como mais novo", então uma release comprometida ou quebrada chegaria a todas as máquinas na próxima execução.

**Decisão** (escolha do usuário): fixar `skills@1.7.0`, `@modelcontextprotocol/server-filesystem@2026.8.31` e `mcp-server-sqlite@2025.4.25` (constantes em `internal/dev/agentskills` e `internal/dev/mcpservers`), cada um testado rodando; atualizar é uma troca deliberada de versão.

**Rever se**: manter as versões em dia virar um peso, ou os registros passarem a oferecer um jeito de verificar as releases.

## 2026-09-29 — Código sem ligação deixado pela TUI: removido, menos duas funcionalidades

**Contexto**: Uma passada do `deadcode` (golang.org/x/tools) encontrou funções sem nenhum chamador, restos da TUI/CLI.

**Decisão** (escolha do usuário): remover os helpers de "atualizar tudo o que está instalado" (`ide`/`llm`/`terminal.Update`, `checklist.UpdateInstalled`), o `appstack.ContainerLogs` e o `flutter.Upgrade`. Manter o `internal/dev/terminal/contextmenu.go` (entradas "abrir terminal aqui" do gerenciador de arquivos) e o `terminal.UninstallStarship`, para ligar na GUI depois. O `golang.EnsurePathInBashrc` era uma chamada faltando, não código morto: instalar o Go nunca adicionava `/usr/local/go/bin` ao PATH, e agora adiciona.

**Rever se**: as duas funcionalidades mantidas continuarem sem ligação — aí remover também. (Ambas ligadas em 2026-10-01 — veja abaixo.)

## 2026-09-29 — Testes do frontend em Node puro, e um workflow de CI

**Contexto**: O frontend não tinha teste automatizado, e o único workflow (`release.yml`) só rodava as verificações Go ao publicar uma tag.

**Decisão** (escolha do usuário, em vez de trazer o harness com Chrome headless para o repositório): uma suíte `node:test` sem dependências npm — os scripts clássicos rodam num contexto `vm` do Node com um DOM falso mínimo — mais `node --check` em todos os scripts (`make test-frontend`). Um `ci.yml` novo roda gofmt, go vet, golangci-lint, `go test -race` e a suíte do frontend em todo push para a `main` e em todo pull request, com as mesmas permissões mínimas e actions fixadas por SHA do `release.yml`; usa o Node já instalado no runner em vez de adicionar uma action de setup.

**Rever se**: escapar uma regressão que só um navegador real pegaria — aí adicionar o walk com Chrome headless como job opcional do CI.

## 2026-10-01 — Entradas de menu de contexto dos terminais seguem o estado instalado; Starship ganha botão de remoção

**Contexto**: `internal/dev/terminal/contextmenu.go` e `terminal.UninstallStarship` foram mantidos sem ligação em 2026-09-29, para serem ligados na GUI depois.

**Decisão** (escolha do usuário): toda execução de "Aplicativos: Terminais" termina sincronizando as entradas "Abrir no <terminal>" com o que está instalado (`SyncContextMenuEntries`) — sem tela ou opção extra, e cobrindo terminais instalados antes (ou fora do Perci). O Starship ganha um segundo botão extra, **Remover Starship**, com confirmação e sempre habilitado (a função é idempotente); a entrada do frontend em `MULTISELECT_SCREENS` passa a aceitar uma lista `extras`.

**Rever se**: usuários quiserem as entradas sem o terminal ser gerenciado pelo Perci, ou por terminal — aí uma opção separada por item.

## 2026-10-01 — Entradas de SKILL com curinga são removidas pelo nome; Go instala num único lote com linha de PATH segura para o fish

**Contexto**: Escrever testes para `internal/dev/{agentskills,golang,llm,sdks}` revelou três bugs: remover uma entrada de SKILL com `Arg: "*"` rodava `skills remove --skill '*'`, que apaga todas as skills dos quatro agentes no escopo (confirmado com `skills@1.7.0` num HOME temporário); instalar o Go fazia uma chamada ao pkexec por passo; e a linha de PATH do Go sem aspas quebrava o `PATH` no fish (confirmado no fish 4.0.2).

**Decisão** (escolhas do usuário): as entradas com curinga são removidas pelos nomes das skills cuja origem registrada é o repositório da entrada — consultados com `skills list --json` (legível por máquina, com a origem normalizada para `owner/repo`) em vez de interpretar a saída só para humanos de `skills add --list`; se a consulta falhar, a remoção é recusada. A extração e a troca do Go rodam num único script privilegiado, que também reverifica o checksum numa cópia de dono root. A linha de PATH do Go fica entre aspas (válida em bash, zsh e fish); a linha antiga continua reconhecida.

**Rever se**: a CLI `skills` mudar o formato de `list --json` ou deixar de registrar `source` — aí o catálogo precisa de nomes explícitos de skills por repositório.

## 2026-10-01 — Alacritty num único lote, restos do OpenCode removidos, caminho do workspace só pelo diálogo

**Contexto**: Testar `internal/dev/terminal`, `internal/dev/llm` e os métodos bindados da GUI revelou: o Alacritty na família Debian rodava `add-apt-repository universe` (inexistente no próprio Debian), `apt-get update` e `apt-get install` como três pedidos de senha; desinstalar o OpenCode deixava `~/.opencode/bin` e as linhas de PATH que o instalador dele acrescenta ao rc do shell; e `SetWorkspacePath` gravava qualquer caminho enviado pela página, diferente de todos os outros métodos bindados que recebem caminho.

**Decisão** (escolhas do usuário): o Alacritty instala via `distro.InstallPkgs` (um lote; o pacote está no main do Debian e no universe do Ubuntu, habilitado por padrão). Desinstalar o OpenCode remove `~/.opencode/bin` (`~/.opencode` só se ficar vazio) e só as linhas `# opencode` exatas que o instalador escreveu; as configurações em `~/.config/opencode` ficam. `SetWorkspacePath` aceita só uma pasta escolhida no diálogo nativo nesta sessão (`requireApprovedPath`), já que o Docker cria pastas dentro do workspace e o monta nos contêineres.

**Rever se**: uma distro suportada da família Debian vier sem o `universe` habilitado, ou o instalador do OpenCode mudar a pasta de instalação ou as linhas do rc.

## 2026-10-02 — Faixas de versão do Moodle seguem a faixa de PHP de cada release

**Contexto**: A faixa "5.1+" oferecia PHP 8.2–8.4, mas cobre o Moodle 5.1, 5.2 e 5.3, e o 5.2/5.3 exigem PHP 8.3–8.4. A faixa única "4.x" oferecia PHP 8.0–8.1, enquanto o Moodle 4.1 suporta 7.4–8.1, o 4.2/4.3 suportam 8.0–8.2 e o 4.4/4.5 suportam 8.1–8.3 (requisitos de PHP do docs.moodle.org).

**Decisão** (escolhas do usuário): "5.1+" oferece só PHP 8.3/8.4 (continua válido para o 5.1). "4.x" foi dividido em três faixas com os intervalos exatos: `4.1` (7.4–8.1), `4.2-4.3` (8.0–8.2) e `4.4-4.5` (8.1–8.3), todas com a receita clássica do Nginx. `4.x` deixa de ser oferecido, mas continua válido, com a faixa do 4.1, para contêineres já salvos com ele, e o formulário de edição continua mostrando esse valor.

**Rever se**: uma nova versão do Moodle mudar a faixa de PHP ou o layout do Nginx, ou o perci adicionar uma versão de PHP.

## 2026-10-09 — Flatpaks do sistema atualizados dentro do lote privilegiado

**Contexto**: O "Atualizar Sistema" pedia a senha de administrador várias vezes numa máquina Zorin. Pacotes, Snap e journal já rodavam num único lote privilegiado; o Flatpak rodava depois como usuário, então atualizar ou limpar a instalação do sistema passava pela autorização própria do Flatpak (polkit), cujo resultado depende das regras de cada distro e dos grupos do usuário.

**Decisão** (escolha do usuário): `flatpak update --system` e, com a opção de autoremove, `flatpak uninstall --system --unused` da instalação do sistema rodam como root dentro do mesmo lote (ambos Soft, para que uma falha do Flatpak não aborte a atualização). A instalação do usuário é atualizada depois, sem privilégios, e a opção de autoremove agora limpa runtimes sem uso nas duas instalações, em vez de só no escopo configurado.

**Rever se**: outra etapa da atualização passar a pedir autorização fora do lote.

## 2026-10-09 — Plugins Moodle são montados no host, não linkados

**Contexto**: Um plugin desenvolvido na sua própria pasta (ex.: `oito2-hub/mdle-exemplo`) precisa rodar em vários contêineres Moodle (4.1 e 5.1) e aparecer dentro do código de cada Moodle no editor. Um link simbólico falha no Docker: o contêiner do app monta só a própria pasta, e o PHP resolve links em `__DIR__`, então `require_once(__DIR__ . '/../../config.php')` procura fora do Moodle (testado no `perci-php81`). Uma montagem só no Docker funciona no contêiner, mas deixa a pasta vazia no editor. Uma montagem (bind mount) no host foi testada de ponta a ponta: depois de reiniciar o contêiner, o plugin é uma pasta comum dentro dele (`__DIR__` continua no Moodle), dois contêineres compartilham a mesma origem, arquivos gravados pelo contêiner ficam com o usuário como dono, e a montagem só leitura do Nginx o enxerga.

**Decisão** (escolhas do usuário): **Dev Tools → Plugins Moodle** substitui a tela Links Simbólicos, que não chegou a ser lançada. O plugin é montado com uma unidade de montagem do systemd (`Before=docker.service`); tipo e nome vêm do `version.php` (travados) ou, com a pasta vazia, do formulário; o Moodle é escolhido numa lista dos contêineres Moodle do Perci e o destino segue o `lib/components.json` dele. A coluna de status confere o estado real (dispositivo e inode no host, no contêiner e no Nginx); a linha reinicia o contêiner e o Nginx para aplicar, e remover também reinicia. Um destino com conteúdo é recusado.

**Rever se**: o Moodle deixar de resolver as páginas de plugin por `__DIR__`, os contêineres passarem a montar o workspace inteiro, ou unidades de montagem do systemd não estiverem disponíveis numa distro suportada.

## 2026-10-13 — Itens da pós-instalação são aplicados um de cada vez

**Contexto**: Linux → Pós-instalação era uma checklist com um botão "Executar"; cada item selecionado fazia sua própria escalada, então uma execução pedia a senha de administrador uma vez por item, e nada mostrava o que já tinha sido aplicado.

**Decisão** (escolhas do usuário): cada item tem seu próprio botão **Aplicar**, que vira **Aplicado** desabilitado quando o item já está aplicado; o botão "Executar" saiu e um item aplicado não roda de novo. O estado é detectado no sistema quando o item deixa um (pacotes, arquivo do sysctl, swap, `fstrim.timer`, apps do Flathub, componentes do Ubuntu, RPM Fusion); os itens que não deixam nenhum (atualizar o índice de pacotes, atualizar o sistema, detectar drivers, limpeza) são registrados em `postinstall_applied` depois de rodarem com sucesso. Um item cuja dependência não está aplicada fica com o botão desabilitado e a dica "Requer: <item>".

**Rever se**: algum item passar a ter estado detectável, ou rodar de novo um item aplicado passar a ser necessário.

## 2026-10-13 — Pré-requisitos são gerenciados um item de cada vez, em ordem de dependência

**Contexto**: Desenvolvimento → Pré-requisitos era uma checklist aplicada em lote (marcar para instalar, desmarcar para remover), sem como atualizar um item e sem nada impedindo instalar um item antes do que o instalador dele precisa (curl para GitHub CLI, Node.js e AppImageTool; Go para o Wails3 CLI).

**Decisão** (escolhas do usuário): uma tabela com uma linha por item e seus próprios botões **Instalar**, **Atualizar** e **Desinstalar** — Instalar só enquanto o item não está instalado, os outros dois só quando está. Atualizar atualiza os pacotes do próprio item pelo gerenciador de pacotes, leva o Node.js à última LTS (pelo nvm, trazendo os pacotes npm globais da versão padrão) e reinstala o Wails3 CLI e o AppImageTool, que têm versão fixa. Requisitos: Curl → GitHub CLI, Node.js, AppImageTool; Flatpak → Flatpak Builder; Go (Linguagens e SDKs) → Wails3 CLI; esses vêm primeiro na lista, e o Instalar de um item dependente fica desabilitado com "Requer: …" até eles estarem instalados. Desinstalar é sempre permitido para um item instalado (um dependente já instalado continua funcionando) e sempre pede confirmação.

**Rever se**: algum instalador passar a precisar de outro item do catálogo, ou as versões fixas passarem a acompanhar as releases de origem.

## 2026-10-13 — Linguagens e SDKs usa a tabela de Pré-requisitos

**Contexto**: Linguagens e SDKs (Go e Flutter) era a mesma checklist de instalar/remover que Pré-requisitos tinha. O instalador do Go baixa com `curl` e o do Flutter começa com `git clone`, ambos itens de Pré-requisitos.

**Decisão** (escolha do usuário: mesmo layout e funcionamento de Pré-requisitos): uma linha da tabela por SDK com Instalar/Atualizar/Desinstalar, compartilhada com Pré-requisitos (uma tela no frontend, um helper no service). Atualizar instala o Go estável mais recente só quando ele é mais novo que o instalado, e roda `flutter upgrade` no Flutter. Go requer Curl e Flutter requer Git, mostrados como "Requer: … (Pré-requisitos)" até estarem instalados. Desinstalar sempre pede confirmação.

**Rever se**: algum instalador de SDK deixar de depender dessas ferramentas, ou entrarem mais SDKs com requisitos entre si.

## 2026-10-13 — Suporte ao Budgie (Ubuntu Budgie 24.04/26.04, Fedora Budgie 44)

**Contexto**: O Budgie 10.9 (Ubuntu Budgie 24.04) define `XDG_CURRENT_DESKTOP=Budgie:GNOME` e o Budgie 10.10 (Ubuntu Budgie 26.04, Fedora Budgie 44, Wayland com labwc) define `Budgie` — lido nos arquivos de sessão do projeto. O Ubuntu Budgie informa `ID=ubuntu`, então já caía no perfil do Ubuntu GNOME, que instala `gnome-tweaks` e `gnome-software-plugin-flatpak`; este depende de `gnome-software` (conferido em `ubuntu:24.04`/`ubuntu:26.04`), uma segunda loja de aplicativos num desktop que usa o Nemo e a loja de snaps. O Ubuntu Budgie 26.04 já traz o VLC como pacote. As ações de pós-instalação do Fedora não dependem do desktop; o spin Budgie usa apps do KDE (Dolphin, Konsole, Discover) e já traz o remote do Flathub.

**Decisão** (escolhas do usuário): o `DetectDE` reconhece o Budgie (antes do GNOME). O Ubuntu Budgie ganha perfis próprios 24.04/26.04: a lista do Ubuntu sem `gnome-tweaks` e `gnome-software-plugin-flatpak`, e no 26.04 o passo do Flatpak instala só o gerenciador de web apps. O Fedora Budgie ganha o perfil "Fedora 44 Budgie" com as ações do Fedora.

**Rever se**: o Ubuntu Budgie adotar o GNOME Software, ou uma versão do Budgie mudar o `XDG_CURRENT_DESKTOP`.

## 2026-10-13 — Endurecimento após code review

**Contexto**: Um code review completo (segurança, bugs, consistência, documentação) levantou mais de cem achados, vários com mais de uma correção razoável. Eles foram implementados juntos antes de publicar a 1.2.0.

**Decisão** (escolhas do usuário): implementar o plano inteiro antes da release. Remover o Linux Toys por completo, sem desinstalador. Manter o clone completo do Flutter. Limitar o tempo dos comandos de rede sem privilégio (`curl --connect-timeout/--max-time`, limite de 30 minutos); nada interrompe `apt`/`dnf` e não há botão Cancelar. Registrar, por agente, quais agentes aceitaram uma SKILL ou servidor MCP, e mostrar "parcial" quando algum falta. Nunca sobrescrever um `credential.helper` existente; no Fedora o "Libsecret Tools" instala o `git-credential-libsecret`, e a queda para `cache` é avisada. Avisar na descrição do Flutter que as licenças do Android SDK são aceitas. Gerar o `MOODLE_PATH` como o relativo `.`. Vincular o registro da pós-instalação ao `/etc/machine-id`. Fechar ou sair durante uma ação não encerra: a janela principal abre na aba Execução com um aviso. Separar o workflow de release em jobs reutilizáveis `test` e `build` e adicionar o ShellCheck ao CI. Manter os instaladores oficiais, com timeouts, e fixar o nvm (tag), o Codex (versão) e a Nerd Font (SHA-256). Fixar as SKILLs por commit, desligar a telemetria do CLI `skills` e mostrar um aviso de conteúdo de terceiros. Fixar composer/pecl, pegar o Node da imagem oficial do Node em vez do NodeSource e renovar o certificado do mkcert antes de vencer. Deixar de gerar o `.claudeignore` (o Claude Code o ignora): mesclar regras de leitura `permissions.deny` no `.claude/settings.json` e manter o `.aiexclude`. Validar a configuração do Nginx com `nginx -t` num contêiner descartável quando o Nginx está parado. Agrupar a saída do terminal a cada ~30 ms. Usar `--mount` em vez de `-v`, recusando vírgula no caminho do workspace.

**Rever se**: os pacotes do PHP 7.4/8.0 sumirem do `archive.debian.org` (essas imagens teriam de ser descartadas), o CLI `skills` ganhar um lockfile para instalações fixadas, ou um botão Cancelar para ações longas for desejado.

## 2026-10-14 — Montagens de plugins Moodle ficam fora da sincronização do MEGA

**Contexto**: O MEGAsync identifica arquivos pelo inode. Uma montagem de plugin mostra os inodes da pasta de desenvolvimento dentro da pasta `html` do Moodle; com as duas pastas em sincronizações diferentes do MEGA, o MEGAsync tomou a montagem por uma mudança entre sincronizações e mandou os arquivos de desenvolvimento para a pasta `.debris` dele.

**Decisão** (escolha do usuário, em vez de recusar ou só avisar): antes de montar, o Perci grava `-dpG:<destino>` no `.megaignore` mais próximo acima do destino, recusa a montagem quando não consegue e remove a linha junto com a montagem. Uma linha que o usuário já tinha fica como está. Testado com montagens reais, edições pelo caminho do Moodle, reinício do MEGAsync e do computador.

**Rever se**: o MEGAsync deixar de respeitar as regras de caminho do `.megaignore` antes da detecção de mudanças por inode, ou outro cliente de sincronização com o mesmo comportamento precisar de suporte.

## 2026-10-14 — Docker → Exportar/Importar, item a item, mesclando

**Contexto**: O Exportar/Importar ficava em Gerenciar Contêineres, levava sempre a stack Docker inteira, a substituía na importação e deixava de fora as montagens de plugins Moodle.

**Decisão** (escolhas do usuário): item de menu próprio com as abas Exportar e Importar. As duas listam o Nginx, o MariaDB, cada app e cada montagem de plugin para marcar um a um; marcar um item marca as dependências dele. Só as definições viajam (sem arquivos dos projetos nem bancos). A importação mescla: os itens marcados são adicionados ou substituem o app com a mesma pasta, nada mais sai. As montagens de plugin são importadas com uma única senha; uma que não pode ser feita na máquina (pasta de desenvolvimento ou código do Moodle ausentes) é pulada com um aviso.

**Rever se**: os próprios arquivos e bancos precisarem viajar também.

🌐 [English](../../en/architecture/managers.md) | **Português** | 🏠 [Índice](../index.md)

---

# Managers (`internal/manager/*`)

Sustenta a categoria **Dev Tools** da GUI — Repositórios (`manager/repo`, `manager/gitignore`) e IA: Contextos (`manager/ai`) — mais as ações de Backup/Restore em **Docker → Gerenciar Containers** (`manager/db`); veja [Stack Docker de Aplicativos](./appstack.md).

## `manager/ai` — Geração de Contexto IA

Gera arquivos de contexto de agente de IA para um **projeto-alvo** (a pasta escolhida pelo seletor de pastas da GUI, `PickAIContextFolder`) — `AGENTS.md`, `CLAUDE.md`, `.aiexclude`, um `.claude/settings.json` com regras de bloqueio de leitura e guias de linguagem/tópico `.instructions/*.md`, a partir de um catálogo definido em `context.go` (`models`).

- A seleção de modelo/guia é um checklist na tela **IA: Contextos** da GUI (`GetAIContextItems`/`ApplyAIContext`, `cmd/prci-gui/service_aicontext.go`, parte do `DevToolsService`), uma entrada por item do catálogo `manager/ai.Models()`; uma seleção vazia é válida e gera só as regras gerais, sem seção de linguagem.
- Clicar em **Aplicar** sempre sobrescreve (`GenerateSharedFiles(..., overwrite=true, ...)`) sem nenhuma confirmação extra — o próprio clique já é a confirmação, mesma convenção de toda ação não-interativa da GUI. (`ConfirmOverwrite`, que tinha variantes CLI/TUI/não-interativa, foi removida junto com a TUI/CLI.)
- Arquivos `.instructions/*.md` referenciados via `@.instructions/...` no `AGENTS.md` gerado são de fato gravados em disco (`WriteInstruction`), não só referenciados.
- O `AGENTS.md` gerado lista os padrões ativos como um índice que funciona em qualquer agente: uma instrução explícita de "leia o arquivo aplicável antes de começar" e uma linha por modelo com o texto `AppliesTo` (quais tarefas ele cobre) e o caminho `@.instructions/...`. Só o Claude Code transforma `@caminho` em conteúdo (pelo import `@AGENTS.md` do `CLAUDE.md`); o Antigravity transforma um `@caminho` solto em referência de caminho e o Codex não tem mecanismo de import nenhum, então, para os dois, é o texto do índice que faz o agente abrir o arquivo certo. Colar os arquivos dentro do `AGENTS.md` foi descartado: dois padrões já estouram o limite de 32 KiB do Codex e o de 24 KB por regra do Antigravity.
- O guia `PROJECT-DOCUMENTATION.md` do catálogo ("Documentação de projeto") — o padrão que este próprio site de documentação segue — é opcional, não é forçado em todo projeto gerado; veja a seção "When NOT to Use" dele.
- Complementos de documentação por tipo de projeto ("Documentação: Servidor MCP/Plugin Moodle/Aplicação Desktop/Aplicação Mobile/Aplicação Web" → `PROJECT-DOCUMENTATION-{MCP,MOODLE,DESKTOP,MOBILE,WEB}.md`) acrescentam regras específicas de cada tipo sobre essa base. Um `Model` do catálogo pode declarar `Requires` (o nome de outro modelo); o `GenerateSharedFiles` inclui automaticamente qualquer modelo exigido que não tenha sido selecionado (`withRequired`), registra isso no log e mantém a ordem do catálogo, para que o `AGENTS.md` referencie a base antes dos complementos.
- Um `Model` também pode listar nomes `Legacy` (nomes antigos do seu arquivo de instrução, antes de uma renomeação — ex.: `DOCUMENTATION-SITE.md` para a base): o `DetectActiveModels` continua considerando-os ativos, e uma execução com sobrescrita (todo **Aplicar** da GUI) os apaga depois de gravar o arquivo atual — ou quando o modelo é desmarcado —, então os projetos migram no próximo Aplicar.
- Placeholders específicos do Moodle (`{{MOODLE_VERSION}}`, `{{MOODLE_FULLVERSION}}`, `{{MOODLE_PATH}}` (sempre o caminho relativo `.`, nunca o caminho absoluto do usuário), `{{WORKSPACE_PATH}}`) são preenchidos automaticamente detectando o `version.php` (ou `public/version.php`, layout Moodle 5.1+) na pasta escolhida; quando não encontra, os placeholders literais ficam no lugar e um aviso aparece no terminal da aba Execução. Leituras de `version.php` são limitadas a 1 MiB.
- **Arquivos de exclusão**: o `.aiexclude` (o filtro de contexto lido por ferramentas como o Gemini Code Assist) é gerado a partir de um modelo. O Claude Code não lê o `.claudeignore`, então o Perci deixou de gerá-lo; em vez disso, o `.claude/settings.json` recebe regras `permissions.deny` `Read(...)` equivalentes aos padrões do `.aiexclude` (um padrão de diretório `dir/` vira `Read(dir/**)`; o próprio `.claude/` fica de fora). Um `settings.json` existente é mesclado — todas as outras chaves e as regras de bloqueio existentes são mantidas — e um que não pode ser mesclado (não é um objeto JSON, ou `permissions`/`deny` de outro tipo) é deixado intacto, com um aviso. Quando um `.claudeignore` de uma versão antiga é encontrado, um aviso diz que ele não tem efeito e pode ser removido.
- **Arquivos gerados e symlinks**: todo arquivo gerado é gravado com `fsutil.WriteGenerated` — através de um symlink (um `AGENTS.md` gerenciado por dotfiles continua sendo link) e, quando já existe uma versão diferente, a anterior é mantida como `<arquivo>.perci-bak`. Um `CLAUDE.md` que já é link para o `AGENTS.md` é mantido como está, para que o stub nunca seja gravado dentro do `AGENTS.md` através do link.
- **Aprovação de pasta**: a pasta-alvo precisa ter sido escolhida no diálogo nativo de pastas durante a sessão (ou ser uma pasta já registrada); caso contrário a GUI recusa com "pasta não escolhida no seletor".
- Não existe uma ação de "limpar tudo" na GUI — desmarcar um modelo antes selecionado em **IA: Contextos** só remove o `.instructions/*.md` daquele modelo (`GenerateSharedFiles` comparando contra `DetectActiveModels`), não os arquivos compartilhados `AGENTS.md`/`CLAUDE.md`/`.aiexclude`/`.claude/settings.json` — e só enquanto esse arquivo ainda for exatamente o que o Perci gerou (`isPristine`): uma cópia editada ou escrita à mão é mantida, com um aviso. Cópias com nome antigo (`Model.Legacy`) são removidas em execuções com sobrescrita, como antes.

## `manager/db` — Operações de Banco de Dados

Backup (`Backup`) e restauração (`Restore`) do MariaDB. Sem tela própria na GUI — invocado diretamente pelas ações Backup/Restore de [Docker → Gerenciar Containers](./appstack.md#docker--gerenciar-containers) na linha do MariaDB, contra `appstack.MariaDBContainerName` e `cfg.Docker.MariaDB.DBUser`/`DBPass`. Caminhos de backup/restauração são validados e expandidos (`~`) antes do uso. O backup é um `mariadb-dump --all-databases` com `--single-transaction --skip-lock-tables` (um snapshot consistente sem travar tabelas) e `--routines --events` (rotinas e eventos incluídos). O dump é gravado num arquivo temporário ao lado do destino e só o substitui quando termina.

## `manager/repo` — Controle de Repositório Git

Configuração de identidade Git global (`ConfigureGlobal`), init/clone de repositório com identidade por repo (`Init`, `Clone`), sobrescrita de identidade local (`ApplyLocalIdentityAt`), e geração de arquivo de Código de Conduta (`CreateConduct`: `CODE_OF_CONDUCT.md` na raiz mais o espelho em português em `docs/pt-br/codigo-de-conduta.md`, a partir de templates embutidos do Contributor Covenant, com o e-mail de contato para denúncias que o usuário digita em **Repositórios → Arquivos** — pré-preenchido com o e-mail do Git da pasta; nenhum endereço fixo é gravado. Um `CODIGO_DE_CONDUTA.md` deixado na raiz por versões anteriores é avisado, não apagado). O `ConfigureGlobal` nunca sobrescreve um `credential.helper` já configurado; quando precisa escolher um e o `git-credential-libsecret` não é encontrado, cai para `cache` (senha mantida em memória por 15 minutos) e mostra um aviso apontando para **Pré-requisitos → Libsecret Tools**. O `Clone` clona direto na pasta escolhida, então recusa uma pasta que não esteja vazia (`IsEmptyDir` — arquivos ocultos contam) antes de rodar o `git`; a GUI verifica o mesmo antes (`RepoFolderState.IsEmpty`) e mostra um aviso no lugar do formulário.

Os cartões de pastas vêm de `repo_folders` no `config.yaml`: **Novo repositório** adiciona um (a pasta precisa vir do diálogo nativo de pastas), **Remover** esquece um (nada é apagado do disco), e entradas cuja pasta não existe mais são removidas sempre que a lista é lida.

## `manager/gitignore`

Gera um `.gitignore` sob medida para o projeto (`Generate`). O tipo de projeto é detectado pelos arquivos típicos na **raiz** da pasta (`DetectStacks`, um dicionário em `gitignore.go`), sem depender do `.instructions/`:

| Tipo | Marcador | Tipo | Marcador |
|---|---|---|---|
| Go | `go.mod` | Node.js | `package.json` |
| Flutter | `pubspec.yaml` com `sdk: flutter` | Python | `pyproject.toml`, `requirements.txt`, `setup.py` |
| Dart | `pubspec.yaml` sem Flutter | Ruby | `Gemfile` |
| Moodle | `version.php` com `$plugin->component` | Rust | `Cargo.toml` |
| PHP | `composer.json` | Java | `pom.xml`, `build.gradle`, `build.gradle.kts` |
| Shell | qualquer `*.sh` | | |

As seções de editores e de sistema operacional sempre entram; sem tipo reconhecido, entram as seções genéricas (ambiente, logs, build/dependências). Vários tipos podem valer ao mesmo tempo, e um padrão comum a duas seções é escrito uma vez só. Um `.gitignore` existente é **mesclado, nunca sobrescrito**: o conteúdo fica como está e só os padrões que faltam são acrescentados, sob o título da seção — rodar de novo não acrescenta nada.

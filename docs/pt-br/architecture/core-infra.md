🌐 [English](../../en/architecture/core-infra.md) | **Português** | 🏠 [Índice](../index.md)

---

# Infraestrutura Central

Pacotes transversais dos quais todas as outras camadas dependem.

## `internal/config`

`Load()`/`Update()` para o `~/.perci/config.yaml` — o `Update` (carrega, altera e salva sob uma única trava) é o único caminho de escrita; a gravação em si é atômica, via `internal/fsutil.WriteFileAtomic` (arquivo temporário no mesmo diretório, `chmod 0600`, `fsync`, depois `os.Rename`) e mantém o próprio `~/.perci` em `0700` — o arquivo guarda as credenciais do MariaDB. `Load` retorna um `Config` vazio (não um erro) quando o arquivo ainda não existe e nunca preenche campo nenhum: um `workspace_path` vazio significa "não escolhido", e `Config.Workspace()` resolve a pasta (`~/workspace` quando não definida) para quem precisa dela. `ExpandPath` resolve um `~`/`~/` à frente contra o diretório home do usuário — usado sempre que um valor de config é um caminho de sistema de arquivos. Veja a [Referência de Configuração](../reference/configuration.md) para todas as chaves.

## `internal/distro`

`Detect()` (família Debian/Fedora) e `DetectDE()` (ambiente de desktop) são a **única** forma sancionada de ramificar por distro em qualquer lugar do código — pacotes de domínio nunca implementam sua própria detecção. Arch não é uma família suportada. A detecção lê o `/etc/os-release` (inclusive valores entre aspas simples) e o `XDG_CURRENT_DESKTOP`.

Todo `apt-get` sem interação passa por `AptNonInteractiveOpts`/`AptEnv`/`AptArgs`: `--force-confold` mais `DEBIAN_FRONTEND=noninteractive`, para que um arquivo de configuração editado pelo usuário nunca trave o `dpkg` num prompt.

## `internal/executor`

O único ponto pelo qual todo comando privilegiado é escalado. O `sudo` é resolvido a partir do caminho fixo `/usr/bin/sudo` (não resolvido via `$PATH`), e o mesmo vale para todo programa executado por `pkexec`/`sudo`: o `trustedPath` converte o nome num caminho absoluto dentro dos diretórios do sistema (`/usr/local/sbin`, `/usr/local/bin`, `/usr/sbin`, `/usr/bin`, `/sbin`, `/bin`), nunca pelo `$PATH` do usuário. Um comando sem privilégio roda no seu próprio grupo de processos: quando o contexto é cancelado (um limite de tempo de rede), o grupo inteiro é encerrado, então os filhos de um pipeline do shell também param. Variáveis de ambiente são propagadas através dele via `sudo env KEY=VALUE ...` em vez de depender do próprio mecanismo de passthrough de env do `sudo` (inconsistente, dependente de distro).

`PrivilegedInstall`/`PrivilegedInstallSteps` instalam um arquivo que o usuário sem privilégio baixou (e verificou) num local do root sem confiar nele depois do pedido de senha: o root copia o arquivo para um temporário próprio com `0600` (`<destino>.perci-new`), verifica o SHA-256 de novo nessa cópia, aplica o modo final e renomeia sobre o destino como `root:root` — um lote, um prompt. O `PrivilegedInstallSteps` devolve três passos (`install`, um `sh` de verificação e um `sh` de finalização); o último remove o arquivo temporário quando o `chmod` ou o `mv` falha, então nenhum `.perci-new` sobra. Usado pela auto-atualização e pela instalação do appimagetool; o `install.sh` faz o mesmo em shell.

O `RunSudoSequence` recusa `Options.Env` (seria descartado em silêncio): variáveis por passo vão em `PrivilegedStep.Env`. Num lote, um passo `Soft` que falha mostra o aviso e o lote continua (sem mudança). Somar `SoftReport` mantém isso, mas faz o lote terminar com `ErrCompletedWithWarnings` quando algum desses passos falhou — usado pelos lotes de apps Flatpak e de fontes, que só têm passos soft e reportavam sucesso mesmo quando todos os itens falhavam.

`CommandAvailable`/`Which` resolvem um comando no `$PATH` no próprio processo (`exec.LookPath`), nunca pelo binário externo `which`; os testes trocam `Executor.LookPath` para simular o que está instalado. `RunConcurrent(ctx, items, limit, fn)` para de iniciar itens quando o `ctx` termina e converte um panic em `fn` num erro devolvido.

## `internal/fsutil`

`WriteFileAtomic(path, data, mode)`: arquivo temporário no mesmo diretório, `chmod` para `mode`, `fsync`, rename — o modo final sempre vale, inclusive sobre um arquivo existente (o `os.WriteFile` só o aplica na criação). Usado no `config.yaml`, no export da stack Docker, nos logs de contêiner exportados e nos `.gitignore` gerados.

`WriteGenerated(path, data, mode)` é o gravador dos arquivos gerados em projetos (arquivos de contexto de IA, `.claude/settings.json`, o arquivo de código de conduta do repositório, o wrapper do Flutter, entradas do menu de contexto do gerenciador de arquivos): grava através de um symlink em `path` (o destino do link é escrito e o link é mantido), de forma atômica, e quando já existe uma versão *diferente* a salva antes como `<path>.perci-bak`.

`WriteThroughLink(path, data, mode)` grava do mesmo jeito (através de symlink, de forma atômica) sem nenhuma cópia de backup — usado nos arquivos de configuração de MCP, que podem conter tokens.

`VerifyFile(path, hash, want)`: a verificação única de checksum dos artefatos baixados (autoatualização, Go, ferramentas Android do Flutter, appimagetool) — hex, sem diferenciar maiúsculas.

## `internal/selfupdate`

A auto-atualização (`Run`) verifica o GitHub Releases, baixa, verifica e substitui o binário em execução. Uma build de desenvolvimento (`version.Version == "dev"`) se recusa a se atualizar. A checagem de atualização (`LatestRelease`) só lista a release; o `checksums.txt` é baixado quando a atualização é aplicada, e uma release sem ele aparece, mas a atualização é recusada. Quando o diretório do binário é gravável pelo usuário, a troca é um rename atômico no mesmo diretório; senão, passa pelo `executor.PrivilegedInstall`. Todo redirecionamento de download de asset é validado de novo contra os hosts confiáveis, o download do binário é limitado pelo contexto de 5 minutos do `Run` (e não por um timeout por requisição) e o arquivo recebe `fsync` antes da troca. A auto-desinstalação (`Uninstall`) remove o binário, o alias `perci` (só quando é um symlink para o binário — um arquivo ou diretório não relacionado chamado `perci` é deixado em paz), a entrada no menu de aplicativos (`.desktop` + conjunto de ícones hicolor, num único `rm` privilegiado) e, opcionalmente, `~/.perci`, tudo num lote com um único pedido de senha. `InstallMenuIcons` reinstala o conjunto de ícones hicolor (embutido pelo pacote `packaging`) na cor escolhida em **Home → Configurações → Ícone do Aplicativo**, num único lote privilegiado — os PNGs chegam ao root como um arquivo tar em memória pelo stdin, nunca como arquivos numa pasta temporária gravável pelo usuário.

## `internal/shellrc`

Helpers idempotentes para editar arquivos rc de shell (`~/.bashrc`): `AppendIfMissing`, `RemoveEntry`, `Rewrite` (preserva as permissões do arquivo), `Dedup` (retorna um novo slice deduplicado — não muta a entrada in-place). Usado por todo pacote que precisa persistir um export de PATH (veja [Ambiente de Desenvolvimento](./dev-environment.md)). As edições seguem symlinks até o destino (um `~/.bashrc` gerenciado por stow/chezmoi continua sendo symlink), ler-alterar-gravar acontece sob uma trava por arquivo, as entradas são sempre de uma linha (blocos de várias linhas entram linha a linha) e um comentário vazio nunca casa com linhas em branco na remoção.

## `internal/version`

Guarda a string de versão, injetada em tempo de build via `-ldflags "-X .../internal/version.Version=vX.Y.Z"`. Sustenta o `prci version`/`--version`.

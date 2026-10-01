🌐 [English](../../en/troubleshooting/common-issues.md) | **Português** | 🏠 [Índice](../index.md)

---

# Problemas Comuns

## Instalação

**`install.sh` falha com "Required tool 'jq' is not installed."**
Instale `curl`, `jq` e `sha256sum` (geralmente parte do `coreutils`) primeiro — o script verifica os três antes de fazer qualquer coisa e se recusa a continuar sem eles, de propósito, já que a verificação de checksum depende deles.

**"Checksum mismatch" durante a instalação**
O script aborta em vez de instalar um binário que não bate com o `checksums.txt` publicado. Rode o instalador de novo — se a divergência persistir, pode indicar um download corrompido ou uma release em estado ruim; não contorne essa verificação manualmente.

**A janela não abre, com erros sobre bibliotecas `libgtk-4` / `libwebkitgtk-6.0` ausentes**
As bibliotecas de tempo de execução do GTK4/WebKitGTK 6.0 não estão instaladas. Instale-as como mostrado em [Instalação → Requisitos de Tempo de Execução](../getting-started/installation.md#requisitos-de-tempo-de-execução-linux).

**O Perci trava logo ao abrir com `bwrap: setting up uid map: Permission denied` (Ubuntu 23.10+, Zorin OS 18+)**
Não é bug do Perci: essas versões definem `kernel.apparmor_restrict_unprivileged_userns=1`, o que bloqueia o sandbox que o WebKitGTK inicia via `bwrap` — afeta outros apps GTK4/WebKit também. Libere user namespaces para o `bwrap` com um perfil AppArmor, como `/etc/apparmor.d/bwrap`:

```text
abi <abi/4.0>,
include <tunables/global>

profile bwrap /usr/bin/bwrap flags=(unconfined) {
  userns,
  include if exists <local/bwrap>
}
```

depois recarregue o AppArmor com `sudo systemctl reload apparmor` e abra o Perci de novo.

**Uma entrada de pós-instalação que eu esperava ver em Linux não aparece na lista**
Entradas de pós-instalação em **Linux** são restritas por distro + ambiente de desktop — só a entrada correspondente à sua distro/DE detectada aparece, as demais ficam de fora da lista por completo (veja [Instalação → Distribuições Suportadas](../getting-started/installation.md#distribuições-suportadas)). Confira `distro`/`de` em `~/.perci/config.yaml` — se a auto-detecção errou o seu setup, sobrescreva ali (veja a [Referência de Configuração](../reference/configuration.md)).

## Stack Docker de Aplicativos

**Um Container Aplicativo novo não fica acessível na sua URL `*.localhost`**
Confira se "Criar Container Nginx" já rodou (é o proxy reverso por onde todo app é roteado) e se o navegador confia no certificado wildcard do mkcert — o `mkcert -install` roda automaticamente na primeira vez que o container Nginx é criado, mas só registra a CA pro usuário que invocou. Confirme também se o container do app está de fato rodando (**Docker → Gerenciar Containers**).

**Criar o Container Nginx pela primeira vez trava ou falha silenciosamente**
Na primeira criação, `mkcert -install` pode escalar para root por conta própria (é o próprio binário `mkcert`, não o mecanismo de privilégio do Perci) para registrar a CA local — algo que a GUI não consegue responder, já que não tem um terminal interativo pra digitar a senha (a tela "Docker → Criar Container" já avisa sobre isso quando detecta que é a primeira vez). Se acontecer: abra um terminal fora do Perci e rode `mkcert -install` manualmente uma vez — depois disso o certificado já existe e "Criar Container Nginx" nunca mais precisa chamar essa parte do mkcert de novo.

**Editar um container causou uma indisponibilidade breve só daquele app**
Esperado — "Editar" é remove-e-recria com os novos parâmetros (a única forma real de trocar a imagem/versão de PHP/Node de um container em execução). Os volumes (`html`/`data`) são preservados; os outros containers não são afetados.

**Recriar o MariaDB mudou minha senha root do banco**
Não deveria — o gerador reaproveita `cfg.Docker.MariaDB.DBRootPass` da config existente em vez de gerar uma nova quando o container já está provisionado. Se a senha root real do MariaDB não bater, a config e o volume provavelmente saíram de sincronia manualmente; resolva na mão em vez de regenerar.

**Remover um app não liberou espaço em disco**
Por design — "Remover" só remove o container e sua entrada no `config.yaml`, nunca as pastas `html`/`data` em disco. Apague-as manualmente se tiver certeza de que não precisa mais delas.

**Meu "Comando do worker em background" (ou outro recurso adicionado à receita de build de uma imagem compartilhada) parece não fazer nada**
Uma imagem `perci-php<versão>`/`perci-php<versão>-node<versão>`/`perci-node<versão>` construída por um Perci mais antigo é reconstruída automaticamente na próxima vez que um app precisar dela (o label `perci.hash` não bate mais — veja [Stack Docker de Aplicativos](../architecture/appstack.md)), mas um container existente continua rodando na imagem com que foi criado. Depois dessa reconstrução o terminal lista os containers que ainda estão na imagem antiga: use **Recriar** em cada um deles em **Docker → Gerenciar Containers**.

**`composer-<pasta>`/`npm-<pasta>` falha com "Permission denied" ao gravar em `vendor/` ou `node_modules/`**
Wrappers criados por versões anteriores rodavam como root dentro dos containers da família PHP e combo, deixando essas pastas com dono root no host; os wrappers atuais rodam como `www-data` (o seu próprio UID). Devolva as pastas ao seu usuário uma vez, a partir da pasta do projeto: `sudo chown -R "$USER": vendor node_modules`.

## Contexto IA / Skills

**Clicar em "Aplicar" em IA: Contextos sobrescreveu meu `AGENTS.md` personalizado**
Isso é por design — **Aplicar** sempre regenera e sobrescreve `AGENTS.md`/`CLAUDE.md`/`.instructions/*.md` de todo modelo marcado, sem nenhuma caixa de confirmação; o próprio clique já é a confirmação, mesma convenção de toda ação não-interativa da GUI. Faça backup de qualquer edição manual nesses arquivos antes de clicar em Aplicar de novo, ou mantenha suas personalizações num arquivo que o gerador não toca.

**Placeholders do Moodle (`{{MOODLE_VERSION}}`, etc.) não foram preenchidos**
O gerador procura por `version.php` (ou `public/version.php`) na pasta que você escolheu em **IA: Contextos**. Se não encontrar, a GUI deixa os placeholders literais no lugar e registra um aviso em vez de perguntar os valores — preencha na mão, ou escolha a raiz real do projeto e clique em Aplicar de novo.

**Remover DaisyUI, "Dart: Oficial" ou "Flutter: Oficial" em IA: SKILLs removeu minhas outras skills também**
Corrigido: versões anteriores removiam essas três entradas (que instalam todas as skills do repositório) com `skills remove --skill '*'`, que a CLI `skills` trata como "todas as skills instaladas desses agentes" no escopo escolhido. Agora o Perci consulta, com `skills list --json`, quais skills instaladas vieram do repositório da entrada e remove só essas, pelo nome — e recusa a remoção se essa consulta falhar. Reinstale pela mesma tela as skills que você perdeu.

## Ambiente de Desenvolvimento

**Depois de instalar o Go, meu shell fish perdeu o `/usr/bin` (e a maioria dos comandos) do `PATH`**
Corrigido: versões anteriores gravavam `export PATH=$PATH:/usr/local/go/bin` no `~/.config/fish/config.fish`; no fish essa linha sem aspas deixa o `PATH` só com `/bin` e o `bin` do Go. A linha agora fica entre aspas (`export PATH="$PATH:/usr/local/go/bin"`, válida em bash, zsh e fish). Para consertar uma instalação existente, desmarque o Go em **Linguagens e SDKs** e execute (remove o Go e as duas formas da linha); depois marque de novo para reinstalar. À mão: apague a linha antiga do `config.fish` e abra um shell novo.

## Limitações Conhecidas e Aceitas (Não São Bugs)

- **Android cmdline-tools** (`internal/dev/flutter`) e **`appimagetool`** (`internal/dev/prereqs`) são baixados sem verificação de checksum. Nem o Google nem o projeto AppImage publicam um checksum oficial para esses artefatos específicos — isso é um risco documentado e aceito, não um descuido. Veja o [log de decisões interno](../architecture/decisions.md) para o raciocínio completo (e por que o `appimagetool` especificamente fica na tag de release `continuous` em vez de uma "stable").
- **Distros baseadas em Arch não são suportadas.** O `internal/distro` só detecta as famílias Debian e Fedora; não há plano de adicionar suporte a Arch a menos que isso mude.
- O perci em si não tem telemetria, processo em segundo plano nem daemon — se algo parece "travado", é um único comando síncrono; confira a saída do terminal diretamente em vez de procurar um serviço para reiniciar.

## Ainda com Problemas?

Confira a [Referência de Configuração](../reference/configuration.md) para o comportamento exato esperado da configuração ou tela que você está usando, ou abra uma issue com a versão exibida em **Home → Visão Geral** e os passos exatos que deram problema.

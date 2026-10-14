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
Entradas de pós-instalação em **Linux** são restritas por distro + ambiente de desktop — só a entrada correspondente à sua distro/DE detectada aparece, as demais ficam de fora da lista por completo (veja [Instalação → Distribuições Suportadas](../getting-started/installation.md#distribuições-suportadas)). O Perci detecta a distribuição pelo `/etc/os-release` e o desktop pelo `XDG_CURRENT_DESKTOP`; não existe configuração para sobrescrevê-los (as chaves `distro`/`de` citadas em docs antigas nunca são lidas). Se a entrada não aparece, confira esses dois valores num terminal (`cat /etc/os-release`, `echo $XDG_CURRENT_DESKTOP`) e compare com a lista de suportadas.

## Linux e Uso Geral

**A janela não fecha, ou "Sair" na bandeja não faz nada, e a aba Execução mostra um aviso**
Uma ação ainda está rodando. O Perci não encerra no meio de uma ação (uma execução interrompida do gerenciador de pacotes poderia deixar o sistema meio configurado); espere ela terminar na aba **Execução** e feche de novo.

**Uma ação terminou como "concluído com avisos" em vez de sucesso**
O trabalho principal terminou, mas uma ou mais etapas secundárias falharam — por exemplo Snap, Flatpak, limpeza do journal ou `autoremove` em **Atualizar Sistema**, ou um app de um lote de Flatpak ou de fontes. As linhas de aviso no terminal dizem o que falhou; rode a ação de novo depois de corrigir a causa.

**Itens de pós-instalação que apliquei antes aparecem disponíveis de novo**
O registro dos itens sem estado detectável está vinculado ao `/etc/machine-id` da máquina (`postinstall_machine_id`). Depois de reinstalar o SO mantendo o `$HOME` (ou na primeira vez que você roda uma versão que grava o ID), os registros antigos deixam de valer e esses itens são oferecidos mais uma vez. Aplicá-los de novo é seguro; os itens que podem ser detectados no sistema (pacotes, arquivo do sysctl, swap, `fstrim.timer`, Flathub) nunca foram afetados.

**A instalação de uma fonte falha com "nenhuma fonte .ttf encontrada" ou a remoção de uma fonte é recusada**
As fontes baixadas são instaladas em `~/.local/share/fonts/perci/<fonte>`; um arquivo sem nenhum `.ttf` falha em vez de instalar nada. Em sistemas da família Debian, uma fonte de pacote da qual outro pacote depende não é removida (o `dpkg --purge` recusa) — remova antes o pacote dependente.

## Stack Docker de Aplicativos

**Um Container Aplicativo novo não fica acessível na sua URL `*.localhost`**
Confira se "Criar Container Nginx" já rodou (é o proxy reverso por onde todo app é roteado) e se o navegador confia no certificado wildcard do mkcert — o `mkcert -install` roda automaticamente na primeira vez que o container Nginx é criado, mas só registra a CA pro usuário que invocou. Confirme também se o container do app está de fato rodando (**Docker → Gerenciar Containers**).

**O navegador diz que o certificado `*.localhost` não é confiável (`SEC_ERROR_UNKNOWN_ISSUER`)**
O navegador não tem a CA do mkcert. O `mkcert -install` cobre o repositório do sistema, o Chrome/Chromium e os perfis do Firefox em `~/.mozilla/firefox`; para navegadores instalados como Flatpak (Firefox, Zen, LibreWolf, Chrome…), o Perci a registra quando o container Nginx é criado ou recriado — feche o navegador e use **Recriar** na linha do Nginx em **Docker → Gerenciar Containers**, depois abra o navegador de novo. Para fazer à mão, feche o navegador e rode `certutil -A -d "sql:<pasta do perfil>" -t "C,," -n "mkcert development CA" -i "$(mkcert -CAROOT)/rootCA.pem"`, ou importe o `rootCA.pem` de `mkcert -CAROOT` nas configurações de certificados do navegador (Autoridades), confiando nele para identificar sites.

**O Firefox ou o Zen dizem `SSL_ERROR_BAD_CERT_DOMAIN` para um app `*.localhost`**
Os certificados gerados por versões anteriores só nomeavam `*.localhost`, um curinga que o Firefox e os derivados recusam. Use **Recriar** na linha do Nginx em **Docker → Gerenciar Containers**: o certificado é regerado com o nome de cada app.

**Criar o Container Nginx pela primeira vez trava ou falha silenciosamente**
Na primeira criação, `mkcert -install` pode escalar para root por conta própria (é o próprio binário `mkcert`, não o mecanismo de privilégio do Perci) para registrar a CA local — algo que a GUI não consegue responder, já que não tem um terminal interativo pra digitar a senha (a tela "Docker → Criar Container" já avisa sobre isso quando detecta que é a primeira vez). Se acontecer: abra um terminal fora do Perci e rode `mkcert -install` manualmente uma vez — depois disso o certificado já existe e "Criar Container Nginx" nunca mais precisa chamar essa parte do mkcert de novo.

**Um plugin Moodle montado fica em "Reinício pendente" ou as páginas do plugin falham**
Um contêiner em execução só enxerga uma montagem nova depois de reiniciar — use a ação de reiniciar da linha na aba **Plugins montados** de **Dev Tools → Plugins Moodle** (ela reinicia também o Nginx). "Montagem inativa" significa que a montagem no host sumiu (unidade removida ou com falha: `systemctl status <unidade>`); remova a linha e crie de novo. Não use um link simbólico para o plugin no lugar: dentro do contêiner o link aponta para fora do código montado, e o `__DIR__` do PHP o resolve, então o `config.php` não é encontrado.

**O Moodle mostra um plugin montado como "faltando no disco" e os arquivos foram para a `.debris`**
A pasta de desenvolvimento e a pasta `html` do Moodle são sincronizadas pelo MEGAsync, que identifica arquivos pelo inode e tomou a montagem por uma mudança entre sincronizações, mandando os arquivos para a pasta `.debris` da sincronização da pasta de desenvolvimento. Restaure-os pelo MEGA (Lixeira ou histórico de versões, que mantêm os caminhos originais), depois remova a linha na aba **Plugins montados** de **Dev Tools → Plugins Moodle** e crie de novo: o Perci agora exclui o destino no `.megaignore` mais próximo antes de montar (veja [Montagens de plugins Moodle](../architecture/appstack.md#montagens-de-plugins-moodle-moodleplugingo)). Montagens criadas por uma versão anterior não têm essa linha — remova e crie de novo.

**Um app não pode ser criado ou recriado: "o caminho … não pode ser montado no contêiner"**
As pastas do projeto são montadas com `--mount`, cujo valor não pode conter vírgula, aspas duplas nem quebra de linha. Renomeie a pasta (ou a pasta do workspace) sem esses caracteres.

**Um contêiner de app não inicia depois que a pasta `html` ou `logs` dele foi apagada**
Com `--mount`, o Docker não recria mais uma pasta de origem que sumiu, então iniciar (ou o reinício no boot) falha. Use **Recriar** em Docker → Gerenciar Containers: ele cria as pastas de novo.

**A imagem do PHP 7.4 / 8.0 constrói, mas me preocupo com atualizações**
Essas duas imagens são baseadas no Debian bullseye, que está fora de suporte, então seus pacotes vêm do `archive.debian.org` e elas não recebem mais atualizações de segurança. Use-as só para projetos legados; prefira o 8.1 ou mais novo quando o projeto permitir.

**O Xdebug não alcança minha IDE num app criado antes**
Os containers agora sobem com `--add-host host.docker.internal:host-gateway`; um container existente não o tem até você **Recriar** em **Docker → Gerenciar Containers**.

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

**Aplicar falha com "pasta não escolhida no seletor"**
As pastas passadas às telas de Contexto IA, Repositórios e MCP precisam ter sido escolhidas no diálogo nativo de pastas nesta sessão (ou já estar registradas). Use o botão do seletor de pastas (**Selecionar pasta**) em vez de digitar ou colar um caminho.

**Não existe mais `.claudeignore`, e apareceram arquivos `*.perci-bak`**
O Claude Code ignora o `.claudeignore`, então o Perci parou de gerá-lo: as regras de leitura equivalentes são mescladas no `.claude/settings.json` do projeto (`permissions.deny`), ao lado do `.aiexclude`. Se um `.claudeignore` antigo for encontrado, um aviso diz que ele pode ser apagado. Um `<arquivo>.perci-bak` é a versão anterior de um arquivo gerado que o Perci substituiu por conteúdo diferente — apague-o quando não precisar mais.

**Registrar um servidor MCP falha com "já existe um servidor com esse nome que não foi registrado pelo Perci"**
Um agente (Antigravity, Claude Code ou Codex) já tem um servidor com esse nome que o Perci não criou; o Perci nunca sobrescreve nem remove esses. Renomeie ou remova-o na configuração do próprio agente e registre de novo.

**Uma linha de servidor MCP ou SKILL diz "parcial: …"**
O registro chegou só aos agentes listados depois de "parcial"; os outros falharam ou não estavam disponíveis. Veja a saída do terminal da execução, corrija a causa (por exemplo um CLI ausente) e rode a ação dessa linha de novo.

**Placeholders do Moodle (`{{MOODLE_VERSION}}`, etc.) não foram preenchidos**
O gerador procura por `version.php` (ou `public/version.php`) na pasta que você escolheu em **IA: Contextos**. Se não encontrar, a GUI deixa os placeholders literais no lugar e registra um aviso em vez de perguntar os valores — preencha na mão, ou escolha a raiz real do projeto e clique em Aplicar de novo.

**Remover DaisyUI, "Dart: Oficial" ou "Flutter: Oficial" em IA: SKILLs removeu minhas outras skills também**
Corrigido: versões anteriores removiam essas três entradas (que instalam todas as skills do repositório) com `skills remove --skill '*'`, que a CLI `skills` trata como "todas as skills instaladas desses agentes" no escopo escolhido. Agora o Perci consulta, com `skills list --json`, quais skills instaladas vieram do repositório da entrada e remove só essas, pelo nome — e recusa a remoção se essa consulta falhar. Reinstale pela mesma tela as skills que você perdeu.

## Ambiente de Desenvolvimento

**Depois de instalar o Go, meu shell fish perdeu o `/usr/bin` (e a maioria dos comandos) do `PATH`**
Corrigido: versões anteriores gravavam `export PATH=$PATH:/usr/local/go/bin` no `~/.config/fish/config.fish`; no fish essa linha sem aspas deixa o `PATH` só com `/bin` e o `bin` do Go. A linha agora fica entre aspas (`export PATH="$PATH:/usr/local/go/bin"`, válida em bash, zsh e fish). Para consertar uma instalação existente, clique em **Desinstalar** no Go em **Linguagens e SDKs** (remove o Go e as duas formas da linha); depois em **Instalar** para reinstalar. À mão: apague a linha antiga do `config.fish` e abra um shell novo.

**Desinstalar um item é recusado com "… também removeria: …"**
Antes de remover um pacote de pré-requisito, terminal (Alacritty, GNOME Console) ou IDE (VS Code, VSCodium), o Perci simula a remoção e para quando o gerenciador de pacotes removeria também outros pacotes — nada é removido. Veja os pacotes listados; se tiver certeza, remova você mesmo pelo terminal.

**A instalação do Flutter menciona licenças do Android**
Instalar o Flutter aceita as licenças do Android SDK em seu nome (`sdkmanager --licenses`); os termos estão linkados na descrição do item. Não instale se não concordar com eles.

**Um download longo ou um passo do `npx` termina com erro de timeout**
Comandos de rede sem privilégio (nvm, `npx`, `git`, downloads com `curl`) são cortados depois de 30 minutos para que uma conexão travada não prenda para sempre a única ação em execução. Verifique a conexão e rode a ação de novo. Os passos de `apt`/`dnf` nunca são interrompidos assim. O clone do Flutter (o histórico completo é grande) não tem limite total: o git o interrompe quando a transferência fica abaixo de 1 KB/s por 2 minutos.

## Limitações Conhecidas e Aceitas (Não São Bugs)

- **Distros baseadas em Arch não são suportadas.** O `internal/distro` só detecta as famílias Debian e Fedora; não há plano de adicionar suporte a Arch a menos que isso mude.
- O perci em si não tem telemetria, processo em segundo plano nem daemon — se algo parece "travado", é um único comando síncrono; confira a saída do terminal diretamente em vez de procurar um serviço para reiniciar.

## Ainda com Problemas?

Confira a [Referência de Configuração](../reference/configuration.md) para o comportamento exato esperado da configuração ou tela que você está usando, ou abra uma issue com a versão exibida em **Home → Visão Geral** e os passos exatos que deram problema.

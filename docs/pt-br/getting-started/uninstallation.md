🌐 [English](../../en/getting-started/uninstallation.md) | **Português** | 🏠 [Índice](../index.md)

---

# Desinstalação

## Desinstalador Embutido (Recomendado)

Abra o Perci, vá em **Home → Visão Geral** e use a seção **Desinstalar Perci**. Quatro opções, todas opcionais, vêm antes de remover o binário em si:

| Opção | O que faz |
|---|---|
| Fazer backup das configurações | Exporta a definição da stack Docker (Nginx, credenciais do MariaDB, todos os apps) e as montagens de plugins Moodle para um arquivo YAML que você escolhe — o mesmo formato de **Docker → Exportar/Importar**, então pode ser importado numa instalação nova. Se o backup falhar, **nada** é removido e a desinstalação para; só "nada para exportar" (nenhum contêiner ou montagem de plugin registrado ainda) é um aviso. |
| Fazer backup dos repositórios | Exporta as contas Git e seus repositórios para outro arquivo YAML que você escolhe — o mesmo formato de **Repositórios → Gerenciar Repositórios**. É um arquivo próprio, separado do backup das configurações. Uma falha interrompe a desinstalação do mesmo jeito. |
| Remover contêineres Docker | Remove todos os contêineres registrados (primeiro os apps, depois o MariaDB, depois o Nginx). Os dados dos projetos em disco **não** são tocados. |
| Remover `~/.perci` | Apaga o diretório de configuração do Perci, inclusive os dumps automáticos do MariaDB em `~/.perci/appstack/backups` — copie-os para outro lugar antes se quiser mantê-los. Depois que o binário é removido, tira também a regra `includeIf` de cada conta Git do `~/.gitconfig` e o token de API dela do chaveiro do sistema. Sem esta opção, as contas continuam funcionando só com o `git`. |

Seja qual for a escolha, o desinstalador sempre remove:

- O binário (`/usr/local/bin/prci`) e o symlink do alias `perci` (só quando ele realmente é um symlink para o `prci`; um arquivo não relacionado chamado `perci` é deixado em paz). Tudo roda num único lote, com um só pedido de senha.
- A entrada no menu de aplicativos: `/usr/share/applications/perci.desktop`, o conjunto de ícones `/usr/share/icons/hicolor/<tamanho>/apps/perci.png` (512, 256, 128, 64, 48 e 32) e, de instalações antigas, `/usr/share/pixmaps/perci.png`.
- A entrada de autostart da bandeja, `~/.config/autostart/perci.desktop` (se a bandeja chegou a ser ligada).

A janela fecha sozinha quando a desinstalação termina.

## Desinstalação Manual

Se o Perci não abre mais, remova os mesmos arquivos à mão:

```bash
sudo rm -f /usr/local/bin/prci /usr/local/bin/perci
sudo rm -f /usr/share/applications/perci.desktop /usr/share/pixmaps/perci.png
sudo rm -f /usr/share/icons/hicolor/{512x512,256x256,128x128,64x64,48x48,32x32}/apps/perci.png
rm -f ~/.config/autostart/perci.desktop
rm -rf ~/.perci    # opcional: configurações
```

Apagar `~/.perci` à mão deixa as regras das contas Git no `~/.gitconfig` apontando para arquivos que não existem mais (o Git as ignora, então a identidade e a chave das contas deixam de valer). Liste e remova as regras, e os tokens:

```bash
git config --global --get-regexp '^includeif\.gitdir:.*\.path$'       # uma linha por conta
git config --global --unset-all 'includeIf.gitdir:<pasta>/.path'       # para cada <pasta> listada
secret-tool clear service perci-git account <id>                      # para cada conta com token
```

## O Que Fica no Sistema

O Perci nunca remove o que ele instalou ou criou **para você** — isso pertence ao seu sistema, não ao Perci. Depois de desinstalar, estes itens continuam lá até você removê-los:

| O que fica | Como remover |
|---|---|
| Contêineres Docker (se você não marcou a opção de remoção) | `docker rm -f <nome>` — veja [Ambiente da stack Docker](../guides/environments/docker.md) para os nomes dos contêineres. |
| Imagens base `perci-php<versão>` / `perci-node<versão>` | `docker image ls 'perci-*'` e depois `docker image rm <imagem>`. |
| A rede `docker-php-network` | `docker network rm docker-php-network` |
| Dados dos projetos no seu workspace (`workspace/localhost/{html,data,logs}`, dados do MariaDB) | Apague as pastas à mão, **só** se não precisar mais dos projetos ou bancos. |
| Montagens de plugins Moodle (**Dev Tools → Plugins Moodle**): unidades `/etc/systemd/system/*.mount` cujo `Description` começa com `perci:` | Remova cada plugin nessa tela **antes** de desinstalar o Perci. Senão: `grep -l "^Description=perci:" /etc/systemd/system/*.mount`, depois para cada unidade `sudo systemctl disable --now <unidade>` e `sudo rm /etc/systemd/system/<unidade>`, e por fim `sudo systemctl daemon-reload`. Desmonte antes de apagar qualquer pasta do Moodle. As linhas `-dpG:` que o Perci acrescentou a um `.megaignore` para essas montagens ficam até serem removidas à mão. |
| Wrappers por app em `~/.local/bin` (`php-<pasta>`, `composer-<pasta>`, `npm-<pasta>`, …) | Remover um app no Perci remove os wrappers dele. Para apps ainda registrados ao desinstalar o Perci (ou removidos por uma versão anterior): `ls ~/.local/bin/*-<pasta>` e remova-os. |
| Entradas do menu de contexto do gerenciador de arquivos ("Abrir no <terminal>"): `~/.local/share/nautilus/scripts/Abrir no *`, `~/.local/share/nemo/scripts/Abrir no *`, `~/.local/share/kio/servicemenus/perci-*.desktop` | Desmarcar o terminal em **Desenvolvimento → Aplicativos: Terminais** remove as entradas dele. Caso contrário, apague esses arquivos. |
| Linux Toys instalado por um Perci antigo (o Perci não o instala nem o remove mais) | `sudo apt-get purge linuxtoys` no Debian/Ubuntu/Mint/Zorin, `sudo dnf remove linuxtoys` no Fedora. |
| MEGAsync instalado por um Perci antigo (o Perci não instala nem remove mais), com o repositório e a chave de assinatura da MEGA | Debian/Ubuntu/Mint/Zorin: `sudo apt-get purge megasync` e depois `sudo rm /etc/apt/sources.list.d/mega.nz.list /etc/apt/keyrings/mega.nz.gpg`. Fedora: `sudo dnf remove megasync` e depois `sudo rm /etc/yum.repos.d/megasync.repo /etc/pki/rpm-gpg/RPM-GPG-KEY-megasync` (a chave da MEGA no banco do rpm aparece em `rpm -qa 'gpg-pubkey*'`). |
| Fontes baixadas pelo Perci (JetBrains Mono NF e similares): `~/.local/share/fonts/perci/<fonte>/` | Desinstalar a fonte em **Linux → Gerenciar Fontes** apaga a pasta dela; senão `rm -r ~/.local/share/fonts/perci` e `fc-cache -f`. |
| `~/.config/starship.toml.perci-bak`, o backup que **Remover Starship** faz da sua configuração do Starship, e as cópias `<arquivo>.perci-bak` ao lado de arquivos gerados em projetos (contexto de IA, `.claude/settings.json`, código de conduta, …) | Guardam a versão anterior de um arquivo que o Perci substituiu. Restaure ou apague como quiser — o Perci nunca as remove. |
| Chaves SSH geradas para as contas Git: `~/.ssh/perci_<id>` e `~/.ssh/perci_<id>.pub` | Elas podem continuar cadastradas no GitHub/GitLab: remova-as lá e depois apague os arquivos. |
| Repositórios nas pastas das contas Git | São arquivos seus: o Perci nunca os apaga. |
| Tudo o que foi instalado pelas telas do Perci (pacotes, fontes, apps Flatpak, SDKs, IDEs, arquivos de contexto de IA, …) | Desinstale cada um pelo seu próprio gerenciador de pacotes ou ferramenta — ou, para a maioria, pela tela correspondente do Perci **antes** de desinstalar o Perci. |

🌐 [English](../../en/getting-started/uninstallation.md) | **Português** | 🏠 [Índice](../index.md)

---

# Desinstalação

## Desinstalador Embutido (Recomendado)

Abra o Perci, vá em **Home → Visão Geral** e use a seção **Desinstalar Perci**. Um diálogo de confirmação oferece três etapas opcionais antes de remover o binário em si:

| Opção | O que faz |
|---|---|
| Fazer backup das configurações | Exporta a definição da stack Docker (Nginx, credenciais do MariaDB, todos os apps) para um arquivo YAML que você escolhe — o mesmo formato de **Docker → Exportar configurações**, então pode ser importado numa instalação nova. |
| Remover contêineres Docker | Remove todos os contêineres registrados (primeiro os apps, depois o MariaDB, depois o Nginx). Os dados dos projetos em disco **não** são tocados. |
| Remover `~/.perci` | Apaga o diretório de configuração do Perci. |

Seja qual for a escolha, o desinstalador sempre remove:

- O binário (`/usr/local/bin/prci`) e o symlink do alias `perci`.
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

## O Que Fica no Sistema

O Perci nunca remove o que ele instalou ou criou **para você** — isso pertence ao seu sistema, não ao Perci. Depois de desinstalar, estes itens continuam lá até você removê-los:

| O que fica | Como remover |
|---|---|
| Contêineres Docker (se você não marcou a opção de remoção) | `docker rm -f <nome>` — veja [Ambiente da stack Docker](../guides/environments/docker.md) para os nomes dos contêineres. |
| Imagens base `perci-php<versão>` / `perci-node<versão>` | `docker image ls 'perci-*'` e depois `docker image rm <imagem>`. |
| A rede `docker-php-network` | `docker network rm docker-php-network` |
| Dados dos projetos no seu workspace (`workspace/localhost/{html,data,logs}`, dados do MariaDB) | Apague as pastas à mão, **só** se não precisar mais dos projetos ou bancos. |
| Wrappers por app em `~/.local/bin` (`php-<pasta>`, `composer-<pasta>`, `npm-<pasta>`, …) | Remover um app no Perci remove os wrappers dele. Para apps ainda registrados ao desinstalar o Perci (ou removidos por uma versão anterior): `ls ~/.local/bin/*-<pasta>` e remova-os. |
| Entradas do menu de contexto do gerenciador de arquivos ("Abrir no <terminal>"): `~/.local/share/nautilus/scripts/Abrir no *`, `~/.local/share/nemo/scripts/Abrir no *`, `~/.local/share/kio/servicemenus/perci-*.desktop` | Desmarcar o terminal em **Desenvolvimento → Aplicativos: Terminais** remove as entradas dele. Caso contrário, apague esses arquivos. |
| Tudo o que foi instalado pelas telas do Perci (pacotes, fontes, apps Flatpak, SDKs, IDEs, arquivos de contexto de IA, …) | Desinstale cada um pelo seu próprio gerenciador de pacotes ou ferramenta — ou, para a maioria, pela tela correspondente do Perci **antes** de desinstalar o Perci. |

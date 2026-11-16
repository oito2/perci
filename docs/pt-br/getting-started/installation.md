🌐 [English](../../en/getting-started/installation.md) | **Português** | 🏠 [Índice](../index.md)

---

# Instalação

> [!IMPORTANT]
> O Perci foi descontinuado e substituído pelo **Perci82 System** e pelo **Perci82 Dev** — instale esses no lugar: veja [Migrar para o Perci82](../leiame.md#migrar-para-o-perci82). A versão 1.10.0 é a última release do Perci; esta página fica como registro de como o Perci era instalado.

O perci.gnl é distribuído como um único binário Go — uma aplicação gráfica de desktop (feita com [Wails v3](https://v3.wails.io)), só linux/amd64.

## Plataformas Suportadas

### Linux — suportado

O perci.gnl visa `linux/amd64`, construído sobre a pilha GTK4 + WebKitGTK 6.0 do Wails v3. As seções abaixo cobrem instalação, pré-requisitos de execução/compilação, e exatamente quais distribuições recebem a experiência completa (veja "Distribuições Suportadas").

### Windows — não suportado

Não existe build pra Windows, e nenhuma está planejada. Isso não é uma lacuna de empacotamento — a arquitetura do app é específica de Linux de ponta a ponta: a escalada de privilégio passa por `pkexec`/PolicyKit, o gerenciamento de pacotes do sistema é fixo em `apt`/`dnf`, a detecção de distro/ambiente de desktop lê `/etc/os-release`, e a própria GUI é construída sobre GTK4 + WebKitGTK 6.0. As releases só publicam o binário `prci-linux-amd64`.

### macOS — não suportado

Mesmo motivo do Windows: as camadas de escalada de privilégio, gerenciamento de pacotes e detecção de distro são todas exclusivas de Linux. Não existe build pra macOS.

## Requisitos de Tempo de Execução (Linux)

Diferente de um binário de CLI estático, a GUI precisa de GTK4 e WebKitGTK 6.0 instalados pra rodar de verdade. São bibliotecas de tempo de execução, não os cabeçalhos `-dev`/`-devel` necessários pra compilar o Perci — veja "Compilação a Partir da Fonte" abaixo para esses.

**Baseado em Debian/Ubuntu** (Linux Mint, Ubuntu, Zorin OS) — normalmente já presentes numa instalação desktop; se o app não abrir, instale-os explicitamente:

```bash
sudo apt install -y libgtk-4-1 libwebkitgtk-6.0-4
```

**Fedora** — normalmente já presentes no Fedora Workstation (GNOME); se necessário:

```bash
sudo dnf install -y gtk4 webkitgtk6.0
```

## Instalador Automático (Aposentado)

O `install.sh` não instala mais o Perci: ele mostra os comandos de instalação do Perci82 System e do Perci82 Dev e sai com status 1. Para instalar uma release do Perci mesmo assim, siga "Instalação Manual" abaixo.

## Instalação Manual

1. Baixe `prci-linux-amd64` e `checksums.txt` na [página de Releases](https://github.com/oito2/perci/releases).
2. Verifique o checksum, instale o binário no seu `$PATH` com o root como dono e (opcionalmente) crie o alias `perci`:

```bash
sha256sum --ignore-missing -c checksums.txt
sudo install -m 755 -o root -g root prci-linux-amd64 /usr/local/bin/prci
sudo ln -sf /usr/local/bin/prci /usr/local/bin/perci
```

Para conferir também a procedência dos arquivos, toda release traz uma attestation de build gerada pelo workflow de release — verifique com o [GitHub CLI](https://cli.github.com/):

```bash
gh attestation verify prci-linux-amd64 --repo oito2/perci
```

Esse caminho manual não cria entrada no menu de aplicativos — o Perci só ficará alcançável via `prci`/`perci` no terminal, a menos que você crie uma você mesmo, usando [`packaging/perci.desktop`](https://github.com/oito2/perci/blob/main/packaging/perci.desktop) como modelo e os ícones de [`packaging/icons/`](https://github.com/oito2/perci/tree/main/packaging/icons) (copie a árvore `hicolor/` de uma das cores para `/usr/share/icons/hicolor/`).

## Compilação a Partir da Fonte

Requer Go 1.26 ou superior, mais as dependências de build do Wails/WebKitGTK:

```bash
# Baseado em Debian/Ubuntu
sudo apt install -y libgtk-4-dev libwebkitgtk-6.0-dev

# Fedora
sudo dnf install -y gtk4-devel webkitgtk6.0-devel
```

```bash
git clone https://github.com/oito2/perci.git
cd perci
go build -tags production -trimpath -ldflags "-X github.com/oito2/perci/internal/version.Version=v1.0.0" -o prci ./cmd/prci-gui
sudo install -m 755 -o root -g root prci /usr/local/bin/prci
```

GTK4 + WebKitGTK 6.0 é a pilha padrão do Wails v3, então a única tag é
`production`, que desliga o modo debug do Wails (as ferramentas de
desenvolvedor do webview); o `make build` e o `make release` também a usam.
A pilha antiga GTK3/WebKit2GTK 4.1 só existe como caminho legado opcional
via `-tags gtk3`, que este projeto não usa.

Ou, usando o `Makefile` disponível:

```bash
make build              # compila ./prci com VERSION=dev
make build VERSION=v1.0.0
make install            # compila e instala em /usr/local/bin com o alias perci; os tamanhos de ícone vêm da árvore de empacotamento
```

## Artefatos de Release

Cada release no GitHub publica exatamente três arquivos, gerados por `make release` e publicados pelo `.github/workflows/release.yml` a cada tag que aponte para um commit da `main`:

| Artefato | Descrição |
|---|---|
| `prci-linux-amd64` | O binário da GUI em si — nenhum pacote instalador (`.deb`, `.rpm`, AppImage, Flatpak) é gerado. |
| `perci-menu.tar.gz` | A entrada no menu de aplicativos que o instalador do Perci 1.9.0 e anteriores instalava: `share/applications/perci.desktop` + `share/icons/hicolor/*/apps/perci.png` (azul), extraídos em `/usr`. Gerado de forma reproduzível. |
| `checksums.txt` | SHA-256 dos dois arquivos acima, verificado pelo instalador do Perci 1.9.0 e anteriores e (para o binário) por **Home → Atualizar Perci** antes da 1.10.0. Uma release sem `checksums.txt` aparece na lista, mas a atualização é recusada. |

Os três têm attestation de procedência assinada (`gh attestation verify <arquivo> --repo oito2/perci`). O workflow de release roda antes o workflow de CI (lint, testes, ShellCheck do `install.sh`), depois compila com um token só de leitura; só o job separado de publicação consegue escrever na release.

Para gerá-los localmente:

```bash
make release VERSION=v1.0.0   # gera dist/prci-linux-amd64, dist/perci-menu.tar.gz e dist/checksums.txt
```

## Arquiteturas Suportadas

Só linux/amd64 é publicado. A GUI usa cgo (bindings GTK4/WebKitGTK 6.0), então não faz cross-compile pra arm64 do jeito simples que um binário Go estático faria — precisaria de um toolchain cross pra arm64 e das versões arm64 dos pacotes -dev de GTK4/WebKitGTK, que o pipeline de release ainda não provisiona. Compilar a partir da fonte numa máquina arm64 de verdade deve funcionar normalmente, seguindo os passos acima.

## Distribuições Suportadas

Os recursos de pós-instalação e sensíveis a ambiente de desktop do perci.gnl são restritos às distros que o autor realmente usa no dia a dia. Outras distros da família Debian/Fedora devem funcionar normalmente para os recursos agnósticos de distro (SDKs de dev, stack Docker, contexto IA, etc.), mas os itens de menu que exigem uma combinação específica de distro/DE ficam ocultos da categoria "Linux" fora destas:

| Distribuição | Ambiente de Desktop |
|---|---|
| Linux Mint 22.3 | Cinnamon, XFCE |
| Ubuntu 24.04, 26.04 | GNOME (padrão) |
| Ubuntu Budgie 24.04, 26.04 | Budgie |
| Fedora 44 | GNOME (padrão), KDE Plasma, Budgie |
| Zorin OS 18.1 | Core (GNOME), Lite (XFCE) |
| Pop!_OS 24.04 | COSMIC |

Distros da família Arch não são suportadas — o `internal/distro` só detecta as famílias Debian e Fedora.

## Desinstalando

Veja [Desinstalação](./uninstallation.md) — o desinstalador embutido e tudo o mais que o Perci pode ter deixado no sistema.

## Próximos Passos

Continue para o [Início Rápido](./quickstart.md) para a sua primeira execução.

🌐 [English](../../en/getting-started/quickstart.md) | **Português** | 🏠 [Índice](../index.md)

---

# Início Rápido

## Abrindo o Perci

Instale o perci.gnl primeiro — veja [Instalação](./installation.md) se ainda não instalou. Depois, a partir de um terminal (ou de um lançador de aplicativos, uma vez que você o tenha fixado):

```bash
prci
```

Isso abre a janela do Perci. Não tem nada pra digitar depois disso — toda ação vive na interface gráfica.

## O Que Você Vai Ver

Uma barra lateral à esquerda com cinco categorias, e um painel principal à direita mostrando o item selecionado:

| Categoria | Cobre |
|---|---|
| **Home** | Dashboard ("Visão Geral"), configurações do app ("Configurações"). |
| **Linux** | Atualizações de sistema, pós-instalação sensível a distro/DE, fontes, modelos de arquivos, aplicativos Flatpak/Linux Toys/MegaSync. |
| **Desenvolvimento** | Pré-requisitos de build, SDKs de linguagens, aplicativos de IA/terminais, IDEs (Zed, VS Code, VSCodium, Android Studio, Antigravity). |
| **Docker** | Criar e gerenciar a stack Docker de aplicativos por projeto. |
| **Dev Tools** | Repositórios Git, geração de contexto de IA, skills de agente de IA, registro de servidores MCP. |

A janela abre em **Home :: Visão Geral** — um dashboard que resume o estado do seu sistema (distro/DE, ferramentas de dev instaladas, containers Docker, etc.), além de atalhos pra atualizar ou desinstalar o próprio Perci. Veja [Como o perci.gnl funciona](../concepts/how-perci-works.md) para o que tem dentro de cada categoria.

## Configuração na Primeira Execução

O Perci cria o `~/.perci/config.yaml` na primeira vez que você altera uma configuração pela GUI — ele nunca preenche valores padrão por conta própria. Enquanto você não escolhe um workspace, a tela Configurações avisa que ele ainda não foi escolhido e tudo o que precisa de uma pasta usa `~/workspace`. Você não precisa mexer no arquivo manualmente: abra **Home :: Configurações** pra escolher a pasta do workspace, mudar o escopo do Flatpak, o logo da sidebar, o ícone do aplicativo ou o tema, tudo pela interface gráfica. Veja a [Referência de Configuração](../reference/configuration.md) para todas as chaves, se preferir editar o arquivo diretamente.

## Uma Primeira Sessão Típica

1. **Home :: Visão Geral** — confira o estado atual do seu sistema antes de mudar qualquer coisa.
2. **Linux :: Atualizar Sistema** — rode uma primeira passada de atualização `apt`/`dnf` + Flatpak + Snap.
3. **Linux :: Pós-instalação** — um checklist restrito à sua distro/DE detectada.
4. **Desenvolvimento :: Pré-requisitos** — instale o toolchain de compilação, Git, curl, Docker Engine, mkcert e NPM.
5. **Docker :: Criar Container** — crie os containers Nginx e MariaDB, depois seu primeiro container de aplicativo (veja o [guia da stack Docker de aplicativos](../guides/environments/docker.md) para o ciclo de vida completo).
6. **Dev Tools :: IA: Contextos** — já dentro de um projeto real, gere `AGENTS.md`/`CLAUDE.md`/`.instructions/*.md`.

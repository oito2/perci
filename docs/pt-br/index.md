# Documentação — perci.gnl

🌐 [English](../en/index.md) | **Português** | 🏠 [Voltar ao LEIAME](./leiame.md)

---

perci.gnl (`prci`) é uma interface gráfica de desktop pessoal (Go + [Wails](https://wails.io)) que transforma uma instalação Linux nova em uma estação de desenvolvimento totalmente configurada em minutos. Ele reúne pós-instalação de SO, manutenção de sistema, setup de SDKs/IDEs/ferramentas de dev, e um gerenciador de stack Docker por projeto (PHP, Node, ou os dois juntos, com suporte de primeira classe pro Moodle), tudo atrás de um único binário, um único arquivo de configuração e cinco categorias na sidebar: Home, Linux, Desenvolvimento, Docker e Dev Tools.

---

## 🚀 Primeiros Passos

- [Instalação](./getting-started/installation.md) — instalador automático, instalação manual do binário ou compilação a partir da fonte.
- [Desinstalação](./getting-started/uninstallation.md) — o desinstalador embutido, a remoção manual e o que fica no sistema.
- [Início Rápido](./getting-started/quickstart.md) — primeira execução: a interface gráfica, o arquivo de configuração e o caminho mais rápido até um setup funcionando.

---

## 🧠 Conceitos

- [Por que o perci.gnl existe](./concepts/why-perci.md) — o problema que ele resolve e para quem ele realmente serve.
- [Como o perci.gnl funciona](./concepts/how-perci-works.md) — as cinco categorias da sidebar e como a interface gráfica aciona o mesmo código de domínio.
- [Visão geral da arquitetura](./concepts/architecture.md) — mapa de componentes de alto nível e fluxo de dados, para quem quer entender o quadro geral.
- [Glossário](./concepts/glossary.md) — termos específicos do projeto (stack, workspace, família de distro, skills…).

---

## 🏗️ Arquitetura (para contribuintes)

- [Gerenciamento de Sistema](./architecture/system-management.md) — `internal/system/*` (pós-instalação, fontes, templates, update, apps, linuxtoys, megasync).
- [Ambiente de Desenvolvimento](./architecture/dev-environment.md) — `internal/dev/*` (SDKs, IDEs, CLIs de LLM/MCP, terminais).
- [Stack Docker de Aplicativos](./architecture/appstack.md) — `internal/appstack` (ciclo de vida de containers Nginx/PHP/Node/MariaDB por projeto, roteamento Moodle).
- [Backend do Dev Tools](./architecture/managers.md) — `internal/manager/*` (contexto IA, skills IA, banco de dados, repositório, gitignore).
- [Infraestrutura Central](./architecture/core-infra.md) — `internal/config`, `internal/distro`, `internal/executor`, `internal/selfupdate`, `internal/shellrc`.
- [Log de decisões internas](./architecture/decisions.md) — riscos aceitos e trade-offs técnicos que não são óbvios só pela leitura do código.

---

## 📘 Guias

- [Fluxo de trabalho ponta a ponta](./guides/workflows/examples.md) — de uma máquina nova até um ambiente Moodle de dev funcionando.
- [Ambiente da stack Docker de aplicativos](./guides/environments/docker.md) — operação do dia a dia da stack Nginx/PHP/Node/MariaDB por projeto.

---

## 📖 Referência

- [Referência de Configuração](./reference/configuration.md) — toda chave do `~/.perci/config.yaml`.

---

## 🛠️ Solução de Problemas

- [Problemas comuns](./troubleshooting/common-issues.md) — limitações conhecidas, pegadinhas e como contorná-las.

---

## Em outros lugares

- [Guia de Contribuição](./contribuindo.md) — ambiente de desenvolvimento, convenções de código, como enviar alterações.
- [Código de Conduta](./codigo-de-conduta.md)
- [Changelog](../../CHANGELOG.md)

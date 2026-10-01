🌐 [English](../../en/concepts/why-perci.md) | **Português** | 🏠 [Índice](../index.md)

---

# Por Que o perci.gnl Existe

Este é um projeto bem pessoal. É o resultado de vários scripts `.sh` que o autor já tinha por aí, reunidos num único binário Go.

O objetivo é simples: chegar numa máquina nova e ter tudo o necessário configurado em minutos. Por isso ele só cobre as distros realmente usadas no dia a dia — Linux Mint 22.3 (Cinnamon e XFCE), Ubuntu 26.04, Fedora 44 e Zorin OS 18.1 (Core e Lite) — em vez de tentar ser uma ferramenta de setup genérica e universal.

## O Problema

Configurar uma estação de desenvolvimento Linux do zero significa repetir a mesma sequência sempre:

- Limpeza pós-instalação e instalação de codecs/essenciais, que muda por distro e ambiente de desktop.
- Instalar o mesmo conjunto de fontes, apps Flatpak e ferramentas de terminal.
- Instalar e manter atualizados alguns SDKs (Go, Flutter), IDEs (VS Code, VSCodium, Zed) e CLIs de LLM/MCP.
- Subir uma stack Docker Nginx + PHP + MariaDB por projeto, especialmente para desenvolvimento Moodle — múltiplas versões de PHP, múltiplas instalações Moodle roteadas pela mesma stack.
- Gerar arquivos de contexto consistentes para agentes de IA (`AGENTS.md`, `CLAUDE.md`, `.instructions/*.md`) e arquivos de skill para cada projeto novo.

Cada um desses itens costumava ser um script shell descartável, rodado manualmente, atualizado de forma inconsistente, sem UI compartilhada nem rede de segurança.

## A Abordagem

O perci.gnl consolida tudo isso em um único binário com:

- **Um único arquivo de config** (`~/.perci/config.yaml`) em vez de variáveis de ambiente espalhadas ou caminhos fixos em uma dúzia de scripts.
- **Um único executor** para toda operação privilegiada (`internal/executor`), então o uso de `sudo` fica centralizado, auditável e consistente.
- **Uma única GUI** (aplicativo desktop baseado em Wails, `cmd/prci-gui`, cinco categorias: Home, Linux, Desenvolvimento, Docker, Dev Tools), então todo fluxo — formulários, checklists, assistentes de múltiplas etapas — se comporta da mesma forma.
- **Nenhum prompt interativo dentro do código de domínio** — cada pacote de domínio recebe parâmetros simples e é chamado diretamente por um método Go ligado ao frontend da GUI, então existe exatamente um caminho do clique até o trabalho de fato, não uma TUI e um caminho scriptável separado.

## Para Quem É

Primariamente: o autor, nas máquinas do autor, para a stack real do dia a dia do autor (desenvolvimento Moodle em Linux da família Debian/Fedora). É compartilhado publicamente e livre para usar e adaptar, mas deliberadamente não tenta cobrir toda distro, todo ambiente de desktop ou toda stack de dev possível — veja [Como o perci.gnl funciona](./how-perci-works.md) para o formato que ele de fato cobre, e [Gerenciamento do sistema](../architecture/system-management.md) para exatamente quais distros/DEs são suportados.

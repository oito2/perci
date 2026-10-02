🌐 [English](../../../en/guides/workflows/examples.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Fluxo de Trabalho Ponta a Ponta: Máquina Nova → Ambiente Moodle de Dev Funcionando

Um passo a passo completo combinando todas as categorias, do início ao fim.

## 1. Instalar o perci

```bash
curl -fsSL https://raw.githubusercontent.com/oito2/perci/main/install.sh | bash
```

Veja [Instalação](../../getting-started/installation.md) para alternativas.

## 2. Configurar

```bash
prci
```

**Home → Configurações** — defina o caminho do workspace, o tema, a logo da sidebar/ícone do app e o escopo do Flatpak. Isso grava o `~/.perci/config.yaml`.

## 3. Pós-Instalação

**Linux → Atualizar Sistema**, depois a entrada de pós-instalação correspondente à sua distro/DE (só uma fica habilitada por vez — veja [Instalação → Distribuições Suportadas](../../getting-started/installation.md#distribuições-suportadas)).

Opcionalmente: **Gerenciar Fontes**, **Gerenciar Templates de Arquivos**, **Aplicativos: Flatpak**.

## 4. Ambiente de Dev

**Desenvolvimento →**

1. **Pré-requisitos** — toolchain de compilação, Git, curl, Docker Engine, mkcert, NPM.
2. **Linguagens e SDKs** — Go e/ou Flutter, se suas ferramentas/plugins Moodle precisarem.
3. **IDE: Zed Editor / VS Code / VSCodium / Android Studio / Antigravity IDE** — o que você usa no dia a dia (cada IDE tem sua própria tela de instalar/atualizar).

## 5. Stack Docker de Aplicativos

**Docker → Criar Container**, escolhendo um "Tipo de contêiner" a cada vez:

1. **Nginx** — configuração única, cria o proxy reverso, a rede compartilhada e o certificado HTTPS wildcard `*.localhost`.
2. **MariaDB** — configuração única, pede um usuário/senha de banco.
3. **Moodle** — Versão do Moodle `5.1+`, PHP 8.3 ou 8.4, URL (ex.: `mdle.localhost`), Acesso a banco = sim.

O container do app sobe automaticamente como parte da criação — sem passo separado de "iniciar". Veja o [guia da stack Docker de aplicativos](../environments/docker.md) para o ciclo de vida completo.

## 6. Clonar uma Instalação Moodle Pra Dentro Dele

**Dev Tools → Repositórios**, aba "Repositórios" → **Clonar**, com:

- URL: `https://github.com/sua-org/seu-site-moodle.git`
- Pasta: `~/workspace/localhost/html/mdle`

Clone direto na própria pasta `html` do app (`{{workspace_path}}/localhost/html/<pasta>`, batendo com a `pasta` escolhida na criação do container) — o container já estava roteado pra ela, sem precisar rodar/reiniciar nada de novo. Um segundo site Moodle é um segundo Container Aplicativo independente (sua própria pasta, URL e container), não uma entrada nova numa stack compartilhada.

## 7. Manutenção de Banco de Dados

**Docker → Gerenciar Containers**, selecione a linha do MariaDB, depois os ícones **Backup**/**Restore**.

As duas usam as credenciais de `cfg.Docker.MariaDB.*` contra o container MariaDB compartilhado — só na GUI, não existe equivalente de linha de comando.

## 8. Contexto IA para o Novo Projeto

**Dev Tools → IA: Contextos**, escolha a pasta do projeto clonado, marque **Moodle** e clique em **Aplicar**.

Isso gera `AGENTS.md`/`CLAUDE.md`/`.instructions/MOODLE.md` (com `{{MOODLE_VERSION}}` etc. preenchidos automaticamente a partir do `version.php`). (O modelo `php` foi removido em 2026-09-11 — orientação genérica de PHP agora é coberta por skills de terceiros, ex. "PHP: Specialist"/"PHP: Pro 8.3+" via `Dev Tools :: IA: SKILLs` na GUI, que também substituiu o antigo fluxo de aplicar skills — "Claude Code Mode" e o resto do catálogo próprio de `internal/manager/skills` foram removidos no mesmo dia.)

## 9. Manter Tudo Atualizado

**Linux → Atualizar Sistema** atualiza o SO + Flatpak + Snap. Para atualizar o próprio perci, use a ação **Atualizar** em **Home → Visão Geral**.

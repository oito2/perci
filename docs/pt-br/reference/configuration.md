🌐 [English](../../en/reference/configuration.md) | **Português** | 🏠 [Índice](../index.md)

---

# Referência de Configuração

As configurações vivem em `~/.perci/config.yaml`, criado na primeira vez que uma configuração é salva; toda gravação passa por `config.Update` (atômica, permissões `0600`). Uma chave ausente fica com o valor vazio — nada é preenchido na leitura.

```yaml
workspace_path: ~/workspace
flatpak_scope: system
docker:
  nginx_created: true
  mariadb:
    db_user: dev_user
    db_pass: aB3xQ9zK2mP7wR1t
    db_root_pass: zK2mP7wR1taB3xQ9
  apps:
    - name: Meu Moodle
      folder: mdle
      type: moodle
      url: mdle.localhost
      php_version: "8.4"
      moodle_version: "5.1+"
      db_access: true
    - name: Minha API + SPA
      folder: myapp
      type: php_node
      url: myapp.localhost
      php_version: "8.3"
      db_access: true
      node_version: "24"
      dev_command: npm run dev
      dev_port: 5173
```

## Chaves de Nível Superior

| Chave | Tipo | Definida via | Descrição |
|---|---|---|---|
| `workspace_path` | string | Home → Configurações | Pasta raiz dos projetos. Aceita `~`. Pela GUI, só é definido pelo seletor de pastas (o "Criar Workspace" recusa um caminho que não foi escolhido nele). Vazio significa ainda não escolhido: a tela Configurações avisa, o **Importar configurações** adota o workspace do arquivo exportado, e tudo o que precisa de uma pasta usa `~/workspace`. |
| `flatpak_scope` | string | Home → Configurações | `system` ou `user` — determina a flag `--system`/`--user` passada pra toda invocação `flatpak`. |
| `docker` | objeto | Docker → Criar Container / Gerenciar Containers | A definição inteira da stack Docker de aplicativos — veja abaixo. Omitido do arquivo por completo enquanto vazio. |

A distribuição e o ambiente de desktop são sempre detectados (`/etc/os-release`, `XDG_CURRENT_DESKTOP`); as chaves `distro`/`de` que versões anteriores documentavam nunca eram lidas e são ignoradas.

## `docker.*` (Stack Docker de Aplicativos)

Veja a [arquitetura da stack Docker de aplicativos](../architecture/appstack.md) pro modelo completo que isso sustenta.

| Chave | Tipo | Descrição |
|---|---|---|
| `docker.nginx_created` | bool | Se o container `nginx` compartilhado (proxy reverso de todo app) já foi criado. |
| `docker.mariadb.db_user` | string | O usuário de aplicação compartilhado do MariaDB que todo app com `db_access: true` usa pra conectar. |
| `docker.mariadb.db_pass` | string | Senha desse usuário — gerada aleatoriamente (`GenPassword()`), distinta de `db_root_pass`. |
| `docker.mariadb.db_root_pass` | string | Senha root do MariaDB — gerada aleatoriamente, distinta de `db_pass`. Reaproveitada (não regenerada) em recriações seguintes contra um volume já provisionado. |
| `docker.mariadb.data_user` / `data_pass` | string | As credenciais com que o diretório de dados do MariaDB foi inicializado. Mantidas (junto com `db_root_pass`) quando o contêiner sai da stack, para recusar recriá-lo sobre os mesmos dados com outras credenciais — a imagem ignora credenciais novas em dados existentes. |
| `docker.apps` | lista | Uma entrada por Container Aplicativo — veja abaixo. |

### `docker.apps[]`

| Chave | Tipo | Descrição |
|---|---|---|
| `name` | string | Rótulo de exibição ("Nome do aplicativo") — puramente cosmético. |
| `folder` | string | Nome da pasta = nome do container = hostname na `docker-php-network` (identificador técnico). Validado por `appstack.ValidAppFolder`. |
| `type` | string | `moodle` \| `php` \| `generic` (comportamento idêntico a `php`, só rótulo de catálogo diferente) \| `node` \| `php_node`. |
| `url` | string | `<pasta>.localhost`. Validado por `appstack.ValidAppURL`. |
| `php_version` | string | `7.4`–`8.4`. Usado por todo tipo exceto `node` puro; no Moodle, restrito à faixa que `moodle_version` permite. |
| `moodle_version` | string | Só `moodle`, omitido nos demais tipos. `3.x` (PHP 7.4) \| `4.1` (PHP 7.4–8.1) \| `4.2-4.3` (PHP 8.0–8.2) \| `4.4-4.5` (PHP 8.1–8.3) \| `5.0` (PHP 8.2–8.4) \| `5.1+` (PHP 8.3–8.4, cobre 5.1–5.3). `4.x` (PHP 7.4–8.1) continua aceito para contêineres salvos com ele, mas não é mais oferecido. Escolhe a receita de roteamento Nginx (`appstack.moodleRoutingFor`) — as faixas de PHP se sobrepõem e as receitas diferem, por isso é campo próprio em vez de derivado de `php_version`. Vazio/ausente é tratado como `5.1+`. |
| `db_access` | bool | Se as env vars `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASS`/`DB_NAME` (de `docker.mariadb.*`) são injetadas no container. Todo app é alcançável na `docker-php-network` independentemente disso — isso só controla se as credenciais são injetadas. |
| `node_version` | string | Só `node`/`php_node`. `22`, `24` ou `26`. Validado por `appstack.ValidNodeVersion`. |
| `dev_command` | string | Só `node`/`php_node`. Texto livre (ex.: `npm run dev`) — o comando de start do dev server. Não validado por regex (é um comando de shell por natureza); isolado via variável de ambiente na fronteira do container. |
| `dev_port` | int | Só `node`/`php_node`. A porta do dev server, `proxy_pass`'d pelo Nginx. Validado por `appstack.ValidDevPort` (faixa não-privilegiada, não pode colidir com 9000/fastcgi ou 3306/MariaDB). |
| `php_memory_limit` | string | Todo tipo exceto `node` puro. Sobrescreve o `memory_limit` do PHP embutido na imagem compartilhada (`appstack.DefaultPHPMemoryLimit`, `512M`) — ex.: `768M`, `1G`, `-1`. Validado por `appstack.ValidPHPMemoryLimit`. Vazio significa "usar o padrão da imagem". Gravado num snippet `conf.d` por app (veja abaixo), não na imagem — por isso sobrevive a Recriar/Editar. |
| `worker_command` | string | Só `php_node`. Texto livre — um processo extra em background gerenciado pelo `supervisord` (ex.: um consumidor de fila/mensageria), rodando junto do PHP-FPM e do dev server. Vazio significa nenhum processo extra; o container ainda roda um placeholder ocioso no lugar, já que a seção `[program:worker]` da imagem é compartilhada entre todo app combo. |

## Arquivos Relacionados (Fora do `config.yaml`)

| Caminho | Propósito |
|---|---|
| `~/.perci/appstack/certs/` | Certificado TLS wildcard `*.localhost` gerado pelo mkcert, criado uma vez por "Criar Container Nginx". |
| `~/.perci/appstack/nginx/default.conf` | Config gerada do Nginx, um bloco `server{}` por app em `docker.apps`; regenerada e recarregada automaticamente a cada criação/edição/remoção. |
| `~/.perci/appstack/php-conf/<pasta>/zz-perci-overrides.ini` | Snippet de php.ini gerado por app (hoje só `memory_limit`), bind-montado somente leitura no próprio container do app em `/usr/local/etc/php/conf.d/`; regenerado a cada criação/edição. |
| `{{workspace_path}}/localhost/html\|data\|logs\|databases/*` | Pastas bind-montadas por app — veja [arquitetura da stack Docker de aplicativos](../architecture/appstack.md#convenções-de-caminho). |
| `<projeto-alvo>/AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.instructions/*.md` | Gerados pela tela **Dev Tools → IA: Contextos** da GUI dentro de um projeto alvo — não é config do próprio perci. |
| `<projeto-alvo>/.agents/skills/<slug>/`, `~/.agents/skills/<slug>/` | Escrito pela ferramenta `npx skills` (só na GUI, "Dev Tools :: IA: SKILLs") pra OpenCode/Antigravity/Codex — Claude Code usa `.claude/skills/`/`~/.claude/skills/` em vez disso; não é config do próprio perci. |
| `<projeto-alvo>/.agents/mcp_config.json`, `~/.gemini/config/mcp_config.json` | Registro de servidores MCP do Antigravity, editado diretamente (só na GUI, "Dev Tools :: IA: MCPs", `internal/dev/mcpservers` — Claude Code/Codex usam seu próprio `mcp add`/config em vez disso). |

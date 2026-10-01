🌐 [English](../../en/architecture/appstack.md) | **Português** | 🏠 [Índice](../index.md)

---

# Stack Docker de Aplicativos (`internal/appstack`)

Sustenta **Docker → Criar Container** (criação) e **Docker → Gerenciar Containers** (ciclo de vida) — só na GUI, orientada a formulário; não existe interface de linha de comando nenhuma. Um contêiner por projeto, em vez de um único `docker-compose.yml` compartilhado por todos — veja o [log de decisões](./decisions.md) para o racional do design.

## Modelo

Cada projeto é seu próprio **Container Aplicativo**, com sua própria URL (`<pasta>.localhost`), sua própria versão de PHP e/ou Node, e pastas de dados isoladas — em vez de uma frota compartilhada de PHP-FPM servindo subpastas arbitrárias. Os containers são gerenciados individualmente via `docker run`/`create`/`start`/`stop`/`rm` — não existe `docker-compose.yml` para essa stack. `cfg.Docker` (`internal/config`) é a única fonte de verdade sobre *quais* containers existem; `docker inspect` só é consultado para exibir status ao vivo.

```go
type DockerConfig struct {
    NginxCreated bool
    MariaDB      MariaDBConfig
    Apps         []AppContainer
}

type AppContainer struct {
    Name, Folder, Type, URL, PHPVersion string
    MoodleVersion                       string // só AppTypeMoodle — "3.x" | "4.x" | "5.0" | "5.1+"
    DBAccess                            bool
    NodeVersion, DevCommand             string // só AppTypeNode/AppTypePHPNode
    DevPort                             int    // só AppTypeNode/AppTypePHPNode
    PHPMemoryLimit                      string // todo tipo exceto AppTypeNode — vazio = padrão da imagem
    WorkerCommand                       string // só AppTypePHPNode — vazio = nenhum processo extra em background
}
```

`Type` é um de `AppTypeMoodle`, `AppTypePHP`, `AppTypeGeneric` (tecnicamente idêntico a `AppTypePHP` — só um rótulo de catálogo diferente), `AppTypeNode`, `AppTypePHPNode`.

## Convenções de caminho

Todos sob `{{cfg.WorkspacePath}}/localhost/`:

| Caminho | Usado por |
|---|---|
| `html/<pasta>` | Tipos da família PHP (Moodle/PHP/Genérico) → `/var/www/html`; `AppTypeNode` → `/app` |
| `html/<pasta>/api`, `html/<pasta>/app` | Só `AppTypePHPNode` — api/ (PHP) → `/var/www/html`, app/ (Node) → `/app` |
| `data/<pasta>` | Só `AppTypeMoodle` (moodledata) → `/var/www/data` |
| `logs/<pasta>` | Tipos da família PHP e combo (log de erro próprio do PHP-FPM); containers só-Node logam em stdout/stderr via `docker logs`, sem arquivo |
| `databases/mariadb` | Diretório de dados do container MariaDB compartilhado |

## Nginx (`nginx.go`)

Um único container `nginx:1.26-alpine` (nome `nginx`), criado uma vez via "Criar Container Nginx", montando a pasta inteira `localhost/html` como somente leitura (`/var/www/localhost/html:ro`) — assim, um app novo nunca exige *recriar* o Nginx, só recarregar a config. Portas `127.0.0.1:80`/`127.0.0.1:443`.

- `EnsureNetwork`/`NetworkExists` — a rede Docker compartilhada `docker-php-network` que todo container entra (mesmo nome que a stack antiga usava; sem preocupação de colisão simultânea, já que só uma stack roda por vez).
- `EnsureWildcardCert` — um certificado wildcard `*.localhost` via **mkcert** (item do catálogo de [Desenvolvimento → Pré-requisitos](../architecture/dev-environment.md), não assumido como já instalado), escrito uma vez em `~/.perci/appstack/certs/`, cobrindo qualquer app futuro sem passo de certificado por projeto.
- `BuildNginxConf`/`writeNginxConf` — gera `~/.perci/appstack/nginx/default.conf` a partir de `cfg.Docker.Apps`, um bloco `server{}` por app (`server_name <url>`). `buildAppServerBlock` ramifica por `app.Type`:
  - **Moodle/PHP/Genérico** — `fastcgi_pass <pasta>:9000`. PHP/Genérico sempre recebem o mesmo `root <pasta>/`; a receita de roteamento e o `root` do Moodle dependem de `MoodleVersion` (`moodleRoutingFor`) — três layouts por era do Moodle, cada um verificado em 2026-08-21 contra a documentação oficial: `3.x`/`4.x` (`appMoodleClassicRouting`, `docs.moodle.org/{311,402}/en/Nginx`) servem direto de `root <pasta>/` via `index.php`, sem `r.php`, sem nenhum `location /` de rewrite (a própria documentação do Moodle avisa que um `try_files` aqui quebraria as URLs com slash-arguments); `5.0` (`appMoodleR50Routing`, `docs.moodle.org/500/en/Nginx`) ainda tem `root <pasta>/` mas introduz o `r.php` como front controller; `5.1+` (`appMoodleRouting`, `docs.moodle.org/502/en/Nginx`) além disso move o `root` pra `<pasta>/public` (a reestruturação descrita em `moodledev.io/docs/5.1/guides/restructure`) — de qualquer forma mais simples que o da stack antiga, já que cada app já tem seu próprio vhost dedicado, sem precisar de rewrite de `SCRIPT_NAME`. Um `MoodleVersion` vazio/não reconhecido (entradas de `config.yaml` de antes desse campo existir) cai no fallback da receita `5.1+`, a única que o Container Aplicativo já gerou antes disso.
  - **Node** — um único `location / { proxy_pass http://<pasta>:<devport>; ... }` com cabeçalhos de upgrade WebSocket (`Upgrade`/`Connection: upgrade`), sem `root`/`fastcgi_pass` — necessário pro HMR do Vite/webpack-dev-server sobreviver atrás do proxy.
  - **Combo PHP+Node** — `location /api/` (fastcgi, sem rewrite — o próprio app Slim define as rotas já com o prefixo `/api`) mais `location /` (proxy_pass pro dev server, mesmos cabeçalhos de upgrade do Node), mais `location ^~ /uploads/ { alias <mount>/<pasta>/api/uploads/; }` servindo arquivos enviados pelo usuário direto do filesystem do próprio Nginx — sem isso, caem no proxy do `location /` e dão 404 contra o dev server (falha real no app learnerflow, 2026-08-28). O `root` fica no nível da pasta do projeto (não `.../api`), pra `root`+URI resolver `/api/index.php` corretamente.
  - Blocos de segurança (`appDenyBlocks` — dotfiles exceto `.well-known`, `vendor/`, `node_modules/`, `composer.json`, etc.) se aplicam só aos tipos que servem arquivos direto do filesystem do próprio Nginx (Moodle/PHP/Genérico). Node não recebe bloco de negação nenhum — seu `location /` é um `proxy_pass` puro, e o nginx despacha blocos `location` regex como esses antes de qualquer `location /` de prefixo, independente da ordem de escrita, então mantê-los devolveria 404 nos próprios caminhos legítimos do dev server (o cache de módulos `/node_modules/.vite/deps/...` do Vite, achado nos testes da Fase 6, 2026-08-24). O combo recebe um `appComboDenyBlocks` separado, ancorado em `^/api/`, protegendo só sua metade fastcgi. Uma entrada com `Folder`/`URL`/`DevPort` inválido é pulada na geração em vez de emitida malformada — mesma defesa em profundidade já aplicada a `ValidDBIdentifier` na stack antiga.
  - Todo `fastcgi_pass`/`proxy_pass` acima passa por uma variável `set $backend <pasta>[:<porta>];`, pareada com um `resolver 127.0.0.11 valid=10s;` (o próprio DNS interno do Docker) no topo do arquivo gerado — não um hostname literal. Sem isso, o Nginx resolve um hostname de upstream literal uma vez só, de forma antecipada, na hora do `nginx -t`/reload, então qualquer *outro* app apenas parado faria a validação falhar (`host not found in upstream`) e reverteria o arquivo inteiro; a variável adia a resolução pra hora da requisição — confirmado contra uma falha real durante os testes ponta a ponta da Fase 3/4 (2026-08-24).
  - `SCRIPT_FILENAME`/`DOCUMENT_ROOT` em todo bloco fastcgi são fixados no caminho de montagem do próprio container do app (`/var/www/html`, ou `/var/www/html/public` pro layout `/public` do `5.1+`) em vez de derivados de `$document_root`/`$realpath_root` do Nginx — Nginx e o php-fpm de cada app são containers separados, com montagens diferentes do mesmo projeto (o Nginx enxerga a árvore `localhost/html` inteira de uma vez; o container do app sempre enxerga só a própria raiz do projeto, no caminho fixo `/var/www/html`), então um valor construído a partir da visão de filesystem do Nginx não resolve pra nada do lado do php-fpm. O bloco `/api/` do combo captura o restante da URI depois do `/api` via regex da location, já que só a `api/` — não o projeto inteiro — é montada naquele container.
- `ReloadNginxConfig` — regenera, `nginx -t`, `nginx -s reload`; reverte o arquivo se o teste falhar. Roda automaticamente depois de toda criação/edição/remoção de qualquer app, do MariaDB ou do próprio Nginx.

## MariaDB (`mariadb.go`)

Um único container `mariadb:11.4` (nome `mariadb`), porta `127.0.0.1:3306`, dados em `localhost/databases/mariadb` — um caminho **novo**, deliberadamente não compartilhado com o `workspace/databases/mariadb` da stack antiga (sem migração automática). `cfg.Docker.MariaDB.{DBUser,DBPass,DBRootPass}` guarda as credenciais compartilhadas; todo `AppContainer` com `DBAccess=true` ganha acesso só de rede usando elas (`GRANT ALL … WITH GRANT OPTION`) — sem banco ou usuário dedicado por projeto. As credenciais chegam ao `docker run` só como nomes (`-e MYSQL_ROOT_PASSWORD`/`-e MYSQL_USER`/`-e MYSQL_PASSWORD`), com os valores no ambiente do próprio cliente docker (`executor.Options.Env`) — nunca no argv, que qualquer usuário local lê via `ps`/`/proc/<pid>/cmdline`. `ValidDBIdentifier` (`^[A-Za-z0-9_]{1,32}$`) e `GenPassword()` (16 caracteres aleatórios) vivem nativamente aqui — originalmente reexportadas de `internal/stack/config` (legado), migradas pra dentro quando esse pacote foi removido.

## Imagens PHP (`image.go`)

`SupportedPHPVersions = 7.4, 8.0, 8.1, 8.2, 8.3, 8.4`. Uma imagem `perci-php<versão>` por versão, construída na primeira vez que é usada (`EnsureImage`) e reaproveitada por todo app naquela versão. Cada imagem leva um label `perci.hash` — um hash do Dockerfile, dos arquivos de configuração embutidos (`php.ini`, `supervisord.conf`) e dos build args; quando o label de uma imagem existente não bate com o que este Perci construiria (uma imagem de uma versão anterior, ou um template alterado), ela é reconstruída automaticamente, e o terminal lista os contêineres existentes que continuam na imagem antiga até serem recriados. O mesmo vale para as imagens Node e combo. `AppTypeMoodle` restringe ainda mais as versões de PHP oferecidas conforme `MoodleVersion` (`PHPVersionsForMoodleVersion`/`ValidPHPVersionForMoodleVersion`): `3.x` → só 7.4, `4.x` → 8.0–8.1, `5.0`/`5.1+` → 8.2–8.4 (o Moodle 5.0 elevou o próprio piso pra igualar o do 5.1+ — `moodledev.io/general/releases/5.0`, confirmado em 2026-08-24; o `5.0` originalmente foi implementado aqui como 8.0–8.1 também, junto com o `4.x`, até testes reais ponta a ponta revelarem a divergência) — reforçado tanto nos campos de formulário "Versão do Moodle"/"Versão PHP" da GUI quanto de novo em `CreateApp`. `MoodleVersion` precisa ser seu próprio campo persistido em `AppContainer`, em vez de derivado de `PHPVersion`, porque `5.0` e `5.1+` agora compartilham a mesma faixa de PHP mas exigem receitas de Nginx diferentes (ver acima); a inferência do fluxo de edição da GUI para entradas anteriores ao campo `MoodleVersion` checa `5.1+` antes de `5.0` num match ambíguo de faixa de PHP, pra manter intacto o fallback histórico pra `5.1+`.

`phpIni` embute `memory_limit = DefaultPHPMemoryLimit` (`512M`) em toda imagem. Um app individual pode sobrescrever isso via `AppContainer.PHPMemoryLimit` (`ValidPHPMemoryLimit`, ex.: `768M`/`1G`/`-1`) sem reconstruir a imagem compartilhada: `WritePHPMemoryLimitConf` escreve `~/.perci/appstack/php-conf/<pasta>/zz-perci-overrides.ini` (só `memory_limit = <valor>`), bind-montado somente leitura no próprio container do app em `/usr/local/etc/php/conf.d/` — o mesmo mecanismo de conf.d que o `docker-php-ext-enable` já usa pra empilhar arquivos ini de extensão em tempo de build, só que por app e em tempo de execução. O arquivo é sempre montado, mesmo quando `PHPMemoryLimit` está vazio (aí só repete o padrão) — regenerado a cada criação/edição, então sobrevive a Recriar/Editar, ao contrário de uma edição feita à mão dentro de um container rodando (achado no app learnerflow, 2026-08-28: um ajuste manual no `php.ini` foi silenciosamente perdido na recriação seguinte).

## Imagens Node (`node.go`)

`SupportedNodeVersions = 22, 24, 26`. Uma imagem `perci-node<versão>` por versão (`EnsureNodeImage`), construída a partir da imagem oficial `node:<versão>-bookworm` com o UID ajustado (`usermod`) pro mesmo do host — mesmo raciocínio do tratamento de UID do `www-data` no PHP.

## Imagens combo PHP+Node (`combo.go`)

`EnsureComboImage` constrói `perci-php<versãoPHP>-node<versãoNode>` — a mesma base PHP (`phpDockerfile`/`phpIni`, ver abaixo) mais Node (script oficial do NodeSource via apt) mais `supervisor` (pacote Debian). O **supervisord** roda três processos gerenciados:

```ini
[supervisord]
logfile=/dev/null
pidfile=/var/run/supervisord.pid

[program:php-fpm]
command=php-fpm -F
[program:dev-server]
command=%(ENV_DEV_COMMAND)s
directory=/app
user=www-data
[program:worker]
command=sh -c "if [ -n \"$WORKER_COMMAND\" ]; then exec $WORKER_COMMAND; else exec sleep infinity; fi"
directory=/var/www/html
user=www-data
```

`[supervisord]` é obrigatório — o `CMD` do `comboDockerfile` aponta o `supervisord -c` direto pra esse arquivo, em vez do `/etc/supervisor/supervisord.conf` próprio do Debian (que fornece essa seção via seu próprio `[include]`), então esse arquivo precisa ser uma config completa e autocontida, não só um fragmento de definição de programas; todo container combo ficava em loop de crash com `Error: .ini file does not include supervisord section` até isso ser adicionado (confirmado em 2026-08-24).

`DevCommand` (ex.: `npm run dev`) é passado como variável de ambiente (`-e DEV_COMMAND=...`) em vez de interpolado no arquivo de config — a própria sintaxe `%(ENV_X)s` do supervisord lê isso em tempo de início do processo (confirmado contra `supervisord.org`, 2026-08-21), então um valor livre digitado pelo usuário nunca toca a sintaxe do `.conf`. A config em si nunca varia entre containers, então fica embutida na imagem em vez de bind-montada por projeto.

`[program:worker]` é o `AppContainer.WorkerCommand` — um processo extra opcional em background (ex.: um consumidor de fila do Symfony Messenger ou de uma queue do Laravel), adicionado em 2026-08-28 ao ligar uma fila assíncrona de jobs no app learnerflow. Diferente do `dev-server`, seu `command=` é um wrapper de shell fixo embutido na imagem, em vez de `%(ENV_WORKER_COMMAND)s` direto: o supervisord se recusa a iniciar um programa cujo `%(ENV_X)s` referenciado não existe de jeito nenhum no ambiente, o que quebraria todo app combo que deixa `WorkerCommand` vazio (o caso comum). `comboAppRunArgs` sempre define `WORKER_COMMAND` (possivelmente como `""`), e o wrapper lê isso do próprio ambiente de shell herdado em tempo de execução — executando-o num `sh -c` próprio se não estiver vazio (então variáveis, `&&` e pipes funcionam como digitados), ou ficando ocioso em `sleep infinity` caso contrário, então o programa sempre inicia limpo de qualquer forma.

`phpDockerfile`/`phpIni` (em `image.go`) são a única fonte de verdade para a base PHP de toda imagem da família PHP (standalone e combo) — originalmente reexportadas de `internal/stack/config` (legado), migradas pra dentro nativamente quando esse pacote foi removido.

## Apps (`app.go`, `node.go`, `combo.go`)

`CreateApp` despacha por `app.Type`: garante a rede, a(s) imagem(ns) certa(s), a estrutura de pastas, roda o container, escreve os wrappers de CLI, persiste a entrada em `cfg.Docker.Apps` (substituindo qualquer entrada existente pela mesma `Folder`, em vez de duplicar), e recarrega o Nginx. `RemoveApp` nunca apaga `html`/`data` em disco — só o container e a entrada de config.

- **Moodle/PHP/Genérico** — `appRunArgs`: monta `html` (+`data` para Moodle) e o override `php-conf/.../zz-perci-overrides.ini` por app (`WritePHPMemoryLimitConf`, ver acima), injeta `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASS`/`DB_NAME` só quando `DBAccess=true` (como `-e NOME`, com os valores no ambiente do cliente, igual às credenciais do MariaDB) (a conectividade de rede em `docker-php-network` é incondicional pra todo tipo de app — `DBAccess` só controla se as credenciais são injetadas).
- **Node** — `nodeAppRunArgs`: monta a pasta inteira do projeto em `/app` (sem convenção fixa de docroot entre ferramentas Node), roda como o usuário `node` da própria imagem (UID ajustado), `DevCommand` vira o `CMD` do container via `sh -c "<comando>"` como um único elemento de argv — nunca interpolado numa shell do host. Sem PHP, também sem montagem de `php-conf`.
- **Combo PHP+Node** — `comboAppRunArgs`: monta `api/`, `app/` e o override `php-conf/...` por app separadamente, injeta `DEV_COMMAND` e `WORKER_COMMAND` (sempre, mesmo vazio — ver `[program:worker]` acima) mais `DB_*` se `DBAccess`, via `-e`, roda como root (o próprio supervisord precisa disso pra derrubar privilégios por processo — o master do `php-fpm` não tem `user=` no seu bloco de programa do supervisord, mesmo comportamento implícito de "inicia root, depois cai" que todo container PHP-FPM já tem; `dev-server`/`worker` rodam explicitamente como `www-data`, reaproveitando o mesmo usuário com UID ajustado que `phpDockerfile` já configura, em vez de um usuário `node` separado).

## Wrappers de CLI (`wrappers.go`)

Um conjunto de wrappers `~/.local/bin/` por projeto, cada um um `docker exec` fino pro container daquele projeto: `php-<pasta>`, `composer-<pasta>`, `phpunit-<pasta>`, etc. pros tipos da família PHP; `npm-<pasta>` pra Node/combo. Um app combo ganha os dois conjuntos lado a lado, apontando pro mesmo container. Os wrappers da família PHP e do combo rodam como `www-data` (com o UID do usuário do host, `HOME=/tmp`), então `vendor/` e `node_modules/` não ficam com dono root no host; um container só Node já roda como o usuário `node`. Remover um app remove os wrappers dele; regravar um wrapper substitui o arquivo em vez de seguir um symlink naquele caminho.

## Exportar/Importar (`export.go`)

Permite que a definição inteira de `cfg.Docker` (incluindo credenciais do MariaDB, em texto simples — uma escolha explícita e consciente, já que o arquivo só trafega por um canal que o usuário já controla, ex.: MegaSync) viaje pra outra máquina e seja replicada numa única ação — útil entre várias estações compartilhando um workspace sincronizado. `ImportConfig` reaplica a trava: recusa se a máquina que está importando já tiver um `WorkspacePath` *diferente* configurado; adota o caminho da exportação numa máquina nova sem nenhum ainda definido. Recria tudo (rede, imagens PHP/Node, Nginx, MariaDB, cada app) reaproveitando as mesmas funções `Recreate*` que "Containers Docker" já usa — sem lógica de criação duplicada.

## Docker → Gerenciar Containers

`GetContainerRows` (`cmd/prci-gui/service_docker.go`, parte do `DockerService`) lista todo container que `cfg.Docker` conhece (Nginx, MariaDB, cada app) com status ao vivo via `docker inspect` — só exibição; o arquivo de config continua sendo a fonte de verdade da *lista*. Ações por container: Iniciar/Parar/Reiniciar/Recriar/Ver logs/Remover pra todos os tipos; **Editar** pra MariaDB e todo tipo de app (Nginx não tem nenhum parâmetro editável, então Editar seria idêntico a Recriar e foi omitido); **Backup**/**Restore** só pro MariaDB, delegando pro [`manager/db`](./managers.md#managerdb--operações-de-banco-de-dados) com as credenciais de `cfg.Docker.MariaDB.*`. Editar é, na prática, "remove e recria com os novos parâmetros, volumes preservados" — a única forma real de trocar a imagem/versão de um container já em execução — despachado pelo `Type` real da entrada, pra editar um container Node nunca abrir o formulário da família PHP por engano.

## Segurança

Campos livres novos ganham a mesma defesa em profundidade já aplicada a `ValidDBIdentifier`/`ValidMoodleFolderName` da stack antiga:

| Validador | Protege |
|---|---|
| `ValidAppFolder` (`^[A-Za-z0-9_-]{1,64}$`) | Pasta/container/hostname |
| `ValidAppURL` | Sufixo `.localhost` + caracteres permitidos, antes de chegar no `server_name` |
| `ValidDBIdentifier` | Qualquer identificador que ainda toque SQL |
| `ValidDevPort` | Faixa não-privilegiada, rejeita colisão com 9000 (fastcgi) e 3306 (MariaDB) |

Todo app é validado por inteiro em `ValidateApp` (cada validador acima, versões de PHP/Node/Moodle suportadas e a compatibilidade entre elas, mais os nomes de pasta reservados `nginx`/`mariadb`, que colidiriam com os contêineres de infraestrutura) **antes** de qualquer remoção: Editar (`ValidateAppInStack`, que também recusa uma URL já usada por outro app), Recriar e Importar validam primeiro, então uma entrada inválida nunca deixa um app rodando fora do ar. As operações que mudam a stack e regeram o roteamento do Nginx (criar/recriar/remover app, importar) são serializadas por `stackMu`, para que duas delas não gravem o `default.conf` a partir de uma lista de apps desatualizada.

O `ImportConfig` valida o arquivo inteiro (`validateExported`: todos os apps, pastas e URLs únicas, o usuário do MariaDB, um caminho de workspace absoluto) antes de gravar qualquer coisa; apps registrados localmente mas ausentes do arquivo saem da stack com um aviso que cita os contêineres deles (nunca removidos), e o Nginx é sempre recarregado no fim.

O MariaDB só aplica as credenciais a um diretório de dados vazio. `data_user`/`data_pass` registram aquelas com que ele foi inicializado (mantidas, junto com `db_root_pass`, quando o contêiner sai da stack), e o `CreateMariaDB` — verificado pelo Editar antes de remover o contêiner em execução — recusa credenciais diferentes sobre dados já inicializados, indicando `ALTER USER` ou a remoção do diretório de dados.

`DevCommand` é o único campo genuinamente livre sem whitelist de caracteres — é um comando de shell por natureza — isolado em vez disso via a indireção de variável de ambiente descrita acima, não por validação com regex.

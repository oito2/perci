🌐 [English](../../../en/guides/environments/docker.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Ambiente da Stack Docker de Aplicativos

Operação do dia a dia da stack Docker de aplicativos por projeto que o perci gerencia. Para detalhes de implementação, veja a [arquitetura da stack Docker de aplicativos](../../architecture/appstack.md).

## Pré-requisitos

O próprio Docker e o **mkcert** (pro certificado HTTPS `*.localhost`) vêm de **Desenvolvimento → Pré-requisitos** — instale os dois antes de criar o container Nginx.

## Criando Containers

Tudo fica numa única tela **Docker → Criar Container** — orientada a formulário; não existe interface de linha de comando nenhuma. Escolha primeiro um "Tipo de contêiner"; os campos exibidos dependem da escolha:

1. **Nginx** — configuração única. Cria o container do proxy reverso compartilhado, a rede Docker `docker-php-network`, e o certificado HTTPS wildcard `*.localhost` (mkcert, cobre qualquer app futuro e é regenerado quando faltam menos de 30 dias para vencer). Se já existir um container `nginx`, esse tipo fica desabilitado no select — use **Gerenciar Containers → Recriar** na linha existente.
2. **MariaDB** — configuração única (a menos que você precise trocar as credenciais). Pede um usuário/senha de banco (padrão: senha aleatória); a senha root é gerada uma vez e reaproveitada nas recriações seguintes. Fica desabilitado no select assim que já existe um container `mariadb`, igual ao Nginx.
3. **Moodle / PHP / Servidor Genérico** — um projeto Moodle, PHP puro, ou "Servidor Genérico" (idêntico a PHP puro, só um rótulo de catálogo diferente). Pede: nome de exibição, nome da pasta (também o nome do container e o hostname), tipo, URL (`<pasta>.localhost`), versão do PHP, e se precisa de acesso a banco. Moodle pede uma pergunta extra antes — "Versão do Moodle" (`3.x`, `4.1`, `4.2-4.3`, `4.4-4.5`, `5.0`, `5.1+`) — que então restringe a etapa de versão do PHP à faixa suportada por aquela versão (`3.x` → PHP 7.4, `4.1` → 7.4–8.1, `4.2-4.3` → 8.0–8.2, `4.4-4.5` → 8.1–8.3, `5.0` → 8.2–8.4, `5.1+` → 8.3–8.4) e escolhe a receita de roteamento Nginx correspondente.
4. **Node** — um projeto Node/JS independente (Vue, React, Svelte, uma API Node pura, ou qualquer coisa que suba via um comando de dev server) — sem PHP envolvido. Pede os mesmos campos básicos mais a versão do Node, o comando de start do dev server (padrão `npm run dev`) e sua porta (padrão `5173`).
5. **PHP + Node** — um único container rodando uma API PHP e um frontend Node juntos (gerenciados via supervisord), pra um projeto dividido nas subpastas `api/` (PHP) e `app/` (Node). Roteia `<pasta>.localhost/api/*` pro PHP e o resto pro dev server Node, os dois na mesma origem — sem precisar configurar CORS. Pede tanto uma versão de PHP quanto de Node, mais os mesmos campos de comando/porta do container Node.

Enviar um tipo de app (Moodle/PHP/Genérico/Node/PHP+Node) com um nome de pasta que já existe simplesmente substitui essa entrada e recria o container direto — não existe uma confirmação separada de "já existe"; enviar o formulário já é a confirmação, mesma convenção de toda ação não-interativa da GUI. Todos os fluxos de criação terminam com um reload automático da config do Nginx — sem precisar reiniciar nada na mão pra um app novo entrar no ar.

As imagens do PHP 7.4 e 8.0 são baseadas no Debian bullseye, que está fora de suporte: seus pacotes vêm do `archive.debian.org` e elas não recebem mais atualizações de segurança, então use-as só para projetos legados. As pastas do projeto são montadas com `--mount`, então um nome de workspace ou de pasta com vírgula, aspas duplas ou quebra de linha é recusado.

Gera (por app): o próprio container, wrappers de CLI em `~/.local/bin/` (`php-<pasta>`, `composer-<pasta>`, `npm-<pasta>`, etc. dependendo do tipo), e seu bloco `server{}` no Nginx. Nada instala as dependências do seu projeto (`composer install`/`npm install`) automaticamente — isso continua manual, mesma filosofia do `config.php` do Moodle não ser gerado automaticamente.

## Ciclo de Vida do Dia a Dia

**Docker → Gerenciar Containers** lista todo container que o perci conhece (Nginx, MariaDB, cada app) com status ao vivo. Escolha um, depois uma ação:

- **Iniciar / Parar / Reiniciar** — controle de ciclo de vida direto.
- **Recriar** — reaplica os parâmetros já salvos, sem formulário. Reaproveita a imagem que já está na máquina, então não traz novas correções do PHP/Node/Nginx/MariaDB.
- **Atualizar imagem** — baixa o build mais recente da mesma versão (ex.: `php:8.2-fpm` passa do 8.2.x para a última correção do 8.2), reconstrói a imagem do Perci quando a base mudou e recria o container só se a imagem dele mudou. Num app, os outros apps que usam a mesma imagem (mesma versão de PHP/Node) também são recriados. No MariaDB, um dump completo é salvo antes em `~/.perci/appstack/backups/mariadb-antes-da-atualizacao-<data>.sql` (o MariaDB precisa estar rodando) e o servidor atualiza as tabelas de sistema ao iniciar (`MARIADB_AUTO_UPGRADE`). Arquivos, bancos e plugins Moodle montados ficam no host e são mantidos; um container parado volta a ficar parado no fim.
- **Atualizar imagens** (acima da tabela) — o mesmo para todos os containers de uma vez; use quando sair uma correção de segurança.
- **Ver logs** — acompanha os logs do container (Ctrl+C pra parar de seguir). Num container combo PHP+Node, a saída dos dois processos vem intercalada, prefixada pelo supervisord.
- **Remover** — remove o container e sua entrada no `config.yaml`. **Nunca apaga `html`/`data` em disco.**
- **Editar** (MariaDB e todo tipo de app, exceto Nginx) — remove e recria com novos parâmetros, volumes existentes preservados. A única forma real de trocar a imagem ou a versão de PHP/Node de um container já em execução.
- **Backup / Restore** (só MariaDB) — despeja ou restaura contra o container MariaDB compartilhado usando as credenciais de `cfg.Docker.MariaDB.*`.

## Regerando Sobre um App Já Existente

Rodar um fluxo de criação de novo sobre um MariaDB ou app já provisionado reaproveita o que já está salvo (ex.: a senha root do MariaDB) em vez de gerar valores novos, mantendo as credenciais sincronizadas com o que está de fato configurado no container/volume em execução.

## Exportando/Importando Sua Configuração Pra Outra Máquina

Útil se você trabalha em várias estações compartilhando um workspace sincronizado (ex.: via MegaSync). **Docker → Exportar/Importar** tem duas abas:

- **Exportar** lista o Nginx, o MariaDB, cada app e cada montagem de plugin Moodle, todos marcados; desmarque o que não quiser e clique em **Exportar…** para gravar o arquivo. As credenciais do MariaDB vão em texto simples, então trate o arquivo exportado como um segredo. Só as definições são exportadas — arquivos dos projetos, `moodledata` e bancos não.
- **Importar**: escolha um arquivo, marque os itens a importar e clique em **Importar**. Os itens marcados são criados, ou substituem o app com a mesma pasta; todo o resto desta máquina fica. Os contêineres são recriados (rede → Nginx → MariaDB → cada app) e as montagens de plugin são feitas com uma única senha. Uma montagem de plugin cuja pasta de desenvolvimento ou código do Moodle não existe nesta máquina aparece como indisponível e é pulada.

Marcar um item marca também aquilo de que ele depende (um app precisa do Nginx, e do MariaDB quando usa o banco; uma montagem de plugin precisa do seu Moodle), e marcar um Moodle marca as montagens de plugin dele. O import recusa se a máquina de destino já tiver um caminho de workspace *diferente* configurado; numa máquina nova sem nenhum definido, adota o da exportação.

## O Que Deliberadamente Não Está Aqui

- Sem banco/usuário dedicado por projeto — todo app com "Acesso a banco" compartilha as mesmas credenciais do MariaDB, só conectividade de rede.
- Sem geração automática de `config.php` pro Moodle — o perci prepara a infraestrutura (pastas, container, rede, URL); configurar o Moodle em si continua manual.
- Sem ajuste "Otimiza"/"Otimiza para Moodle" do MariaDB nessa stack (existia na stack antiga; deliberadamente não portado — veja o [documento de arquitetura](../../architecture/appstack.md) pro porquê).
- Sem domínios customizados fora de `*.localhost`.

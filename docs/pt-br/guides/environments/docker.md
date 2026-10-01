🌐 [English](../../../en/guides/environments/docker.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Ambiente da Stack Docker de Aplicativos

Operação do dia a dia da stack Docker de aplicativos por projeto que o perci gerencia. Para detalhes de implementação, veja a [arquitetura da stack Docker de aplicativos](../../architecture/appstack.md).

## Pré-requisitos

O próprio Docker e o **mkcert** (pro certificado HTTPS `*.localhost`) vêm de **Desenvolvimento → Pré-requisitos** — instale os dois antes de criar o container Nginx.

## Criando Containers

Tudo fica numa única tela **Docker → Criar Container** — orientada a formulário; não existe interface de linha de comando nenhuma. Escolha primeiro um "Tipo de contêiner"; os campos exibidos dependem da escolha:

1. **Nginx** — configuração única. Cria o container do proxy reverso compartilhado, a rede Docker `docker-php-network`, e o certificado HTTPS wildcard `*.localhost` (mkcert, gerado uma vez, cobre qualquer app futuro). Se já existir um container `nginx`, esse tipo fica desabilitado no select — use **Gerenciar Containers → Recriar** na linha existente.
2. **MariaDB** — configuração única (a menos que você precise trocar as credenciais). Pede um usuário/senha de banco (padrão: senha aleatória); a senha root é gerada uma vez e reaproveitada nas recriações seguintes. Fica desabilitado no select assim que já existe um container `mariadb`, igual ao Nginx.
3. **Moodle / PHP / Servidor Genérico** — um projeto Moodle, PHP puro, ou "Servidor Genérico" (idêntico a PHP puro, só um rótulo de catálogo diferente). Pede: nome de exibição, nome da pasta (também o nome do container e o hostname), tipo, URL (`<pasta>.localhost`), versão do PHP, e se precisa de acesso a banco. Moodle pede uma pergunta extra antes — "Versão do Moodle" (`3.x`, `4.x`, `5.0`, `5.1+`) — que então restringe a etapa de versão do PHP à faixa suportada por aquela versão (`3.x` → PHP 7.4, `4.x` → PHP 8.0–8.1, `5.0`/`5.1+` → PHP 8.2–8.4) e escolhe a receita de roteamento Nginx correspondente.
4. **Node** — um projeto Node/JS independente (Vue, React, Svelte, uma API Node pura, ou qualquer coisa que suba via um comando de dev server) — sem PHP envolvido. Pede os mesmos campos básicos mais a versão do Node, o comando de start do dev server (padrão `npm run dev`) e sua porta (padrão `5173`).
5. **PHP + Node** — um único container rodando uma API PHP e um frontend Node juntos (gerenciados via supervisord), pra um projeto dividido nas subpastas `api/` (PHP) e `app/` (Node). Roteia `<pasta>.localhost/api/*` pro PHP e o resto pro dev server Node, os dois na mesma origem — sem precisar configurar CORS. Pede tanto uma versão de PHP quanto de Node, mais os mesmos campos de comando/porta do container Node.

Enviar um tipo de app (Moodle/PHP/Genérico/Node/PHP+Node) com um nome de pasta que já existe simplesmente substitui essa entrada e recria o container direto — não existe uma confirmação separada de "já existe"; enviar o formulário já é a confirmação, mesma convenção de toda ação não-interativa da GUI. Todos os fluxos de criação terminam com um reload automático da config do Nginx — sem precisar reiniciar nada na mão pra um app novo entrar no ar.

Gera (por app): o próprio container, wrappers de CLI em `~/.local/bin/` (`php-<pasta>`, `composer-<pasta>`, `npm-<pasta>`, etc. dependendo do tipo), e seu bloco `server{}` no Nginx. Nada instala as dependências do seu projeto (`composer install`/`npm install`) automaticamente — isso continua manual, mesma filosofia do `config.php` do Moodle não ser gerado automaticamente.

## Ciclo de Vida do Dia a Dia

**Docker → Gerenciar Containers** lista todo container que o perci conhece (Nginx, MariaDB, cada app) com status ao vivo. Escolha um, depois uma ação:

- **Iniciar / Parar / Reiniciar** — controle de ciclo de vida direto.
- **Recriar** — reaplica os parâmetros já salvos, sem formulário.
- **Ver logs** — acompanha os logs do container (Ctrl+C pra parar de seguir). Num container combo PHP+Node, a saída dos dois processos vem intercalada, prefixada pelo supervisord.
- **Remover** — remove o container e sua entrada no `config.yaml`. **Nunca apaga `html`/`data` em disco.**
- **Editar** (MariaDB e todo tipo de app, exceto Nginx) — remove e recria com novos parâmetros, volumes existentes preservados. A única forma real de trocar a imagem ou a versão de PHP/Node de um container já em execução.
- **Backup / Restore** (só MariaDB) — despeja ou restaura contra o container MariaDB compartilhado usando as credenciais de `cfg.Docker.MariaDB.*`.

## Regerando Sobre um App Já Existente

Rodar um fluxo de criação de novo sobre um MariaDB ou app já provisionado reaproveita o que já está salvo (ex.: a senha root do MariaDB) em vez de gerar valores novos, mantendo as credenciais sincronizadas com o que está de fato configurado no container/volume em execução.

## Exportando/Importando Sua Configuração Pra Outra Máquina

Útil se você trabalha em várias estações compartilhando um workspace sincronizado (ex.: via MegaSync): **Exportar configurações** grava toda a configuração Docker (rede, Nginx, credenciais do MariaDB — em texto simples, então trate o arquivo exportado como um segredo — e cada app) num arquivo à sua escolha. **Importar configurações** em outra máquina lê de volta e recria tudo numa única ação (rede → Nginx → MariaDB → cada app). O import recusa se a máquina de destino já tiver um caminho de workspace *diferente* configurado; numa máquina nova sem nenhum definido, adota o da exportação.

## O Que Deliberadamente Não Está Aqui

- Sem banco/usuário dedicado por projeto — todo app com "Acesso a banco" compartilha as mesmas credenciais do MariaDB, só conectividade de rede.
- Sem geração automática de `config.php` pro Moodle — o perci prepara a infraestrutura (pastas, container, rede, URL); configurar o Moodle em si continua manual.
- Sem ajuste "Otimiza"/"Otimiza para Moodle" do MariaDB nessa stack (existia na stack antiga; deliberadamente não portado — veja o [documento de arquitetura](../../architecture/appstack.md) pro porquê).
- Sem domínios customizados fora de `*.localhost`.

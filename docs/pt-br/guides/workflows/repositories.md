🌐 [English](../../../en/guides/workflows/repositories.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Contas Git e Repositórios

O menu **Repositórios**, logo depois do Docker, gerencia suas contas do GitHub e do GitLab e os repositórios de cada uma. Os itens são **Configurar Conta**, **Gerenciar Repositórios** e um item por conta.

## Como uma Conta Funciona

Cada conta tem uma **pasta**. Todo repositório dentro dela, em qualquer profundidade, usa as configurações da conta automaticamente, tanto no Perci quanto no terminal:

- **Identidade dos commits**: o nome e o e-mail da conta.
- **Chave SSH**: a chave da conta, e só ela (`IdentitiesOnly=yes`).
- **Chaves dos servidores**: conferidas com `~/.perci/git/known_hosts`, que o Perci preenche com as chaves oficiais do github.com e do gitlab.com, ou com as chaves que você confirma para um servidor próprio.

O Perci faz isso com uma regra `includeIf "gitdir:<pasta>/"` por conta no `~/.gitconfig`, apontando para `~/.perci/git/<id>.gitconfig`. Ele adiciona e remove a regra com `git config --global`; o resto do seu `~/.gitconfig` nunca é editado. Duas contas não podem usar a mesma pasta, e a pasta de uma conta não pode ficar dentro da de outra.

As contas são pessoais: os repositórios mostrados são aqueles cujo dono é o usuário da conta. Repositórios de organizações, de outros usuários e remotes com apelido de host SSH (`git@github-trabalho:...`) aparecem à parte, como ocultos. Para trabalhar num desses, clone-o com o `git` numa pasta fora das pastas das contas.

## 1. Criar uma Conta

**Repositórios → Configurar Conta → Nova conta**:

| Campo | Observações |
|---|---|
| Nome da conta | O nome no menu (`OITO2`). Único; também gera o ID da conta usado nos nomes de arquivo (`oito2`). |
| Provedor / Host / Porta SSH | GitHub ou GitLab. O host padrão é `github.com`/`gitlab.com` e pode ser trocado por um servidor próprio, que também mostra a porta SSH. |
| Usuário | Seu usuário no provedor. |
| Nome / E-mail nos commits | A identidade aplicada dentro da pasta da conta. |
| Pasta da conta | Escolhida no diálogo de pastas. |
| Chave SSH | **Gerar uma chave nova (ed25519)** em `~/.ssh/perci_<id>`, com senha opcional, ou **Usar uma chave existente**, escolhida no diálogo de arquivos. Ao editar, **Manter a chave atual** é o padrão. |
| Token da API (opcional) | Faz a lista de repositórios da conta incluir os privados. Só leitura basta: no GitHub, um token fine-grained com Repository access "All repositories" e só a permissão Metadata (somente leitura); no GitLab, um personal access token com o escopo `read_api`. Fica no chaveiro do sistema (`secret-tool`), nunca no `config.yaml`. |

O formulário resume os dois passos para o provedor escolhido, com botões que abrem a página de chaves SSH e a página de tokens dele (no host digitado, para um servidor próprio). A chave é uma chave **SSH**, cadastrada como chave de autenticação — não uma chave GPG.

Depois de salvar, os detalhes da conta mostram:

1. A **chave pública**, com **Copiar** e um link para a página de chaves SSH do provedor. Cadastre-a lá.
2. A **chave do servidor**. As do github.com e do gitlab.com já vêm confirmadas. Para um servidor próprio, **Verificar chave do servidor** mostra as impressões digitais para você confirmar.
3. **Desbloquear chave**, quando a chave tem senha e ainda não está carregada no agente SSH da sessão.
4. **Testar conexão**, que se autentica com a chave e confere se o servidor cumprimenta o usuário da conta.

A aba **Identidade padrão** define o nome e o e-mail globais (`git config --global`), usados fora das pastas das contas.

## 2. Trabalhar com os Repositórios da Conta

Abra **Repositórios → <conta>**. A tabela lista todos os repositórios da pasta da conta com a branch, o último commit e as alterações. Abrir a tela nunca acessa a rede: o que há para baixar (↓) é contado desde o último **Verificar**.

| Ação | O que faz |
|---|---|
| **Verificar Alterações** | `git fetch --prune` em todos os repositórios (4 por vez), ou só numa linha. Os arquivos não mudam. |
| **Baixar Alterações** | Busca e depois avança a branch atual até o upstream (fast-forward). Um repositório com alterações não commitadas, sem upstream, com HEAD solto ou que divergiu é pulado, com o motivo na aba Execução. |
| **Ajustar todas as URLs** | Troca o origin HTTPS de todos os repositórios listados da conta pela URL SSH, depois de conferir com `git ls-remote` que a chave da conta o alcança. Um token embutido na URL antiga vai embora junto. |
| **+** (por linha) | Abre o diálogo do repositório: configurações, remote e arquivos (abaixo). |

As ações de rede nunca pedem senha: rodam com `BatchMode=yes` e falham com uma mensagem. Uma chave com senha precisa ser desbloqueada antes.

### O Diálogo do Repositório

O botão **+** de uma linha abre um diálogo com três abas.

**Configurações**

- **E-mail de contato**: o endereço gravado no `CODE_OF_CONDUCT.md` (denúncias) e no `SUPPORT.md` (contato privado). O e-mail da conta vale para todos os repositórios dela; o e-mail próprio do repositório, quando preenchido, o substitui só naquele repositório — deixe-o vazio para usar o da conta. Cada campo tem o seu **Salvar**.
- **Corrigir identidade**, quando o repositório define o próprio `user.name`/`user.email`: remove os dois, para ele usar os da conta. Os commits já feitos não mudam.

**Remote e SSH**

- Um `origin` que já usa SSH com a conta só é mostrado.
- Um `origin` HTTPS da conta oferece **Trocar para SSH**, a mesma verificação do Ajustar todas as URLs, só para este repositório.
- Sem remote, **Definir remote** adiciona o `origin` a partir do nome do repositório no provedor. Ele não precisa existir lá ainda: crie-o vazio no site antes do primeiro push.

**Arquivos**

Uma tabela dos arquivos de comunidade que o Perci gera, com o estado de cada um e as suas ações:

| Arquivo | O que o Perci grava |
|---|---|
| `.gitignore` | Os padrões do tipo de projeto detectado na raiz do repositório (Go, Flutter, Node.js, Python, ...), ou os genéricos. **Atualizar** só acrescenta os padrões que faltam. |
| `LICENSE` | O texto sem alterações da licença escolhida acima da tabela: GPL-3.0 (padrão), AGPL-3.0, LGPL-3.0, MIT, Apache-2.0, MPL-2.0 ou BSD-3-Clause. MIT e BSD-3-Clause pedem também o titular dos direitos (o nome Git da conta) e o ano. |
| `CODE_OF_CONDUCT.md` | O Contributor Covenant com o e-mail de contato, mais o espelho em português `docs/pt-br/codigo-de-conduta.md`. |
| `SUPPORT.md` | Onde obter ajuda: as issues do repositório e o e-mail de contato, mais o espelho em português `docs/pt-br/suporte.md`. |
| Modelos de issue | GitHub: `.github/ISSUE_TEMPLATE/bug_report.yml` e `feature_request.yml` (issue forms) e `config.yml`, que desliga as issues em branco e aponta para o `SUPPORT.md`. GitLab: `.gitlab/issue_templates/Bug.md` e `Feature.md`. |
| Modelo de pull request | GitHub: `.github/PULL_REQUEST_TEMPLATE.md`. GitLab: `.gitlab/merge_request_templates/Default.md`. Resumo, issue relacionada, como foi testado e um checklist. |
| `.github/FUNDING.yml` | Ainda não disponível. |

- **Status**: **não existe**; **em dia** (igual ao que o Perci grava agora); **diferente** (editado, de uma versão anterior, com outro e-mail ou endereço, ou faltando um dos seus arquivos); **faltam N padrões** no `.gitignore`; **existe** quando o arquivo depende do e-mail de contato e nenhum está definido. No `LICENSE`, mostra a licença encontrada no arquivo — outro titular ou ano continua em dia — ou diz **licença não reconhecida**; outros arquivos de licença (`COPYING`, `LICENSE.md`, ...) são avisados.
- **Criar** grava um arquivo que falta. **Atualizar** substitui os arquivos depois de uma confirmação, e a versão substituída de cada arquivo fica em `<arquivo>.perci-bak`. **Remover** apaga os arquivos da linha depois de uma confirmação, mais as pastas `.github/` ou `.gitlab/` que ficarem vazias; `docs/pt-br/` nunca é removida.
- `CODE_OF_CONDUCT.md` e `SUPPORT.md` precisam de um e-mail de contato: sem ele, Criar e Atualizar ficam desabilitados.
- Os links do `SUPPORT.md` e do `config.yml` apontam para a página do repositório, tirada do `origin` (SSH ou HTTPS da conta). Sem remote, viram texto simples; depois que um remote é definido, os arquivos aparecem como diferente e o Atualizar acrescenta os links.
- Os arquivos gerados são em inglês, com espelho em português só do `CODE_OF_CONDUCT.md` e do `SUPPORT.md`. Nada é commitado: revise os arquivos e faça o commit você mesmo.

## 3. Adicionar um Repositório

**Novo Repositório** oferece três caminhos:

- **Escolher da lista da conta**: lista os repositórios da conta no GitHub/GitLab — os públicos sem token, também os privados com ele. Os já clonados mostram onde estão.
- **Clonar por URL ou nome**: um nome (`perci`, `grupo/app`) ou uma URL da conta; uma URL HTTPS é clonada por SSH.
- **Iniciar um repositório local**: `git init` numa pasta nova, opcionalmente já com o `origin` definido.

O destino é uma pasta relativa à pasta da conta (`labs/perci`), nova ou vazia, e fora de outro repositório. Se um clone falha, as pastas que ele criou são apagadas.

## 4. Levar as Contas para Outra Máquina

**Repositórios → Gerenciar Repositórios**:

1. **Exportar**: marque as contas e os repositórios a exportar e escolha o arquivo. De cada conta vão o nome, o provedor, o host, a porta SSH, o usuário, a identidade dos commits, a pasta e o e-mail de contato; de cada repositório, a pasta, a URL SSH e o e-mail de contato próprio. Chaves SSH e tokens nunca vão no arquivo. Repositórios sem remote ficam de fora, com um aviso. Este arquivo é separado do export do Docker.
2. **Importar** (na outra máquina): escolha o arquivo e marque o que importar.
   - Uma conta do mesmo usuário no mesmo servidor de uma já cadastrada mantém os dados e a pasta dela, e só ganha os repositórios novos e os e-mails de contato que ela ainda não tem.
   - Uma conta nova ganha a pasta (a exportada, ou outra escolhida em **Trocar pasta**), uma chave SSH e a regra no `~/.gitconfig`. A chave é `~/.ssh/perci_<id>`: reaproveitada quando existe, senão gerada sem senha. A aba Execução mostra a chave pública a cadastrar.
   - Um repositório cuja pasta já tem o mesmo repositório é pulado como já clonado; um cuja pasta tem outra coisa é pulado com um aviso.
3. Os repositórios importados aparecem como **Pendentes** na tela de cada conta. Cadastre a chave pública da conta no provedor e use **Clonar pendentes**: ele testa a conexão antes e clona os pendentes um por vez. **Descartar** tira um da lista sem cloná-lo.

## Relacionados

- [Referência de configuração](../../reference/configuration.md#git_accounts) — as chaves de `git_accounts` e os arquivos que cada conta usa.
- [Desinstalação](../../getting-started/uninstallation.md) — o que desinstalar faz com as contas.
- [Problemas comuns](../../troubleshooting/common-issues.md#contas-git-e-repositórios) — chaves recusadas, repositórios ocultos e outros erros.

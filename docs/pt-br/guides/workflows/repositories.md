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
| **Ajustar URL** / **Ajustar todas as URLs** | Troca um origin HTTPS da conta pela URL SSH do mesmo repositório, depois de conferir com `git ls-remote` que a chave da conta o alcança. Um token embutido na URL antiga vai embora junto. |
| **Definir remote** | Adiciona o `origin` a um repositório sem remote, a partir do nome do repositório no provedor. |
| **Corrigir identidade** | Remove o `user.name`/`user.email` que o repositório define para si, para ele usar os da conta. |
| **.gitignore** / **Código de Conduta** | Gera ou mescla o `.gitignore` do projeto; grava `CODE_OF_CONDUCT.md` e o espelho em português. |

As ações de rede nunca pedem senha: rodam com `BatchMode=yes` e falham com uma mensagem. Uma chave com senha precisa ser desbloqueada antes.

## 3. Adicionar um Repositório

**Novo Repositório** oferece três caminhos:

- **Escolher da lista da conta**: lista os repositórios da conta no GitHub/GitLab — os públicos sem token, também os privados com ele. Os já clonados mostram onde estão.
- **Clonar por URL ou nome**: um nome (`perci`, `grupo/app`) ou uma URL da conta; uma URL HTTPS é clonada por SSH.
- **Iniciar um repositório local**: `git init` numa pasta nova, opcionalmente já com o `origin` definido.

O destino é uma pasta relativa à pasta da conta (`labs/perci`), nova ou vazia, e fora de outro repositório. Se um clone falha, as pastas que ele criou são apagadas.

## 4. Levar as Contas para Outra Máquina

**Repositórios → Gerenciar Repositórios**:

1. **Exportar**: marque as contas e os repositórios a exportar e escolha o arquivo. De cada conta vão o nome, o provedor, o host, a porta SSH, o usuário, a identidade dos commits e a pasta; de cada repositório, a pasta e a URL SSH. Chaves SSH e tokens nunca vão no arquivo. Repositórios sem remote ficam de fora, com um aviso. Este arquivo é separado do export do Docker.
2. **Importar** (na outra máquina): escolha o arquivo e marque o que importar.
   - Uma conta do mesmo usuário no mesmo servidor de uma já cadastrada mantém os dados e a pasta dela, e só ganha os repositórios novos.
   - Uma conta nova ganha a pasta (a exportada, ou outra escolhida em **Trocar pasta**), uma chave SSH e a regra no `~/.gitconfig`. A chave é `~/.ssh/perci_<id>`: reaproveitada quando existe, senão gerada sem senha. A aba Execução mostra a chave pública a cadastrar.
   - Um repositório cuja pasta já tem o mesmo repositório é pulado como já clonado; um cuja pasta tem outra coisa é pulado com um aviso.
3. Os repositórios importados aparecem como **Pendentes** na tela de cada conta. Cadastre a chave pública da conta no provedor e use **Clonar pendentes**: ele testa a conexão antes e clona os pendentes um por vez. **Descartar** tira um da lista sem cloná-lo.

## Relacionados

- [Referência de configuração](../../reference/configuration.md#git_accounts) — as chaves de `git_accounts` e os arquivos que cada conta usa.
- [Desinstalação](../../getting-started/uninstallation.md) — o que desinstalar faz com as contas.
- [Problemas comuns](../../troubleshooting/common-issues.md#contas-git-e-repositórios) — chaves recusadas, repositórios ocultos e outros erros.

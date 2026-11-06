🌐 [Português](../../../pt-br/guides/workflows/repositories.md) | **English** | 🏠 [Index](../../index.md)

---

# Git Accounts and Repositories

The **Repositórios** menu, right after Docker, manages your GitHub and GitLab accounts and the repositories of each one. Its items are **Configurar Conta**, **Gerenciar Repositórios** and one item per account.

## How an Account Works

Each account owns a **folder**. Every repository inside that folder, at any depth, uses the account's settings automatically, in Perci and in the terminal alike:

- **Commit identity**: the account's Git name and e-mail.
- **SSH key**: the account's key, and only that key (`IdentitiesOnly=yes`).
- **Server keys**: checked against `~/.perci/git/known_hosts`, which Perci fills with the official keys of github.com and gitlab.com, or with the keys you confirm for a self-hosted server.

Perci does this with one `includeIf "gitdir:<folder>/"` rule per account in `~/.gitconfig`, pointing to `~/.perci/git/<id>.gitconfig`. It adds and removes the rule with `git config --global`; the rest of your `~/.gitconfig` is never edited. Two accounts can't share a folder, and one account's folder can't be inside another's.

Accounts are personal: the repositories shown are the ones whose owner is the account user. Organization repositories, other users' repositories and remotes using an SSH host alias (`git@github-work:...`) are listed apart as hidden. To work on one of those, clone it with `git` in a folder outside the account folders.

## 1. Create an Account

**Repositórios → Configurar Conta → Nova conta**:

| Field | Notes |
|---|---|
| Nome da conta | The menu label (`OITO2`). Unique; it also gives the account ID used in file names (`oito2`). |
| Provedor / Host / Porta SSH | GitHub or GitLab. The host defaults to `github.com`/`gitlab.com` and can be changed for a self-hosted server, which also offers the SSH port. |
| Usuário | Your user name on the provider. |
| Nome / E-mail nos commits | The identity applied inside the account folder. |
| Pasta da conta | Picked in the folder dialog. |
| Chave SSH | **Gerar uma chave nova (ed25519)** at `~/.ssh/perci_<id>`, with an optional passphrase, or **Usar uma chave existente**, picked in the file dialog. When editing, **Manter a chave atual** is the default. |
| Token da API (opcional) | Lets the account's repository list include private repositories. Read-only is enough: on GitHub a fine-grained token with Repository access "All repositories" and only the Metadata (read-only) permission; on GitLab a personal access token with the `read_api` scope. Stored in the system keyring (`secret-tool`), never in `config.yaml`. |

The form summarizes both steps for the chosen provider, with buttons that open its SSH keys page and its token page (on the typed host, for a self-hosted server). The key is an **SSH** key, registered as an authentication key — not a GPG key.

After saving, the account's details show:

1. The **public key**, with **Copiar** and a link to the provider's SSH keys page. Register it there.
2. The **server key**. github.com and gitlab.com are confirmed already. For a self-hosted server, **Verificar chave do servidor** shows its fingerprints for you to confirm.
3. **Desbloquear chave**, when the key has a passphrase and isn't loaded in the session's SSH agent yet.
4. **Testar conexão**, which authenticates with the key and checks that the server greets the account user.

The **Identidade padrão** tab sets the global name and e-mail (`git config --global`), used outside the account folders.

## 2. Work with the Account's Repositories

Open **Repositórios → <account>**. The table lists every repository under the account folder with its branch, last commit and changes. Opening the screen never touches the network: what there is to download (↓) is counted from the last **Verificar**.

| Action | What it does |
|---|---|
| **Verificar Alterações** | `git fetch --prune` in every repository (4 at a time), or only in one row. Files aren't changed. |
| **Baixar Alterações** | Fetches, then fast-forwards the current branch to its upstream. A repository with uncommitted changes, without upstream, with a detached HEAD or that diverged is skipped, with the reason on the Execução tab. |
| **Ajustar todas as URLs** | Replaces the HTTPS origin of every listed repository of the account with its SSH URL, after checking with `git ls-remote` that the account key reaches it. Any token embedded in the old URL goes away. |
| **+** (per row) | Opens the repository's dialog: settings, remote and files (below). |

Network actions never ask for a password: they run with `BatchMode=yes` and fail with a message instead. A key with a passphrase must be unlocked first.

### The Repository Dialog

The **+** button of a row opens a dialog with three tabs.

**Configurações**

- **Contact e-mail**: the address written into `CODE_OF_CONDUCT.md` (reports) and `SUPPORT.md` (private contact). The account's e-mail applies to all its repositories; the repository's own e-mail, when filled in, replaces it for that repository only — leave it empty to use the account's. Each field has its own **Salvar**.
- **Corrigir identidade**, when the repository sets its own `user.name`/`user.email`: removes them, so it uses the account's. Commits already made don't change.

**Remote e SSH**

- An `origin` that already uses SSH with the account is just shown.
- An HTTPS `origin` of the account offers **Trocar para SSH**, the same check as Ajustar todas as URLs, for this repository only.
- Without a remote, **Definir remote** adds `origin` from the repository name on the provider. The repository doesn't need to exist there yet: create it empty on the site before the first push.

**Arquivos**

A table of the community files Perci generates, with the state of each one and its actions:

| File | What Perci writes |
|---|---|
| `.gitignore` | The patterns of the project type detected at the repository root (Go, Flutter, Node.js, Python, ...), or generic ones. **Atualizar** only appends the missing patterns. |
| `LICENSE` | The unmodified text of the license chosen above the table: GPL-3.0 (default), AGPL-3.0, LGPL-3.0, MIT, Apache-2.0, MPL-2.0 or BSD-3-Clause. MIT and BSD-3-Clause also ask for the copyright holder (the account's Git name) and the year. |
| `CODE_OF_CONDUCT.md` | The Contributor Covenant with the contact e-mail, plus its Portuguese mirror `docs/pt-br/codigo-de-conduta.md`. |
| `SUPPORT.md` | Where to get help: the repository's issues and the contact e-mail, plus its Portuguese mirror `docs/pt-br/suporte.md`. |
| Issue templates | GitHub: `.github/ISSUE_TEMPLATE/bug_report.yml` and `feature_request.yml` (issue forms) and `config.yml`, which turns off blank issues and links to `SUPPORT.md`. GitLab: `.gitlab/issue_templates/Bug.md` and `Feature.md`. |
| Pull request template | GitHub: `.github/PULL_REQUEST_TEMPLATE.md`. GitLab: `.gitlab/merge_request_templates/Default.md`. Summary, related issue, how it was tested and a checklist. |
| `.github/FUNDING.yml` | Not available yet. |

- **Status**: **não existe**; **em dia** (the same as what Perci writes now); **diferente** (edited, from an earlier version, with another e-mail or address, or missing one of its files); **faltam N padrões** for the `.gitignore`; **existe** when the file depends on the contact e-mail and none is set. For `LICENSE` it names the license found in the file — another holder or year still counts as em dia — or says **licença não reconhecida**; other license files (`COPYING`, `LICENSE.md`, ...) are noted.
- **Criar** writes a missing file. **Atualizar** replaces the files after a confirmation, and the replaced version of each file is kept as `<file>.perci-bak`. **Remover** deletes the row's files after a confirmation, plus the `.github/` or `.gitlab/` folders left empty; `docs/pt-br/` is never removed.
- `CODE_OF_CONDUCT.md` and `SUPPORT.md` need a contact e-mail: without one, Criar and Atualizar stay disabled.
- The links in `SUPPORT.md` and `config.yml` point to the repository page, taken from its `origin` (SSH or HTTPS of the account). Without a remote they become plain text; once a remote is set, the files show as diferente and Atualizar adds the links.
- The generated files are in English, with Portuguese mirrors only for `CODE_OF_CONDUCT.md` and `SUPPORT.md`. Nothing is committed: review the files and commit them yourself.

## 3. Add a Repository

**Novo Repositório** offers three ways:

- **Escolher da lista da conta**: lists the account's repositories on GitHub/GitLab — public ones without a token, private ones too with it. Repositories already cloned show where they are.
- **Clonar por URL ou nome**: a name (`perci`, `grupo/app`) or a URL of the account; an HTTPS URL is cloned over SSH.
- **Iniciar um repositório local**: `git init` in a new folder, optionally with `origin` already set.

The destination is a folder relative to the account folder (`labs/perci`), new or empty, and not inside another repository. If a clone fails, the folders it created are removed.

## 4. Move Accounts to Another Machine

**Repositórios → Gerenciar Repositórios**:

1. **Exportar**: check the accounts and repositories to export and pick the file. Per account it holds the name, provider, host, SSH port, user, commit identity, folder and contact e-mail; per repository, its folder, SSH URL and own contact e-mail. SSH keys and tokens never go into the file. Repositories without a remote are left out, with a note. This file is separate from the Docker export.
2. **Importar** (on the other machine): pick the file and check what to import.
   - An account of the same user on the same server as one already registered keeps its data and folder, and only gains the new repositories and the contact e-mails it doesn't have yet.
   - A new account gets its folder (the exported one, or another picked with **Trocar pasta**), an SSH key and its `~/.gitconfig` rule. The key is `~/.ssh/perci_<id>`: reused when it exists, otherwise generated without a passphrase. The Execução tab shows the public key to register.
   - A repository whose folder already holds the same repository is skipped as already cloned; one whose folder holds something else is skipped with a warning.
3. The imported repositories show as **Pendentes** on each account's screen. Register the account's public key on the provider, then use **Clonar pendentes**: it tests the connection first and clones the pending repositories one at a time. **Descartar** removes one from the list without cloning it.

## Related

- [Configuration reference](../../reference/configuration.md#git_accounts) — the `git_accounts` keys and the files each account uses.
- [Uninstallation](../../getting-started/uninstallation.md) — what uninstalling does with the accounts.
- [Common issues](../../troubleshooting/common-issues.md#git-accounts-and-repositories) — refused keys, hidden repositories and other errors.

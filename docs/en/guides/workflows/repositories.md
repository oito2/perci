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
| **Ajustar URL** / **Ajustar todas as URLs** | Replaces an HTTPS origin of the account with the SSH URL of the same repository, after checking with `git ls-remote` that the account key reaches it. Any token embedded in the old URL goes away. |
| **Definir remote** | Adds `origin` to a repository without a remote, from the repository name on the provider. |
| **Corrigir identidade** | Removes the `user.name`/`user.email` the repository sets for itself, so it uses the account's. |
| **.gitignore** / **Código de Conduta** | Generates or merges the project's `.gitignore`; writes `CODE_OF_CONDUCT.md` and its Portuguese mirror. |

Network actions never ask for a password: they run with `BatchMode=yes` and fail with a message instead. A key with a passphrase must be unlocked first.

## 3. Add a Repository

**Novo Repositório** offers three ways:

- **Escolher da lista da conta**: lists the account's repositories on GitHub/GitLab — public ones without a token, private ones too with it. Repositories already cloned show where they are.
- **Clonar por URL ou nome**: a name (`perci`, `grupo/app`) or a URL of the account; an HTTPS URL is cloned over SSH.
- **Iniciar um repositório local**: `git init` in a new folder, optionally with `origin` already set.

The destination is a folder relative to the account folder (`labs/perci`), new or empty, and not inside another repository. If a clone fails, the folders it created are removed.

## 4. Move Accounts to Another Machine

**Repositórios → Gerenciar Repositórios**:

1. **Exportar**: check the accounts and repositories to export and pick the file. Per account it holds the name, provider, host, SSH port, user, commit identity and folder; per repository, its folder and SSH URL. SSH keys and tokens never go into the file. Repositories without a remote are left out, with a note. This file is separate from the Docker export.
2. **Importar** (on the other machine): pick the file and check what to import.
   - An account of the same user on the same server as one already registered keeps its data and folder, and only gains the new repositories.
   - A new account gets its folder (the exported one, or another picked with **Trocar pasta**), an SSH key and its `~/.gitconfig` rule. The key is `~/.ssh/perci_<id>`: reused when it exists, otherwise generated without a passphrase. The Execução tab shows the public key to register.
   - A repository whose folder already holds the same repository is skipped as already cloned; one whose folder holds something else is skipped with a warning.
3. The imported repositories show as **Pendentes** on each account's screen. Register the account's public key on the provider, then use **Clonar pendentes**: it tests the connection first and clones the pending repositories one at a time. **Descartar** removes one from the list without cloning it.

## Related

- [Configuration reference](../../reference/configuration.md#git_accounts) — the `git_accounts` keys and the files each account uses.
- [Uninstallation](../../getting-started/uninstallation.md) — what uninstalling does with the accounts.
- [Common issues](../../troubleshooting/common-issues.md#git-accounts-and-repositories) — refused keys, hidden repositories and other errors.

🌐 [Português](../../../pt-br/guides/workflows/ai-contexts.md) | **English** | 🏠 [Index](../../index.md)

---

# AI Contexts

**Dev Tools → IA: Contextos** writes the files that teach AI coding agents a project's conventions: `AGENTS.md`, the standards under `.instructions/`, `.instructions/PROJECT.md` and the files of each agent you check. The screen has three tabs:

| Tab | What it does |
|---|---|
| **Contextos** | Lists every folder with a context Perci applied or registered, with its state and the **Atualizar**, **Alterar** and **Remover** actions. |
| **Definir Contexto** | Picks a folder, its name, the agents and the standards, and applies them. |
| **Exportar/Importar** | Saves the list to a YAML file and imports it on another machine. |

The files each agent gets are described in [Managers](../../architecture/managers.md#managerai--ai-context-generation).

## 1. Define a Context

On **Definir Contexto**:

1. **Selecionar Pasta**: pick the project folder.
2. **Nome**: the name the folder gets on the Contextos tab. It is required, up to 80 characters, and can't be the name of another folder (case is ignored).
3. **Agentes de IA**: check the agents you use.
4. **Tipo de projeto**: checks the standards of a project kind at once. A folder without standards starts on the type its files suggest. You can also check the standards by hand; a complement checks its base.
5. **Aplicar**: lists the files it will create, update or remove, then writes them. A replaced file keeps a `<file>.perci-bak` copy.

After a successful Aplicar, the folder appears on the **Contextos** tab with the name, standards and agents you chose. Applying again to the same folder updates that record. When no file changes but the name or the selection does, the confirmation says only the record is updated.

### Folders from Earlier Versions

Before 1.8.0, Perci kept no list of folders. When you open, on **Definir Contexto**, a folder whose files hold a context Perci generated but that isn't on the list, the **Contexto não registrado** alert shows the standards and agents found in its files. Type a **Nome** and click **Registrar**: the folder joins the list with what its files hold, and no file is written. Applying the context also registers it.

## 2. The Contextos Tab

Each row shows the folder's name, path, standards, agents and state:

| State | Meaning |
|---|---|
| **em dia** | The files are what this version of Perci writes for the recorded selection. |
| **desatualizado** | A file was written by an earlier version of Perci, or is missing. |
| **editado** | `AGENTS.md` has lines written by hand, or a recorded standard was edited. Atualizar replaces it and keeps a `.perci-bak`; rules of your own belong in `.instructions/PROJECT.md`. |
| **pendente** | The folder doesn't exist on this machine (for example, a repository not cloned yet). Once the folder exists, the row turns **desatualizado** and Atualizar writes the context. |

A **seleção diferente** badge means the folder's files hold other standards or agents than the recorded ones (for example, after applying by hand with another selection). Atualizar applies the recorded selection; Alterar lets you change it.

| Action | What it does |
|---|---|
| **Atualizar** | Regenerates the folder with its recorded standards and agents, after listing the files that change. |
| **Atualizar todos** | Does the same for every folder marked desatualizado or editado, with the changes grouped by folder. Only folders whose files change are regenerated; a folder that fails doesn't stop the others. |
| **Alterar** | Opens the folder on **Definir Contexto** with its recorded name, agents and standards. |
| **Remover** | Lists the files it will remove, then removes the ones Perci generated that are still unchanged and takes the folder off the list. Files you edited and `.instructions/PROJECT.md` stay. A pending folder only leaves the list. |

## 3. Move the List to Another Machine

### Exportar

On **Exportar/Importar**, check the folders to export and click **Exportar**, then choose where to save the file (`perci-contextos-ia.yaml` by default). For each folder, the file holds its name, path, standards and agents. A folder inside the workspace (`workspace_path`) is saved with its path relative to the workspace, so on the other machine it lands in that machine's workspace. Any other folder keeps its absolute path.

The file is separate from the Docker and Repositórios exports. Uninstalling Perci can also save it: see [Uninstallation](../../getting-started/uninstallation.md).

### Importar

Click **Selecionar arquivo** and pick an exported file. Each folder of the file is listed with its state on this machine:

| State | Meaning |
|---|---|
| **recebe o contexto** | The folder exists: the import writes its context and registers it. |
| **pendente: a pasta não existe** | The folder doesn't exist yet: it is registered as pending. |
| **já registrada aqui** | The folder is already on this machine's list: it is kept as it is and can't be checked. |
| **indisponível** | It can't be imported; the reason is shown (no agent this version knows, or the path is a file). |

For each checked folder you can change its **name** and, with **Trocar pasta**, the folder it goes to. A name that is empty, already used on this machine or given to two folders is shown in a warning and blocks **Importar** until changed. Standards and agents from a newer Perci are listed as left out.

**Importar** first lists, folder by folder, the files it will write, then writes the contexts and registers the folders.

A common sequence on a new machine: import the repositories (**Repositórios → Gerenciar Repositórios**), import the AI contexts (the repositories not cloned yet become pending), clone the pending repositories, then click **Atualizar todos** on the Contextos tab.

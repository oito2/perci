🌐 [English](../../../en/guides/workflows/ai-contexts.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Contextos de IA

**Dev Tools → IA: Contextos** grava os arquivos que ensinam as convenções de um projeto aos agentes de IA de código: o `AGENTS.md`, os guias em `.instructions/`, o `.instructions/PROJECT.md` e os arquivos de cada agente que você marcar. A tela tem três abas:

| Aba | O que faz |
|---|---|
| **Contextos** | Lista todas as pastas com um contexto que o Perci aplicou ou registrou, com o estado de cada uma e as ações **Atualizar**, **Alterar** e **Remover**. |
| **Definir Contexto** | Escolhe uma pasta, o nome dela, os agentes e os guias, e os aplica. |
| **Exportar/Importar** | Salva a lista num arquivo YAML e a importa em outra máquina. |

Os arquivos que cada agente recebe estão descritos em [Gerenciadores](../../architecture/managers.md#managerai--geração-de-contexto-ia).

## 1. Definir um Contexto

Em **Definir Contexto**:

1. **Selecionar Pasta**: escolha a pasta do projeto.
2. **Nome**: o nome da pasta na aba Contextos. É obrigatório, tem até 80 caracteres e não pode ser o nome de outra pasta (maiúsculas e minúsculas não contam).
3. **Agentes de IA**: marque os agentes que você usa.
4. **Tipo de projeto**: marca de uma vez os guias de um tipo de projeto. Uma pasta sem guias começa no tipo que os arquivos dela sugerem. Você também pode marcar os guias à mão; um complemento marca a sua base.
5. **Aplicar**: lista os arquivos que vai criar, atualizar ou remover e depois os grava. Um arquivo substituído fica com uma cópia `<arquivo>.perci-bak`.

Depois de um Aplicar bem-sucedido, a pasta aparece na aba **Contextos** com o nome, os guias e os agentes escolhidos. Aplicar de novo na mesma pasta atualiza esse registro. Quando nenhum arquivo muda mas o nome ou a seleção mudam, a confirmação avisa que só o registro é atualizado.

### Pastas de Versões Anteriores

Antes da 1.8.0, o Perci não guardava uma lista de pastas. Ao abrir em **Definir Contexto** uma pasta cujos arquivos têm um contexto gerado pelo Perci mas que não está na lista, o aviso **Contexto não registrado** mostra os guias e agentes encontrados nos arquivos. Digite um **Nome** e clique em **Registrar**: a pasta entra na lista com o que os arquivos dela têm, e nenhum arquivo é gravado. Aplicar o contexto também a registra.

## 2. A Aba Contextos

Cada linha mostra o nome, o caminho, os guias, os agentes e o estado da pasta:

| Estado | Significado |
|---|---|
| **em dia** | Os arquivos são o que esta versão do Perci grava para a seleção registrada. |
| **desatualizado** | Um arquivo foi gravado por uma versão anterior do Perci, ou está faltando. |
| **editado** | O `AGENTS.md` tem linhas escritas à mão, ou um guia registrado foi editado. Atualizar o substitui e guarda um `.perci-bak`; regras suas ficam em `.instructions/PROJECT.md`. |
| **pendente** | A pasta não existe nesta máquina (por exemplo, um repositório ainda não clonado). Quando a pasta passa a existir, a linha vira **desatualizado** e Atualizar grava o contexto. |

O selo **seleção diferente** indica que os arquivos da pasta têm outros guias ou agentes que os registrados (por exemplo, depois de aplicar à mão com outra seleção). Atualizar aplica a seleção registrada; Alterar permite mudá-la.

| Ação | O que faz |
|---|---|
| **Atualizar** | Gera de novo a pasta com os guias e agentes registrados, depois de listar os arquivos que mudam. |
| **Atualizar todos** | Faz o mesmo em todas as pastas marcadas como desatualizado ou editado, com as mudanças agrupadas por pasta. Só as pastas cujos arquivos mudam são geradas de novo; uma pasta que falha não interrompe as outras. |
| **Alterar** | Abre a pasta em **Definir Contexto** com o nome, os agentes e os guias registrados. |
| **Remover** | Lista os arquivos que vai remover, apaga os que o Perci gerou e continuam sem alteração, e tira a pasta da lista. Arquivos que você editou e o `.instructions/PROJECT.md` ficam. Uma pasta pendente só sai da lista. |

## 3. Levar a Lista para Outra Máquina

### Exportar

Em **Exportar/Importar**, marque as pastas a exportar e clique em **Exportar**; depois escolha onde salvar o arquivo (`perci-contextos-ia.yaml` por padrão). Para cada pasta, o arquivo guarda o nome, o caminho, os guias e os agentes. Uma pasta dentro do workspace (`workspace_path`) é salva com o caminho relativo ao workspace, então na outra máquina ela vai para o workspace daquela máquina. Qualquer outra pasta mantém o caminho absoluto.

O arquivo é separado das exportações do Docker e dos Repositórios. A desinstalação do Perci também pode salvá-lo: veja [Desinstalação](../../getting-started/uninstallation.md).

### Importar

Clique em **Selecionar arquivo** e escolha um arquivo exportado. Cada pasta do arquivo aparece com o seu estado nesta máquina:

| Estado | Significado |
|---|---|
| **recebe o contexto** | A pasta existe: a importação grava o contexto dela e a registra. |
| **pendente: a pasta não existe** | A pasta ainda não existe: ela é registrada como pendente. |
| **já registrada aqui** | A pasta já está na lista desta máquina: fica como está e não pode ser marcada. |
| **indisponível** | Não pode ser importada; o motivo aparece (nenhum agente que esta versão conheça, ou o caminho é um arquivo). |

Para cada pasta marcada você pode trocar o **nome** e, com **Trocar pasta**, a pasta de destino. Um nome vazio, já usado nesta máquina ou dado a duas pastas aparece num aviso e bloqueia o **Importar** até ser trocado. Guias e agentes de um Perci mais novo aparecem como deixados de fora.

**Importar** primeiro lista, pasta por pasta, os arquivos que vai gravar; depois grava os contextos e registra as pastas.

Uma sequência comum numa máquina nova: importe os repositórios (**Repositórios → Gerenciar Repositórios**), importe os contextos de IA (os repositórios ainda não clonados ficam pendentes), clone os repositórios pendentes e clique em **Atualizar todos** na aba Contextos.

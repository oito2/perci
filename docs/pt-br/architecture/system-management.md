🌐 [English](../../en/architecture/system-management.md) | **Português** | 🏠 [Índice](../index.md)

---

# Gerenciamento de Sistema (`internal/system/*`)

Sustenta a categoria **Linux** da GUI.

| Pacote | Responsabilidade |
|---|---|
| `postinstall` | Scripts de pós-instalação específicos por distro/DE. Mint Cinnamon e Mint XFCE compartilham uma implementação consolidada `runMint`; Ubuntu, Fedora e Zorin Core/Lite têm cada um seu próprio fluxo. Usa `tee`/`debconf-set-selections` em vez de interpolação de string com `bash -c` para coisas como config de sysctl e aceitação do EULA das fontes MS TTF. |
| `fonts` | Instalar/remover JetBrains Mono, Noto Fonts, Carlito, Caladea e outras. O download da "JetBrains Mono NF" é fixado numa versão específica (v3.4.0) com verificação de checksum SHA-256 — não é um fetch de "latest". Carlito, Caladea e as fontes Noto vêm de pacotes da distribuição nas duas famílias (`apt` no Debian/Ubuntu, `dnf` no Fedora; a remoção no Fedora usa `rpm -e`, que recusa quando outro pacote ainda depende da fonte). O estado de instalada é uma comparação exata com os nomes de família que o `fc-list` informa. |
| `templates` | Modelos de documentos em branco (Office, LibreOffice, texto, código) na pasta Templates/Modelos do usuário. Cai de volta para checar uma pasta `Modelos` literal em locales pt-BR quando `xdg-user-dir TEMPLATES` não resolve nenhuma; cria o diretório de templates se estiver faltando antes de escrever nele. Nunca sobrescreve um arquivo existente e só remove nomes de arquivo simples dentro dessa pasta. |
| `apps` | Catálogo Flatpak (slice `Catalogue` em `catalogue.go`) — instalação/desinstalação em passo único com confirmação visual. `EnsureFlatpak` (usado também pelos perfis de pós-instalação) instala o flatpak quando falta e sempre garante o remote Flathub no escopo configurado; seus passos (`FlatpakSetupSteps`) entram no mesmo lote de instalação de quem chama, então instalar apps no escopo do sistema pede a senha uma vez. Um lote em que algum app falhou reporta "concluído com avisos" (`executor.ErrCompletedWithWarnings`) em vez de sucesso. |
| `update` | Passada unificada de atualização `apt`/`dnf` + Flatpak + Snap, executada a partir da tela **Atualizar Sistema**. Os apps Flatpak são atualizados na instalação do usuário e na do sistema. Duas limpezas são opcionais, em caixas desmarcadas por padrão: apagar entradas do journal com mais de 7 dias e remover pacotes órfãos (`autoremove`) junto com runtimes Flatpak sem uso. |
| `linuxtoys` | Instalador de ferramentas comunitárias de terminal ("Toys"). Baixa o script instalador oficial e o executa como root via pkexec (um pedido de senha) — os `sudo` internos dele não conseguem pedir senha dentro da GUI. O script chega ao root pelo stdin (`bash -s`), nunca como um caminho de arquivo que outro processo do mesmo usuário poderia trocar durante o diálogo de senha. |
| `megasync` | Cliente de sincronização de desktop do MEGA. Instalado a partir do repositório oficial assinado do MEGA (verificado via GPG), não de um `.deb`/`.rpm` baixado manualmente e sem verificação. Pode ser reexecutado com segurança (a chave é convertida com `--batch --yes` para um arquivo temporário antes); desinstalar também remove o repositório e a chave da MEGA, no mesmo lote privilegiado. |

Todo fluxo de instalação/download que embute um script instalador upstream estilo `curl | sh` roda com `pipefail` habilitado (a única exceção é o script de instalação do `nvm`, que não é compatível com `set -u`/`nounset`).

## Restrição por Distro/DE

Nenhum desses pacotes implementa sua própria detecção de distro — eles são chamados a partir de métodos Go em `cmd/prci-gui` (vinculados ao frontend JS da GUI) depois de uma checagem contra `internal/distro.Detect()`/`DetectDE()`. Veja [Infraestrutura Central](./core-infra.md#internaldistro).

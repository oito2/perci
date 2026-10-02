// Copyright (C) 2026  oito2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Docker :: Criar Container and Docker :: Gerenciar Containers.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ===========================================================================
// "Docker :: Criar Container" / "Docker :: Gerenciar Containers".
// ===========================================================================

// --- "Docker :: Criar Container" ------------------------------------------

let __dcCatalog = null;

function dcFinalKind() {
  return dcKind.value === "__app__" ? dcAppType.value : dcKind.value;
}

function renderDcFields() {
  const kind = dcFinalKind();
  const prefill = kind === "mariadb" ? { dbPass: (__dcCatalog && __dcCatalog.__genPass) || "" } : {};
  dcFields.innerHTML = buildContainerFieldsHTML("dc", kind, __dcCatalog, prefill);
  wireMoodlePHPFilter("dc");
  wireFolderURLAutofill("dc");
}

function dcKindChanged() {
  dcAppTypeWrap.classList.toggle("hidden", dcKind.value !== "__app__");
  if (dcKind.value === "mariadb") {
    App.GenMariaDBPassword().then(function (pass) {
      __dcCatalog.__genPass = pass;
      renderDcFields();
    });
  } else {
    renderDcFields();
  }
}
dcKind.addEventListener("change", dcKindChanged);
dcAppType.addEventListener("change", renderDcFields);

function renderDockerCreateScreen() {
  dcHint.textContent = "";
  App.GetDockerCreateCatalog().then(function (catalog) {
    __dcCatalog = catalog;
    dcKind.querySelector('option[value="nginx"]').disabled = catalog.nginxExists;
    dcKind.querySelector('option[value="mariadb"]').disabled = catalog.mariadbExists;
    if (dcKind.selectedOptions[0] && dcKind.selectedOptions[0].disabled) {
      dcKind.value = catalog.nginxExists ? (catalog.mariadbExists ? "__app__" : "mariadb") : "nginx";
    }
    dcKindChanged();
  });
}

btnDcCreate.addEventListener("click", function () {
  if (running) return;
  const kind = dcFinalKind();
  const req = readContainerFieldsValues("dc", kind);
  startExecution("Criar Contêiner");
  App.CreateDockerContainer(req).catch(failRun);
});

// --- "Docker :: Gerenciar Containers" --------------------------------------

function iconButton(name, title) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "btn btn-ghost btn-xs";
  btn.title = title;
  btn.setAttribute("aria-label", title); // icon only: title alone isn't announced reliably
  btn.innerHTML = LUCIDE.svg(name, { size: 16 });
  return btn;
}

function openEditModal(row) {
  Promise.all([
    App.GetContainerForEdit(row.family, row.folder),
    App.GetDockerCreateCatalog(),
  ]).then(function (results) {
    const values = results[0];
    const catalog = results[1];
    editModalTitle.textContent = "Editar — " + row.name;
    editFields.innerHTML = buildContainerFieldsHTML("edit", values.kind, catalog, values);
    wireMoodlePHPFilter("edit");
    wireFolderURLAutofill("edit");
    const folderEl = document.getElementById("edit-folder");
    if (folderEl && row.family === "app") folderEl.disabled = true; // the folder is the container's identity: not editable

    btnEditSave.onclick = function () {
      const req = readContainerFieldsValues("edit", values.kind);
      req.folder = values.folder; // the original folder, even though its field is disabled
      editModal.close();
      startExecution("Editar " + row.name);
      App.UpdateDockerContainer(req).catch(failRun);
    };
    editModal.showModal();
  }).catch(function (err) { openError("Falha ao carregar dados para edição: " + err); });
}

function buildDockerRow(row) {
  const tr = document.createElement("tr");

  const tdType = document.createElement("td");
  tdType.textContent = row.family === "app"
    ? row.name + " (" + (CONTAINER_TYPE_LABELS[row.type] || row.type) + ")"
    : (CONTAINER_TYPE_LABELS[row.type] || row.type);
  tr.appendChild(tdType);

  const tdStatus = document.createElement("td");
  tdStatus.innerHTML = statusBadgeHTML(row.status);
  tr.appendChild(tdStatus);

  function setRowLoading() {
    tdStatus.innerHTML = '<span class="loading loading-spinner loading-sm"></span>';
  }
  function refreshRowStatus() {
    App.GetContainerRows().then(function (fresh) {
      const updated = (fresh || []).find(function (r) { return r.folder === row.folder; });
      tdStatus.innerHTML = statusBadgeHTML(updated ? updated.status : "");
    });
  }
  function wireQuickAction(btn, fn) {
    btn.addEventListener("click", function () {
      if (running) return;
      setRowLoading();
      fn().then(refreshRowStatus).catch(function (err) {
        refreshRowStatus();
        openError(String(err));
      });
    });
  }

  const tdActions = document.createElement("td");
  const wrap = document.createElement("div");
  wrap.className = "flex gap-1 flex-wrap";

  const btnStart = iconButton("play", "Iniciar");
  wireQuickAction(btnStart, function () { return App.StartDockerContainer(row.folder); });
  wrap.appendChild(btnStart);

  const btnStop = iconButton("square", "Parar");
  wireQuickAction(btnStop, function () { return App.StopDockerContainer(row.folder); });
  wrap.appendChild(btnStop);

  const btnRestart = iconButton("rotateCw", "Reiniciar");
  wireQuickAction(btnRestart, function () { return App.RestartDockerContainer(row.folder); });
  wrap.appendChild(btnRestart);

  const btnRecreate = iconButton("refreshCw", "Recriar");
  btnRecreate.addEventListener("click", function () {
    if (running) return;
    openConfirm("Recriar", 'Recriar "' + row.name + '" com os parâmetros atuais?', function () {
      if (running) return; // re-checked: the confirm dialog is async
      startExecution('Recriar "' + row.name + '"');
      App.RecreateDockerContainer(row.family, row.folder).catch(failRun);
    });
  });
  wrap.appendChild(btnRecreate);

  const btnLogs = iconButton("scrollText", "Ver Logs");
  btnLogs.addEventListener("click", function () { openLogs(row.folder, row.name); });
  wrap.appendChild(btnLogs);

  if (row.canEdit) {
    const btnEdit = iconButton("pencil", "Editar");
    btnEdit.addEventListener("click", function () {
      if (running) return;
      openEditModal(row);
    });
    wrap.appendChild(btnEdit);
  }

  if (row.hasDb) {
    const btnBackup = iconButton("download", "Backup");
    btnBackup.addEventListener("click", function () {
      if (running) return;
      App.PickBackupPath().then(function (path) {
        if (!path || running) return; // re-checked: the dialog is async
        startExecution("Backup do MariaDB");
        App.BackupMariaDBContainer(path).catch(failRun);
      });
    });
    wrap.appendChild(btnBackup);

    const btnRestore = iconButton("upload", "Restore");
    btnRestore.addEventListener("click", function () {
      if (running) return;
      App.PickRestorePath().then(function (path) {
        if (!path) return;
        openConfirm("Restore", "Restaurar o backup selecionado? Isso sobrescreve os dados atuais do MariaDB.", function () {
          if (running) return; // re-checked: the dialogs are async
          startExecution("Restore do MariaDB");
          App.RestoreMariaDBContainer(path).catch(failRun);
        });
      });
    });
    wrap.appendChild(btnRestore);
  }

  const btnRemove = iconButton("trash2", "Remover");
  btnRemove.addEventListener("click", function () {
    if (running) return;
    openConfirm("Remover", 'Remover "' + row.name + '"? Isso não apaga html/data em disco.', function () {
      setRowLoading();
      App.RemoveDockerContainer(row.family, row.folder).then(function () {
        renderDockerListScreen();
      }).catch(function (err) {
        refreshRowStatus();
        openError(String(err));
      });
    });
  });
  wrap.appendChild(btnRemove);

  tdActions.appendChild(wrap);
  tr.appendChild(tdActions);
  return tr;
}

function renderDockerListScreen() {
  dockerlistRows.innerHTML = "";
  App.GetContainerRows().then(latestOnly("dockerlist", function (rows) {
    rows = rows || [];
    dockerlistEmpty.classList.toggle("hidden", rows.length > 0);
    rows.forEach(function (row) { dockerlistRows.appendChild(buildDockerRow(row)); });
  }));
}

btnDockerExport.innerHTML = LUCIDE.svg("download", { size: 16 }) + " Exportar configurações";
btnDockerExport.addEventListener("click", function () {
  App.PickExportConfigPath().then(function (path) {
    if (!path) return;
    return App.ExportDockerConfig(path).then(function () {
      openMessage("Exportado", "Configurações exportadas para:\n" + path, false);
    });
  }).catch(function (err) { openError("Falha ao exportar: " + err); });
});

btnDockerImport.innerHTML = LUCIDE.svg("upload", { size: 16 }) + " Importar configurações";
btnDockerImport.addEventListener("click", function () {
  if (running) return;
  App.PickImportConfigPath().then(function (path) {
    if (!path) return;
    return App.PreviewImportConfig(path).then(function (preview) {
      const msg = "Workspace: " + (preview.workspacePath || "(nenhum)") + "\n" +
        "Nginx: " + (preview.hasNginx ? "sim" : "não") + " | MariaDB: " + (preview.hasMariaDb ? "sim" : "não") +
        " | Apps: " + preview.appsCount + "\n\n" +
        "Isso substitui a configuração atual do Docker e recria tudo (Nginx → MariaDB → cada Aplicativo). Continuar?";
      openConfirm("Importar configurações", msg, function () {
        if (running) return; // re-checked: the dialogs are async
        startExecution("Importar configurações");
        App.ImportDockerContainerConfig(path).catch(failRun);
      });
    });
  }).catch(function (err) { openError("Falha ao ler arquivo: " + err); });
});

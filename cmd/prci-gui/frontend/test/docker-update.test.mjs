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

import { test } from "node:test";
import assert from "node:assert/strict";
import { loadScripts, scriptOrder, evalIn } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));

const ROWS = [
  { family: "nginx", type: "nginx", name: "Nginx", folder: "nginx", status: "running" },
  { family: "mariadb", type: "mariadb", name: "MariaDB", folder: "mariadb", status: "running", canEdit: true, hasDb: true },
  { family: "app", type: "php", name: "Loja", folder: "loja", status: "exited", canEdit: true },
];

async function render() {
  const loaded = loadScripts(scriptOrder(), { fixtures: { GetContainerRows: ROWS } });
  loaded.ctx.renderDockerListScreen();
  await tick();
  const rows = loaded.element("dockerlist-rows").children;
  const updateButton = (i) => {
    const found = [];
    const walk = (el) => { (el.children || []).forEach((c) => { if (c.title === "Atualizar imagem") found.push(c); walk(c); }); };
    walk(rows[i]);
    return found;
  };
  return { ...loaded, rows, updateButton };
}

test("Gerenciar: every row has one Atualizar imagem button that confirms, then updates that container", async () => {
  const { ctx, rows, updateButton, calls } = await render();
  assert.equal(rows.length, 3);
  for (let i = 0; i < rows.length; i++) assert.equal(updateButton(i).length, 1, "row " + i);

  updateButton(2)[0].listeners.click[0]();
  assert.equal(calls.filter((c) => c.name === "UpdateDockerContainerImage").length, 0, "ran before confirming");
  evalIn(ctx, "__confirmCallback()");
  assert.deepEqual(calls.filter((c) => c.name === "UpdateDockerContainerImage").map((c) => c.args), [["app", "loja"]]);
});

test("Gerenciar: the confirmation says what is kept and announces the MariaDB backup", () => {
  const { ctx } = loadScripts(scriptOrder());
  const app = ctx.updateImageMessage(ROWS[2]);
  assert.ok(app.includes("plugins montados são mantidos") && app.includes("mesma imagem"), app);
  const db = ctx.updateImageMessage(ROWS[1]);
  assert.ok(db.includes("backup completo") && db.includes("~/.perci/appstack/backups"), db);
});

test("Gerenciar: Atualizar imagens confirms, then updates every container", () => {
  const { ctx, element, calls } = loadScripts(scriptOrder());
  element("btn-dockerlist-update-all").listeners.click[0]();
  assert.equal(calls.filter((c) => c.name === "UpdateAllDockerImages").length, 0, "ran before confirming");
  evalIn(ctx, "__confirmCallback()");
  assert.equal(calls.filter((c) => c.name === "UpdateAllDockerImages").length, 1);
});

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
import { loadScripts, scriptOrder } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));

const PROFILE = {
  supported: true,
  label: "Ubuntu 24.04",
  actions: [
    { id: "enable-repos", label: "Habilitar repositórios", description: "Universe e multiverse." },
    { id: "update-index", label: "Atualizar lista de pacotes", description: "apt-get update.", dependsOn: "enable-repos" },
    { id: "upgrade-system", label: "Atualizar pacotes", description: "full-upgrade.", dependsOn: "update-index" },
  ],
};

async function render(status) {
  const loaded = loadScripts(scriptOrder(), {
    fixtures: { GetPostinstallProfile: PROFILE, GetPostinstallStatus: status },
  });
  loaded.ctx.renderPostinstallScreen();
  await tick();
  await tick();
  const rows = loaded.element("postinstall-actions").children;
  const button = (i) => rows[i].children[1];
  const texts = (i) => rows[i].children[0].children.map((c) => c.textContent);
  return { ...loaded, rows, button, texts };
}

test("Pós-instalação: applied, available and blocked items", async () => {
  const { rows, button, texts } = await render({ "enable-repos": true });
  assert.equal(rows.length, 3);

  assert.equal(button(0).textContent, "Aplicado");
  assert.equal(button(0).disabled, true);

  assert.equal(button(1).textContent, "Aplicar");
  assert.equal(button(1).disabled, false);
  assert.deepEqual(texts(1), ["Atualizar lista de pacotes", "apt-get update."]);

  assert.equal(button(2).textContent, "Aplicar");
  assert.equal(button(2).disabled, true);
  assert.deepEqual(texts(2), ["Atualizar pacotes", "full-upgrade.", "Requer: Atualizar lista de pacotes"]);
});

test("Pós-instalação: Aplicar runs only its own item", async () => {
  const { button, calls } = await render({ "enable-repos": true });
  button(1).listeners.click[0]();
  const runs = calls.filter((c) => c.name === "RunPostinstallAction");
  assert.deepEqual(runs.map((c) => c.args), [["update-index"]]);
  assert.equal(button(1).disabled, true, "buttons stay disabled while the item runs");
});

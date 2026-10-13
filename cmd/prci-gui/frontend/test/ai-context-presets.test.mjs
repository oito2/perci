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
import { evalIn, loadScripts, scriptOrder } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));

const PRESETS = [
  { name: "Plugin Moodle", models: ["Moodle", "Documentação de projeto"] },
  { name: "Aplicativo Rust + Tauri v2", models: ["Rust + Tauri v2"], hint: "Marque também o complemento do frontend." },
];
const ITEMS = [
  { id: "Moodle", label: "Moodle", installed: false },
  { id: "Rust + Tauri v2", label: "Rust + Tauri v2", installed: true },
  { id: "Documentação de projeto", label: "Documentação de projeto", installed: false },
];

async function render() {
  const loaded = loadScripts(scriptOrder(), { fixtures: { GetAIContextPresets: PRESETS, GetAIContextItems: ITEMS } });
  await tick();
  evalIn(loaded.ctx, 'aicFolder = "/ws/proj";');
  loaded.ctx.renderAicItems();
  await tick();
  await tick();
  const checked = () => loaded.element("aic-items").children.map((li) => li.children[0].checked);
  return { ...loaded, checked };
}

test("IA: Contextos: a project type checks exactly its models and shows its hint", async () => {
  const { element, checked } = await render();
  const select = element("aic-preset");
  assert.equal(select.disabled, false);
  assert.deepEqual(select.children.map((o) => o.textContent), ["Personalizado", "Plugin Moodle", "Aplicativo Rust + Tauri v2"]);

  select.value = "Plugin Moodle";
  select.onchange();
  assert.deepEqual(checked(), [true, false, true]);

  select.value = "Aplicativo Rust + Tauri v2";
  select.onchange();
  assert.deepEqual(checked(), [false, true, false]);
  assert.match(element("aic-preset-hint").textContent, /complemento do frontend/);
});

test("IA: Contextos: changing a checkbox by hand returns to Personalizado", async () => {
  const { element } = await render();
  const select = element("aic-preset");
  select.value = "Plugin Moodle";
  select.onchange();
  const firstBox = element("aic-items").children[0].children[0];
  firstBox.listeners.change.forEach((fn) => fn());
  assert.equal(select.value, "");
  assert.equal(element("aic-preset-hint").textContent, "");
});

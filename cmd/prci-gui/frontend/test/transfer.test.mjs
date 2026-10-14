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

// fileItems is an export file with Nginx, MariaDB, an app using the
// database, a Moodle, a plugin mount that can be made here and one that
// can't.
const fileItems = () => ({
  workspacePath: "/ws", nginx: true, mariadb: true, localNginx: true, localMariaDb: false,
  apps: [
    { folder: "blog", name: "Blog", type: "php", url: "blog.localhost", dbAccess: true, exists: false },
    { folder: "mdle", name: "Moodle", type: "moodle", url: "mdle.localhost", dbAccess: false, exists: true },
  ],
  plugins: [
    { key: "mdle/local/x", component: "local_x", app: "mdle", devPath: "/dev/x", mounted: false, reason: "" },
    { key: "mdle/local/y", component: "local_y", app: "mdle", devPath: "/dev/y", mounted: false, reason: "a pasta não existe" },
  ],
  commands: ["blog: php artisan serve"],
});

const none = () => ({ nginx: false, mariadb: false, apps: [], plugins: [] });

function load(fixtures) {
  return loadScripts(scriptOrder(), { fixtures });
}

test("transferDependencies: apps need Nginx (and MariaDB when they use the database), plugins their Moodle", () => {
  const { ctx } = load({});
  const items = fileItems();
  assert.deepEqual([...ctx.transferDependencies(items, "app:blog")], ["nginx", "mariadb"]);
  assert.deepEqual([...ctx.transferDependencies(items, "app:mdle")], ["nginx"]);
  assert.deepEqual([...ctx.transferDependencies(items, "plugin:mdle/local/x")], ["app:mdle"]);
  assert.deepEqual([...ctx.transferDependencies(items, "nginx")], []);
});

test("transferNotes: an import of an app whose database exists nowhere is blocked", () => {
  const { ctx } = load({});
  const items = fileItems();
  const sel = { ...none(), apps: ["blog"] };
  const imp = ctx.transferNotes(items, sel, "import");
  assert.equal(imp.blocking, true);
  assert.match(imp.notes.join("\n"), /marque o MariaDB/);
  assert.doesNotMatch(imp.notes.join("\n"), /Nginx/, "Nginx exists here");

  const exp = ctx.transferNotes(items, sel, "export");
  assert.equal(exp.blocking, false);
  assert.match(exp.notes.join("\n"), /Nginx: ele precisa já existir/);
  assert.match(exp.notes.join("\n"), /MariaDB precisa já existir/);

  const ok = ctx.transferNotes(items, { ...sel, mariadb: true }, "import");
  assert.equal(ok.blocking, false);
  assert.equal(ok.notes.length, 0);
});

test("transferNotes: a plugin without its Moodle", () => {
  const { ctx } = load({});
  const items = fileItems();
  const sel = { ...none(), plugins: ["mdle/local/x"] };
  assert.equal(ctx.transferNotes(items, sel, "import").notes.length, 0, "mdle exists here");
  assert.match(ctx.transferNotes(items, sel, "export").notes.join("\n"), /depende do Moodle mdle/);
  items.apps[1].exists = false;
  assert.match(ctx.transferNotes(items, sel, "import").notes.join("\n"), /será pulado/);
});

test("Importar: lists the file, leaves an unavailable plugin out and imports the selection", async () => {
  const { element, calls } = load({
    PickImportConfigPath: "/tmp/perci-export.yaml",
    PreviewImportConfig: fileItems(),
  });
  element("btn-transfer-pick").listeners.click[0]();
  await tick();
  await tick();
  await tick();

  const list = element("transfer-import-list");
  const boxes = list.querySelectorAll("input[data-key]");
  assert.deepEqual(boxes.map((b) => b.dataset.key), ["nginx", "mariadb", "app:blog", "app:mdle", "plugin:mdle/local/x", "plugin:mdle/local/y"]);
  const unavailable = list.querySelector('input[data-key="plugin:mdle/local/y"]');
  assert.equal(unavailable.disabled, true);
  assert.equal(unavailable.checked, false);
  assert.equal(element("btn-transfer-import").disabled, false);

  // Unchecking MariaDB while Blog (which uses the database) is checked
  // blocks the import; checking Blog again re-checks MariaDB.
  const mariadb = list.querySelector('input[data-key="mariadb"]');
  mariadb.checked = false;
  mariadb.listeners.change[0]();
  assert.equal(element("btn-transfer-import").disabled, true);
  const blog = list.querySelector('input[data-key="app:blog"]');
  blog.checked = true;
  blog.listeners.change[0]();
  assert.equal(mariadb.checked, true);
  assert.equal(element("btn-transfer-import").disabled, false);

  element("btn-transfer-import").listeners.click[0]();
  assert.match(element("confirm-modal-message").textContent, /• blog: php artisan serve/);
  assert.match(element("confirm-modal-message").textContent, /1 plugin\(s\) montado\(s\)/);
  element("btn-confirm-ok").listeners.click[0]();
  const call = calls.find((c) => c.name === "ImportDockerContainerConfig");
  assert.ok(call, "ImportDockerContainerConfig not called");
  assert.equal(call.args[0], "/tmp/perci-export.yaml");
  assert.deepEqual(JSON.parse(JSON.stringify(call.args[1])), {
    nginx: true, mariadb: true, apps: ["blog", "mdle"], plugins: ["mdle/local/x"],
  });
});

test("Exportar: exports the checked items to the chosen file", async () => {
  const items = fileItems();
  items.plugins = items.plugins.slice(0, 1);
  const { ctx, element, calls } = load({
    GetExportItems: items,
    PickExportConfigPath: "/tmp/out.yaml",
  });
  ctx.renderTransferScreen();
  await tick();
  await tick();

  const list = element("transfer-export-list");
  const nginx = list.querySelector('input[data-key="nginx"]');
  nginx.checked = false;
  nginx.listeners.change[0]();
  assert.match(element("transfer-export-note-text").textContent, /depende do Nginx/);

  element("btn-transfer-export").listeners.click[0]();
  await tick();
  await tick();
  const call = calls.find((c) => c.name === "ExportDockerConfig");
  assert.ok(call, "ExportDockerConfig not called");
  assert.equal(call.args[0], "/tmp/out.yaml");
  assert.deepEqual(JSON.parse(JSON.stringify(call.args[1])), {
    nginx: false, mariadb: true, apps: ["blog", "mdle"], plugins: ["mdle/local/x"],
  });
});

test("checking a Moodle checks the available plugin mounts on it", async () => {
  const { ctx, element } = load({
    PickImportConfigPath: "/tmp/perci-export.yaml",
    PreviewImportConfig: fileItems(),
  });
  assert.deepEqual([...ctx.transferCompanions(fileItems(), "app:mdle")], ["plugin:mdle/local/x", "plugin:mdle/local/y"]);
  assert.deepEqual([...ctx.transferCompanions(fileItems(), "app:blog")], []);

  element("btn-transfer-pick").listeners.click[0]();
  await tick();
  await tick();
  await tick();
  const list = element("transfer-import-list");
  const mdle = list.querySelector('input[data-key="app:mdle"]');
  const available = list.querySelector('input[data-key="plugin:mdle/local/x"]');
  const unavailable = list.querySelector('input[data-key="plugin:mdle/local/y"]');
  mdle.checked = false;
  available.checked = false;
  mdle.listeners.change[0]();
  available.listeners.change[0]();
  assert.equal(available.checked, false, "unchecking the Moodle leaves the plugin as it is");

  mdle.checked = true;
  mdle.listeners.change[0]();
  assert.equal(available.checked, true);
  assert.equal(unavailable.checked, false, "an unavailable plugin mount stays out");
});

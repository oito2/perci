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
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dist = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist");

// usedIcons collects every icon name the scripts ask for: iconButton("x"),
// LUCIDE.svg("x") and the values of CATEGORY_ICONS.
function usedIcons() {
  const names = new Set();
  const dirs = [path.join(dist, "js"), path.join(dist, "js", "screens")];
  for (const dir of dirs) {
    for (const f of fs.readdirSync(dir).filter((n) => n.endsWith(".js"))) {
      const src = fs.readFileSync(path.join(dir, f), "utf8");
      for (const m of src.matchAll(/(?:iconButton|LUCIDE\.svg)\("([A-Za-z0-9]+)"/g)) names.add(m[1]);
      const cat = /const CATEGORY_ICONS = \{([^}]*)\}/.exec(src);
      if (cat) for (const m of cat[1].matchAll(/:\s*"([A-Za-z0-9]+)"/g)) names.add(m[1]);
    }
  }
  return [...names].sort();
}

test("every icon the frontend uses is in the vendored lucide set", () => {
  const icons = fs.readFileSync(path.join(dist, "vendor", "lucide", "icons.js"), "utf8");
  const used = usedIcons();
  assert.ok(used.includes("circleArrowUp") && used.includes("gitBranch"), "the scan finds the icons in use");
  const missing = used.filter((name) => !new RegExp("^\\s+" + name + ":\\s*'", "m").test(icons));
  assert.deepEqual(missing, []);
});

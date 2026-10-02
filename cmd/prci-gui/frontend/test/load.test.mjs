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

// Every script the page loads runs its top-level code without throwing,
// in order, in one shared scope — a missing global or a const declared
// twice across files fails here.
test("all classic scripts load together", () => {
  const order = scriptOrder();
  assert.ok(order.length >= 10, "index.html script list: " + order.join(", "));
  const { ctx } = loadScripts(order);
  for (const fn of ["escHTML", "escAttr", "reconcileKeyedList", "latestOnly", "linkifyDesc", "createScopedTable", "runScreenAction"]) {
    assert.equal(typeof ctx[fn], "function", fn + " is defined");
  }
});

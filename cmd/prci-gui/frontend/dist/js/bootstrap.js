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

// The only ES module of the frontend: imports the generated Wails bindings
// and the runtime (/wails/runtime.js, served by the Wails process itself)
// and exposes them as globals for the classic scripts under js/ that make
// up the rest of the app. Scripts run in the order of their <script> tags:
// module and `defer` scripts all run after parsing, in document order,
// this one first.
//
// App is a merge of the domain services — Object.assign keeps every
// App.Method(...) call working without naming each service.
import {
  HomeService, LinuxService, DevSetupService, DockerService, DevToolsService, TrayService,
} from "../bindings/github.com/oito2/perci/cmd/prci-gui/index.js";
import { Events, Browser } from "/wails/runtime.js";

window.App = Object.assign({}, HomeService, LinuxService, DevSetupService, DockerService, DevToolsService, TrayService);
window.Events = Events;
window.Browser = Browser;

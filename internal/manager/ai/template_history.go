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

package ai

// templateHistory lists, per instruction file, the SHA-256 of every
// committed version of it, so a project's copy can be told apart as an
// older Perci version (outdated) or an edit. TestTemplateHistory lists
// the hashes a newly committed version needs here.
var templateHistory = map[string][]string{
	"BASH.md": {
		"80cd8b8a80eb5f8cc9986ec6cdb9f2135cfd2c6561db815393f630e77cd64f30",
		"a99a918ced55164fff05fdaaaa155d86e279430470d3c0ba8809e55423a61848",
	},
	"FLUTTER.md": {
		"27957fedca0f07352fd6d23b5c6b88329fb82a81e37a776e860b14206dcae19b",
		"22ed33b90e9ad6f75adf6100f4f358f9b5b6fc061a80c7637e0db55ed7a8da26",
	},
	"GO-MCP.md": {
		"2a76e8e62b0a8bf88dbb9770f549500d56fed67b1502960e9e2319df4f5e7e72",
	},
	"GO.md": {
		"f769b9087c58f58b58583d315528a4a0ffd6781c156883ecfb6f33b95234d795",
	},
	"GODOT.md": {
		"5d90ab1f071922924b0ad2d33fc5894e69442fc99b8d7a834b642cd69035bb10",
	},
	"LANDING-PAGE.md": {
		"67e027b87824e7246f4a74be39ad4f9eb790cd8cb9b65f9573d03269c132217a",
		"7c0d87063b4165e0db1ba8013f4d0c015f66e3cd03ab379e067b75f7fac2792e",
	},
	"MOODLE.md": {
		"859fcdce6caac37bc8ca5bd1f206ed68f4b5a95cacf231c5d4fe7e9c97a8503b",
		"ae1c65489ea2ef5b58fee891de567f5a658a582bd6238eac74b2f2c716bc10ea",
	},
	"MYSQL-MARIADB.md": {
		"470bf7045e4fa2e5d29cbb99db2ff76fc098fd5516139c2c2fef1566e013f42b",
		"2ae6488006035e19f2f3fdc74996a04ca4729223cdc2eed79733e7c6dfba10ae",
	},
	"PHP-SLIM.md": {
		"fbae92f1525f94e41d57a637c92ff33d8b45c88e1851e768561db66bd4cf57b8",
	},
	"PROJECT-DOCUMENTATION-DESKTOP.md": {
		"776a285c14a19d2fddb17ba8d09df6843104b3675fe269bfb222d0a0de714c79",
		"835106f9445aa42339ce3a4ae2e91f50af3404fdcd2c5916b3fa4f3b609ab040",
	},
	"PROJECT-DOCUMENTATION-GAME.md": {
		"a3c76fcb18116384f1a8975c249a49ea52fdb236219b8e5522e36c7b09447aa1",
	},
	"PROJECT-DOCUMENTATION-MCP.md": {
		"4e1489c52921a6fa12a3907f7ca24eb18c4e94cc8a4a53013bb6db65b573d204",
	},
	"PROJECT-DOCUMENTATION-MOBILE.md": {
		"799db12325c1b944e8ca5af35dd2c326d1b86c768a86c59036242b7cff2018c4",
		"9376ea33b9d41133af70a461c0ee67ab7976ba1333e14990a3c6cc6996dc6d63",
	},
	"PROJECT-DOCUMENTATION-MOODLE.md": {
		"f446aa0cb6f01eb5b7549d0630f96bdbacc623361872f73422188fd5ddfc9c8b",
		"e4a558102f041f0c54d8a8592ff66a6925f7af8adcf96ce33e5c99c0d430b0cc",
	},
	"PROJECT-DOCUMENTATION-WEB.md": {
		"ef201f9ce8f7567861d277fe9825eb4b015aee91f4b3acd417b1f766187e5ddc",
		"89cb61c841ae0a4ea19486f2cb32aff2d6234c1f18b5256866aa9bb40bae2667",
	},
	"PROJECT-DOCUMENTATION.md": {
		"2c2116d0f987712a12cb838487ea1dee6dce7f9339497b3ff61a796992d6f194",
		"f0441d6229ae302812ddcc96f67f3305db5a6175cb5342fe549c9796409e5572",
	},
	"SLIM-DBAL.md": {
		"43db20a44a5b5954a875583258fad44a49071c5ef52018d2a168eee9db266fbf",
	},
	"SLIM-ELOQUENT.md": {
		"16c777f3ed296a0cd5bc2be9a68e4ecce498d066873844b4a69b213537e8eb97",
	},
	"SLIM-PDO.md": {
		"2c7c70bd6881ba463d52006462ce5250e420cb14599b894c71a7daafd86cb387",
	},
	"TAURI-REACT.md": {
		"ad662267f8e932fad90a1245f9501e9574799608ee4f63aadfa3dd3bec7b1333",
	},
	"TAURI-SVELTEKIT.md": {
		"134c035099adc4905655c42460e0858b4dfc4e4e208a8e152d0bb604211c9c8a",
	},
	"TAURI-VANILLA.md": {
		"27782fe4692f2d24872d93122c8b77e00c8cfd6345dac5e305ae8847fa093428",
	},
	"TAURI.md": {
		"dbc73b6ee6086a89896f0525b642d402a7d8de2a9b7ec3f47c1a48e6b736e5c8",
	},
	"WAILS.md": {
		"bcdab4eb6f630cb3f24983ec67c9e757e50661011815dfd1c6c4d58a0366d11a",
	},
	"WEBSITE.md": {
		"eaeeca355096205b3aac9d84c745a897d554b80ad8c7f2215781597fb61db9ee",
	},
}

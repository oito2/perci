# Godot 4 + GDScript Development Standard

Standard for building 2D and 3D games with **Godot 4** and **GDScript**, shipped to **Android** and run on desktop (Linux, Windows, macOS) during development. Applies whenever a task creates or changes scenes, scripts, resources, project settings, export presets or tests of a Godot project. C#/.NET and GDExtension code are out of scope.

This standard assumes **Godot 4.7** (current stable line: 4.7.x) as the minimum. Before applying any rule, read `config/features` in `project.godot` to confirm the project's engine version; if the project targets an older 4.x release, check that each setting below exists in that version before using it.

---

## 1. Project Layout

Godot imposes no structure. Keep the one the project already has; for a new project, group files **by feature**, keeping each scene next to its script and assets, as the official project organization guide recommends:

```text
project.godot
export_presets.cfg          # committed — contains no secrets (Godot 4.1+)
addons/                     # third-party plugins (GUT, gdUnit4, ...) — never edited in place
autoloads/                  # scripts registered as autoloads, one per file
common/                     # shared components, themes, fonts, shaders
characters/player/          # player.tscn, player.gd, player.png, ...
characters/enemies/slime/
levels/level_01/
ui/hud/  ui/main_menu/
resources/                  # custom Resource scripts and their .tres data
test/                       # unit and integration tests
```

- **snake_case** for every folder and file name (`main_menu.tscn`, `player_stats.gd`): Godot's exported PCK filesystem is case-sensitive, and Windows/macOS filesystems usually are not.
- An empty `.gdignore` file inside a folder stops Godot from importing it (e.g. `docs/`). Resources in that folder can no longer be loaded with `load()`/`preload()`.
- Third-party code lives under `addons/`. Never modify a vendored addon; update it by replacing its folder.

---

## 2. GDScript Style

Follow the official GDScript style guide:

| Element | Convention | Example |
|---|---|---|
| Files | snake_case | `yaml_parser.gd` |
| `class_name` | PascalCase | `class_name PlayerStats` |
| Nodes (in scenes) | PascalCase | `Player`, `HealthBar` |
| Functions, variables | snake_case | `func load_level() -> void:` |
| Signals | snake_case, **past tense** | `signal health_changed`, `signal door_opened` |
| Constants, enum members | CONSTANT_CASE | `const MAX_SPEED := 200.0` |
| Enum names | PascalCase | `enum State { IDLE, RUNNING }` |
| Private members, virtual methods to override | leading `_` | `var _velocity`, `func _on_timer_timeout()` |

- **Indent with tabs**, keep lines under 100 characters (80 when possible).
- Member order: `@tool`/`@icon`/`@static_unload` → `class_name` → `extends` → `##` doc comment → signals → enums → constants → static variables → `@export` variables → other variables → `@onready` variables → `_static_init()` → static methods → built-in virtuals (`_init`, `_enter_tree`, `_ready`, `_process`, `_physics_process`, ...) → public methods → private methods → inner classes.
- Document public classes and methods with `##` doc comments, in English.

---

## 3. Static Typing

Every variable, constant, parameter and return value has a static type — declared explicitly or inferred with `:=` when the type is obvious from the same line.

```gdscript
class_name Player
extends CharacterBody2D

signal health_changed(new_health: int)

const SPEED := 300.0

@export var max_health: int = 100
@export var stats: PlayerStats

var _health: int
var _inventory: Array[Item] = []
var _cooldowns: Dictionary[StringName, float] = {}

@onready var _sprite: AnimatedSprite2D = $AnimatedSprite2D


func take_damage(amount: int) -> void:
	_health = maxi(_health - amount, 0)
	health_changed.emit(_health)


func find_item(item_id: StringName) -> Item:
	for item: Item in _inventory:
		if item.id == item_id:
			return item
	return null
```

- Use typed collections (`Array[T]`, `Dictionary[K, V]`); nested typed collections (`Array[Array[int]]`) are not supported.
- Cast results of untyped APIs before use (`var body := collider as Enemy`, then null-check).
- Make the editor enforce typing. In **Project Settings → Debug → GDScript** (Advanced Settings on), raise these warnings to **Error** — stored in `project.godot` as level `2` (`0` = Ignore, `1` = Warn, `2` = Error):

```ini
[debug]

gdscript/warnings/untyped_declaration=2
gdscript/warnings/unsafe_property_access=2
gdscript/warnings/unsafe_method_access=2
gdscript/warnings/unsafe_cast=1
gdscript/warnings/unsafe_call_argument=2
```

- Do not enable `inferred_declaration`: the style guide prefers `:=` when the type is visible on the same line.
- Keep the default `debug/gdscript/warnings/directory_rules` entry that excludes `res://addons`, so third-party plugins are not checked against the project's rules. Include a first-party addon explicitly when the project develops it.
- Never silence a warning with `@warning_ignore` to make an error go away; fix the type. Use `@warning_ignore` only for a deliberate case, on the single line it covers.

---

## 4. Scenes, Nodes and Communication

- **Composition over inheritance.** Build behavior from small, reusable scenes and component nodes (`HealthComponent`, `Hitbox`, `StateMachine`) instead of deep `extends` chains. Keep `extends` chains shallow — an engine class plus at most one project base class.
- **Self-contained scenes.** A scene must run with no knowledge of its parent. Dependencies are injected by the parent (an `@export` property, a setter, or a signal connection made by the parent).
- **Call down, signal up:**

| Direction | Mechanism |
|---|---|
| Parent → child | Direct method call or property on a typed `@onready` reference |
| Child → parent / siblings | Emit a signal; the parent connects it |
| Unrelated systems | Signal on an autoload event bus — only when no common parent exists |

- Connect signals in code with the typed form `button.pressed.connect(_on_button_pressed)`; never use string-based `connect("pressed", ...)`.
- Never reach up or across the tree with `get_parent().get_parent()` or long absolute paths (`/root/Main/World/Player`). Use `@onready` references to own children, `%UniqueName` for nodes marked as unique in the scene, or groups (`get_tree().get_nodes_in_group("enemies")`).
- Use `_physics_process(delta)` for movement and physics; `_process(delta)` for visuals only. Disable processing (`set_process(false)`) on nodes that do not need it.
- Use `await` with signals (`await get_tree().create_timer(0.5).timeout`) instead of manual flags. Before touching a node after an `await`, check it is still valid (`is_instance_valid(node)`).

---

## 5. Autoloads

Autoloads are global state; use them sparingly.

- Allowed: cross-scene services with a single instance — scene transitions, save/load service, settings, audio bus control, an event bus of signals.
- Not allowed: gameplay logic of a single scene, references to nodes of the current level, data that belongs to one entity.
- One responsibility per autoload, registered in **Project Settings → Globals → Autoload**, script under `autoloads/`. Autoload names are PascalCase (`SaveManager`).
- An autoload never assumes a specific scene is loaded. Expose signals and methods; let scenes connect to them.

---

## 6. Data with Custom Resources

Store game data (item definitions, enemy stats, level configs, balancing tables) in custom `Resource` classes edited in the Inspector, not in hardcoded dictionaries or JSON parsed at runtime.

```gdscript
class_name EnemyStats
extends Resource

@export var display_name: String = ""
@export var max_health: int = 10
@export_range(0.0, 500.0) var speed: float = 80.0
@export var drops: Array[ItemData] = []
```

- Save instances as `.tres` (text, diff-friendly) under `resources/` or next to the feature that uses them.
- Resources loaded from disk are **shared** between every user of that path. Call `duplicate()` (or `duplicate(true)` for nested resources) before mutating per-instance state, or enable **Local to Scene**.
- Use `preload()` for resources known at compile time; `ResourceLoader.load_threaded_request()` for large scenes loaded during gameplay.

---

## 7. Input

- Define every action in **Project Settings → Input Map** (`move_left`, `jump`, `pause`) and read input only through actions (`Input.is_action_pressed(&"jump")`, `Input.get_vector(&"move_left", &"move_right", &"move_up", &"move_down")`). Never hardcode keycodes or mouse buttons in gameplay code.
- Gameplay input goes in `_unhandled_input()` so UI controls consume their own events first.
- **Touch:** handle `InputEventScreenTouch` (press/release) and `InputEventScreenDrag` (movement). Track multi-touch by the event's `index` — one entry per finger in a `Dictionary[int, Vector2]` — and release it on `pressed == false` or `canceled == true`.
- **Virtual controls:** use `TouchScreenButton` with its `action` property set to an Input Map action, so touch and keyboard share one code path; set `visibility_mode` to touchscreen-only to hide them on desktop. Keep virtual controls inside the safe area (`DisplayServer.get_display_safe_area()`).
- For desktop testing of touch code, enable `input_devices/pointing/emulate_touch_from_mouse`. Leave `emulate_mouse_from_touch` at its default (`true`) unless the game handles all touch events itself.
- Handle the Android back button through `NOTIFICATION_WM_GO_BACK_REQUEST` in `_notification()`, and save progress on `NOTIFICATION_APPLICATION_PAUSED`.

---

## 8. Mobile Performance

| Area | Rule |
|---|---|
| Renderer | **Mobile** is Godot's default on mobile and needs Vulkan; **Compatibility** (OpenGL ES 3.0) supports the widest range and older devices and is usually enough for 2D. Choose per the project's minimum device and record it in `rendering/renderer/rendering_method` / `rendering/renderer/rendering_method.mobile`. Switching later may require manual scene, lighting and environment fixes — never switch without asking. |
| Texture compression | Enable `rendering/textures/vram_compression/import_etc2_astc` — Android export refuses to run without it. After changing it, re-import (delete `.godot/imported/` with the editor closed). Use **Lossless**/**Lossy** compression for 2D pixel art and UI, **VRAM Compressed** for 3D textures. |
| Draw calls | 2D items with the same texture and material are batched automatically — use texture atlases and shared materials. In 3D, reduce distinct materials, use `MultiMeshInstance3D` for many copies of a mesh, and LOD for distant objects. |
| Fill rate | Mobile GPUs are tile-based: avoid overdraw (large stacked transparent sprites, full-screen effects) and dense geometry in small screen areas. |
| Object pooling | Pool frequently spawned short-lived scenes (bullets, particles, hit numbers) only when the profiler shows `instantiate()`/`queue_free()` cost; hide and disable pooled nodes instead of freeing them. |
| Physics tick | `physics/common/physics_ticks_per_second` defaults to `60`; keep it unless the game needs more precision. Enable `physics/common/physics_interpolation` when the tick rate is not a multiple of 60. |
| Frame rate | Cap with `application/run/max_fps` or V-Sync to save battery. |
| Measurement | Profile on a real low-end target device (Debugger → Profiler/Monitors over one-click deploy), not only on desktop. |

---

## 9. UI and Multiple Resolutions

- Set a base resolution (`display/window/size/viewport_width`/`viewport_height`) matching the main orientation (`display/window/handheld/orientation`); a portrait game swaps width and height manually.
- `display/window/stretch/mode`: `canvas_items` for most games, `viewport` for pixel art. `display/window/stretch/aspect`: `expand` so any aspect ratio shows more of the world instead of black bars. Both are the defaults for projects created in Godot 4.7; set them explicitly in older projects.
- Pixel art: `viewport` mode, `display/window/stretch/scale_mode="integer"`, and `rendering/textures/canvas_textures/default_texture_filter` set to Nearest.
- Lay out UI with **Containers** and **anchors** — never absolute positions. Test at least 16:9, 19.5:9 and 4:3 (tablet), portrait and landscape as applicable, using the editor's window size override.
- Keep interactive controls inside the safe area (notches, rounded corners) and size touch targets for fingers.
- Style through a shared `Theme` resource, not per-node overrides.

---

## 10. Saving Data

- Write only to `user://`; `res://` is read-only in an exported game. On Android, `user://` is private to the app.
- Settings and small key/value data: `ConfigFile` (`user://settings.cfg`).
- Game state: a versioned dictionary serialized with `JSON` or `FileAccess.store_var()`/`get_var()`. Include a `version` field and migrate old saves on load.
- Never call `get_var(true)` / `bytes_to_var_with_objects()` or load a `.tres`/`.res` from a save file the player can edit: deserialized objects can carry code that executes.
- Write atomically: save to `user://save.tmp`, then rename it over the real file with `DirAccess.rename_absolute()`, so a crash mid-write does not corrupt the save.
- `FileAccess.open_encrypted_with_pass()` and `ConfigFile.save_encrypted_pass()` (AES-256) deter casual editing only: the password ships inside the game and can be extracted. Never treat them as protection for secrets.
- Check the return value of every `FileAccess.open()` (`null` on failure) and report `FileAccess.get_open_error()`.

---

## 11. Android Export

| Requirement | Value |
|---|---|
| Export templates | Same Godot version as the editor (templates are looked up by the editor's full version, e.g. `4.7.2.stable`) — install via **Editor → Manage Export Templates** |
| JDK | OpenJDK 17 (higher is supported; 17 recommended) |
| Android SDK | Platform-Tools 35.0.0+, Build-Tools 35.0.1, Platform 35, Command-line Tools (latest), CMake 3.10.2.4988404, NDK r28b (28.1.13356709) |
| Editor Settings | `export/android/java_sdk_path`, `export/android/android_sdk_path` (per machine, never in the repo) |

- **Debug keystore:** the editor generates one automatically when `keytool` is available — never commit it.
- **Release keystore:** generated once with `keytool -v -genkey -keystore <game>.keystore -alias <alias> -keyalg RSA -validity 10000`. Keystore password and key password must be identical for Godot. Store it outside the repository and back it up; for Google Play it is the **upload key** (Play App Signing holds the app signing key).
- **Where secrets live:** `export_presets.cfg` holds the non-secret export configuration and **is committed**. Keystore paths and passwords go to `.godot/export_credentials.cfg`, which **must never be committed**. In CI, inject them via `GODOT_ANDROID_KEYSTORE_RELEASE_PATH`, `GODOT_ANDROID_KEYSTORE_RELEASE_USER`, `GODOT_ANDROID_KEYSTORE_RELEASE_PASSWORD`.
- **AAB vs APK:** Google Play requires an **AAB** for new apps; AAB requires a Gradle build. Use APK for sideloading and device testing.
- **Gradle build** (`gradle_build/use_gradle_build`) only when needed: AAB export, Android plugins/third-party SDKs, or changes to the generated Android project. Install it with **Project → Install Android Build Template** (creates `res://android/`).
- **Versioning:** increment `version/code` for every Play Store upload; keep `version/name` in sync with the project version.
- **SDK levels:** respect Google Play's current target API level requirement (Android 16 / API 36 since August 31, 2026); Godot 4.7.2 defaults to `min_sdk` 24 and `target_sdk` 36.
- **Permissions:** enable only the permissions the game uses in the preset's `permissions/*` options; document each one.
- **Command-line export** (CI); the preset name must match `export_presets.cfg`, and relative output paths resolve from the project directory:

```bash
godot --headless --path . --export-release "Android" build/android/game.aab
godot --headless --path . --export-debug "Android" build/android/game-debug.apk
```

---

## 12. Version Control

Use the `.gitignore` and `.gitattributes` Godot generates (**Project → Version Control → Generate Version Control Metadata**). The generated `.gitignore`:

```gitignore
# Godot 4+ specific ignores
.godot/
/android/
```

- `.godot/` holds the import cache **and `export_credentials.cfg`** — never commit it.
- Commit `project.godot`, `export_presets.cfg`, every `*.import` file next to its asset, `.tscn`, `.tres`, `.gd`, `.gdshader` and `.uid` files.
- Also ignore build output (`build/`, `*.apk`, `*.aab`), keystores (`*.keystore`, `*.jks`) and `*.translation` files generated from CSV.
- The generated `.gitattributes` enforces LF line endings; keep it.
- **Git LFS** for large binary assets (`.png`, `.wav`, `.ogg`, `.glb`, `.blend`, fonts) — set it up **before** the first commit of those files; every collaborator and CI runner needs Git LFS installed. Adding LFS to an existing repository rewrites history: propose it to the user, never do it unasked.

---

## 13. Testing

| Framework | Godot 4.7 compatible version | Install location |
|---|---|---|
| GUT | 9.7.x (9.x = Godot 4.x) | `addons/gut/` |
| gdUnit4 | 6.2.x | `addons/gdUnit4/` |

- Use the framework the project already has; for a new project, ask which one to adopt.
- Test pure logic (resources, state machines, damage formulas, save migration) in unit tests; test scenes by instantiating them in the test and asserting on signals and state.
- Tests live under `test/`, files prefixed `test_` (GUT default prefix).
- Run headless (exit code `0` = all passed):

```bash
# GUT
godot --headless -d --path . -s addons/gut/gut_cmdln.gd -gdir=res://test -ginclude_subdirs -gexit

# gdUnit4 (GODOT_BIN points to the Godot executable)
GODOT_BIN=/path/to/godot ./addons/gdUnit4/runtest.sh -a res://test
```

- Import resources once before tests in a fresh checkout/CI: `godot --headless --path . --import`.

---

## 14. Linting and Formatting

- **gdtoolkit 4.x** (`pip3 install "gdtoolkit==4.*"`, or `pipx`) provides `gdlint` and `gdformat` for Godot 4.
- Check formatting without writing: `gdformat --check <dirs>`; lint: `gdlint <dirs>`. Run both on project folders, never on `addons/`.
- Lint configuration lives in `gdlintrc` at the project root (`gdlint --dump-default-config` creates it). Do not disable rules without the user's approval.
- If gdtoolkit fails to parse syntax the installed Godot version accepts, report it — never rewrite valid code to satisfy the tool.

---

## Anti-Patterns

| Anti-pattern | Why |
|---|---|
| Untyped `var`, parameters or returns | Loses compile-time checks; runtime errors appear only on device |
| `get_parent().get_parent()` / long absolute node paths | Breaks as soon as the scene tree changes |
| String-based `connect("signal", ...)` / `call("method")` | No type or name checking |
| Autoload holding references to level nodes | Dangling references after scene changes |
| Mutating a loaded `Resource` shared by many nodes | Changes leak into every user of that resource |
| Deep script inheritance for game entities | Rigid hierarchies; use component scenes |
| Hardcoded keycodes or screen coordinates | Breaks remapping, touch and other resolutions |
| Saving to `res://` or loading `.tres`/objects from save files | Read-only in exports; code execution risk |
| Committing `.godot/`, keystores or passwords | Leaks signing credentials and bloats the repo |
| Export templates of a different Godot version | Export fails or produces a broken build |
| Physics/movement code in `_process()` | Frame-rate dependent behavior |

---

## Quality

- Zero GDScript errors and warnings in the editor's Output/Debugger on project load and on running the main scene.
- `gdformat --check` and `gdlint` clean on project folders.
- All tests pass headless.
- A debug export for Android succeeds (`--export-debug`) whenever a task changes export presets, project settings or Android-specific code; install and run it on a device when one is available, and report when it was not possible.

# AI Rules for Dart + Flutter + shadcn_ui (oito2 Standard)

Strict standard for building Dart/Flutter applications under the oito2 organization. Applies to any new oito2 Flutter app unless the user explicitly overrides a rule for that specific project. Follow every rule below exactly. Do not deviate without an explicit user instruction to do so.

---

## 1. Tech Stack (Mandatory Baseline)

- **Framework**: Flutter, stable channel, SDK `^3.12.2` or newer.
- **Language**: Dart, null-safe, no `dynamic` unless interfacing with untyped JSON.
- **State management**: `flutter_bloc` + `bloc_concurrency`. Do not use `provider`, `riverpod`, `get`, `getx`, `mobx`, or bare `ChangeNotifier`/`ValueNotifier` app-wide state. `equatable` for all Bloc states/events.
- **UI component library**: `shadcn_ui`. Do not build custom design-system components that shadcn_ui already provides.
- **Icons**: `lucide_icons_flutter`.
- **Fonts**: `google_fonts`, resolved through a single configurable `uiFontFamily` string (default `'Outfit'`), never hardcoded to one `GoogleFonts.xyz()` call.
- **Local embedded database** (only if the app needs indexed/queryable local storage): `isar_community` + `isar_community_flutter_libs` + `isar_community_generator` (dev) + `build_runner` (dev).
- **Internationalization**: Flutter's official l10n tooling (`flutter_localizations`, `intl`, `.arb` files, `flutter gen-l10n`). Never a third-party i18n package.
- **Testing**: `flutter_test` + `bloc_test`.
- **Linting**: `flutter_lints`, enabled via `analysis_options.yaml`, no rules disabled without explicit user approval.
- **Formatting**: `dart format .`, default settings, no custom `.dart_format` config.

---

## 2. Project Structure

Use this exact skeleton. Do not invent an alternative top-level layout.

```
lib/
  core/
    constants/     # App-wide constants shared by multiple features
    l10n/           # <lang>_pt.arb (template) + <lang>_en.arb (translation) + generated AppLocalizations
    theme/          # AppThemePreset, palettes/, the ShadThemeData + ThemeData builders
      palettes/     # One file per color-theme family, raw Color constants only
    utils/          # Small pure helper functions, no Flutter widget imports
    widgets/        # Small widgets shared across 2+ features (e.g. a Material-ancestor shim)
  features/
    <feature_name>/
      data/          # Repository implementations, DB models, datasources (filesystem/HTTP/etc.)
      domain/        # Entities + repository interfaces. Pure Dart. No Flutter/Isar/HTTP imports.
      presentation/
        bloc/        # <Feature>Bloc, <Feature>Event, <Feature>State
        screens/      # Full-page widgets
        widgets/      # Reusable widgets scoped to this feature
        utils/        # Pure presentation-layer helpers (formatters, converters)
  app.dart            # ShadApp.custom shell: theme + locale + top-level providers
  main.dart           # Async preload (DB + settings) in parallel, then runApp
test/
  <mirrors lib/ file names, one *_test.dart per unit>
```

Rules:
- Never put feature-specific code in `core/`. Move it to the feature's own folder the moment it's used by only one feature.
- Never let `domain/` import anything from `data/`, `presentation/`, Flutter, Isar, or `http`. It depends on nothing but plain Dart and its own entities.
- One `Bloc` per feature. Add new behavior for an existing feature as a new event + handler on that feature's existing Bloc — never a second parallel state holder for the same feature.
- Extract a screen's private `_build*` helper methods into standalone widget files under `presentation/widgets/` once the screen file exceeds roughly 800–1000 lines, or once a section becomes independently reusable/testable.

---

## 3. UI Framework Rules (shadcn_ui)

- App shell: `ShadApp.custom(theme:, appBuilder: (context) => MaterialApp(...))`. The inner `MaterialApp` supplies `Navigator`, localization delegates, and `supportedLocales`; `ShadToaster`/`ShadSonner` are wired manually inside the inner `MaterialApp`'s `builder:`.
- Prefer a shadcn_ui widget over a Material or Cupertino widget for every user-facing UI element.
- Permitted Material-only exceptions (no shadcn_ui equivalent exists — do not attempt to reinvent these in Shad): the primary free-text `TextField`/`TextFormField` for large text input, `CircularProgressIndicator`, `ListTile`, `ExpansionTile`, and a color picker package (e.g. `flex_color_picker`) for any user-chosen accent/tag color.
- **Mandatory shim**: any `InkWell`, `ListTile`, or other widget requiring a `Material` ancestor, when placed inside a `ShadDialog`, `ShadSheet`, or any other shadcn_ui container, must be wrapped in `Material(type: MaterialType.transparency)`. shadcn_ui containers do not provide a `Material` ancestor themselves; omitting this shim causes a `No Material widget found` crash at the first tap.
- **Dialog vs. Sheet rule**: use `ShadSheet` (`side: ShadSheetSide.right`) for anything with a list, a search box, multiple actions, or content a user might want to scroll through. Use `ShadDialog` only for a short confirmation, a single yes/no choice, or a form with at most 2–3 fields.
- Extract any UI pattern reused by 2 or more dialogs/sheets (e.g. a shared "loading state" layout, a shared footer button row) into a small shared widget/helper under `core/widgets/` or the nearest common feature folder — never copy-paste it.
- `ShadContextMenuItem` must use `fontWeight: FontWeight.w300` by default — set this once, centrally, via `ShadThemeData.contextMenuTheme`, not per call site. Heavier UI fonts (e.g. Outfit) read as bold at the default weight in a right-click menu otherwise.
- Any modal (`ShadDialog`, the `.alert` variant, `ShadSheet`) must use the app's "mainbar" surface tone as background (see §4), not shadcn_ui's default `background` role, via `ShadThemeData.primaryDialogTheme`/`alertDialogTheme`/`sheetTheme`.
- `ShadPopover`-based components (including `ShadContextMenu`) steal keyboard focus on open with no public opt-out as of shadcn_ui 0.56.x — do not use `ShadContextMenu` for a menu anchored to a focused `TextField` (e.g. a text-editor selection context menu). Build a custom Material-based context menu for that specific case instead, following the `Material(type: MaterialType.transparency)` shim rule above for its interactive rows.
- Define one shared `sheetWidth` constant (e.g. `440.0`) in `core/constants/` and reuse it for every `ShadSheet`'s content width — never a magic number per call site.
- `radius: BorderRadius.circular(6.0)` is the standard corner radius for `ShadThemeData` across all presets. Do not vary radius per theme.

---

## 4. Theming System (Mandatory — 13 Named Presets)

This is a required subsystem for every oito2 Dart+Flutter+shadcn_ui app. Implement all 13 presets exactly as specified below; do not ship a subset, and do not invent additional presets without explicit user request.

### 4.1 The preset enum

Define an `AppThemePreset` enum with exactly these 13 values, each carrying an `isDark` flag:

| Preset | `isDark` |
|---|---|
| `everforestLight` | `false` |
| `everforestDark` | `true` |
| `rosePine` | `true` |
| `rosePineMoon` | `true` |
| `rosePineDawn` | `false` |
| `nordDark` | `true` |
| `nordLight` | `false` |
| `dracula` | `true` |
| `alucard` | `false` |
| `catppuccinLatte` | `false` |
| `catppuccinFrappe` | `true` |
| `catppuccinMacchiato` | `true` |
| `catppuccinMocha` | `true` |

There is no user-customizable accent color. Each preset uses its own fixed signature color. Do not add an "accent color picker" feature to the theme system.

### 4.2 Palette source files

One Dart file per color-theme family under `core/theme/palettes/`, containing only raw `Color` constants (no `ShadColorScheme`/`ThemeData` logic) and a doc comment citing the official upstream source. Required files and their exact constants:

**`nord_palette.dart`** — source: nordtheme.com. Nord has no official light variant; `nordLight` is an unofficial derivation (swap Polar Night ↔ Snow Storm roles, use `nord10` instead of `nord8` as primary for contrast on a light background).
```
nord0  #2E3440   nord4  #D8DEE9   nord7  #8FBCBB   nord11 #BF616A
nord1  #3B4252   nord5  #E5E9F0   nord8  #88C0D0   nord12 #D08770
nord2  #434C5E   nord6  #ECEFF4   nord9  #81A1C1   nord13 #EBCB8B
nord3  #4C566A                    nord10 #5E81AC   nord14 #A3BE8C
                                                     nord15 #B48EAD
```

**`dracula_palette.dart`** — source: github.com/dracula/dracula-theme. `alucard` is Dracula's official light variant, same source.
```
Dracula:                          Alucard:
background  #282A36               background  #FFFBEB
currentLine #44475A               currentLine #6C664B
foreground  #F8F8F2               foreground  #1F1F1F
comment     #6272A4               comment     #6C664B
cyan        #8BE9FD               cyan        #036A96
green       #50FA7B               green       #14710A
orange      #FFB86C               orange      #A34D14
pink        #FF79C6               pink        #A3144D
purple      #BD93F9               purple      #644AC9
red         #FF5555               red         #CB3A2A
yellow      #F1FA8C               yellow      #846E15
```

**`everforest_palette.dart`** — source: github.com/sainnhe/everforest `palette.md`, **medium contrast** variant (Everforest's own default).
```
Dark:                              Light:
bgDim  #232A2E   fg     #D3C6AA    bgDim  #EFEBD4   fg     #5C6A72
bg0    #2D353B   red    #E67E80    bg0    #FDF6E3   red    #F85552
bg1    #343F44   orange #E69875    bg1    #F4F0D9   orange #F57D26
bg2    #3D484D   yellow #DBBC7F    bg3    #E6E2CC   yellow #DFA000
bg3    #475258   green  #A7C080    bg4    #E0DCC7   green  #8DA101
bg4    #4F585E   aqua   #83C092                     aqua   #35A77C
grey0  #7A8478   blue   #7FBBB3    grey0  #A6B0A0   blue   #3A94C5
grey1  #859289   purple #D699B6    grey1  #939F91   purple #DF69BA
grey2  #9DA9A0                     grey2  #829181
```
(Light variant has no separate `bg2`: Everforest publishes `bgDim`/`bg2` as the same tone in Light.)

**`rose_pine_palette.dart`** — source: github.com/rose-pine/palette `palette.json`. 3 variants: main (dark), Moon (dark), Dawn (light).
```
Main:                    Moon:                    Dawn:
base    #191724         base    #232136          base    #FAF4ED
surface #1F1D2E         surface #2A273F          surface #FFFAF3
overlay #26233A         overlay #393552          overlay #F2E9E1
muted   #6E6A86         muted   #6E6A86          muted   #9893A5
subtle  #908CAA         subtle  #908CAA          subtle  #797593
text    #E0DEF4         text    #E0DEF4          text    #464261
love    #EB6F92         love    #EB6F92          love    #B4637A
gold    #F6C177         gold    #F6C177          gold    #EA9D34
rose    #EBBCBA         rose    #EA9A97          rose    #D7827E
pine    #31748F         pine    #3E8FB0          pine    #286983
foam    #9CCFD8         foam    #9CCFD8          foam    #56949F
iris    #C4A7E7         iris    #C4A7E7          iris    #907AA9
```

**Catppuccin** — do not hardcode hex values. Depend on the official `catppuccin_flutter` pub package and read colors by role name (`catppuccin.latte`, `.frappe`, `.macchiato`, `.mocha`, each exposing `.mantle`, `.base`, `.text`, `.mauve`, `.surface0`, `.overlay0`, `.red`, `.surface1`, etc.).

### 4.3 Mapping palettes to `ShadColorScheme`

Write one `_composeScheme({...})` private helper that takes the ~8 role colors a family actually needs and derives the rest, so the per-preset mapping never repeats all 20 `ShadColorScheme` fields by hand. Required inputs: `background`, `card`, `foreground`, `primary`, `secondary`, `mutedForeground`, `destructive`, `border`. Optional overrides: `popover`, `muted`, `input`.

Derivation rules inside `_composeScheme` (apply exactly):
- `popover` defaults to `card` when the family has no distinct 3rd surface layer.
- `cardForeground`, `popoverForeground`, `secondaryForeground`, `accentForeground` all reuse `foreground`.
- `accent` always equals `secondary` (same visual role, split into two slots only because `ShadColorScheme` has both).
- `muted` defaults to `secondary`.
- `input` defaults to `border`.
- `primaryForeground` and `destructiveForeground` are computed by a `_contrastingForeground(background)` helper: return black if `background.computeLuminance() > 0.5`, else white. Never hardcode black/white per preset.
- `ring` equals `primary`.
- `selection` is `primary.withValues(alpha: 0.35)` — never a separately chosen color.

When a family's official palette doesn't define a given role (e.g. Dracula has no 3rd surface layer, Alucard has no 2nd surface layer at all), derive it with `Color.lerp` between two existing roles of that same family (e.g. a border blended 12–15% toward `foreground` from `background`/`card`) — document in a comment which roles in that preset are derived vs. official.

Per-family primary color and role assignment (use these unless the user specifies different signature colors):
- Everforest: `primary` = `green`, `destructive` = `red`, `card`/`bg0`, `background`/`bgDim`, `secondary`/`bg1`, `mutedForeground`/`grey1`, `border`/`bg2` (dark) or `bg3` (light).
- Rosé Pine (all 3 variants): `primary` = `pine`, `destructive` = `love`, `card`/`surface`, `background`/`base`, `secondary` & `border`/`overlay`, `mutedForeground`/`muted`.
- Nord: `primary` = `nord8` (dark) / `nord10` (light, more contrast on light bg), `destructive` = `nord11` (both), `card`/`nord1` (dark) or `nord5` (light), `background`/`nord0` (dark) or `nord6` (light).
- Dracula/Alucard: `primary` = `purple`, `destructive` = `red`, `card`/`currentLine` (Dracula) or `background` itself (Alucard, no 2nd layer), `secondary`/`currentLine` (Dracula) or derived via lerp (Alucard).
- Catppuccin (all 4 variants): `primary` = `mauve`, `destructive` = `red`, `card`/`base`, `background`/`mantle`, `secondary` & `border`/`surface0`/`surface1`, `mutedForeground`/`overlay0`.

### 4.4 Chrome surface tinting (sidebar/mainbar)

Do not assign a distinct hardcoded color per preset for structural chrome (a narrow icon sidebar column, a resizable mainbar/file-list panel). Derive both from the theme's own base colors so the relationship holds across all 13 presets automatically:
- `sidebarColor = Color.alphaBlend(onSurface.withValues(alpha: 0.03), background)`.
- `mainbarColor = Color.alphaBlend(Colors.white.withValues(alpha: 0.05), sidebarColor)` — mainbar is lighter than sidebar in every preset, by construction.
- Single documented exception: in the lightest preset(s) where blending white makes the mainbar look washed out, blend `Colors.black.withValues(alpha: 0.04)` instead (darker than sidebar) for that one preset only. Gate this exception on the specific `AppThemePreset` value, not a generic "is light theme" check.
- Provide two entry points to this derivation: one taking a `ThemeData` (for use inside widgets, with `BuildContext`), and one taking a raw `ShadColorScheme` with no `ThemeData` (for use while building the `ShadThemeData` itself, before a `ThemeData` exists — the `ThemeData` is built FROM the `ShadColorScheme`, never the reverse).

### 4.5 Building `ShadThemeData` and Material `ThemeData`

- `ShadThemeData`: `radius: BorderRadius.circular(6.0)`, `textTheme` from `ShadTextTheme.fromGoogleFont` wrapping the configurable `uiFontFamily`, `contextMenuTheme` overridden to `w300` (§3), modal theme backgrounds set to the mainbar tone (§4.4).
- Material `ThemeData` is built FROM the same `ShadColorScheme` (never an independent color choice): `colorScheme.surface`/`onSurface` come from `card`/`cardForeground`, `scaffoldBackgroundColor` from `background`, `dividerColor` from `border`, `useMaterial3: true`.
- Text theme: base off `Typography.material2021().white` (dark) or `.black` (light), wrapped with `GoogleFonts.getTextTheme(uiFontFamily, base)`, with explicit font sizes for `bodyLarge`/`bodyMedium`/`bodySmall` (16/14/12) and `FontWeight.w600` forced on every heading/title/label style (`headlineLarge/Medium/Small`, `titleLarge/Medium/Small`, `labelLarge`) — Material's defaults under-weight headings against most Google Fonts.

### 4.6 Syntax highlighting (code blocks)

Code-block syntax highlighting needs ~10 distinct token roles per theme (far more than `ShadColorScheme`'s handful of generic UI roles), so it is **not** derived from `ShadColorScheme` — it reads the same raw palette files (and `catppuccin_flutter`) directly. Define a `SyntaxHighlightPalette` value type with exactly these roles: `foreground`, `comment`, `keyword`, `string`, `number`, `function`, `type`, `tag`, `attribute`, `constant`, `invalid`. Provide one factory mapping per preset. There is no official syntax theme published by any of these projects for Dart specifically — follow the token-color conventions already established by that theme's most popular VS Code/Neovim port where one exists.

Generate the highlighter theme **in memory** at runtime (e.g. via the `syntax_highlight` package's `HighlighterTheme.fromConfiguration`) from the `SyntaxHighlightPalette` values. Do not ship 13 static per-theme JSON theme assets — building it in memory is synchronous and needs no bundled asset.

### 4.7 Independent print/export style

Any export/print surface (PDF, HTML, or similar rendered document output) must expose its own theme selector, independent from the UI's active `AppThemePreset` — options are "Default" (plain paper, no theme colors) plus all 13 presets. Selecting a UI theme must never silently change the export/print style, and vice versa.

---

## 5. Data Layer Rules

- Use `isar_community` only for data that must be **indexed or queried** (e.g. search, a registry of known top-level entities). Model classes go in `data/models/` with `@Collection`; regenerate with `dart run build_runner build --delete-conflicting-outputs` after any schema change. Commit the resulting `*.g.dart` files (do not gitignore them) — this avoids requiring `build_runner` on a fresh clone.
- Use a plain JSON config file colocated with a user-facing folder/entity (not a DB row) for **per-entity configuration that should travel with that entity** if it's moved, copied, or externally synced (cloud storage, etc.) — e.g. sort order, a tag registry scoped to that entity, per-entity sync settings.
- `domain/` defines repository *interfaces* only. `data/` provides the implementation(s). Never let a Bloc import a `data/` class directly — only the `domain/` interface, injected at construction.
- Never store secrets (OAuth client secrets, tokens) in a committed file. Runtime-provided values (`--dart-define`) or `flutter_secure_storage` only.

---

## 6. Internationalization Rules

- One `.arb` file is the **template** (source of truth — every key is authored here first); every other locale's `.arb` is a **translation**. Document which one is the template in a code comment at the top of `l10n.yaml`'s referenced arb-dir.
- `l10n.yaml` at the project root: `arb-dir: lib/core/l10n`, `template-arb-file: <template>.arb`, `output-class: AppLocalizations`, `output-dir: lib/core/l10n`.
- `flutter: generate: true` in `pubspec.yaml`, so `flutter pub get`/`flutter run` regenerate `AppLocalizations` automatically — never require a manual `flutter gen-l10n` step in the standard workflow.
- Adding a user-facing string: add the key to the template `.arb` first (with an `@key` metadata block if it needs placeholders/ICU plurals), add the same key translated to every other `.arb`, then regenerate.
- **Never** duplicate a key within the same `.arb` file. ARB is parsed as JSON — a duplicate key is silently resolved "last one wins," leaving the earlier definition dead with no error raised. Verify key-count parity across all locale files before merging any l10n change.
- Never hardcode user-facing text in a widget. Every user-facing string goes through `AppLocalizations.of(context)!`.
- Count-dependent strings use ICU plural syntax in the `.arb`, not manual `if (count == 1)` string branching in Dart.

---

## 7. Code Style & Comments

- All code comments (`//`, `///`, `/* */`) are written in English, regardless of the team's spoken language or the app's UI locale.
- A comment describes what the current code does and why (its rationale), as standalone documentation that stays correct forever. It never narrates project history: no "fixed on <date>", no issue/finding/phase numbers, no references to internal tracking documents. That context belongs in the commit message, never in the source.
- User-facing string literals and test descriptions (`test(...)`, `testWidgets(...)`, `group(...)`) are not comments and are exempt from the above — they may be in the app's primary spoken language if that's the team's convention.
- `///` dartdoc comments on public classes/methods/fields are mandatory for anything exported from a feature's `domain/` layer or reused across features.
- Run `dart format .` before every commit. Zero `flutter analyze` warnings/errors beyond pre-existing, explicitly acknowledged ones.

---

## 8. License Header (Mandatory Per-File)

Every hand-written `.dart` file (in `lib/` and `test/`) starts with this exact block, verbatim, before any `import`:

```dart
// Copyright (C) <YEAR>  oito2
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
```

Replace `<YEAR>` with the current year at the time the file is first created; never update it retroactively on later edits to the same file.

**Exempt** (never add the header, it would be overwritten anyway): machine-generated files — `*.g.dart` (Isar/build_runner), `lib/core/l10n/app_localizations*.dart` (`flutter gen-l10n` output), and any other `*_generated.dart`-style file produced by a build step.

The project root must also contain a plain-text `LICENSE` file with the full GPLv3 license text.

---

## 9. Testing Rules

- Every Bloc gets a `bloc_test`-based test file covering each event/handler pair, including error paths.
- Every pure function (in any `utils/` folder) gets direct unit tests — no widget wrapper needed.
- Widget tests use `flutter_test`; wrap the widget under test with the same `ShadApp.custom`/`MaterialApp` shell used in production, plus real `BlocProvider`s (fakes only for the repository layer, not the Bloc itself), so localization/theme resolve exactly as in the app.
- **Known pitfall**: a test that `await`s real filesystem I/O (`dart:io`) directly inside a `testWidgets` body can hang indefinitely in some CI/sandbox environments even when the code under test is correct. Isolate real I/O into a `setUp()`/helper function pattern instead of awaiting it inline inside the test body.
- A temporary golden-file test (`matchesGoldenFile`) is an acceptable, fast way to confirm a visual/layout change in a headless sandbox with no GPU/GUI — delete it once the change is confirmed rather than keeping it as a permanent golden test, unless the user asks to keep golden testing long-term.
- `flutter analyze` and `flutter test` must both be clean before any change is considered complete.

---

## 10. Documentation & Repository Hygiene

- Root of the repository, in English: `README.md` (GitHub-focused: badges, summary, table of contents, what the app does, installation quick-start with prerequisites, how to update, a documentation table, and a `GPLv3 — see [`LICENSE`](LICENSE)` line), `CODE_OF_CONDUCT.md` (Contributor Covenant), `CONTRIBUTING.md` (dev setup pointer + every rule in this document that a contributor needs to know + PR process).
- A Portuguese (Brazil) translation of each of those 3 root files lives under `docs/pt-br/` as `leiame.md`, `codigo-de-conduta.md`, `contribuindo.md` — never duplicated at the repository root.
- `docs/en/` and `docs/pt-br/` each hold the same set of deeper technical docs (architecture, full feature walkthrough, development guide), mirrored 1:1 between the two languages, cross-linked to each other and back to the root README.
- `.gitignore`: keep Flutter's default generated root `.gitignore` plus the platform-specific ones (`android/.gitignore`, `linux/.gitignore`, etc. — these already correctly cover `local.properties`, `key.properties`, `*.keystore`/`*.jks`, and platform ephemeral/build directories). Additionally always gitignore: any AI-assistant tool's local-only state file (e.g. a `*.local.json` settings file), and any project-linking config file for a personal external tool that embeds a machine-specific absolute path.
- `pubspec.lock` is committed (this is an application, not a published package — set `publish_to: 'none'`).
- Never commit a secret, API key, OAuth client secret, or signing keystore. Provide them via `--dart-define` at build time or a secure runtime store.

---

## 11. Supported Platforms

Default baseline: **Linux desktop** and **Android**. Treat Windows and Web as a documented future-roadmap item, not implemented until explicitly requested. Do not scaffold macOS support unless the user explicitly asks for it — it is excluded from the oito2 default platform set.

---

## 12. Optional Subsystem Patterns (Apply Only If the App Needs Them)

- **Cloud sync** (e.g. Google Drive): one `Bloc` per sync target, OAuth via `google_sign_in`/`googleapis`, credentials never persisted in plaintext (`flutter_secure_storage`), conflicts resolved by writing a conflict-copy of the affected file/record rather than silently overwriting either side.
- **Document export** (PDF/DOCX/HTML or similar): every export surface and any live preview must render from the **same** intermediate representation/AST — never let one output format drift from another by using a second, independent parser/renderer for it.
- **Undo/redo layered on a native text-editing widget**: when a custom action (e.g. a toolbar button) needs to be a single atomic undo/redo step but the underlying widget already has its own undo history, snapshot the pre/post state for that action explicitly and check it before falling back to the widget's native undo/redo — do not assume the native history groups discrete UI actions the same way it groups keystrokes.

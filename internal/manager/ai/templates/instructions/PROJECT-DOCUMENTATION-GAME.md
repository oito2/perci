# Documentation Complement — Games

## Overview

Type-specific documentation requirements for **games** (Godot, Unity, Unreal, custom engines). This complements the base standard in `.instructions/PROJECT-DOCUMENTATION.md` — apply both. Like the base, it's a standard for when documentation is created or updated, not a task to run on its own. When the game ships on mobile or desktop, also apply `PROJECT-DOCUMENTATION-MOBILE.md` or `PROJECT-DOCUMENTATION-DESKTOP.md` where they cover the same topic for that platform.

Source every version, platform and command from the project itself — the engine's project file (e.g. `config/features` in `project.godot`), export/build presets (e.g. `export_presets.cfg`), CI workflows, store metadata folders. **Document only what the project has:** no store links, platforms, controls or features that don't exist.

---

## 1. README Specifics

Within the base README shape (Section 4 of the base standard):

- **Header:** the description is the game's **one-line pitch** plus its **genre** (e.g. "A 2D roguelike platformer where ..."). Add an engine badge with the version from the project file, e.g. `[![Godot](https://img.shields.io/badge/Godot-4.7-478CBF?logo=godotengine)](https://godotengine.org)`.
- **Hero media** right after the Overview — a gameplay GIF or screenshot placeholder:
  ```markdown
  ![Gameplay](docs/img/gameplay.gif)
  <!-- TODO: capture a short gameplay GIF -->
  ```
  Keep each `TODO` until a real capture replaces it. Both language trees reference the same files under `docs/img/`.
- **Controls** — a table per input method the game really supports (from the Input Map / input config):

  | Action | Keyboard | Gamepad | Touch |
  |---|---|---|---|
  | Move | `A` / `D` | Left stick | Virtual joystick |
  | Jump | `Space` | `A` | Jump button |

- **Platforms & download** — a list of the platforms the project really exports, with store/download links **only if a published listing exists** (Google Play, itch.io, Steam, GitHub Releases). No placeholder store links.
- **Credits** — a one-line link to the credits file (Section 4).

---

## 2. Engine Version and Running the Project

In `docs/en/getting-started/installation.md` (summarized in the README's Prerequisites & Quick Installation):

| Item | Content |
|---|---|
| Engine | Exact version the project targets (e.g. Godot 4.7.x, standard or .NET build) and where to download it |
| Opening | How to import/open the project (e.g. Godot Project Manager → **Import** → select `project.godot`) |
| First open | Expected asset import time; any required plugin to enable |
| Running | Main scene / run command (e.g. `godot --path .` from the repository root) |
| Large assets | `git lfs install` before cloning, **only if the repository uses Git LFS** (check `.gitattributes`) |

---

## 3. Export and Build per Platform

Document only the platforms the project actually exports (check the export presets and CI). For each: required export templates (same engine version as the editor), SDKs/toolchains, the preset name, the exact command and the output path.

| Platform | Example (Godot) |
|---|---|
| Android AAB (Google Play) | `godot --headless --path . --export-release "Android" build/game.aab` — requires the Gradle build template |
| Android APK (sideload/testing) | `godot --headless --path . --export-debug "Android" build/game.apk` |
| Desktop | `godot --headless --path . --export-release "<preset>" build/<file>` |

- **Android:** list the JDK and Android SDK versions and the editor settings to configure, as the engine's export docs require for the version used.
- **Signing without exposing secrets:** explain how to create the release (upload) keystore and where its path and passwords are configured — the local credentials file (`.godot/export_credentials.cfg` in Godot) or CI environment variables (e.g. `GODOT_ANDROID_KEYSTORE_RELEASE_PATH`, `_USER`, `_PASSWORD`). State that the keystore and credentials stay out of version control. Never write a real path, alias or password in the docs.
- **Version bump:** where to increment the build number (e.g. `version/code` in the Android preset) before each store upload.
- Detail the full environment setup in `docs/en/getting-started/prerequisites.md` when it takes more than a few commands.

---

## 4. Credits and Third-Party Asset Licenses

Game assets often use licenses different from the code (e.g. CC BY 4.0, CC0, OFL for fonts, a store's custom license). The root `LICENSE` covers the code; assets are listed separately:

- Create `CREDITS.md` at the root, in English. Like `LICENSE`, it is a reference file and is not mirrored under `docs/pt-br/`; the Portuguese docs link to it. List every third-party asset the project ships:

  | Asset | Author | Source | License | Files |
  |---|---|---|---|---|
  | Pixel tileset | Author name | URL | CC BY 4.0 | `levels/common/tiles.png` |
  | Font "Example Sans" | Foundry | URL | OFL 1.1 | `common/fonts/` |

- Copy attribution text exactly as each license requires (CC BY needs author, title, source and license link).
- Include engine and plugin licenses that require notice in distributed builds (e.g. the Godot license and its third-party notices).
- When the license of an asset is unknown, mark it `UNKNOWN — verify before release` and tell the user; never guess a license.
- Link `CREDITS.md` from the README and from `docs/en/index.md`.

---

## 5. Store Release Notes

Only if the project publishes to a store:

- Keep release notes in the store's language(s) and character limits (Google Play: per-language "What's new" text), stored where the project already keeps them (e.g. a fastlane metadata folder or `docs/`).
- Derive them from `CHANGELOG.md` when the project has one; never create a CHANGELOG just for this.

---

## 6. Troubleshooting

Add to `docs/en/troubleshooting/common-issues.md` only the issues that apply to the project:

| Symptom | Typical cause / fix |
|---|---|
| "No export template found" / export fails | Export templates of a different engine version — install the templates matching the editor's exact version |
| Android export option missing or Gradle error | Java SDK / Android SDK paths not set in Editor Settings, wrong JDK version, or Android build template not installed |
| Keystore error on release export | Wrong path, alias or password in the credentials file or environment variables; keystore and key passwords differ (Godot requires them equal) |
| Google Play rejects the upload | APK instead of AAB, `version/code` not incremented, or target API below Play's current requirement |
| "ETC2/ASTC texture compression is required for Android export" | Enable **Rendering → Textures → VRAM Compression → Import ETC2 ASTC** in Project Settings |
| Missing assets after clone | Git LFS not installed — run `git lfs install` and `git lfs pull` |

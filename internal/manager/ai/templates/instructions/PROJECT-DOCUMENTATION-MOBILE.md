# Documentation Complement — Mobile Applications

Type-specific documentation requirements for **mobile applications** (Flutter, React Native, native Swift/Kotlin). Complements `.instructions/PROJECT-DOCUMENTATION.md` — apply both; this file lists only what it adds.

Last verified: 2026-11-04

**Which complement:** a non-game mobile app uses this file. A **game** (including a mobile game) uses `.instructions/PROJECT-DOCUMENTATION-GAME.md`, which covers engine export, signing and store builds; take from this file only what GAME does not cover for the platform (e.g. runtime permissions).

Source every version and command from the project itself — `pubspec.yaml`, `package.json`, `android/app/build.gradle(.kts)` (`minSdk`, `targetSdk`, `compileSdk`), `ios/Podfile` / the Xcode project (deployment target), CI workflows.

---

## 1. Environment and SDK Versions (README)

In the README's **Prerequisites & Quick Installation** section, include:

| Item | Example source |
|---|---|
| Framework and version | Flutter/Dart SDK constraint (`pubspec.yaml`), React Native version, Swift/Kotlin version |
| Android | `minSdk` / `targetSdk` / `compileSdk`, required JDK version |
| iOS | Minimum deployment target, required Xcode version (macOS only) |
| Supported platforms | Android, iOS — mark any platform the project doesn't target as not supported |

Detail the environment setup (SDK install, Android Studio/Xcode, `flutter doctor` or equivalent) in `docs/en/getting-started/prerequisites.md`.

---

## 2. Running the App

Document both targets, with the project's real commands:

- **Emulator / simulator:** creating/starting an Android emulator (AVD) and the iOS Simulator; then e.g. `flutter run`, `npx react-native run-android`, `npx react-native run-ios`.
- **Physical device:** enabling Developer options/USB debugging (Android), device trust and signing team (iOS), listing devices (`flutter devices`, `adb devices`), running on a specific one (`flutter run -d <device-id>`).
- Flavors/schemes and environment configuration (e.g. `--flavor`, `--dart-define`, `.env` files), if the project uses them.

---

## 3. Building for Distribution

Document only the artifacts and channels the project actually uses:

| Target | Example command |
|---|---|
| Android APK | `flutter build apk --release` / `./gradlew assembleRelease` |
| Android App Bundle | `flutter build appbundle --release` / `./gradlew bundleRelease` |
| iOS / TestFlight | `flutter build ipa`, then upload via Xcode Organizer, Transporter, or the project's fastlane lane |

Explain release signing (keystore and `key.properties` for Android, certificates/provisioning profiles for iOS) **without committing or exposing secrets** — document where each file goes and that it must stay out of version control.

---

## 4. Other Content

- **Permissions:** list every runtime permission the app requests (from `AndroidManifest.xml` / `Info.plist`) and why.
- **Screenshots:** placeholders under `docs/img/` (`![Home screen](docs/img/home.png)` + `<!-- TODO: capture -->`), one set per supported platform when they differ.
- **Troubleshooting:** common build/run issues that actually apply (Gradle/JDK mismatches, CocoaPods, signing errors).

---

## Quality

In addition to the base standard's checks:

```bash
# Values the docs must match (adapt paths to the project)
grep -nE 'minSdk|targetSdk|compileSdk' android/app/build.gradle*
grep -n 'IPHONEOS_DEPLOYMENT_TARGET' ios/Runner.xcodeproj/project.pbxproj | sort -u
grep -n 'uses-permission' android/app/src/main/AndroidManifest.xml
```

- [ ] SDK levels, deployment target and framework versions in the docs match the project files.
- [ ] Every documented build command was run (or its output path checked in CI) for the platforms the project ships.
- [ ] Every permission requested by the app is listed with its reason; no signing secrets or keystore paths in the docs.

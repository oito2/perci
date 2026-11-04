# Dart + Flutter Development Standard

Standard for Dart and Flutter applications and packages: structure, language rules, UI, state, testing and releases. Apply it whenever Dart code, `pubspec.yaml`, tests, lint configuration or platform build files are written or reviewed. The project's existing conventions (state-management library, UI kit, folder layout) apply first; this file fills the gaps.

Last verified: 2026-11-04

---

## Toolchain & `pubspec.yaml`

- Target **the current stable** Flutter channel and the Dart SDK bundled with it; check with `flutter --version` and the release notes at <https://docs.flutter.dev/release/release-notes>. Never assume a version number from memory.
- `environment: sdk:` is the **minimum** Dart version the code needs (`sdk: ^3.x.0`). Raise it only when the code uses a newer language feature or API.
- If the project pins Flutter (`.fvmrc`, CI matrix, `environment: flutter:`), use that version.
- Dependencies use caret constraints (`^1.4.0`). Commit `pubspec.lock` for applications. Never commit `dependency_overrides` or `path:` dependencies that point outside the repository.
- Lints: `flutter_lints` for apps and Flutter packages, `lints` for pure Dart packages, included from `analysis_options.yaml`. Do not disable a rule without the user's approval; prefer a targeted `// ignore: rule_name` with a reason.

---

## Project Layout

Keep the layout the project already has. For a new app, a feature-first layout with a separate data layer:

```text
lib/
├── main.dart              # bootstrap only: bindings, dependencies, runApp
├── app.dart               # MaterialApp/CupertinoApp, theme, routes, localization
├── features/<feature>/
│   ├── data/              # repositories, DTOs, API/database clients
│   ├── domain/            # models and business rules (no Flutter imports)
│   └── ui/                # screens, widgets, view models/controllers
├── core/                  # shared widgets, theme, utilities, error types
└── l10n/                  # .arb files, when the app is localized
test/                      # mirrors lib/
integration_test/          # end-to-end tests on a device
```

- File names `snake_case.dart`; types `UpperCamelCase`; members `lowerCamelCase` (Effective Dart).
- One public widget/class per file, unless small private helpers belong to it.
- Import inside the package with `package:` imports; never reach into another package's `src/`.

---

## Language Rules

- **Null safety:** no `!` unless the invariant is local and obvious; prefer flow analysis, `?.`, `??` and early returns. Use `late` only when initialization is guaranteed before use.
- **Types:** no `dynamic` except at untyped boundaries (decoded JSON), converted to typed models immediately. Avoid `Object?` casts scattered through code.
- **Immutability:** `final` locals and fields by default; `const` constructors for value types and widgets; models are immutable with `copyWith`. Expose collections as unmodifiable views.
- **Value equality:** implement `==`/`hashCode` (by hand, `equatable`, or code generation such as `freezed`) only where comparison is needed, consistently across the project.
- Use Dart 3 features where they clarify: `sealed` classes with exhaustive `switch` for states and results, records for small multi-value returns, patterns for JSON destructuring.

---

## Widgets

- Compose small widgets instead of large `build` methods; extract a **widget class**, not a helper method returning `Widget`, so it rebuilds independently and can be `const`.
- Mark every widget and constructor call that can be `const` as `const`.
- `build` is pure and cheap: no I/O, no object creation that should be cached, no side effects. Start work in `initState`, in the state layer, or in response to events.
- Pass data down through constructors; avoid reaching for global singletons from widgets.
- Use `Key`s (`ValueKey`) for list items whose order or identity changes.
- Responsive layout through `LayoutBuilder`, `MediaQuery.sizeOf(context)` and flexible widgets, never fixed device sizes.

---

## State Management

- Use the solution the project already uses (e.g. `ChangeNotifier`/`ValueNotifier` with `ListenableBuilder`, Provider, Riverpod, Bloc). For a new project, ask the user (selectable options, with a recommendation) and use **one** approach consistently.
- Business logic lives outside widgets — in view models, controllers, blocs/cubits or notifiers that can be unit-tested without a widget tree.
- Data access goes through repositories; the UI never calls HTTP clients, databases or platform channels directly.
- Separate ephemeral UI state (a text field, an animation) kept in `StatefulWidget` from app state shared across screens.

---

## Async, Streams & Disposal

- Every `Future` is awaited or explicitly handed off with `unawaited(...)` (enable the `unawaited_futures` lint).
- After an `await` inside a `State`, check `if (!mounted) return;` before using `context` or calling `setState` (for a `BuildContext` parameter: `context.mounted`).
- Dispose what you create: `AnimationController`, `TextEditingController`, `ScrollController`, `FocusNode`, `StreamSubscription`, `Timer`, `ChangeNotifier`. Cancel subscriptions in `dispose`.
- Prefer `StreamBuilder`/`FutureBuilder` only with a future/stream created outside `build` (e.g. in `initState`); creating it in `build` restarts the work on every rebuild.
- Move CPU-heavy work (large JSON parsing, image processing) off the UI isolate with `Isolate.run` / `compute`.

---

## Error Handling

- Throw typed exceptions from the data layer (`class NetworkException implements Exception`) and convert them to UI-facing states or `Result`-like types at the boundary; never let raw exceptions reach widgets.
- Catch specific types (`on SocketException`); never swallow errors with an empty `catch`.
- Install global handlers in `main`: `FlutterError.onError` for framework errors and `PlatformDispatcher.instance.onError` for uncaught async errors, forwarding to the project's logging/crash reporting.
- Show user-readable messages; keep stack traces, URLs with tokens and server internals in logs.
- Never log secrets or personal data. Use `debugPrint`/a logger, not `print` (the `avoid_print` lint).

---

## Theming

- Define the look once in `ThemeData` built from a `ColorScheme` (e.g. `ColorScheme.fromSeed`) for light and dark modes; read values through `Theme.of(context)` / `TextTheme`, never hard-coded colors or font sizes in widgets.
- Custom design tokens go in a `ThemeExtension`.
- If the project uses a UI kit or design system, follow it instead of mixing a second one.

---

## Accessibility

- Every icon-only button has a `tooltip` or `Semantics(label: ...)`; decorative images use `excludeFromSemantics: true`.
- Support text scaling: no fixed-height containers around text, test with large system font sizes; never clamp `TextScaler` to 1.0.
- Contrast at least 4.5:1 for body text and 3:1 for large text and UI components; touch targets at least 48×48 dp (Material guideline).
- Order and group content for screen readers (`MergeSemantics`, `Semantics` roles) and check with TalkBack/VoiceOver.
- Widget tests can assert guidelines: `await expectLater(tester, meetsGuideline(androidTapTargetGuideline));` (also `textContrastGuideline`, `labeledTapTargetGuideline`).

---

## Localization

When the app is localized (or will be), use Flutter's built-in `gen-l10n` with `flutter_localizations` and `intl`:

- `generate: true` in `pubspec.yaml`, an `l10n.yaml` configuring the `.arb` directory and template file; generated code is produced by `flutter gen-l10n` (or on `flutter run`/`build`).
- Every user-facing string comes from `AppLocalizations.of(context)`; placeholders, plurals and selects are defined in the `.arb` file, never concatenated.
- Format dates, numbers and currency with `intl` using the active locale.

---

## Performance

- Long or unbounded lists use `ListView.builder` / `SliverList` (lazy), not `ListView(children: [...])` with all items.
- Limit rebuild scope: `const` widgets, smaller widgets, selectors in the state solution, `RepaintBoundary` around frequently changing regions when profiling shows repaint cost.
- Avoid `Opacity`, `ClipPath` and `saveLayer`-triggering effects on animated subtrees when a cheaper alternative exists (`FadeTransition`, `AnimatedOpacity`).
- Images: size them for display (`cacheWidth`/`cacheHeight`, resized assets per density `2.0x/`, `3.0x/`), use a placeholder and an error builder for network images.
- Measure in **profile mode** (`flutter run --profile`) on a real device with DevTools; never judge performance in debug mode.

---

## Platform Channels & Plugins

- Prefer an existing, maintained plugin from pub.dev over custom native code.
- Custom native calls use `MethodChannel` (request/response) or `EventChannel` (streams) with a unique, reverse-DNS channel name; for larger APIs generate type-safe bindings with `pigeon`.
- Wrap channels in a Dart class behind an interface so the rest of the app (and tests) never touch the channel directly.
- Handle `PlatformException` and `MissingPluginException`; validate arguments on both sides.

---

## Dependencies

- Each new package needs a reason: maintained, good pub.dev score, compatible license, supports the project's platforms.
- Check with `flutter pub outdated`; upgrade deliberately (`flutter pub upgrade <pkg>`, `--major-versions` only on purpose), one logical upgrade per change, with tests run.
- Generated code (`build_runner`): follow the project's policy on committing generated files; regenerate with `dart run build_runner build --delete-conflicting-outputs` after changing annotated sources.

---

## Testing

| Kind | Package / API | Use for |
|---|---|---|
| Unit | `package:test` / `flutter_test` | Models, repositories, view models, business rules |
| Widget | `flutter_test` (`testWidgets`, `WidgetTester`, `find`) | A widget's rendering and interactions |
| Golden | `matchesGoldenFile` | Visual regressions of stable components |
| Integration | `integration_test` (SDK) | End-to-end flows on a device/emulator |

- Tests mirror `lib/` under `test/`; names describe behavior (`'shows error when login fails'`).
- Mock dependencies with **mocktail** (no code generation) or **mockito** (`@GenerateNiceMocks` + `build_runner`) — whichever the project already uses. Mock at the repository/service boundary, not Flutter internals.
- No real network, file system outside temp dirs, or platform channels in unit/widget tests.
- Use `pumpAndSettle` only when animations must finish; prefer `pump(duration)` for deterministic timing. Never use real delays.
- Goldens differ by platform and fonts: generate and compare them on one platform (usually the CI OS); update with `flutter test --update-goldens` and review the image diff.
- Every bug fix gets a regression test that fails without the fix.

---

## Releases

- **Android:** release signing via a keystore referenced from `android/key.properties` (read in `build.gradle(.kts)` `signingConfigs`); keystore and `key.properties` stay out of version control. Build with `flutter build appbundle --release` for Google Play (APK only for sideloading).
- **iOS:** signing team, certificates and provisioning profiles configured in Xcode (or the project's fastlane setup); build with `flutter build ipa --release`. Requires macOS.
- Optionally obfuscate: `--obfuscate --split-debug-info=build/symbols/<version>` (the flags go together); keep the symbol files per release to decode stack traces with `flutter symbolize`.
- Version comes from `version: x.y.z+build` in `pubspec.yaml`; increment the build number for every store upload.
- Never embed secrets in the app binary (`--dart-define` values are extractable); keep them on a server.

---

## CI

Run on every push and pull request, with the Flutter version pinned:

1. `flutter pub get`
2. `dart format --output=none --set-exit-if-changed .`
3. `flutter analyze`
4. `flutter test` (optionally `--coverage`)
5. A build for each shipped platform (e.g. `flutter build appbundle --debug`, `flutter build ios --no-codesign` on macOS runners)

Use least-privilege workflow permissions and provide signing material only as CI secrets.

---

## Anti-Patterns

| Anti-pattern | Instead |
|---|---|
| Business logic and HTTP calls inside `build` or widgets | View model/controller + repository |
| Helper methods returning large widget trees | Extracted `const`-capable widget classes |
| Creating a `Future`/`Stream` inside `build` | Create it once in `initState` or the state layer |
| Using `context` after `await` without a `mounted` check | `if (!mounted) return;` |
| Forgetting to dispose controllers and subscriptions | Dispose in `dispose()` |
| Hard-coded colors, text sizes and strings | `ThemeData`, `TextTheme`, `AppLocalizations` |
| `ListView(children: hugeList)` | `ListView.builder` |
| Two state-management libraries for the same job | One approach, consistently |
| Secrets in Dart code or `--dart-define` | Server-side secrets |
| Real network calls in tests | Mocks/fakes at the repository boundary |

---

## Quality

Run before reporting any Dart/Flutter change as done:

```bash
dart format --output=none --set-exit-if-changed .
flutter analyze
flutter test
flutter test integration_test   # with a device or emulator available
```

For pure Dart packages use `dart analyze` and `dart test`. When a device was not available for integration tests, say so in the report.

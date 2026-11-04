# Landing Page Design & CRO Standard

Standard for designing and building **landing pages** — a single page focused on one conversion goal — that convert and have a distinctive visual identity. Apply it when a landing page is created or redesigned. Also apply `.instructions/WEBSITE.md` when the project has it: it governs SEO, forms, privacy/consent, fonts, images, hosting and the site-wide quality checks, which this file does not repeat.

Last verified: 2026-11-04

> **Rule:** Never invent content presented as fact — client logos, testimonials, ratings, user counts, media coverage, guarantees or prices. Use what the user provides; otherwise insert a clearly marked placeholder (e.g. `[TESTIMONIAL — provided by the client]`) and list every placeholder in your final report.

---

## 1. Existing Project First

- Follow the project's **existing brand, design system and stack** first: its colors, fonts, components, icon set, CSS approach and build tooling. Don't introduce a framework, CSS library or font the project doesn't use without asking.
- The visual direction below (§2) applies only to what the project leaves open — a new brand, or a page explicitly meant to look different.

---

## 2. Visual Direction

### Discovery

Use the brief first. Only when it lacks the information, ask the missing questions — all at once, following the question protocol in `AGENTS.md`:

1. **Reference:** a real-world place or object the brand would be (e.g. "a 1970s recording studio", "surgical instruments").
2. **Emotion:** the one feeling in the first 3 seconds (Calm, Energized, Curious, Trusted, Rebellious, Sophisticated, Nostalgic, Confident).
3. **Collision:** two unexpected influences to combine (e.g. "medical packaging + skateboard graphics").
4. **Anti-patterns:** 2–3 design choices to avoid.

### Spec

Before coding, output a short spec and get it confirmed when the direction is new:

```text
VIBE: [2–3 word name] — REFERENCE / EMOTION / COLLISION / ANTI-PATTERNS
COLORS: --color-bg, --color-surface, --color-text-main, --color-accent (hex + reason; contrast checked)
TYPOGRAPHY: --font-display, --font-body (licensed for web use), voice
LAYOUT & MOTION: density, shapes, one signature element, motion level, one signature animation
```

### Avoid generic output

- Derive palette, type and layout from the reference — don't fall back to the same template for every page (e.g. purple gradients, default system UI fonts for display text, generic SaaS illustrations) unless the brand uses them.
- Declare the chosen tokens as CSS custom properties in `:root` and use only them in components.

---

## 3. Conversion Copy

- **Hero headline (H1):** benefit-driven, roughly 8–12 words. Useful formulas: *Outcome + without pain*, *Action verb + promise + timeframe*, *Old way is dead → new way*.
- **Subheadline:** 1–2 sentences on *how* the product delivers the promise.
- **Primary CTA:** one action-oriented label (e.g. "Start free trial", not "Submit"), the same goal throughout the page.
- **Friction reducer** under the CTA (e.g. "No credit card required") — only when it is true.

### Page blueprint

Adapt to the offer; drop sections that have no real content:

1. **Hero** — H1, subheadline, primary CTA, friction reducer, hero visual, social proof (real or placeholder).
2. **Social proof** — client logos, ratings or press, only with real material.
3. **Problem → solution / features.**
4. **How it works** — about three numbered steps.
5. **Objections & FAQ** — `<details>`/`<summary>` accordion; guarantee only if the offer has one.
6. **Final CTA** — repeated value proposition.
7. **Footer**, plus an optional sticky CTA bar on mobile.

---

## 4. Implementation Rules

- **Hero/LCP:** no heavy background images or videos in the hero; use CSS, SVG or an optimized image with `fetchpriority="high"` and explicit dimensions (image rules in WEBSITE.md).
- **Fonts:** self-hosted WOFF2 with `font-display`, at most two families (details in WEBSITE.md).
- **Motion:** every non-essential animation (entrance, marquee, float) runs only under `@media (prefers-reduced-motion: no-preference)`; marquees and auto-moving content longer than 5 seconds have a pause control (WCAG 2.2.2).
- **Sticky mobile CTA:** it must not cover the focused element or content the user is reading — reserve space for it (e.g. `scroll-padding-bottom` and bottom padding equal to its height) so keyboard focus is never hidden behind it (WCAG 2.4.11).
- **Accessibility:** WCAG 2.2 AA contrast (4.5:1 text, 3:1 large text and UI components), visible `:focus-visible` styles, one `<h1>`, ordered headings, `<button>` for actions and `<a>` for navigation, CTA targets at least 44×44 CSS px.
- **Layout:** no horizontal overflow down to 320 CSS px — fix the overflowing element instead of `overflow-x: hidden` (which also breaks `position: sticky`).
- **Forms and tracking:** a lead form or analytics/pixels follow WEBSITE.md (labels, LGPD notice, consent before non-essential trackers).

---

## Quality

In addition to WEBSITE.md's quality checks (when the project has it):

- [ ] Brand, design system and stack of the project respected (or the spec confirmed for a new direction).
- [ ] Every placeholder listed in the final report; no invented proof, prices or guarantees.
- [ ] Lighthouse on the built page (mobile): `lighthouse <url> --form-factor=mobile --output=html --output-path=./lighthouse.html` — check LCP, CLS and accessibility.
- [ ] axe: `axe <url> --tags wcag2a,wcag2aa,wcag21aa,wcag22aa --exit`.
- [ ] Manual pass at 320 px, tablet and desktop widths: keyboard-only navigation with the sticky CTA visible, 200% zoom, `prefers-reduced-motion: reduce` enabled.

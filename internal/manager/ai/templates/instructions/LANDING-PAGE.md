# Landing Page Design & CRO Standard

## Overview

Build landing pages that convert AND captivate. This standard combines high-converting copywriting architecture with distinctive, non-generic visual design to create landing pages that stand out in an AI-saturated world.

**The Goal:** High-converting, production-ready landing pages that avoid standard AI slop patterns.

> **Rule:** Never invent content presented as fact — client logos, testimonials, ratings, user counts, media coverage, guarantees or prices. Use what the user provides; otherwise insert a clearly marked placeholder (e.g. `[TESTIMONIAL — provided by the client]`) and list every placeholder in your final report.

---

## MANDATORY: Vibe Discovery (Do This First)

BEFORE writing any code or layout, you MUST run the Vibe Discovery process to generate a unique aesthetic direction.

> **Rule:** No two landing pages should look alike, even for similar products.

### Step 1: Gather Context (Ask These Questions)

If the user provides limited context, ask these 4 questions (offer selectable options where the question has a fixed set, e.g. The Emotion). If they provided a brief, synthesize the answers from their input:

1. **The Reference:** What's one specific real-world place or object this brand would be? *(e.g., A Tokyo convenience store at 2am, a 1970s recording studio, a brutalist parking garage, surgical instruments).*
2. **The Emotion:** What ONE emotion should someone feel in the first 3 seconds? *(Pick ONE: Calm, Energized, Curious, Trusted, Rebellious, Sophisticated, Nostalgic, Confident).*
3. **The Collision:** Pick TWO unexpected influences to collide. *(e.g., "medical packaging + skateboard graphics", "luxury hotel + punk zine", "NASA mission control + kindergarten").*
4. **Anti-Patterns:** Name 2-3 specific design choices to actively avoid. *(e.g., "No purple gradients", "No generic SaaS illustration style", "No rounded friendly shapes").*

### Step 2: Invent The Vibe & Write The Spec

Before coding, generate and output this exact spec:

```text
VIBE NAME: [Invent a 2-3 word name, e.g., "Shinjuku Runway"]
REFERENCE: [Place/Object from Q1] | EMOTION: [From Q2] | COLLISION: [From Q3]
ANTI-PATTERNS: [From Q4]

COLORS & PALETTE:
- Name: [Evocative Name, e.g., "Platform Edge"]
- --color-bg: [hex] - [why]
- --color-surface: [hex] - [why]
- --color-text-main: [hex] - [why]
- --color-accent: [hex] - [why]

TYPOGRAPHY:
- --font-display: [Specific Font Name via Fontshare/Google Fonts]
- --font-body: [Specific Font Name]
- Voice: [Describe the character/tone]

LAYOUT & MOTION:
- Density: [sparse / balanced / dense] | Shapes: [sharp / rounded / organic]
- Signature Element: [One unusual layout choice derived from collision]
- Motion Level: [subtle / kinetic / deliberate]
- Signature Animation: [One specific defining animation]

WILDCARD:
- [One unexpected detail that breaks harmony to create novelty]

```

### Freshness & Anti-Convergence Rules

* **No Color Memory:** Generate colors fresh from the reference, never reuse palettes from previous runs.
* **Font Rotation:** Never default to Inter, Roboto, or Nunito for body text unless explicitly forced. Explore Google Fonts or Fontshare (`Clash Display`, `Syne`, `Satoshi`, `Outfit`, `Newsreader`, `Space Grotesk`).
* **Icon Constraint:** Avoid plain 24px Lucide outlines. Use **Iconify Solar**, **Phosphor**, **Heroicons**, or Custom SVGs with styled strokes/duotone fills.
* **No Generic Purple Gradients:** Default to deep navy/electric accents, warm monochromatic neutrals, or dark mode with neon pops.

---

## Conversion & Copywriting Framework (CRO)

A landing page must convince, not just look pretty. Every page must strictly follow this conversion architecture:

### 1. Headline Formulas (Choose One for the Hero)

* **Outcome + Without Pain:** `[Achieve Desired Result] + without [Major Friction/Fear]`
* **Direct Command:** `[Action Verb] [Big Promise] in [Timeframe]`
* **Category Disruptor:** `The [Old Way] is dead. Welcome to [New Way].`

### 2. Copy Hierarchy Standard

* **Headline (H1):** Clear, benefit-driven hook (max 8-12 words).
* **Subheadline:** 1-2 sentences explaining *HOW* the product delivers the headline's promise.
* **Primary CTA:** Action-oriented text (e.g., *"Claim Your Access"* instead of *"Submit"*).
* **Risk Reversal / Friction Reducer:** Small text under CTA (e.g., *"No credit card required • 2-min setup"*).

---

## Technical & Architecture Rules

### 1. CSS Design System Variables Setup

In your output code, ALWAYS declare the vibe as CSS Variables inside the `:root` block first:

```css
:root {
  --color-bg: #fafaf8;
  --color-surface: #f5f5f0;
  --color-text-main: #1a1a1a;
  --color-accent: #e60012;
  
  --font-display: 'Clash Display', sans-serif;
  --font-body: 'Plus Jakarta Sans', sans-serif;
}

```

### 2. Performance & Web Vitals

* **LCP Optimization:** Never use unoptimized, heavy background images on the Hero section. Use CSS gradients, vectors, or lightweight WebP images with `fetchpriority="high"`.
* **Font Performance:** Load fonts with `font-display: swap` — the `&display=swap` URL parameter on Google Fonts / Fontshare imports, or `font-display: swap;` in your own `@font-face` rules.
* **Clean DOM:** Avoid unnecessary nested `div` wrappers solely for styling; keep the DOM tree shallow.

### 3. Accessibility

* **Contrast:** text and CTAs meet WCAG 2.2 AA (4.5:1 for body text, 3:1 for large text and UI components).
* **Reduced motion:** wrap every non-essential animation (entrance, marquee, float, beam) in `@media (prefers-reduced-motion: no-preference)`, or disable it under `prefers-reduced-motion: reduce`.
* **Keyboard:** every interactive element is reachable by keyboard and has a visible `:focus-visible` style; the FAQ accordion uses `<details>`/`<summary>` or proper ARIA.
* **Semantics:** one `<h1>`, ordered headings, `alt` text on informative images, `<button>` for actions and `<a>` for navigation.

---

## Section Composition Guide

### The 50% Rule

Spend 50% of creative and technical focus on the **Hero Section**. If the Hero doesn't hook the user, the rest of the page is irrelevant.

### Required Page Blueprint

```
1. Hero Section (Primary Focus)
   ├── Value Proposition (H1 + Subheadline)
   ├── Primary CTA + Risk Reversal Badge
   ├── Social Proof (Logo Marquee / User Rating / Trust Badges — real or placeholder, never invented)
   └── Interactive/Visual Hero Asset (Split, Centered, or Asymmetrical)

2. Social Proof / Validation
   └── Marquee animation with client logos or media coverage

3. Problem vs. Solution (or Bento Grid Features)
   └── Interactive cards, hover states, or micro-animations

4. How It Works (3 Step Process)
   └── Numbered steps (01, 02, 03) to build clarity and lower perceived effort

5. Risk Reversal / Objections & FAQ
   └── Accordion FAQ + guarantee badge (only if the offer has one)

6. Final CTA Section
   └── Repeated value proposition + high-contrast action focus

7. Footer + Sticky Mobile CTA
   └── Clean footer links + fixed bottom CTA bar visible on mobile scroll

```

---

## Design & Animation Vocabulary

When prompt engineering or generating components, apply these specific motion primitives:

* **Entrance Animations:** `fade-in`, `blur-in` (opacity + filter blur), `slide-in`, `scale-in`, `stagger` (sequential child reveals).
* **Continuous Animations:** `marquee` (infinite horizontal scroll for social proof), `beam` (light traveling along card borders), `float` (gentle ambient Y movement).
* **Interactive Micro-interactions:** `hover-lift` (`translateY(-4px)` + soft shadow expand), `hover-glow` (border-color transition), `hover-reveal`.
* **Decorative Touches:** Vertical grid lines, background grain overlays, curved SVG connector lines ("noodles"), step counters (`01`, `02`).

---

## Implementation Workflow

1. **Phase 1: Vibe & Copy Spec:** Run the Vibe Discovery, select the Copy Headline Formula, and output the Spec block.
2. **Phase 2: Hero Build:** Construct the Hero layout, CSS variable definitions, primary CTA, and main visual asset.
3. **Phase 3: Section Assembly:** Add section-by-section, keeping typography, CSS variable usage, and animation timing unified.
4. **Phase 4: Responsive & Mobile Check:** Ensure no horizontal overflow — fix the element that overflows instead of hiding it with `overflow-x: hidden` (which also breaks `position: sticky`), test touch targets (minimum 44x44px for CTAs), and enable the Sticky Mobile CTA Bar.
5. **Phase 5: Polish Check:** Verify font imports, icon consistency, accessibility (§3), performance, and that every placeholder is listed for the user.

# Website Development Standard

## Overview

Standards for building and maintaining **websites** — multi-page sites, institutional/corporate sites, blogs and content sites. Stack-neutral: the rules apply whether the site is hand-written HTML, a static site generator, or a framework.

- **Landing pages** (a single conversion-focused page) follow `.instructions/LANDING-PAGE.md`. When a website also contains a landing page, apply **both**: LANDING-PAGE.md for that page's design and copy, this file for everything site-wide (structure, SEO, hosting, quality checks).
- Web *applications* (dashboards, authenticated apps, APIs) are out of scope beyond what overlaps here.

> **Rule:** Never invent content presented as fact — addresses, phone numbers, team members, clients, testimonials, prices, legal/company identifiers. Use what the user provides; otherwise insert a clearly marked placeholder (e.g. `[CONTACT E-MAIL — provided by the client]`) and list every placeholder in your final report.

---

## 1. Technology

- **Follow the project's existing stack** — its framework, static site generator, CSS approach, package manager and folder layout. Don't introduce a framework, CSS library or build tool the project doesn't use without asking.
- For a new site with no stack chosen, ask the user (selectable options, with a recommendation). Prefer the simplest option that fits: static HTML or a static site generator for content sites; a server or framework only when the site needs server-side features.
- Ship working HTML first: content and navigation must not depend on client-side JavaScript to render, unless the project's stack is built that way.

---

## 2. Semantic HTML and Document Structure

- Every page declares `<!doctype html>`, `<html lang="...">`, `<meta charset="utf-8">` and `<meta name="viewport" content="width=device-width, initial-scale=1">`.
- Use landmarks: `<header>`, `<nav>`, `<main>` (one per page), `<footer>`, plus `<article>`/`<section>`/`<aside>` where they fit.
- One `<h1>` per page and headings in order — no skipped levels for styling.
- `<a>` for navigation, `<button>` for actions; lists for lists, `<table>` only for tabular data (with `<th>` and `<caption>`).
- Shared parts (header, navigation, footer) come from one template/partial/component — never copy-pasted per page.
- Provide a useful **404 page** and keep URLs stable; when a URL changes, add a redirect where the host supports it.

---

## 3. Accessibility — WCAG 2.2 Level AA

Target **WCAG 2.2, Level AA** (W3C Recommendation). Key success criteria to check on every page:

| Area | Requirement (criterion, level) |
|---|---|
| Page title | Every page has a descriptive `<title>` (2.4.2, A) |
| Language | `lang` on `<html>` (3.1.1, A); `lang` on passages in another language (3.1.2, AA) |
| Skip link | A mechanism to bypass repeated blocks, e.g. a "Skip to content" link to `<main>` (2.4.1, A) |
| Text contrast | At least 4.5:1, or 3:1 for large text (1.4.3, AA) |
| Non-text contrast | UI components and meaningful graphics at least 3:1 against adjacent colors (1.4.11, AA) |
| Keyboard focus | Focus indicator visible (2.4.7, AA) and not hidden by sticky headers, banners or cookie bars (2.4.11, AA) |
| Resize and reflow | Text resizable to 200% (1.4.4, AA); no two-dimensional scrolling at a 320 CSS px wide viewport (1.4.10, AA) |
| Target size | Pointer targets at least 24×24 CSS px, or spaced as the exceptions allow (2.5.8, AA) |
| Dragging | Any drag interaction has a single-pointer alternative (2.5.7, AA) |
| Forms | Labels or instructions (3.3.2, A); errors identified in text (3.3.1, A) with suggestions (3.3.3, AA); `autocomplete` on personal-data fields (1.3.5, AA) |
| Consistent help | Help/contact links appear in the same relative place across pages (3.2.6, A) |
| Authentication | No cognitive test (e.g. a puzzle CAPTCHA) without an alternative (3.3.8, AA) |
| Custom widgets | Name, role and state programmatically determinable (4.1.2, A) — prefer native elements over ARIA |

Also: meaningful `alt` text on informative images (`alt=""` on decorative ones), captions/transcripts for media, and non-essential motion disabled under `prefers-reduced-motion: reduce`. Automated tools catch only part of these — also check keyboard-only navigation and zoom manually.

---

## 4. Responsive Layout

- Mobile-first CSS; layouts built with flexbox/grid and relative units, not fixed pixel widths.
- No horizontal page overflow at any width down to 320 CSS px — fix the overflowing element instead of hiding it with `overflow-x: hidden`.
- Test at least a phone, a tablet and a desktop width, plus 200% browser zoom.
- Respect user preferences: `prefers-reduced-motion`, and `prefers-color-scheme` when the site offers a dark theme.

---

## 5. Performance — Core Web Vitals

The Core Web Vitals and their "good" thresholds, measured at the **75th percentile** of page loads (mobile and desktop separately):

| Metric | Measures | Good |
|---|---|---|
| **LCP** — Largest Contentful Paint | Loading | ≤ 2.5 s |
| **INP** — Interaction to Next Paint | Responsiveness (replaced FID in 2024) | ≤ 200 ms |
| **CLS** — Cumulative Layout Shift | Visual stability | ≤ 0.1 |

Budgets and practices:

- Agree page-weight and request budgets with the user for the heaviest templates (home, article) and check them in the quality step (§13).
- Avoid render-blocking resources: inline or minimize critical CSS, load scripts with `defer` (or `type="module"`), and drop unused CSS/JS.
- Third-party scripts (analytics, embeds, chat widgets) load only when needed and after consent where consent applies (§10).
- Reserve space for anything that loads late (images, embeds, ads, banners) so it doesn't shift the layout.

---

## 6. Images

- **Modern formats:** serve AVIF and/or WebP, with a fallback through `<picture>` when older formats are still needed; SVG for icons and logos.
- **Dimensions:** always set `width` and `height` on `<img>` (or a CSS `aspect-ratio`) so the browser reserves space — this prevents layout shift.
- **Lazy loading:** `loading="lazy"` only for images **outside** the initial viewport. Never lazy-load the LCP image or other above-the-fold images.
- **LCP image:** `fetchpriority="high"` on the hero/LCP image (a hint — use it on one or two critical images, not everywhere).
- **Responsive sources:** `srcset` with width descriptors plus `sizes`, so the browser picks the right file:
  ```html
  <img src="hero-800.jpg"
       srcset="hero-480.jpg 480w, hero-800.jpg 800w, hero-1600.jpg 1600w"
       sizes="(min-width: 60rem) 50vw, 100vw"
       width="1600" height="900" alt="Describe the image">
  ```
- Compress and resize at build time; never ship originals straight from a camera or design tool.

---

## 7. Fonts

- Prefer **WOFF2**; limit the number of families and weights (a variable font when several weights are needed; system fonts are an option for body text).
- Set `font-display` in every `@font-face` (or `&display=swap` on hosted-font URLs): `swap` for text that must appear in the brand font, `optional` when avoiding layout shift matters more.
- **Preload** only the one or two fonts used above the fold, with `crossorigin` (font requests are CORS):
  ```html
  <link rel="preload" href="/fonts/brand.woff2" as="font" type="font/woff2" crossorigin>
  ```
- When self-hosting, subset the fonts (`unicode-range`) to the scripts the site uses. Check the font's license allows web embedding.

---

## 8. SEO

- **Titles and descriptions:** a unique, descriptive `<title>` and `<meta name="description">` per page — never the same text on every page.
- **Canonical:** `<link rel="canonical" href="https://...">` in the `<head>`, with an **absolute** URL, on every indexable page. Don't use `robots.txt` for canonicalization.
- **Open Graph:** `og:title`, `og:type`, `og:image` and `og:url` (the required set), plus `og:description`, `og:image:alt` and `og:locale` (format `pt_BR`).
- **`sitemap.xml`:** absolute URLs of every indexable page, generated by the build when the stack supports it; at most 50,000 URLs / 50 MB uncompressed per file (use a sitemap index beyond that). `<lastmod>` only when it's accurate; `<priority>` and `<changefreq>` are ignored by Google.
- **`robots.txt`** at the site root, with a `Sitemap:` line pointing to the sitemap's absolute URL. `robots.txt` does **not** keep a page out of search results — use `<meta name="robots" content="noindex">` for that.
- **Structured data:** JSON-LD (schema.org) when relevant — e.g. `Organization`, `BreadcrumbList`, `Article`/`BlogPosting`, `Event`. Mark up only content that is visible on the page, and validate it (§13).

---

## 9. Forms

- Every field has a visible `<label>` (placeholders are not labels); group related controls with `<fieldset>`/`<legend>`; use the right `type` and `autocomplete` values.
- Validate on the server as well as in the browser. Show errors in text, next to the field and summarized at the top, and keep what the user typed.
- **Spam protection without dark patterns:** server-side validation and rate limiting, a honeypot field, or an accessible/invisible challenge. No puzzle CAPTCHA without an alternative (WCAG 3.3.8), no pre-checked consent boxes, no hidden subscriptions.
- **Privacy notice (LGPD — Lei 13.709/2018)** whenever a form collects personal data: next to the form, state the specific purpose, who the controller is and how to contact them, and link to the privacy policy (Art. 9). Collect only the fields that purpose needs. When consent is the legal basis, it must be for a specific purpose (generic authorizations are void, Art. 8 §4) and revocable. Legal texts (privacy policy, terms) are provided or approved by the user — never drafted as final legal advice.
- Never put form submissions in client-side code paths that expose credentials (API keys, SMTP passwords); send them through a server endpoint or a form service the user chose.

---

## 10. Analytics and Cookie Consent

- Add analytics only when the user asks for it, with the provider they choose; prefer privacy-friendly configurations (no unnecessary cookies, IP anonymization where offered).
- Non-essential cookies and trackers (analytics, marketing, embeds that set cookies) are loaded **only after consent**. The consent banner offers *accept* and *reject* with equal prominence, doesn't pre-select optional categories, and lets the user change the choice later (e.g. a footer link).
- The banner must not trap keyboard focus or hide focused content (WCAG 2.4.11).
- List every cookie/tracker the site sets in the privacy or cookie policy.

---

## 11. Internationalization

- `lang` on `<html>` for every page (e.g. `pt-BR`, `en`), and on inline passages in another language.
- One URL per language version (e.g. `/en/...`, `/pt-br/...`) — never switch languages only by cookie or script on the same URL.
- **`hreflang`:** each language version links to **itself and every other version** (links must be reciprocal or they are ignored), using ISO 639-1 language codes with an optional ISO 3166-1 region (`pt-BR`), plus `x-default` for the fallback/selector page. Use `<link rel="alternate" hreflang="...">` in the `<head>`, or the sitemap.
- A visible language switcher that keeps the user on the equivalent page.
- Translations, dates, numbers and currency formats come from the project's i18n mechanism — no hard-coded strings in templates when the project has one.

---

## 12. Hosting and Security Headers

### Security headers

Set these on server hosting (web server or framework config) and on static hosts that allow custom headers:

| Header | Recommended value |
|---|---|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` (add `preload` only after confirming every subdomain serves HTTPS — preload requires `max-age` ≥ 31536000 and `includeSubDomains`) |
| `Content-Security-Policy` | A policy built from the site's real sources (`default-src 'self'` as a starting point), without `'unsafe-inline'` scripts where possible, with `frame-ancestors` to control framing. Roll out with `Content-Security-Policy-Report-Only` first |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | Disable features the site doesn't use, e.g. `geolocation=(), camera=(), microphone=()` |
| `Cross-Origin-Opener-Policy` | `same-origin` (check it doesn't break pop-up based logins/payments the site uses) |

- Prefer CSP `frame-ancestors` over `X-Frame-Options` (use `X-Frame-Options: DENY` only as a fallback).
- Don't set `X-XSS-Protection` (or set it to `0`), `Expect-CT` or `Public-Key-Pins`.
- HSTS is ignored over plain HTTP and has no `<meta>` equivalent. A CSP can be delivered in `<meta http-equiv="Content-Security-Policy">` when the host can't set headers, but `frame-ancestors` doesn't work there — tell the user which protections the host can't provide.

### Static hosting vs server hosting

- **Static hosting** (GitHub Pages, object storage + CDN, ...) for sites whose build output is plain files; **server hosting** when the site needs server-side code (form handling, CMS, authentication). Document the target the project actually uses.
- **GitHub Pages** specifics:
  - Static files only; published site up to 1 GB, deployments time out after 10 minutes; not for commercial storefronts/transactions or for sending passwords or card numbers.
  - Deploy through a GitHub Actions workflow (`actions/configure-pages`, `actions/upload-pages-artifact`, `actions/deploy-pages`; deploy job with `pages: write` and `id-token: write`), with *Settings → Pages → Source* set to *GitHub Actions* by the user.
  - **Base path:** a project site is served at `https://<owner>.github.io/<repo>/`, so all asset and link URLs must account for `/<repo>/` (configure the stack's base/public path); a user/organization site or a custom domain uses `/`.
  - **Custom domain:** set in *Settings → Pages*. With an Actions workflow a `CNAME` file is ignored and not required (it is only used when deploying from a branch). DNS: `A`/`AAAA` records to GitHub's Pages IPs for an apex domain, a `CNAME` record to `<owner>.github.io` for a subdomain. Verify the domain on GitHub and enable *Enforce HTTPS*.
- Changes to repository settings, DNS and hosting dashboards are made by the user — the agent prepares the exact values and steps.

---

## 13. Quality Checks

Run these against the built site (or a local preview) before reporting the work as done, and report the results faithfully:

| Check | Tool (examples) |
|---|---|
| Performance, accessibility, SEO, best practices | Lighthouse CLI — `npm install -g lighthouse`, then `lighthouse <url> --output=html --output-path=./lighthouse.html` (requires a recent Node LTS; see its README) |
| HTML validity | Nu Html Checker — `vnu-jar` npm package or Docker image `ghcr.io/validator/validator`, e.g. `java -jar vnu.jar --skip-non-html <build-dir>` |
| Accessibility | axe — `npm install -g @axe-core/cli`, then `axe <url> --tags wcag2a,wcag2aa,wcag21aa,wcag22aa --exit`; Pa11y — `npm install -g pa11y`, then `pa11y --standard WCAG2AA --runner axe --runner htmlcs <url>` |
| Broken links | lychee — `lychee --offline <build-dir>` for internal links, `lychee <build-dir>` to include external ones (GitHub Action: `lycheeverse/lychee-action`) |
| Structured data | Google Rich Results Test (manual, on the deployed URL) |

- Prefer tools already configured in the project (scripts in `package.json`, CI workflows) over ad-hoc ones.
- Verify each CLI's current flags with `--help` before scripting it.
- Automated accessibility tools find only a subset of issues — finish with a manual keyboard, zoom and screen-reader pass on the main templates.

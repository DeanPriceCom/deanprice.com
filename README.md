# Dean Price — Personal Web Application

[![CI](https://github.com/DeanPriceCom/deanprice.com/actions/workflows/ci.yml/badge.svg)](https://github.com/DeanPriceCom/deanprice.com/actions/workflows/ci.yml)
[![Go Test](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![TinyGo](https://img.shields.io/badge/Compiled_with-TinyGo-blue)](https://tinygo.org)
[![WebAssembly](https://img.shields.io/badge/WebAssembly-WASM-654FF0?logo=webassembly&logoColor=white)](https://webassembly.org)
[![Cloudflare](https://img.shields.io/badge/Cloudflare-Pages_&_Edge_Functions-F38020?logo=cloudflare&logoColor=white)](https://pages.cloudflare.com/)

A deliberately over-engineered personal identity card and systems sandbox. Engineered to explore the limits of TinyGo WebAssembly binary compression (sub-50 KB), Cloudflare Edge stream rewriting, and client-side scraper mitigation—without sacrificing millisecond render times or accessibility.

🌐 **Live Website:** [https://www.deanprice.com](https://www.deanprice.com/)

---

## 💡 Design Philosophy & Threat Model

> **"Why does a personal contact card need WebAssembly and edge middleware?"**  
> It doesn't. This project is an intentional exercise in extreme constraint optimisation—treating a simple contact card as a sandbox to explore how far the modern web platform, TinyGo compiler, and CDN edge can be pushed.

### Threat Model & Boundaries
* **In Scope (Primary Goal):** Economic and computational asymmetry against automated scrapers. Neutralises traditional static HTML harvesters (`curl`, Scrapy, BeautifulSoup, regex extractors) that do not execute WebAssembly, at 0ms latency cost and with zero intrusive CAPTCHAs or UI friction for human visitors.
* **Explicit Non-Goal:** Cryptographic defence against full-DOM headless browsers (Playwright/Puppeteer). In client-side web architecture, any resource rendered in a user's browser is fundamentally inspectable by an emulated browser engine. The defence targets bulk automated harvesting, not targeted manual reverse-engineering.

---

## 🏛️ Architectural Highlights

### 1. Micro-WebAssembly Runtime (TinyGo)
* Compiled with **TinyGo** (`-opt=z -no-debug -panic=trap`) and post-optimised with Binaryen's `wasm-opt -Oz` into a <!-- wasm-size -->**41.6 KB micro-WASM binary** (17.5 KB gzipped)<!-- /wasm-size -->, compared to ~2.5 MB standard Go.
* Eliminates standard Go's heavy OS-thread runtime, reflection, and panic unwinding overhead, executing contact resolution and DOM hydration in **<2 ms** directly inside the browser's native WebAssembly engine.

### 2. Domain-Bound Obfuscation & Scraper Resistance
* **Algorithm:** 64-bit Donald Knuth Linear Congruential Generator (LCG) coupled with Sebastiano Vigna & Guy Steele's **SplitMix64** non-linear bit diffusion.
* **Domain-Bound & Target-Verified:** Keystreams are bound to authorized `location.hostname` domains, field nonces, and runtime initialisation context—providing friction-based static scraper and regex mitigation without intrusive CAPTCHAs or interaction delays.
* **Deterministic Build-Time Seed Injection:** Production contact values and master seeds are injected exclusively at compile time via Cloudflare Pages environment variables, guarded by CI assertions to ensure **zero plaintexts or master seeds exist in the repository**.

### 3. Edge-Driven Regional Localisation
* Cloudflare Pages Middleware (`functions/_middleware.js`) dynamically injects a `<meta name="cf-country">` tag at the CDN edge using streaming `HTMLRewriter`.
* Automatically selects regional dialling numbers with zero UI dropdowns, international dial code confusion, or client-side network waterfalls.
* Cloudflare Pages `_routes.json` restricts Edge Functions execution strictly to HTML document routes (`/`, `/index.html`), dynamic `/robots.txt`, and the `/api/report` telemetry endpoint, serving all static assets directly from the global CDN at 0ms Worker cost.

### 4. Sub-2ms Synchronous DOM Hydration & Static Honeypots
* Automated web crawlers and scrapers that parse static HTML are diverted to public honeypot decoy addresses embedded in the raw markup.
* **Capturing Phase Pre-WASM Guard:** Intercepts `click`, `auxclick`, `contextmenu`, and `dragstart` during the initial WASM compilation window, preventing race conditions, mobile long-press leaks, and rapid right-click honeypot harvesting.
* **Sub-2ms DOM Hydration:** Real human visitors benefit from immediate DOM hydration upon WebAssembly runtime initialisation (<2 ms), providing seamless WCAG accessibility for screen readers (NVDA/JAWS/VoiceOver rotor), keyboard navigation, and native context menus.
* Includes a pre-rendered, accessible fallback shield toggled via the HTML5 `hidden` attribute if WebAssembly execution is blocked by client-side filters.

### 5. Progressive Web App (PWA) & Security Headers
* Strict **Content Security Policy (CSP Level 3)** with build-time SHA-256 script hashing and **Trusted Types** policy enforcement (`trusted-types swPolicy 'allow-duplicates'`).
* Resilient Service Worker using **Network-First navigation with background cache revalidation and offline fallback**, preventing stale online geo-routing and version skew while providing offline availability.
* Service Worker cache invalidation keyed directly to Cloudflare deployment commit SHAs (`CF_PAGES_COMMIT_SHA`).

---

## 🧪 Local Testing & Verification

Run the full test suite across all production domains, dev environments, and regional locales:

```bash
# 1. Regenerate ciphertexts and synchronize CSP headers
go generate .

# 2. Run unit tests with cache bypassed
go test -v -count=1 ./...
```

---

## 📄 Licence
Copyright © 2026 Dean Price. All rights reserved.

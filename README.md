# Dean Price — Personal Web Application

[![CI](https://github.com/DeanPriceCom/deanprice.com/actions/workflows/ci.yml/badge.svg)](https://github.com/DeanPriceCom/deanprice.com/actions/workflows/ci.yml)
[![Go Test](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![TinyGo](https://img.shields.io/badge/Compiled_with-TinyGo-blue)](https://tinygo.org)
[![WebAssembly](https://img.shields.io/badge/WebAssembly-WASM-654FF0?logo=webassembly&logoColor=white)](https://webassembly.org)
[![Cloudflare](https://img.shields.io/badge/Cloudflare-Pages_&_Edge_Functions-F38020?logo=cloudflare&logoColor=white)](https://pages.cloudflare.com)

A high-performance, privacy-hardened digital identity card and infrastructure showcase engineered with Go WebAssembly, Cloudflare Edge Functions, and domain-bound client-side obfuscation for high-friction anti-scraping and crawler defence.

🌐 **Live Website:** [https://www.deanprice.com](https://www.deanprice.com/)

---

## 🏛️ Architectural Highlights

### 1. Micro-WebAssembly Runtime (TinyGo)
* Compiled with **TinyGo** (`-opt=z -no-debug -panic=trap`) and post-optimised with Binaryen's `wasm-opt -Oz` into a **40.7 KB micro-WASM binary** (17.1 KB gzipped, compared to ~2.5 MB standard Go).
* Stripped of the Go runtime scheduler, timer queues, reflection, and panic unwinding via synchronous Edge DOM hydration, executing in **<2 ms** inside the browser's native WebAssembly engine.

### 2. Domain-Bound Anti-Scraping Obfuscation Engine
* **Algorithm:** 64-bit Donald Knuth Linear Congruential Generator (LCG) coupled with Sebastiano Vigna & Guy Steele's **SplitMix64** non-linear bit diffusion.
* **Domain-Bound & Target-Verified:** Keystreams are mathematically locked to `location.hostname`, field nonces, and a runtime initialisation context—providing **high-friction client-side obfuscation and honeypot crawler defence without interaction delays**.
* **12-Factor CI/CD Injection:** Production secrets and master seeds are injected exclusively at build time via Cloudflare Pages environment variables, guarded by CI assertions to ensure **zero plaintexts or private keys exist in the repository**.

### 3. Edge-Driven Regional Localisation
* Cloudflare Pages Middleware (`functions/_middleware.js`) dynamically injects a `<meta name="cf-country">` tag at the CDN edge using streaming `HTMLRewriter`.
* Automatically selects regional dialling numbers with zero UI dropdowns, international dial code confusion, or client-side network waterfalls.
* Cloudflare Pages `_routes.json` routes only HTML document requests to middleware, serving all static assets directly from the global CDN at 0ms Worker cost.

### 4. Eager Post-WASM Hydration & Static Honeypots
* Automated web crawlers and scrapers that parse static HTML are diverted to public honeypot decoy addresses embedded in the raw markup.
* **Capturing Phase Pre-WASM Guard:** Intercepts `click`, `auxclick`, `contextmenu`, and `dragstart` during the initial WASM compilation window, preventing race conditions, mobile long-press leaks, and rapid right-click honeypot harvesting.
* **Eager Post-WASM Hydration:** Real human visitors benefit from immediate DOM hydration upon WebAssembly runtime initialisation (<2 ms), providing seamless WCAG accessibility for screen readers (NVDA/JAWS/VoiceOver rotor), keyboard navigation, and native context menus.
* Includes a pre-rendered, accessible fallback shield toggled via the HTML5 `hidden` attribute if WebAssembly execution is blocked by client-side filters.

### 5. Progressive Web App (PWA) & Security Headers
* Strict **Content Security Policy (CSP Level 3)** with build-time SHA-256 script hashing and **Trusted Types** policy enforcement (`trusted-types swPolicy`).
* Resilient Service Worker using **Network-First navigation with offline cache fallback**, eliminating stale geo-routing and version skew while guaranteeing offline availability.
* Service Worker cache invalidation keyed directly to Cloudflare deployment commit SHAs (`CF_PAGES_COMMIT_SHA`).

---

## 🧪 Local Testing & Verification

Run the test suite across all 15 host environments and 3 localisation regions:

```bash
# Run unit tests with cache bypassed
go test -v -count=1 ./...

# Regenerate ciphertexts and update CSP headers
go generate .
```

---

## 📄 Licence
Copyright © 2026 Dean Price. All rights reserved.

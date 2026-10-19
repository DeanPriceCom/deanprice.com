# AGENTS.md — Guidelines for AI Coding Agents (Jules, etc.)

Welcome to **deanprice.com**! This repository is a deliberately over-engineered personal identity card and systems sandbox engineered for extreme constraint optimization, scraper mitigation, sub-2ms render times, and micro-WebAssembly execution.

---

## 🏛️ Project Architecture & Tech Stack

* **Language / Compiler:** Go 1.24 (`go.mod`), compiled to WebAssembly via **TinyGo 0.41.1** (sub-50 KB micro-WASM binary) and optimized with Binaryen's `wasm-opt -Oz`.
* **Hosting Platform:** **Cloudflare Pages** with Edge Middleware (`functions/_middleware.js`) using streaming `HTMLRewriter` for geo-routing.
* **Frontend:** Vanilla HTML5, CSS3, Service Worker (`sw.js`), and WebAssembly (`main.wasm`). Zero heavy JavaScript frameworks or npm dependencies.
* **Dependencies:** Standard Go library only. No third-party modules in `go.mod`.

---

## 🛠️ Build, Generation & Test Commands

Always verify changes using these standard commands before opening a pull request:

### 1. Code Formatting & Static Analysis
```bash
# Format all Go files (enforced by CI)
gofmt -s -w .

# Run static analysis
go vet ./...
```

### 2. Code Generation & Header Synchronization
```bash
# Regenerate keystreams, assets, and synchronize CSP hashes in _headers
go generate ./...
```
> **IMPORTANT:** Running `go generate ./...` invokes `go run ./tools -task=codegen`. If the inline script in `index.html` or any source asset in `.assets/` changes, you **must** run `go generate ./...` to synchronize embedded assets and update the SHA-256 `script-src` hash in `_headers`. CI will fail if `_headers`, `index.html`, or `404.html` is out of sync (`git diff --exit-code _headers index.html 404.html`).

### 3. Running Unit Tests
```bash
# Run all unit tests with cache bypassed
go test -v -count=1 ./...
```

### 4. Compiling the WebAssembly Binary (TinyGo)
```bash
# Compile micro-WASM binary with TinyGo
tinygo build -opt=z -no-debug -panic=trap -target wasm -o main.wasm .

# (Optional post-optimization if wasm-opt is installed)
wasm-opt -Oz --strip-debug --strip-dwarf --strip-producers -o main.wasm main.wasm
```

### 5. Full Build Pipeline
```bash
# Executes generation, TinyGo build, wasm-opt, and dist/ packaging
bash scripts/build.sh
```

---

## ⚠️ Critical Invariants & Rules for Agents

1. **Never use standard Go (`GOOS=js GOARCH=wasm go build`) for production binaries:**
   Standard Go produces a ~2.5 MB binary with runtime scheduler overhead. This repository strictly targets **TinyGo** with `-opt=z -no-debug -panic=trap` to keep `main.wasm` under 50 KB.
2. **Never break Content Security Policy (CSP Level 3):**
   `_headers` strictly enforces `script-src` SHA-256 hashes, `object-src 'none'`, and `trusted-types`. Do not insert raw unhashed inline scripts or remote third-party scripts without updating `_headers` via `go generate ./...`.
3. **Preserve Edge Middleware Separation:**
   `functions/_middleware.js` runs on Cloudflare Workers and streams HTML modifications (e.g. `<meta name="cf-country">`). Static routing rules live in `_routes.json`. Do not break the route mappings.
4. **Preserve Test Integrity:**
   `main_test.go` and `internal/contact/contact_test.go` contain comprehensive tests verifying embedded asset hashes, regional phone number resolution, honeypot decoy disambiguation, edge telemetry behavior, and domain-bound obfuscation. All tests must pass cleanly.
5. **Keep zero external Go dependencies:**
   Do not run `go get` for external packages unless explicitly requested.

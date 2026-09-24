package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"deanprice.com/internal/contact"
)

func TestEmbeddedAssetsIntegrity(t *testing.T) {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	// 1. Validate all data URIs in index.html decode cleanly without base64 errors
	dataURIRegex := regexp.MustCompile(`data:([a-zA-Z0-9/+.-]+);base64,([A-Za-z0-9+/=]+)`)
	matches := dataURIRegex.FindAllStringSubmatch(htmlStr, -1)
	if len(matches) == 0 {
		t.Fatalf("No data URIs found in index.html")
	}

	for _, match := range matches {
		mime := match[1]
		b64Data := match[2]

		decoded, err := base64.StdEncoding.DecodeString(b64Data)
		if err != nil {
			t.Errorf("Invalid base64 payload for mime %s: %v", mime, err)
			continue
		}
		if len(decoded) == 0 {
			t.Errorf("Decoded payload for mime %s is empty", mime)
			continue
		}

		switch mime {
		case "image/webp":
			if len(decoded) < 12 || string(decoded[0:4]) != "RIFF" || string(decoded[8:12]) != "WEBP" {
				t.Errorf("Corrupted WebP header in data URI (len=%d)", len(decoded))
			}
		case "font/woff2":
			if len(decoded) < 4 || string(decoded[0:4]) != "wOF2" {
				t.Errorf("Corrupted WOFF2 header in data URI (len=%d)", len(decoded))
			}
		}
	}

	// 2. Validate exact byte-for-byte parity with .assets/ directory files
	expectedAssets := []string{
		"email.webp",
		"phone.webp",
		"whatsapp.webp",
		"facebook.webp",
		"instagram.webp",
		"patrickhand.woff2",
	}

	for _, assetName := range expectedAssets {
		assetBytes, err := os.ReadFile(fmt.Sprintf(".assets/%s", assetName))
		if err != nil {
			t.Fatalf("Failed to read source asset .assets/%s: %v", assetName, err)
		}
		expectedB64 := base64.StdEncoding.EncodeToString(assetBytes)
		if !strings.Contains(htmlStr, expectedB64) {
			t.Errorf("index.html does not contain exact Base64 payload for %s (size: %d bytes, b64 len: %d)",
				assetName, len(assetBytes), len(expectedB64))
		}
	}

	// 3. Validate 404.html embedded assets integrity and exact byte parity for patrickhand.woff2
	f404Bytes, err := os.ReadFile("404.html")
	if err != nil {
		t.Fatalf("Failed to read 404.html: %v", err)
	}
	f404Str := string(f404Bytes)

	f404Matches := dataURIRegex.FindAllStringSubmatch(f404Str, -1)
	if len(f404Matches) == 0 {
		t.Fatalf("No data URIs found in 404.html")
	}

	for _, match := range f404Matches {
		mime := match[1]
		b64Data := match[2]

		decoded, err := base64.StdEncoding.DecodeString(b64Data)
		if err != nil {
			t.Errorf("Invalid base64 payload in 404.html for mime %s: %v", mime, err)
			continue
		}
		if mime == "font/woff2" {
			if len(decoded) < 4 || string(decoded[0:4]) != "wOF2" {
				t.Errorf("Corrupted WOFF2 header in 404.html data URI (len=%d)", len(decoded))
			}
		}
	}

	fontBytes, err := os.ReadFile(".assets/patrickhand.woff2")
	if err != nil {
		t.Fatalf("Failed to read source asset .assets/patrickhand.woff2: %v", err)
	}
	expectedFontB64 := base64.StdEncoding.EncodeToString(fontBytes)
	if !strings.Contains(f404Str, expectedFontB64) {
		t.Errorf("404.html does not contain exact Base64 payload for patrickhand.woff2 (size: %d bytes, b64 len: %d)",
			len(fontBytes), len(expectedFontB64))
	}
}

func TestShieldAndCSPIntegrity(t *testing.T) {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	// 1. Verify all three shield cards exist in index.html
	requiredElements := []string{
		`id="shield-legacy"`,
		`id="shield-blocked"`,
		`id="shield-js"`,
		`#shield-js[hidden]`,
	}
	for _, elem := range requiredElements {
		if !strings.Contains(htmlStr, elem) {
			t.Errorf("index.html missing required shield element or CSS: %s", elem)
		}
	}

	// 2. Compute inline script hash independently (Test Oracle principle) and verify parity with _headers
	tag := `<script id="wasm-engine">`
	startIdx := strings.Index(htmlStr, tag)
	if startIdx == -1 {
		t.Fatalf("Could not find %s in index.html", tag)
	}
	startIdx += len(tag)

	relEnd := strings.Index(htmlStr[startIdx:], "</script>")
	if relEnd == -1 {
		t.Fatalf("Could not find closing </script> for wasm-engine in index.html")
	}

	scriptContent := htmlStr[startIdx : startIdx+relEnd]
	if !strings.Contains(scriptContent, "initWasm") {
		t.Fatalf("Extracted script does not contain expected WASM hydration logic")
	}
	scriptContent = strings.ReplaceAll(scriptContent, "\r\n", "\n")

	hash := sha256.Sum256([]byte(scriptContent))
	expectedCSPHash := "sha256-" + base64.StdEncoding.EncodeToString(hash[:])

	headersBytes, err := os.ReadFile("_headers")
	if err != nil {
		t.Fatalf("Failed to read _headers: %v", err)
	}

	if !strings.Contains(string(headersBytes), expectedCSPHash) {
		t.Errorf("CSP hash mismatch in _headers! Expected %s, run 'go generate ./...' to synchronize", expectedCSPHash)
	}
}

func TestReadmeWasmSizeIntegrity(t *testing.T) {
	if os.Getenv("CHECK_WASM_SIZE") == "" {
		t.Skip("Skipping README size verification. Set CHECK_WASM_SIZE=1 to verify.")
	}

	wasmBytes, err := os.ReadFile("main.wasm")
	if os.IsNotExist(err) {
		t.Skip("main.wasm not found, skipping README size verification")
	}
	if err != nil {
		t.Fatalf("Failed to read main.wasm: %v", err)
	}

	rawKB := float64(len(wasmBytes)) / 1024.0

	var gzBuf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if err != nil {
		t.Fatalf("Failed to create gzip writer: %v", err)
	}
	if _, err := gz.Write(wasmBytes); err != nil {
		t.Fatalf("Failed to compress wasm bytes: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}
	gzKB := float64(gzBuf.Len()) / 1024.0

	expectedRaw := fmt.Sprintf("%.1f KB micro-WASM binary", rawKB)
	expectedGz := fmt.Sprintf("%.1f KB gzipped", gzKB)

	readmeBytes, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	readmeStr := string(readmeBytes)

	if !strings.Contains(readmeStr, expectedRaw) || !strings.Contains(readmeStr, expectedGz) {
		t.Errorf("README.md WASM sizes out of sync with main.wasm! Expected %q and %q. Run 'go run ./tools -task=readme' or VS Code build task to synchronize.", expectedRaw, expectedGz)
	}
}

func TestServiceWorkerCacheIntegrity(t *testing.T) {
	swBytes, err := os.ReadFile("sw.js")
	if err != nil {
		t.Fatalf("Failed to read sw.js: %v", err)
	}
	swStr := string(swBytes)

	// Extract ASSETS_TO_CACHE array
	re := regexp.MustCompile(`ASSETS_TO_CACHE\s*=\s*\[([\s\S]*?)\]`)
	match := re.FindStringSubmatch(swStr)
	if len(match) < 2 {
		t.Fatalf("Failed to find ASSETS_TO_CACHE array in sw.js")
	}

	assetRe := regexp.MustCompile(`['"]([^'"]+)['"]`)
	assetMatches := assetRe.FindAllStringSubmatch(match[1], -1)
	if len(assetMatches) == 0 {
		t.Fatalf("No assets parsed from ASSETS_TO_CACHE in sw.js")
	}

	// 1. Ensure control/config files and SEO crawler files are strictly ABSENT from App Shell cache
	bannedPatterns := []string{"_headers", "_routes.json", "robots.txt", "sitemap.xml", "llms"}

	for _, m := range assetMatches {
		urlPath := m[1]
		for _, banned := range bannedPatterns {
			if strings.Contains(urlPath, banned) {
				t.Errorf("Security/PWA violation: %s must NOT be in ASSETS_TO_CACHE in sw.js (breaks cache.addAll or pollutes cache)", urlPath)
			}
		}

		// 2. Resolve URL to local file path
		var localPath string
		if urlPath == "/" {
			localPath = "index.html"
		} else {
			localPath = strings.TrimPrefix(urlPath, "/")
		}

		// 3. Check file existence (CI-aware: main.wasm and wasm_exec.js are build artifacts provisioned by TinyGo/build.sh)
		if localPath == "main.wasm" || localPath == "wasm_exec.js" {
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				t.Logf("Note: %s is not yet compiled/provisioned (expected during pre-build CI testing)", localPath)
			}
			continue
		}

		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			t.Errorf("Asset in ASSETS_TO_CACHE does not exist in repository: %s (local path: %s)", urlPath, localPath)
		}
	}
}

func TestDevHostSyncIntegrity(t *testing.T) {
	// Verify that preview domain rules are in sync across:
	// 1. internal/contact/decrypt.go
	// 2. functions/_middleware.js
	// 3. index.html

	middlewareBytes, err := os.ReadFile("functions/_middleware.js")
	if err != nil {
		t.Fatalf("Failed to read functions/_middleware.js: %v", err)
	}
	middlewareStr := string(middlewareBytes)

	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	requiredHostPatterns := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"[::1]",
		".pages.dev",
		".local",
		".lan",
		".ts.net",
		"192.168.",
		"10.",
		"172.",
		"100.",
	}

	for _, pattern := range requiredHostPatterns {
		if !strings.Contains(middlewareStr, pattern) {
			t.Errorf("functions/_middleware.js missing required dev/preview pattern: %s", pattern)
		}
		if !strings.Contains(htmlStr, pattern) {
			t.Errorf("index.html missing required dev/preview pattern: %s", pattern)
		}
	}

	// Verify Go helper agrees with required patterns
	testDevHosts := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"[::1]", true},
		{"", true},
		{"preview.pages.dev", true},
		{"myphone.local", true},
		{"laptop.lan", true},
		{"node.ts.net", true},
		{"192.168.1.50", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"100.64.0.1", true},
		{"deanprice.com", false},
		{"evil-scraper.com", false},
		{"example.com", false},
	}

	for _, tc := range testDevHosts {
		got := contact.IsDevOrPreviewHost(tc.host)
		if got != tc.expected {
			t.Errorf("contact.IsDevOrPreviewHost(%q) = %v, want %v", tc.host, got, tc.expected)
		}
	}
}

func TestContactResolutionIntegration(t *testing.T) {
	// High-level integration sanity check on the extracted domain package
	country := contact.ResolveCountry("GB")
	if country != "GB" {
		t.Errorf("contact.ResolveCountry(GB) = %q, want GB", country)
	}

	countryTR := contact.ResolveCountry("TR")
	if countryTR != "TR" {
		t.Errorf("contact.ResolveCountry(TR) = %q, want TR", countryTR)
	}

	email := contact.ResolveContact("email", "", "deanprice.com")
	if !strings.HasPrefix(email, "mailto:") {
		t.Errorf("contact.ResolveContact(email) returned unexpected format: %q", email)
	}
}

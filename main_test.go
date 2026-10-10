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

func TestServiceWorkerRegistrationGatingAndTelemetry(t *testing.T) {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	// 1. Ensure service worker registration is gated against crawlers/bots
	if !strings.Contains(htmlStr, "!isBot && 'serviceWorker' in navigator") {
		t.Errorf("index.html must gate Service Worker registration behind !isBot to prevent crawler rejection noise")
	}

	// 2. Ensure service worker policy is cached on window._swPolicy and guards against unblessed string fallback
	if !strings.Contains(htmlStr, "window._swPolicy") {
		t.Errorf("index.html must cache Trusted Types policy in window._swPolicy to prevent duplicate policy creation")
	}
	if !strings.Contains(htmlStr, "if (window.trustedTypes && !swPolicy)") {
		t.Errorf("index.html must guard against unblessed string fallback when Trusted Types is active")
	}

	// 3. Ensure service worker catch block suppresses benign 'rejected', NotSupportedError, trustedscripturl, and offline conditions
	if !strings.Contains(htmlStr, "rejected") || !strings.Contains(htmlStr, "!navigator.onLine") || !strings.Contains(htmlStr, "NotSupportedError") || !strings.Contains(htmlStr, "trustedscripturl") {
		t.Errorf("index.html must suppress benign 'rejected', 'NotSupportedError', 'trustedscripturl', and offline failures in service worker registration catch")
	}

	// 4. Ensure functions/api/report.js filters out sw_registration_failure with 'rejected', 'trustedscripturl', and 'trusted types'
	reportBytes, err := os.ReadFile("functions/api/report.js")
	if err != nil {
		t.Fatalf("Failed to read functions/api/report.js: %v", err)
	}
	reportStr := string(reportBytes)

	if !strings.Contains(reportStr, "sw_registration_failure") || !strings.Contains(reportStr, "rejected") || !strings.Contains(reportStr, "trustedscripturl") {
		t.Errorf("functions/api/report.js must drop sw_registration_failure reports containing 'rejected' and 'trustedscripturl'")
	}

	// 5. Ensure _headers enforces allow-duplicates for swPolicy
	headersBytes, err := os.ReadFile("_headers")
	if err != nil {
		t.Fatalf("Failed to read _headers: %v", err)
	}
	headersStr := string(headersBytes)

	if !strings.Contains(headersStr, "trusted-types swPolicy 'allow-duplicates'") {
		t.Errorf("_headers must configure 'trusted-types swPolicy 'allow-duplicates'' to permit idempotent policy creation")
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
		"PRIVATE_IPV4_REGEX",
		`^(?:10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2}|100\.(?:6[4-9]|[7-9]\d|1[01]\d|12[0-7])(?:\.\d{1,3}){2})$`,
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
		// RFC 1918 10.0.0.0/8
		{"10.0.0.1", true},
		{"10.255.255.254", true},
		// RFC 1918 172.16.0.0/12
		{"172.16.0.1", true},
		{"172.31.255.254", true},
		{"172.15.255.255", false},
		{"172.32.0.1", false},
		// RFC 1918 192.168.0.0/16
		{"192.168.1.50", true},
		{"192.168.0.1", true},
		// RFC 6598 100.64.0.0/10
		{"100.64.0.1", true},
		{"100.127.255.254", true},
		{"100.63.255.255", false},
		{"100.128.0.1", false},
		// Public domains & spoof attempts
		{"deanprice.com", false},
		{"evil-scraper.com", false},
		{"example.com", false},
		{"10.com", false},
		{"100.com", false},
		{"172.com", false},
		{"192.168.com", false},
		{"10.0.0.1.nip.io", false},
		// Malformed & edge cases
		{"10.0.0", false},
		{"10.0.0.0.1", false},
		{"10.01.0.1", false},
		{"10.300.0.1", false},
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

func TestIconRowResponsiveIntegrity(t *testing.T) {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	// 1. Verify rigid legacy constraints are eliminated
	if strings.Contains(htmlStr, "calc(18vw - 2px)") {
		t.Errorf("index.html still contains buggy calculation 'calc(18vw - 2px)'")
	}
	if strings.Contains(htmlStr, "min-width: 45px") {
		t.Errorf("index.html still contains legacy rigid floor 'min-width: 45px'")
	}

	// 2. Verify whitespace-tolerant flex gap on #iconrow
	gapRe := regexp.MustCompile(`(?s)#iconrow\s*\{[^}]*gap:\s*clamp\(`)
	if !gapRe.MatchString(htmlStr) {
		t.Errorf("index.html missing responsive 'gap: clamp(...)' on #iconrow")
	}

	// 3. Verify flex-basis on #iconrow a
	flexRe := regexp.MustCompile(`(?s)#iconrow\s+a\s*\{[^}]*flex:\s*0\s+1\s+78px`)
	if !flexRe.MatchString(htmlStr) {
		t.Errorf("index.html missing 'flex: 0 1 78px' on #iconrow a")
	}

	// 4. Verify fluid image width without rigid min-width floor
	imgRe := regexp.MustCompile(`(?s)#iconrow\s+img\s*\{[^}]*width:\s*100%`)
	if !imgRe.MatchString(htmlStr) {
		t.Errorf("index.html missing fluid 'width: 100%%' on #iconrow img")
	}

	// 5. Verify negative breakout margin to reclaim hero badge sizing on mobile
	breakoutRe := regexp.MustCompile(`(?s)#iconrow\s*\{[^}]*margin-left:\s*-12px`)
	if !breakoutRe.MatchString(htmlStr) || !strings.Contains(htmlStr, "calc(100% + 24px)") {
		t.Errorf("index.html missing breakout sizing 'calc(100%% + 24px)' on #iconrow")
	}
}

func TestRobotsNoindexRouteParity(t *testing.T) {
	// Enforce that any route in _headers carrying "X-Robots-Tag: noindex"
	// is explicitly allowed in robots.txt, functions/_middleware.js, and tools/server/main.go.
	// Otherwise, crawlers will be blocked by robots.txt from fetching the route,
	// preventing them from reading the HTTP header and leading to bare URL indexation.

	headersBytes, err := os.ReadFile("_headers")
	if err != nil {
		t.Fatalf("Failed to read _headers: %v", err)
	}

	robotsBytes, err := os.ReadFile("robots.txt")
	if err != nil {
		t.Fatalf("Failed to read robots.txt: %v", err)
	}
	robotsStr := string(robotsBytes)

	middlewareBytes, err := os.ReadFile("functions/_middleware.js")
	if err != nil {
		t.Fatalf("Failed to read functions/_middleware.js: %v", err)
	}
	middlewareStr := string(middlewareBytes)

	serverBytes, err := os.ReadFile("tools/server/main.go")
	if err != nil {
		t.Fatalf("Failed to read tools/server/main.go: %v", err)
	}
	serverStr := string(serverBytes)

	lines := strings.Split(string(headersBytes), "\n")
	var currentRoute string
	var noindexRoutes []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/") {
			currentRoute = strings.Fields(trimmed)[0]
		} else if strings.HasPrefix(trimmed, "X-Robots-Tag:") && strings.Contains(strings.ToLower(trimmed), "noindex") {
			if currentRoute != "" {
				noindexRoutes = append(noindexRoutes, currentRoute)
			}
		}
	}

	if len(noindexRoutes) == 0 {
		t.Fatalf("Expected to find routes with X-Robots-Tag: noindex in _headers, found none")
	}

	for _, route := range noindexRoutes {
		allowDirective := "Allow: " + route

		if !strings.Contains(robotsStr, allowDirective) {
			t.Errorf("robots.txt missing %q for route with X-Robots-Tag: noindex in _headers (triggers robots.txt vs noindex trap)", allowDirective)
		}
		if !strings.Contains(middlewareStr, allowDirective) {
			t.Errorf("functions/_middleware.js ALIAS_ROBOTS_TXT missing %q for route with X-Robots-Tag: noindex in _headers (triggers robots.txt vs noindex trap)", allowDirective)
		}
		if !strings.Contains(serverStr, allowDirective) {
			t.Errorf("tools/server/main.go aliasRobotsTxt missing %q for route with X-Robots-Tag: noindex in _headers (triggers robots.txt vs noindex trap)", allowDirective)
		}
	}
}

func TestAliasRobotsParity(t *testing.T) {
	middlewareBytes, err := os.ReadFile("functions/_middleware.js")
	if err != nil {
		t.Fatalf("Failed to read functions/_middleware.js: %v", err)
	}
	middlewareStr := strings.ReplaceAll(string(middlewareBytes), "\r\n", "\n")

	serverBytes, err := os.ReadFile("tools/server/main.go")
	if err != nil {
		t.Fatalf("Failed to read tools/server/main.go: %v", err)
	}
	serverStr := strings.ReplaceAll(string(serverBytes), "\r\n", "\n")

	reMiddleware := regexp.MustCompile("(?s)const ALIAS_ROBOTS_TXT = `(.*?)`;")
	matchMiddleware := reMiddleware.FindStringSubmatch(middlewareStr)
	if len(matchMiddleware) < 2 {
		t.Fatalf("Failed to extract ALIAS_ROBOTS_TXT from functions/_middleware.js")
	}

	reServer := regexp.MustCompile("(?s)const aliasRobotsTxt = `(.*?)`")
	matchServer := reServer.FindStringSubmatch(serverStr)
	if len(matchServer) < 2 {
		t.Fatalf("Failed to extract aliasRobotsTxt from tools/server/main.go")
	}

	aliasMiddleware := strings.TrimSpace(matchMiddleware[1])
	aliasServer := strings.TrimSpace(matchServer[1])

	if aliasMiddleware != aliasServer {
		t.Errorf("Parity mismatch between functions/_middleware.js ALIAS_ROBOTS_TXT and tools/server/main.go aliasRobotsTxt!\nGot (middleware):\n%s\n\nWant (server):\n%s",
			aliasMiddleware, aliasServer)
	}
}

func TestDecoyInspectionIntegrity(t *testing.T) {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	// 1. Verify ?decoy=raw logic and observability hooks exist in index.html
	requiredElements := []string{
		"isDecoyRaw",
		"window.__DECOY_STATE = 'raw'",
		"document.documentElement.dataset.decoy = 'raw'",
		"Decoy inspection mode active (?decoy=raw)",
	}
	for _, elem := range requiredElements {
		if !strings.Contains(htmlStr, elem) {
			t.Errorf("index.html missing required decoy inspection logic: %s", elem)
		}
	}

	// 2. Invariant: isDecoyRaw must precede isBot to ensure automated test suites enter decoy mode
	idxDecoy := strings.Index(htmlStr, "else if (isDecoyRaw)")
	idxBot := strings.Index(htmlStr, "else if (isBot)")
	if idxDecoy == -1 {
		t.Errorf("index.html missing 'else if (isDecoyRaw)' branch")
	}
	if idxBot == -1 {
		t.Errorf("index.html missing 'else if (isBot)' branch")
	}
	if idxDecoy != -1 && idxBot != -1 && idxDecoy > idxBot {
		t.Errorf("Precedence violation: 'isDecoyRaw' must precede 'isBot' in index.html execution branching")
	}

	// 3. Verify functions/_middleware.js handles decoy parameter
	middlewareBytes, err := os.ReadFile("functions/_middleware.js")
	if err != nil {
		t.Fatalf("Failed to read functions/_middleware.js: %v", err)
	}
	middlewareStr := string(middlewareBytes)
	if !strings.Contains(middlewareStr, `url.searchParams.get("decoy")?.trim().toLowerCase() === "raw"`) {
		t.Errorf("functions/_middleware.js missing expected decoy parameter handling")
	}

	// 4. Verify tools/server/main.go handles decoy parameter
	serverBytes, err := os.ReadFile("tools/server/main.go")
	if err != nil {
		t.Fatalf("Failed to read tools/server/main.go: %v", err)
	}
	serverStr := string(serverBytes)
	if !strings.Contains(serverStr, `strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("decoy")), "raw")`) {
		t.Errorf("tools/server/main.go missing expected decoy parameter handling")
	}
}

func TestCrawlerParityIntegrity(t *testing.T) {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlBytes)

	robotsBytes, err := os.ReadFile("robots.txt")
	if err != nil {
		t.Fatalf("Failed to read robots.txt: %v", err)
	}
	robotsStr := string(robotsBytes)

	// 1. Extract and compile the client-side isBot regex pattern
	reRegex := regexp.MustCompile(`\|\|\s*/([^/]+)/i\.test`)
	match := reRegex.FindStringSubmatch(htmlStr)
	if len(match) < 2 {
		t.Fatalf("Failed to extract isBot regex pattern from index.html")
	}

	botRegex, err := regexp.Compile("(?i)" + match[1])
	if err != nil {
		t.Fatalf("Failed to compile extracted isBot regex %q: %v", match[1], err)
	}

	// 2. Define the core approved generative AI & search crawlers
	crawlersToVerify := []struct {
		name      string
		token     string
		userAgent string
	}{
		// Google Ecosystem (7)
		{"Googlebot", "Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"},
		{"Google-InspectionTool", "Google-InspectionTool", "Mozilla/5.0 (compatible; Google-InspectionTool/1.0;)"},
		{"Google-Agent", "Google-Agent", "Mozilla/5.0 (compatible; Google-Agent/1.0; +https://developers.google.com)"},
		{"Gemini-Deep-Research", "Gemini-Deep-Research", "Mozilla/5.0 (compatible; Gemini-Deep-Research/1.0;)"},
		{"GoogleOther", "GoogleOther", "Mozilla/5.0 (compatible; GoogleOther/1.0;)"},
		{"Google-GeminiNotebook", "Google-GeminiNotebook", "Mozilla/5.0 (compatible; Google-GeminiNotebook)"},
		{"Google-NotebookLM", "Google-NotebookLM", "Mozilla/5.0 (compatible; Google-NotebookLM)"},
		// OpenAI (3)
		{"GPTBot", "GPTBot", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)"},
		{"ChatGPT-User", "ChatGPT-User", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ChatGPT-User/1.0; +https://openai.com/bot)"},
		{"OAI-SearchBot", "OAI-SearchBot", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot)"},
		// Anthropic (3)
		{"ClaudeBot", "ClaudeBot", "Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)"},
		{"Claude-User", "Claude-User", "Mozilla/5.0 (compatible; Claude-User/1.0; +claudebot@anthropic.com)"},
		{"Claude-SearchBot", "Claude-SearchBot", "Mozilla/5.0 (compatible; Claude-SearchBot/1.0; +claudebot@anthropic.com)"},
		// xAI (2)
		{"GrokBot", "GrokBot", "Mozilla/5.0 (compatible; GrokBot/1.0)"},
		{"xAI-Grok", "xAI-Grok", "Mozilla/5.0 (compatible; xAI-Grok/1.0)"},
		// Perplexity (2)
		{"PerplexityBot", "PerplexityBot", "Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)"},
		{"Perplexity-User", "Perplexity-User", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Perplexity-User/1.0; +https://perplexity.ai/perplexity-user)"},
		// Mistral AI (2)
		{"MistralAI-User", "MistralAI-User", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; MistralAI-User/1.0; +https://docs.mistral.ai/robots)"},
		{"MistralAI-Index", "MistralAI-Index", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; MistralAI-Index/1.0; +https://docs.mistral.ai/robots)"},
		// Core Search Engines (3)
		{"Bravebot", "Bravebot", "Mozilla/5.0 (compatible; Bravebot/1.0; +https://brave.com/search/crawler/)"},
		{"Bingbot", "Bingbot", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"},
		{"Applebot", "Applebot", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)"},
		// Frontier & Foundation AI Crawlers (7)
		{"meta-externalagent", "meta-externalagent", "Mozilla/5.0 (compatible; meta-externalagent/1.1; +https://developers.facebook.com/docs/sharing/webmasters/crawler)"},
		{"Bytespider", "Bytespider", "Mozilla/5.0 (compatible; Bytespider; https://zhanzhang.toutiao.com/)"},
		{"cohere-ai", "cohere-ai", "Mozilla/5.0 (compatible; cohere-ai/1.0; +https://cohere.com/bot)"},
		{"Amazonbot", "Amazonbot", "Mozilla/5.0 (compatible; Amazonbot/0.1; +https://developer.amazon.com/support/amazonbot)"},
		{"YouBot", "YouBot", "Mozilla/5.0 (compatible; YouBot/1.0; +https://you.com/crawler)"},
		{"AI2Bot", "AI2Bot", "Mozilla/5.0 (compatible; AI2Bot/1.0; +https://allenai.org/crawler)"},
		{"CCBot", "CCBot", "CCBot/2.0 (https://commoncrawl.org/faq/)"},
	}

	// Extract Section 1 (VIP) of robots.txt
	vipSection := robotsStr
	if idx := strings.Index(robotsStr, "# --- 2. THE WALL"); idx != -1 {
		vipSection = robotsStr[:idx]
	}

	for _, crawler := range crawlersToVerify {
		// Verify presence in robots.txt VIP section
		expectedDirective := "User-agent: " + crawler.token
		if !strings.Contains(vipSection, expectedDirective) {
			t.Errorf("robots.txt VIP section missing %q", expectedDirective)
		}

		// Verify behavioral match against client-side isBot regex
		if !botRegex.MatchString(crawler.userAgent) {
			t.Errorf("index.html isBot regex failed to detect User-Agent for %s: %q", crawler.name, crawler.userAgent)
		}
	}
}

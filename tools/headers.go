package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func updateCSPHeaders() error {
	htmlBytes, err := os.ReadFile("index.html")
	if err != nil {
		return fmt.Errorf("failed to read index.html: %w", err)
	}

	htmlStr := string(htmlBytes)
	startTag := `<script id="wasm-engine">`
	endTag := "</script>"

	startIdx := strings.Index(htmlStr, startTag)
	if startIdx == -1 {
		return fmt.Errorf("could not find %s tag in index.html", startTag)
	}
	startIdx += len(startTag)

	relEnd := strings.Index(htmlStr[startIdx:], endTag)
	if relEnd == -1 {
		return fmt.Errorf("could not find matching </script> tag for %s in index.html", startTag)
	}

	scriptContent := htmlStr[startIdx : startIdx+relEnd]
	// Normalize CRLF to LF so hash is identical on Windows and Linux/Cloudflare CI
	scriptContent = strings.ReplaceAll(scriptContent, "\r\n", "\n")

	hash := sha256.Sum256([]byte(scriptContent))
	base64Hash := "sha256-" + base64.StdEncoding.EncodeToString(hash[:])

	headersBytes, err := os.ReadFile("_headers")
	if err != nil {
		return fmt.Errorf("failed to read _headers: %w", err)
	}

	cspRegex := regexp.MustCompile(`(script-src[^;]*?)(sha256-[A-Za-z0-9+/=]+)`)
	if !cspRegex.Match(headersBytes) {
		return fmt.Errorf("could not find sha256 hash in script-src in _headers")
	}

	updatedHeaders := cspRegex.ReplaceAll(headersBytes, []byte("${1}"+base64Hash))

	if bytes.Equal(headersBytes, updatedHeaders) {
		fmt.Printf("CSP hash already up to date in _headers: %s\n", base64Hash)
		return nil
	}

	err = os.WriteFile("_headers", updatedHeaders, 0644)
	if err != nil {
		return fmt.Errorf("failed to write _headers: %w", err)
	}

	fmt.Printf("Successfully updated _headers with CSP hash: %s\n", base64Hash)
	return nil
}

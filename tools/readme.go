package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"regexp"
)

func updateReadmeWasmSizes() error {
	wasmBytes, err := os.ReadFile("main.wasm")
	if os.IsNotExist(err) {
		// main.wasm does not exist yet (e.g., Cloudflare build before TinyGo runs, or clean checkout)
		fmt.Println("main.wasm not found; skipping README.md size update.")
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read main.wasm: %w", err)
	}

	rawKB := float64(len(wasmBytes)) / 1024.0

	var gzBuf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	if _, err := gz.Write(wasmBytes); err != nil {
		return fmt.Errorf("failed to compress wasm bytes: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	gzKB := float64(gzBuf.Len()) / 1024.0

	readmeBytes, err := os.ReadFile("README.md")
	if os.IsNotExist(err) {
		// README.md removed or missing (e.g. during Cloudflare deployment cleanup)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read README.md: %w", err)
	}

	re := regexp.MustCompile(`(?i)(\*\*)([0-9.]+)(\s*KB micro-WASM binary\*\*\s*\()([0-9.]+)(\s*KB gzipped)`)
	if !re.Match(readmeBytes) {
		return fmt.Errorf("could not find micro-WASM size pattern in README.md")
	}

	replacement := fmt.Sprintf("${1}%.1f${3}%.1f${5}", rawKB, gzKB)
	updatedReadme := re.ReplaceAll(readmeBytes, []byte(replacement))

	if bytes.Equal(readmeBytes, updatedReadme) {
		fmt.Printf("README.md WASM sizes already up to date: %.1f KB (%.1f KB gzipped)\n", rawKB, gzKB)
		return nil
	}

	if err := os.WriteFile("README.md", updatedReadme, 0644); err != nil {
		return fmt.Errorf("failed to write README.md: %w", err)
	}

	fmt.Printf("Successfully updated README.md with WASM sizes: %.1f KB (%.1f KB gzipped)\n", rawKB, gzKB)
	return nil
}

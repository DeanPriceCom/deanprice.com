package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getCommitSHA() string {
	if sha := strings.TrimSpace(os.Getenv("CF_PAGES_COMMIT_SHA")); sha != "" {
		return sha
	}
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		sha := strings.TrimSpace(string(out))
		if sha != "" {
			return sha
		}
	}
	return "dev"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
	}
	return nil
}

// assembleDist packages all static assets into a clean dist/ directory for Cloudflare Pages.
// It physically isolates all Go source code, build tools, and git databases from public deployment.
func assembleDist() error {
	// 1. Verify build prerequisites
	if _, err := os.Stat("main.wasm"); os.IsNotExist(err) {
		return fmt.Errorf("main.wasm not found. Compile WASM before assembling dist:\n  tinygo build -opt=z -no-debug -panic=trap -target wasm -o main.wasm .")
	}
	if _, err := os.Stat("wasm_exec.js"); os.IsNotExist(err) {
		return fmt.Errorf("wasm_exec.js not found in workspace root")
	}

	// 2. Prepare clean dist/ directory
	if err := os.RemoveAll("dist"); err != nil {
		return fmt.Errorf("failed to clean dist: %w", err)
	}
	if err := os.MkdirAll("dist", 0755); err != nil {
		return fmt.Errorf("failed to create dist: %w", err)
	}

	// 3. Whitelisted explicit root files
	exactFiles := []string{
		"index.html",
		"404.html",
		"main.wasm",
		"wasm_exec.js",
		"manifest.json",
		"robots.txt",
		"sitemap.xml",
		"_headers",
		"_routes.json",
	}

	copiedCount := 0
	for _, f := range exactFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return fmt.Errorf("required deployable file %s not found", f)
		}
		if err := copyFile(f, filepath.Join("dist", f)); err != nil {
			return err
		}
		copiedCount++
	}

	// 4. Whitelisted glob patterns
	patterns := []string{
		"llms*.txt",
		"*.png",
		"*.svg",
		"*.ico",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("glob pattern %s failed: %w", pattern, err)
		}
		for _, match := range matches {
			dest := filepath.Join("dist", filepath.Base(match))
			if err := copyFile(match, dest); err != nil {
				return err
			}
			copiedCount++
		}
	}

	// 5. Copy and dynamically stamp sw.js
	swData, err := os.ReadFile("sw.js")
	if err != nil {
		return fmt.Errorf("failed to read sw.js: %w", err)
	}
	commitSHA := getCommitSHA()
	stampedSW := strings.ReplaceAll(string(swData), "CACHE_VERSION_PLACEHOLDER", commitSHA)
	if err := os.WriteFile(filepath.Join("dist", "sw.js"), []byte(stampedSW), 0644); err != nil {
		return fmt.Errorf("failed to write dist/sw.js: %w", err)
	}
	copiedCount++

	fmt.Printf("Successfully assembled dist/ (%d assets, sw.js cache version: %s)\n", copiedCount, commitSHA)
	return nil
}

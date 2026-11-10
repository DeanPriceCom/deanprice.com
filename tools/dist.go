package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const lastmodPlaceholder = "LASTMOD_PLACEHOLDER"

var (
	reHTMLBlocks = regexp.MustCompile(`(?is)<!--.*?-->|<style\b[^>]*>.*?</style>|<script\b([^>]*)>(.*?)</script>`)
	reJSONLDAttr = regexp.MustCompile(`(?i)(?:^|\s)type\s*=\s*(?:"application/ld\+json[^"]*"|'application/ld\+json[^']*'|application/ld\+json\b)`)
	reDataURI    = regexp.MustCompile(`data:[a-zA-Z0-9/+.-]+;base64,[A-Za-z0-9+/=_-]+`)
	reWhitespace = regexp.MustCompile(`[ \t\r\n]+`)
	reLastmodRow = regexp.MustCompile(`(?m)^[ \t]*<lastmod>LASTMOD_PLACEHOLDER</lastmod>[ \t]*\n?`)
)

func cleanGitEnv() []string {
	raw := os.Environ()
	env := make([]string, 0, len(raw)+1)
	for _, kv := range raw {
		switch {
		case strings.HasPrefix(kv, "GIT_DIR="),
			strings.HasPrefix(kv, "GIT_WORK_TREE="),
			strings.HasPrefix(kv, "GIT_INDEX_FILE="),
			strings.HasPrefix(kv, "GIT_OBJECT_DIRECTORY="),
			strings.HasPrefix(kv, "GIT_ALTERNATE_OBJECT_DIRECTORIES="):
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	return env
}

func gitOutput(repoDir string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = cleanGitEnv()
	return cmd.Output()
}

func getCommitSHA() string {
	if sha := strings.TrimSpace(os.Getenv("CF_PAGES_COMMIT_SHA")); sha != "" {
		return sha
	}
	out, err := gitOutput(".", "rev-parse", "--short", "HEAD")
	if err == nil {
		if sha := strings.TrimSpace(string(out)); sha != "" {
			return sha
		}
	}
	return "dev"
}

// semanticDigest canonicalizes indexable HTML (<head> metadata/links, JSON-LD @graph, and <body> markup)
// while stripping comments, <style> blocks, executable/external <script> blocks, raw base64 payloads, and formatting whitespace.
func semanticDigest(html []byte) [32]byte {
	cleaned := reHTMLBlocks.ReplaceAllFunc(html, func(m []byte) []byte {
		if len(m) >= 7 && bytes.EqualFold(m[:7], []byte("<script")) {
			sub := reHTMLBlocks.FindSubmatch(m)
			if len(sub) == 3 && reJSONLDAttr.Match(sub[1]) {
				body := bytes.TrimSpace(sub[2])
				var parsed any
				if err := json.Unmarshal(body, &parsed); err == nil {
					if canonical, err := json.Marshal(parsed); err == nil {
						body = canonical
					}
				}
				return fmt.Appendf(nil, ` <script type="application/ld+json">%s</script> `, body)
			}
		}
		return []byte(" ")
	})
	cleaned = reDataURI.ReplaceAll(cleaned, []byte("data:"))
	cleaned = reWhitespace.ReplaceAll(cleaned, []byte(" "))
	cleaned = bytes.TrimSpace(cleaned)
	return sha256.Sum256(cleaned)
}

// resolveLastmod walks the first-parent commit history of index.html and returns the UTC YYYY-MM-DD date
// of the most recent commit that altered its semantic digest.
func resolveLastmod(repoDir string) (string, bool) {
	out, err := gitOutput(repoDir, "log", "--first-parent", "--no-renames", "--format=%H %ct", "--", "index.html")
	if err != nil {
		return "", false
	}

	type commitEntry struct {
		sha string
		ct  int64
	}
	var commits []commitEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return "", false
		}
		ct, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return "", false
		}
		commits = append(commits, commitEntry{sha: fields[0], ct: ct})
	}
	if len(commits) == 0 {
		return "", false
	}

	readDigest := func(sha string) ([32]byte, bool) {
		blob, err := gitOutput(repoDir, "show", sha+":index.html")
		if err != nil {
			return [32]byte{}, false
		}
		return semanticDigest(blob), true
	}

	currDigest, ok := readDigest(commits[0].sha)
	if !ok {
		return "", false
	}

	for i := 0; i < len(commits)-1; i++ {
		nextDigest, ok := readDigest(commits[i+1].sha)
		if !ok {
			return "", false
		}
		if currDigest != nextDigest {
			return time.Unix(commits[i].ct, 0).UTC().Format("2006-01-02"), true
		}
		currDigest = nextDigest
	}

	// Reached the oldest listed commit; reject if it is a shallow-clone graft boundary posing as root.
	shallowOut, err := gitOutput(repoDir, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(string(shallowOut)) != "false" {
		return "", false
	}
	return time.Unix(commits[len(commits)-1].ct, 0).UTC().Format("2006-01-02"), true
}

func stampSitemap(tpl []byte, date string, ok bool, strict bool) ([]byte, error) {
	normalized := bytes.ReplaceAll(tpl, []byte("\r\n"), []byte("\n"))
	placeholder := []byte(lastmodPlaceholder)
	if count := bytes.Count(normalized, placeholder); count != 1 {
		return nil, fmt.Errorf("sitemap.xml must contain exactly 1 %s, found %d", lastmodPlaceholder, count)
	}

	if ok {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, fmt.Errorf("invalid lastmod date %q: %w", date, err)
		}
		return bytes.ReplaceAll(normalized, placeholder, []byte(date)), nil
	}

	if strict {
		return nil, fmt.Errorf("failed to resolve semantic lastmod date for index.html in strict CI mode (ensure fetch-depth: 0)")
	}

	if !reLastmodRow.Match(normalized) {
		return nil, fmt.Errorf("sitemap.xml %s must appear inside a standalone <lastmod> line", lastmodPlaceholder)
	}
	return reLastmodRow.ReplaceAll(normalized, nil), nil
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
	// 1. Pre-flight verification before mutating dist/
	if _, err := os.Stat("main.wasm"); os.IsNotExist(err) {
		return fmt.Errorf("main.wasm not found. Compile WASM before assembling dist:\n  tinygo build -opt=z -no-debug -panic=trap -target wasm -o main.wasm .")
	}
	if _, err := os.Stat("wasm_exec.js"); os.IsNotExist(err) {
		return fmt.Errorf("wasm_exec.js not found in workspace root")
	}

	exactFiles := []string{
		"index.html",
		"404.html",
		"wasm-contact-cipher.html",
		"main.wasm",
		"wasm_exec.js",
		"manifest.json",
		"robots.txt",
		"_headers",
		"_redirects",
		"_routes.json",
	}
	for _, f := range exactFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return fmt.Errorf("required deployable file %s not found", f)
		}
	}

	swData, err := os.ReadFile("sw.js")
	if err != nil {
		return fmt.Errorf("failed to read sw.js: %w", err)
	}
	commitSHA := getCommitSHA()
	stampedSW := strings.ReplaceAll(string(swData), "CACHE_VERSION_PLACEHOLDER", commitSHA)

	sitemapData, err := os.ReadFile("sitemap.xml")
	if err != nil {
		return fmt.Errorf("failed to read sitemap.xml: %w", err)
	}
	lastmodDate, lastmodOK := resolveLastmod(".")
	stampedSitemap, err := stampSitemap(sitemapData, lastmodDate, lastmodOK, os.Getenv("GITHUB_ACTIONS") != "")
	if err != nil {
		return err
	}
	if !lastmodOK {
		fmt.Fprintln(os.Stderr, "WARN: could not resolve git semantic lastmod for index.html; omitting <lastmod> from dist/sitemap.xml")
	}

	// 2. Prepare clean dist/ directory
	if err := os.RemoveAll("dist"); err != nil {
		return fmt.Errorf("failed to clean dist: %w", err)
	}
	if err := os.MkdirAll("dist", 0755); err != nil {
		return fmt.Errorf("failed to create dist: %w", err)
	}

	// 3. Copy whitelisted explicit root files
	copiedCount := 0
	for _, f := range exactFiles {
		if err := copyFile(f, filepath.Join("dist", f)); err != nil {
			return err
		}
		copiedCount++
	}

	// 4. Copy whitelisted glob patterns
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

	// 5. Write dynamically stamped sw.js
	if err := os.WriteFile(filepath.Join("dist", "sw.js"), []byte(stampedSW), 0644); err != nil {
		return fmt.Errorf("failed to write dist/sw.js: %w", err)
	}
	copiedCount++

	// 6. Write dynamically stamped sitemap.xml
	if err := os.WriteFile(filepath.Join("dist", "sitemap.xml"), stampedSitemap, 0644); err != nil {
		return fmt.Errorf("failed to write dist/sitemap.xml: %w", err)
	}
	copiedCount++

	lastmodSummary := "omitted"
	if lastmodOK {
		lastmodSummary = lastmodDate
	}
	fmt.Printf("Successfully assembled dist/ (%d assets, sw.js cache: %s, sitemap lastmod: %s)\n", copiedCount, commitSHA, lastmodSummary)
	return nil
}

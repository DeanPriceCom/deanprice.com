package main

import (
	"encoding/xml"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newGitTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// LIFO cleanup clears Windows read-only bits on .git/objects/* before t.TempDir() runs RemoveAll.
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil {
				if d.IsDir() {
					_ = os.Chmod(p, 0777)
				} else {
					_ = os.Chmod(p, 0666)
				}
			}
			return nil
		})
	})
	return dir
}

func gitTestExec(t *testing.T, repoDir string, committerISO string, args ...string) string {
	t.Helper()
	baseArgs := []string{
		"-C", repoDir,
		"-c", "user.name=Test User",
		"-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false",
		"-c", "tag.gpgsign=false",
		"-c", "core.autocrlf=false",
		"-c", "core.fsmonitor=false",
	}
	cmd := exec.Command("git", append(baseArgs, args...)...)
	env := cleanGitEnv()
	env = append(env,
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if committerISO != "" {
		env = append(env,
			"GIT_AUTHOR_DATE="+committerISO,
			"GIT_COMMITTER_DATE="+committerISO,
		)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestSemanticDigest(t *testing.T) {
	baseline := `<!DOCTYPE html>
<html lang="en">
<head>
    <title>Dean Price</title>
    <meta name="description" content="Personal page">
    <link rel="me" href="https://orcid.org/0009-0001-4937-2409">
    <script type="application/ld+json">
    {
        "@context": "https://schema.org",
        "name": "Dean Price",
        "url": "https://www.deanprice.com/"
    }
    </script>
    <style>body { margin: 0; color: #111; }</style>
</head>
<body>
    <!-- build note -->
    <h1>Dean Price</h1>
    <img src="data:image/webp;base64,AAAA==" alt="Email">
    <noscript><style>.card { display: block; }</style></noscript>
    <script src="/wasm_exec.js" defer></script>
    <script id="wasm-engine">var x = 1;</script>
</body>
</html>`

	baseDigest := semanticDigest([]byte(baseline))

	nonSemanticCases := []struct {
		name string
		html string
	}{
		{
			name: "head and noscript CSS changes",
			html: strings.ReplaceAll(
				strings.ReplaceAll(baseline, "margin: 0; color: #111;", "margin: 8px; padding: 4px;"),
				"display: block;", "display: flex;",
			),
		},
		{
			name: "inline runtime JS with comment-like literal",
			html: strings.ReplaceAll(baseline, "var x = 1;", `var x = "<!-- not an html comment"; console.log(x);`),
		},
		{
			name: "external script attributes (fetchpriority)",
			html: strings.ReplaceAll(baseline, `<script src="/wasm_exec.js" defer></script>`, `<script src="/wasm_exec.js" defer fetchpriority="low"></script>`),
		},
		{
			name: "HTML comment containing script-like text",
			html: strings.ReplaceAll(baseline, "<!-- build note -->", "<!-- <script>unclosed in comment -->"),
		},
		{
			name: "re-encoded base64 data URI payload",
			html: strings.ReplaceAll(baseline, "data:image/webp;base64,AAAA==", "data:image/webp;base64,ZZZZ9999+/=="),
		},
		{
			name: "JSON-LD key order and indentation",
			html: strings.ReplaceAll(baseline, `{
        "@context": "https://schema.org",
        "name": "Dean Price",
        "url": "https://www.deanprice.com/"
    }`, `{"url":"https://www.deanprice.com/","name":"Dean Price","@context":"https://schema.org"}`),
		},
	}

	for _, tc := range nonSemanticCases {
		t.Run("unchanged_"+tc.name, func(t *testing.T) {
			if got := semanticDigest([]byte(tc.html)); got != baseDigest {
				t.Errorf("expected non-semantic edit (%s) to preserve digest", tc.name)
			}
		})
	}

	semanticCases := []struct {
		name string
		html string
	}{
		{
			name: "meta description update",
			html: strings.ReplaceAll(baseline, `content="Personal page"`, `content="Updated bio"`),
		},
		{
			name: "new rel=me authority anchor",
			html: strings.ReplaceAll(baseline, `<link rel="me" href="https://orcid.org/0009-0001-4937-2409">`, `<link rel="me" href="https://orcid.org/0009-0001-4937-2409"><link rel="me" href="https://isni.org/isni/0000000531874306">`),
		},
		{
			name: "JSON-LD property value update",
			html: strings.ReplaceAll(baseline, `"name": "Dean Price"`, `"name": "Dean C. Price"`),
		},
		{
			name: "body heading tag change",
			html: strings.ReplaceAll(baseline, "<h1>Dean Price</h1>", "<h2>Dean Price</h2>"),
		},
		{
			name: "image alt text change",
			html: strings.ReplaceAll(baseline, `alt="Email"`, `alt="Email address"`),
		},
	}

	for _, tc := range semanticCases {
		t.Run("changed_"+tc.name, func(t *testing.T) {
			if got := semanticDigest([]byte(tc.html)); got == baseDigest {
				t.Errorf("expected semantic edit (%s) to alter digest", tc.name)
			}
		})
	}
}

func TestResolveLastmod_LinearHistoryUTCAndRevert(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	repoDir := newGitTempDir(t)
	gitTestExec(t, repoDir, "", "init")

	writeFile := func(name, content string) {
		if err := os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	v1 := `<!DOCTYPE html><html><head><title>Dean</title><style>body{margin:0}</style></head><body><h1>Dean</h1><script id="wasm-engine">var a=1;</script></body></html>`
	writeFile("index.html", v1)
	gitTestExec(t, repoDir, "", "add", "index.html")
	gitTestExec(t, repoDir, "2026-10-01T12:00:00Z", "commit", "-m", "initial")

	// Semantic update at 2026-11-07 01:30 +03:00 (= 2026-11-06 22:30 UTC)
	v2 := strings.ReplaceAll(v1, "<h1>Dean</h1>", "<h1>Dean Price</h1>")
	writeFile("index.html", v2)
	gitTestExec(t, repoDir, "", "add", "index.html")
	gitTestExec(t, repoDir, "2026-11-07T01:30:00+03:00", "commit", "-m", "feat: update heading")

	// Non-semantic CSS + runtime JS edit on 2026-11-08
	v3 := strings.ReplaceAll(strings.ReplaceAll(v2, "margin:0", "margin:4px"), "var a=1;", "var a=2;")
	writeFile("index.html", v3)
	gitTestExec(t, repoDir, "", "add", "index.html")
	gitTestExec(t, repoDir, "2026-11-08T14:00:00Z", "commit", "-m", "style: tweak margin and js")

	// Unrelated file commit on 2026-11-09
	writeFile("llms-full.txt", "dissertation note\n")
	gitTestExec(t, repoDir, "", "add", "llms-full.txt")
	gitTestExec(t, repoDir, "2026-11-09T22:50:00Z", "commit", "-m", "docs: update llms-full")

	gotDate, ok := resolveLastmod(repoDir)
	if !ok || gotDate != "2026-11-06" {
		t.Fatalf("resolveLastmod() = (%q, %v), want (\"2026-11-06\", true)", gotDate, ok)
	}

	// Revert semantic content back to v1 on 2026-11-10 -> must count as a semantic change
	writeFile("index.html", v1)
	gitTestExec(t, repoDir, "", "add", "index.html")
	gitTestExec(t, repoDir, "2026-11-10T10:00:00Z", "commit", "-m", "revert: heading")

	gotRevert, ok := resolveLastmod(repoDir)
	if !ok || gotRevert != "2026-11-10" {
		t.Fatalf("after semantic revert, resolveLastmod() = (%q, %v), want (\"2026-11-10\", true)", gotRevert, ok)
	}
}

func TestResolveLastmod_ShallowAndAirGapped(t *testing.T) {
	// 1. Non-git directory returns ok=false
	emptyDir := t.TempDir()
	if d, ok := resolveLastmod(emptyDir); ok || d != "" {
		t.Fatalf("expected non-git dir to return (\"\", false), got (%q, %v)", d, ok)
	}

	// 2. Shallow boundary with only CSS/JS changes above boundary returns ok=false
	repoDir := newGitTempDir(t)
	gitTestExec(t, repoDir, "", "init")

	v1 := `<!DOCTYPE html><html><head><style>a{color:red}</style></head><body><h1>Hi</h1></body></html>`
	if err := os.WriteFile(filepath.Join(repoDir, "index.html"), []byte(v1), 0644); err != nil {
		t.Fatal(err)
	}
	gitTestExec(t, repoDir, "", "add", "index.html")
	gitTestExec(t, repoDir, "2026-10-01T12:00:00Z", "commit", "-m", "c1")
	c1SHA := gitTestExec(t, repoDir, "", "rev-parse", "HEAD")

	v2CSS := strings.ReplaceAll(v1, "color:red", "color:blue")
	if err := os.WriteFile(filepath.Join(repoDir, "index.html"), []byte(v2CSS), 0644); err != nil {
		t.Fatal(err)
	}
	gitTestExec(t, repoDir, "", "add", "index.html")
	gitTestExec(t, repoDir, "2026-11-08T12:00:00Z", "commit", "-m", "c2 css only")
	c2SHA := gitTestExec(t, repoDir, "", "rev-parse", "HEAD")

	shallowPath := filepath.Join(repoDir, ".git", "shallow")

	// Depth-1 shallow boundary at HEAD
	if err := os.WriteFile(shallowPath, []byte(c2SHA+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if d, ok := resolveLastmod(repoDir); ok || d != "" {
		t.Fatalf("depth-1 shallow clone returned (%q, %v), want (\"\", false)", d, ok)
	}

	// Depth-2 shallow boundary at c1 where c2 was CSS-only
	if err := os.WriteFile(shallowPath, []byte(c1SHA+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if d, ok := resolveLastmod(repoDir); ok || d != "" {
		t.Fatalf("depth-2 CSS-only shallow clone returned (%q, %v), want (\"\", false)", d, ok)
	}
}

func TestStampSitemap(t *testing.T) {
	tpl := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n  <url>\n    <loc>https://www.deanprice.com/</loc>\n    <lastmod>LASTMOD_PLACEHOLDER</lastmod>\n  </url>\n</urlset>\n")

	// 1. ok=true substitutes valid date
	stamped, err := stampSitemap(tpl, "2026-11-06", true, true)
	if err != nil {
		t.Fatalf("stampSitemap(ok=true) unexpected error: %v", err)
	}
	if !strings.Contains(string(stamped), "<lastmod>2026-11-06</lastmod>") || strings.Contains(string(stamped), lastmodPlaceholder) {
		t.Errorf("unexpected stamped output:\n%s", string(stamped))
	}

	// 2. ok=false, strict=false omits <lastmod> cleanly and remains valid XML
	omitted, err := stampSitemap(tpl, "", false, false)
	if err != nil {
		t.Fatalf("stampSitemap(ok=false, strict=false) unexpected error: %v", err)
	}
	if strings.Contains(string(omitted), "lastmod") || strings.Contains(string(omitted), lastmodPlaceholder) {
		t.Errorf("expected <lastmod> line to be omitted, got:\n%s", string(omitted))
	}
	var parsed struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(omitted, &parsed); err != nil {
		t.Fatalf("omitted sitemap is not valid XML: %v", err)
	}
	if len(parsed.URLs) != 1 || parsed.URLs[0].Loc != "https://www.deanprice.com/" || parsed.URLs[0].LastMod != "" {
		t.Errorf("unexpected parsed URLs after omitting lastmod: %+v", parsed.URLs)
	}

	// 3. ok=false, strict=true returns error
	if _, err := stampSitemap(tpl, "", false, true); err == nil {
		t.Errorf("expected error when ok=false and strict=true")
	}

	// 4. Missing or duplicate placeholder returns error
	if _, err := stampSitemap([]byte("<urlset></urlset>"), "2026-11-06", true, false); err == nil {
		t.Errorf("expected error when placeholder is missing")
	}
	if _, err := stampSitemap([]byte("LASTMOD_PLACEHOLDER LASTMOD_PLACEHOLDER"), "2026-11-06", true, false); err == nil {
		t.Errorf("expected error when placeholder is duplicated")
	}

	// 5. Malformed date returns error
	if _, err := stampSitemap(tpl, "not-a-date", true, false); err == nil {
		t.Errorf("expected error for malformed date")
	}
}

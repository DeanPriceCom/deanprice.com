package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var countryRegex = regexp.MustCompile(`^[A-Za-z0-9]{2}$`)
var distFlag = flag.Bool("dist", false, "Serve from dist/ instead of workspace root")

func init() {
	// Ensure proper MIME types for WebAssembly, images, text, SVG, and icons
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".webp", "image/webp")
	_ = mime.AddExtensionType(".json", "application/json")
	_ = mime.AddExtensionType(".txt", "text/plain; charset=utf-8")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".ico", "image/x-icon")
}

const aliasRobotsTxt = `# Disallow AI models, training scrapers, and web archives
User-agent: GPTBot
User-agent: ChatGPT-User
User-agent: ClaudeBot
User-agent: Claude-User
User-agent: CCBot
User-agent: PerplexityBot
User-agent: Bytespider
User-agent: Amazonbot
User-agent: meta-externalagent
User-agent: cohere-ai
User-agent: YouBot
User-agent: AI2Bot
User-agent: ia_archiver
User-agent: archive.org_bot
User-agent: special_archiver
User-agent: archive.today
Disallow: /

# Allow Search Indexers and Social Bots to fetch root so they read noindex & OG tags
User-agent: Googlebot
User-agent: Bingbot
User-agent: WhatsApp
User-agent: TelegramBot
User-agent: Twitterbot
User-agent: LinkedInBot
User-agent: Applebot
User-agent: facebookexternalhit
User-agent: Facebot
User-agent: Slackbot
User-agent: Slackbot-LinkExpanding
User-agent: Discordbot
Allow: /$
Allow: /index.html
Allow: /favicon.ico
Allow: /*.png
Allow: /*.svg
Disallow: /

# Catch-all for other crawlers
User-agent: *
Disallow: /
`

func isAliasHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	return host == "deanprice.uk" || host == "deanprice.tr" || host == "deanprice.ie" ||
		strings.HasSuffix(host, ".pages.dev")
}

func extractCountry(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	if strings.HasSuffix(host, ".tr") {
		return "TR"
	}
	if strings.HasSuffix(host, ".ie") {
		return "IE"
	}
	if strings.HasSuffix(host, ".uk") {
		return "GB"
	}

	country := r.URL.Query().Get("country")
	if countryRegex.MatchString(country) {
		return strings.ToUpper(country)
	}
	return ""
}

func resolveBaseDir(useDist bool) string {
	if useDist {
		if _, err := os.Stat("dist"); err == nil {
			return "dist"
		}
		if _, err := os.Stat(filepath.Join("../..", "dist")); err == nil {
			return filepath.Join("../..", "dist")
		}
		log.Fatal("FATAL: dist/ directory not found. Run 'go run ./tools -task=dist' first.")
	}
	if _, err := os.Stat("index.html"); err == nil {
		return "."
	}
	return filepath.Join("../..")
}

func readServerFile(baseDir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(baseDir, name))
}

func handleRootHTML(w http.ResponseWriter, r *http.Request, htmlBytes []byte) {
	country := extractCountry(r)

	meta := fmt.Sprintf("<meta name=\"cf-country\" content=\"%s\">\n    ", country)
	injected := bytes.Replace(htmlBytes, []byte("<head>"), []byte("<head>\n    "+meta), 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")

	if isAliasHost(r.Host) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")
		reRobots := regexp.MustCompile(`<meta name="robots" content="[^"]*">`)
		injected = reRobots.ReplaceAll(injected, []byte(`<meta name="robots" content="noindex, nofollow, noarchive, nosnippet">`))
	} else if r.URL.Query().Has("shield") {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	}

	w.WriteHeader(http.StatusOK)
	w.Write(injected)
}

func newDevHandler(customBaseDir ...string) http.Handler {
	baseDir := ""
	if len(customBaseDir) > 0 && customBaseDir[0] != "" {
		baseDir = customBaseDir[0]
	} else {
		baseDir = resolveBaseDir(false)
	}

	fileServer := http.FileServer(http.Dir(baseDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := path.Clean(r.URL.Path)
		if cleanPath == "/robots.txt" && isAliasHost(r.Host) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(aliasRobotsTxt))
			return
		}

		if cleanPath == "/" || cleanPath == "/index.html" {
			htmlBytes, err := readServerFile(baseDir, "index.html")
			if err != nil {
				http.Error(w, "Failed to load index.html", http.StatusInternalServerError)
				return
			}

			handleRootHTML(w, r, htmlBytes)
			return
		}

		relPath := filepath.Join(baseDir, strings.TrimPrefix(cleanPath, "/"))
		info, err := os.Stat(relPath)
		if err != nil || info.IsDir() {
			notFoundBytes, err404 := readServerFile(baseDir, "404.html")
			if err404 == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				w.Write(notFoundBytes)
				return
			}
		}

		fileServer.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()

	baseDir := resolveBaseDir(*distFlag)
	mux := http.NewServeMux()
	mux.Handle("/", newDevHandler(baseDir))

	log.Printf("Go Dev Server running at http://localhost:8080 (serving from %s)\n", baseDir)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

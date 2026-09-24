package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"mime"
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

func extractCountry(r *http.Request) string {
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

	if r.URL.Query().Has("shield") {
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

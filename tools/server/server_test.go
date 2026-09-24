package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractCountry(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"default empty", "http://localhost:8080/", ""},
		{"country param TR", "http://localhost:8080/?country=TR", "TR"},
		{"country param lowercase tr", "http://localhost:8080/?country=tr", "TR"},
		{"country param IE", "http://localhost:8080/?country=IE", "IE"},
		{"country param lowercase ie", "http://localhost:8080/?country=ie", "IE"},
		{"country param T1", "http://localhost:8080/?country=T1", "T1"},
		{"country param lowercase t1", "http://localhost:8080/?country=t1", "T1"},
		{"valid 2-digit number code 12", "http://localhost:8080/?country=12", "12"},
		{"invalid country code fallback", "http://localhost:8080/?country=TURKEY", ""},
		{"invalid length fallback", "http://localhost:8080/?country=1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			got := extractCountry(req)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestServerMetaInjection(t *testing.T) {
	countries := []struct {
		query    string
		expected string
	}{
		{"?country=TR", `<meta name="cf-country" content="TR">`},
		{"?country=IE", `<meta name="cf-country" content="IE">`},
		{"?country=GB", `<meta name="cf-country" content="GB">`},
		{"?country=T1", `<meta name="cf-country" content="T1">`},
		{"", `<meta name="cf-country" content="">`},
	}

	for _, c := range countries {
		t.Run("Query "+c.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost:8080/"+c.query, nil)
			country := extractCountry(req)
			meta := `<meta name="cf-country" content="` + country + `">`
			if !strings.Contains(meta, c.expected) {
				t.Errorf("expected %q, got %q", c.expected, meta)
			}
		})
	}
}

func TestHandleRootHTML_XRobotsTag(t *testing.T) {
	tests := []struct {
		url         string
		wantNoindex bool
	}{
		{"http://localhost:8080/", false},
		{"http://localhost:8080/?country=TR", false},
		{"http://localhost:8080/?country=GB", false},
		{"http://localhost:8080/?shield=blocked", true},
		{"http://localhost:8080/?shield=legacy", true},
		{"http://localhost:8080/?shield=js", true},
	}

	sampleHTML := []byte("<!DOCTYPE html><html><head></head><body></body></html>")

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			handleRootHTML(w, req, sampleHTML)

			got := w.Header().Get("X-Robots-Tag")
			if tt.wantNoindex {
				if got != "noindex, nofollow, noarchive" {
					t.Errorf("expected X-Robots-Tag: noindex, nofollow, noarchive, got %q", got)
				}
			} else {
				if got != "" {
					t.Errorf("expected no X-Robots-Tag, got %q", got)
				}
			}
		})
	}
}

func TestNewDevHandler_Root(t *testing.T) {
	handler := newDevHandler()
	req := httptest.NewRequest("GET", "http://localhost:8080/?country=TR", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `<meta name="cf-country" content="TR">`) {
		t.Errorf("expected meta tag in body")
	}
}

func TestNewDevHandler_Dist(t *testing.T) {
	distDir := "dist"
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		distDir = filepath.Join("../..", "dist")
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			t.Skip("dist directory does not exist, skipping dist handler test")
		}
	}

	handler := newDevHandler(distDir)
	req := httptest.NewRequest("GET", "http://localhost:8080/?country=IE", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `<meta name="cf-country" content="IE">`) {
		t.Errorf("expected meta tag in body for dist handler")
	}
}

func TestNewDevHandler_NotFound(t *testing.T) {
	handler := newDevHandler()
	req := httptest.NewRequest("GET", "http://localhost:8080/nonexistent-route-xyz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected status 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "404") {
		t.Errorf("expected 404.html content in response body")
	}
}

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var iconFiles = []struct {
	alt      string
	filename string
}{
	{"Email", "email.webp"},
	{"Phone", "phone.webp"},
	{"WhatsApp", "whatsapp.webp"},
	{"Facebook", "facebook.webp"},
	{"Instagram", "instagram.webp"},
}

// syncAssets reads source assets from .assets/ and inlines them as Base64 data URIs into index.html and 404.html.
func syncAssets() error {
	baseDir := "."
	if _, err := os.Stat("index.html"); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join("..", "..", "index.html")); err == nil {
			baseDir = filepath.Join("..", "..")
		}
	}

	assetsDir := filepath.Join(baseDir, ".assets")
	indexPath := filepath.Join(baseDir, "index.html")
	f404Path := filepath.Join(baseDir, "404.html")

	// 1. Prepare Patrick Hand font data URI
	fontPath := filepath.Join(assetsDir, "patrickhand.woff2")
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", fontPath, err)
	}
	fontDataURI := "data:font/woff2;base64," + base64.StdEncoding.EncodeToString(fontBytes)
	fontPattern := regexp.MustCompile(`(src:\s*url\()data:font/woff2;base64,[^)]*(\)\s*format\('woff2'\))`)

	// 2. Synchronize index.html (Font + Icons)
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", indexPath, err)
	}
	indexContent := string(indexBytes)
	indexModified := false

	if !fontPattern.MatchString(indexContent) {
		return fmt.Errorf("patrickhand @font-face placeholder not found in %s", indexPath)
	}
	newIndexContent := fontPattern.ReplaceAllString(indexContent, "${1}"+fontDataURI+"${2}")
	if newIndexContent != indexContent {
		indexContent = newIndexContent
		indexModified = true
	}

	for _, icon := range iconFiles {
		filePath := filepath.Join(assetsDir, icon.filename)
		imgBytes, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		iconDataURI := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(imgBytes)

		// Pattern matches <img src="data:image/webp;base64,..." width="123" height="123" alt="<Alt>"
		pattern := regexp.MustCompile(`(<img\s+src=")data:image/webp;base64,[^"]*("\s+width="123"\s+height="123"\s+alt="` + regexp.QuoteMeta(icon.alt) + `")`)
		if !pattern.MatchString(indexContent) {
			return fmt.Errorf("image placeholder for alt=%q not found in %s", icon.alt, indexPath)
		}

		newContent := pattern.ReplaceAllString(indexContent, `${1}`+iconDataURI+`${2}`)
		if newContent != indexContent {
			indexContent = newContent
			indexModified = true
		}
	}

	if indexModified {
		if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", indexPath, err)
		}
		fmt.Printf("Successfully synchronized .assets/ into %s\n", indexPath)
	} else {
		fmt.Printf("Assets in %s are already up to date\n", indexPath)
	}

	// 3. Synchronize 404.html (Font)
	f404Bytes, err := os.ReadFile(f404Path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", f404Path, err)
	}
	f404Content := string(f404Bytes)

	if !fontPattern.MatchString(f404Content) {
		return fmt.Errorf("patrickhand @font-face placeholder not found in %s", f404Path)
	}
	newF404Content := fontPattern.ReplaceAllString(f404Content, "${1}"+fontDataURI+"${2}")
	if newF404Content != f404Content {
		if err := os.WriteFile(f404Path, []byte(newF404Content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", f404Path, err)
		}
		fmt.Printf("Successfully synchronized .assets/ into %s\n", f404Path)
	} else {
		fmt.Printf("Assets in %s are already up to date\n", f404Path)
	}

	return nil
}

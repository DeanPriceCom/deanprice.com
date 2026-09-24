package contact

import (
	"strings"

	"deanprice.com/internal/cipher"
)

// IsDevOrPreviewHost checks if the hostname matches allowed dev or preview environments.
func IsDevOrPreviewHost(h string) bool {
	return h == "" || h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]" ||
		strings.HasSuffix(h, ".pages.dev") ||
		strings.HasSuffix(h, ".local") ||
		strings.HasSuffix(h, ".lan") ||
		strings.HasPrefix(h, "192.168.") ||
		strings.HasPrefix(h, "10.") ||
		strings.HasPrefix(h, "172.") ||
		strings.HasPrefix(h, "100.") ||
		strings.HasSuffix(h, ".ts.net")
}

func decrypt(encoded []byte, rawHost string, nonce string) string {
	if len(encoded) == 0 {
		return ""
	}

	hostname := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(rawHost)), "www.")
	if IsDevOrPreviewHost(hostname) {
		hostname = "deanprice.com"
	}

	return string(cipher.Crypt(encoded, masterSeed, hostname, "load", nonce))
}

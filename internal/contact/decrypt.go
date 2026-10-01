package contact

import (
	"strings"

	"deanprice.com/internal/cipher"
)

// isPrivateIPv4 performs a zero-allocation single-pass validation of private IPv4 ranges:
// - 10.0.0.0/8 (RFC 1918)
// - 172.16.0.0/12 (RFC 1918)
// - 192.168.0.0/16 (RFC 1918)
// - 100.64.0.0/10 (RFC 6598 CGNAT / Tailscale)
func isPrivateIPv4(h string) bool {
	if len(h) < 7 || len(h) > 15 || h[0] != '1' {
		return false
	}
	var octets [4]int
	idx, val, digits := 0, 0, 0

	for i := 0; i < len(h); i++ {
		ch := h[i]
		if ch == '.' {
			if digits == 0 || val > 255 || idx >= 3 {
				return false
			}
			octets[idx] = val
			idx++
			val, digits = 0, 0
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
		if digits > 0 && val == 0 {
			return false
		}
		val = val*10 + int(ch-'0')
		if val > 255 {
			return false
		}
		digits++
	}
	if digits == 0 || idx != 3 || val > 255 {
		return false
	}
	octets[3] = val

	return octets[0] == 10 ||
		(octets[0] == 172 && octets[1] >= 16 && octets[1] <= 31) ||
		(octets[0] == 192 && octets[1] == 168) ||
		(octets[0] == 100 && octets[1] >= 64 && octets[1] <= 127)
}

// IsDevOrPreviewHost checks if the hostname matches allowed dev or preview environments.
func IsDevOrPreviewHost(h string) bool {
	return h == "" || h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]" ||
		strings.HasSuffix(h, ".pages.dev") ||
		strings.HasSuffix(h, ".local") ||
		strings.HasSuffix(h, ".lan") ||
		strings.HasSuffix(h, ".ts.net") ||
		isPrivateIPv4(h)
}

// IsAuthorizedDomain checks if the hostname is an authorized production or preview host.
func IsAuthorizedDomain(h string) bool {
	h = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), "www.")
	if h == "deanprice.com" || h == "deanprice.uk" || h == "deanprice.tr" || h == "deanprice.ie" {
		return true
	}
	return IsDevOrPreviewHost(h)
}

func decrypt(encoded []byte, rawHost string, nonce string) string {
	if len(encoded) == 0 {
		return ""
	}

	hostname := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(rawHost)), "www.")
	if IsAuthorizedDomain(hostname) {
		hostname = "deanprice.com"
	}

	return string(cipher.Crypt(encoded, masterSeed, hostname, "load", nonce))
}

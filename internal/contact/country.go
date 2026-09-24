package contact

import "strings"

// ResolveCountry maps a raw country or network code (from Cloudflare or dev preview)
// to the active regional configuration.
func ResolveCountry(loc string) string {
	switch strings.ToUpper(strings.TrimSpace(loc)) {
	case "TR":
		return "TR"

	case "GB", "IM", "JE", "GG", // UK & Crown Dependencies
		"", "XX",                // Undetected / Unknown location
		"T1":                    // Tor exit node (treated as unverified/fallback)
		return "GB"

	default:
		// All verified international countries
		return "IE"
	}
}

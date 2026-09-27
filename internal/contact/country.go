package contact

import "strings"

// ResolveCountry maps a raw country or network code (from Cloudflare or dev preview)
// to the active regional configuration. It preserves backward compatibility by delegating
// to ResolveCountryWithHost with an empty hostname.
func ResolveCountry(loc string) string {
	return ResolveCountryWithHost(loc, "")
}

// ResolveCountryWithHost maps location and hostname to the active regional configuration.
// Regional alias domains (.tr, .ie, .uk) are strictly deterministic and take absolute
// priority. For generic domains (e.g. deanprice.com, localhost), the location code
// (from Cloudflare GeoIP or preview mock) determines regional resolution.
func ResolveCountryWithHost(loc string, hostname string) string {
	// 1. Regional alias domains are strictly deterministic
	cleanHost := strings.ToLower(strings.TrimSpace(hostname))
	cleanHost = strings.TrimPrefix(cleanHost, "www.")
	if strings.HasSuffix(cleanHost, ".tr") {
		return "TR"
	}
	if strings.HasSuffix(cleanHost, ".ie") {
		return "IE"
	}
	if strings.HasSuffix(cleanHost, ".uk") {
		return "GB"
	}

	// 2. Generic hosts (deanprice.com, localhost) use loc (GeoIP or mock)
	cleanLoc := strings.ToUpper(strings.TrimSpace(loc))
	switch cleanLoc {
	case "TR":
		return "TR"

	case "GB", "IM", "JE", "GG", // UK & Crown Dependencies
		"", "XX", // Undetected / Unknown location
		"T1": // Tor exit node (treated as unverified/fallback)
		return "GB"

	default:
		// All verified international countries
		return "IE"
	}
}

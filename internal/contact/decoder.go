package contact

import (
	"strings"
)

// ResolveContact decodes the obfuscated contact payload for the given channel,
// applying regional localization and domain validation/normalization.
// It is pure Go, containing zero browser/DOM dependencies, making it fully testable natively.
func ResolveContact(channel string, region string, rawHost string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "email":
		return decrypt(emailBytes, rawHost, "email")
	case "phone":
		resolvedRegion := ResolveCountry(region)
		if b, ok := phoneMap[resolvedRegion]; ok {
			return decrypt(b, rawHost, "phone_"+resolvedRegion)
		}
	case "wa", "whatsapp":
		resolvedRegion := ResolveCountry(region)
		if b, ok := waMap[resolvedRegion]; ok {
			return decrypt(b, rawHost, "wa_"+resolvedRegion)
		}
	}
	return ""
}

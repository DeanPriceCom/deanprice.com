package contact

import (
	"fmt"
	"os"
	"testing"

	"deanprice.com/internal/defaults"
)

var (
	testEmail   = getEnv("SECRET_EMAIL", defaults.DefaultEmail)
	testPhoneGB = getEnv("SECRET_PHONE_GB", defaults.DefaultPhoneGB)
	testPhoneIE = getEnv("SECRET_PHONE_IE", defaults.DefaultPhoneIE)
	testPhoneTR = getEnv("SECRET_PHONE_TR", defaults.DefaultPhoneTR)
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestEmailDecryptionAllHosts(t *testing.T) {
	hosts := []string{
		"",
		"deanprice.com",
		"www.deanprice.com",
		"deanprice.uk",
		"www.deanprice.uk",
		"deanprice.tr",
		"www.deanprice.tr",
		"deanprice.ie",
		"www.deanprice.ie",
		"localhost",
		"127.0.0.1",
		"::1",
		"[::1]",
		"192.168.1.100",
		"10.0.0.5",
		"172.20.10.2",
		"100.64.0.1",
		"iphone.local",
		"laptop.lan",
		"phone.ts.net",
		"preview.pages.dev",
	}
	expected := "mailto:" + testEmail

	for _, host := range hosts {
		got := decrypt(emailBytes, host, "email")
		if got != expected {
			t.Errorf("Host: %s, got %q, want %q", host, got, expected)
		}
	}
}

func TestPhoneDecryptionAllRegions(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"GB", "tel:" + testPhoneGB},
		{"IE", "tel:" + testPhoneIE},
		{"TR", "tel:" + testPhoneTR},
	}

	hosts := []string{"deanprice.com", "deanprice.uk", "deanprice.tr", "deanprice.ie", "www.deanprice.uk"}

	for _, tt := range tests {
		for _, host := range hosts {
			got := decrypt(phoneMap[tt.region], host, "phone_"+tt.region)
			if got != tt.expected {
				t.Errorf("Region: %s, Host: %s, got %q, want %q", tt.region, host, got, tt.expected)
			}
		}
	}
}

func TestWhatsAppDecryptionAllRegions(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"GB", "https://wa.me/" + defaults.CleanDigits(testPhoneGB)},
		{"IE", "https://wa.me/" + defaults.CleanDigits(testPhoneIE)},
		{"TR", "https://wa.me/" + defaults.CleanDigits(testPhoneTR)},
	}

	hosts := []string{"deanprice.com", "deanprice.uk", "deanprice.tr", "deanprice.ie", "www.deanprice.tr"}

	for _, tt := range tests {
		for _, host := range hosts {
			got := decrypt(waMap[tt.region], host, "wa_"+tt.region)
			if got != tt.expected {
				t.Errorf("Region: %s, Host: %s, got %q, want %q", tt.region, host, got, tt.expected)
			}
		}
	}
}

func TestDomainLockProtection(t *testing.T) {
	unauthorizedHosts := []string{"evil-scraper.com", "unauthorized-mirror.com", "deanprice.org", "phishing-deanprice.com"}
	for _, host := range unauthorizedHosts {
		got := decrypt(emailBytes, host, "email")
		if got == "mailto:"+testEmail {
			t.Errorf("Security flaw: %s successfully decrypted email: %q", host, got)
		}
	}
}

func TestResolveCountryWithHost(t *testing.T) {
	// 1. Regional alias domains take absolute precedence over loc
	if got := ResolveCountryWithHost("TR", "deanprice.uk"); got != "GB" {
		t.Errorf("Expected GB for deanprice.uk even with loc=TR, got %q", got)
	}
	if got := ResolveCountryWithHost("GB", "deanprice.tr"); got != "TR" {
		t.Errorf("Expected TR for deanprice.tr even with loc=GB, got %q", got)
	}
	if got := ResolveCountryWithHost("GB", "deanprice.ie"); got != "IE" {
		t.Errorf("Expected IE for deanprice.ie even with loc=GB, got %q", got)
	}

	// 2. Generic hosts (deanprice.com, localhost) respect loc code
	if got := ResolveCountryWithHost("TR", "deanprice.com"); got != "TR" {
		t.Errorf("Expected TR for deanprice.com with loc=TR, got %q", got)
	}
	if got := ResolveCountryWithHost("IE", "localhost"); got != "IE" {
		t.Errorf("Expected IE for localhost with loc=IE, got %q", got)
	}
	if got := ResolveCountryWithHost("GB", "preview.pages.dev"); got != "GB" {
		t.Errorf("Expected GB for preview with loc=GB, got %q", got)
	}
	if got := ResolveCountryWithHost("", "deanprice.com"); got != "GB" {
		t.Errorf("Expected default GB for deanprice.com with empty loc, got %q", got)
	}

	// 3. Offline / un-injected fallback: inspect hostname suffix
	fallbackTests := []struct {
		host     string
		expected string
	}{
		{"deanprice.tr", "TR"},
		{"www.deanprice.tr", "TR"},
		{"preview.deanprice.tr", "TR"},
		{"DEANPRICE.TR", "TR"},
		{"deanprice.ie", "IE"},
		{"www.deanprice.ie", "IE"},
		{"DEANPRICE.IE", "IE"},
		{"deanprice.uk", "GB"},
		{"www.deanprice.uk", "GB"},
		{"DEANPRICE.UK", "GB"},
		{"deanprice.com", "GB"},
		{"www.deanprice.com", "GB"},
		{"localhost", "GB"},
		{"", "GB"},
	}

	for _, tt := range fallbackTests {
		got := ResolveCountryWithHost("", tt.host)
		if got != tt.expected {
			t.Errorf("ResolveCountryWithHost('', %q) = %q, want %q", tt.host, got, tt.expected)
		}
	}
}

func TestEdgeCasesAndFallbacks(t *testing.T) {
	// 1. Nil slice handling
	if got := decrypt(nil, "deanprice.com", "email"); got != "" {
		t.Errorf("Expected empty string for nil slice, got %q", got)
	}

	// 2. Empty slice handling
	if got := decrypt([]byte{}, "deanprice.com", "email"); got != "" {
		t.Errorf("Expected empty string for empty slice, got %q", got)
	}

	// 3. Uppercase & mixed-case dev host normalization
	devHosts := []string{"LOCALHOST", "LocalHost", "127.0.0.1", "PREVIEW.PAGES.DEV", "MYPHONE.LOCAL", "WWW.DEANPRICE.COM"}
	expected := "mailto:" + testEmail
	for _, host := range devHosts {
		got := decrypt(emailBytes, host, "email")
		if got != expected {
			t.Errorf("Case-insensitive host normalization failed for %s: got %q, want %q", host, got, expected)
		}
	}

	// 4. Empty host normalization (direct file:/// or offline preview)
	if got := decrypt(emailBytes, "", "email"); got != expected {
		t.Errorf("Empty host decryption failed: got %q, want %q", got, expected)
	}
}

func TestResolveCountry(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Turkey
		{"TR", "TR"},
		{"tr", "TR"},
		{"  tr  ", "TR"},

		// UK & Crown Dependencies
		{"GB", "GB"},
		{"gb", "GB"},
		{"IM", "GB"},
		{"im", "GB"},
		{"JE", "GB"},
		{"je", "GB"},
		{"GG", "GB"},
		{"gg", "GB"},
		{"  gb  ", "GB"},

		// Undetected / Unknown location -> GB fallback
		{"", "GB"},
		{"   ", "GB"},
		{"XX", "GB"},
		{"xx", "GB"},
		{"  xx  ", "GB"},

		// Tor exit node -> GB fallback
		{"T1", "GB"},
		{"t1", "GB"},
		{"  t1  ", "GB"},

		// Verified international destinations -> IE
		{"IE", "IE"},
		{"ie", "IE"},
		{"US", "IE"},
		{"us", "IE"},
		{"FR", "IE"},
		{"DE", "IE"},
		{"AU", "IE"},
		{"CA", "IE"},
		{"  us  ", "IE"},
		{"OTHER", "IE"},
		{"12", "IE"},
	}

	for _, tt := range tests {
		t.Run("input_"+tt.input, func(t *testing.T) {
			got := ResolveCountry(tt.input)
			if got != tt.expected {
				t.Errorf("ResolveCountry(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolveContact(t *testing.T) {
	expectedEmail := "mailto:" + testEmail
	expectedPhoneGB := "tel:" + testPhoneGB
	expectedPhoneIE := "tel:" + testPhoneIE
	expectedPhoneTR := "tel:" + testPhoneTR
	expectedWAGB := "https://wa.me/" + defaults.CleanDigits(testPhoneGB)
	expectedWAIE := "https://wa.me/" + defaults.CleanDigits(testPhoneIE)
	expectedWATR := "https://wa.me/" + defaults.CleanDigits(testPhoneTR)

	// 1. Channel normalization & regional lookups
	regionalTests := []struct {
		channel  string
		region   string
		expected string
	}{
		{"email", "", expectedEmail},
		{"EMAIL", "GB", expectedEmail},
		{"  email  ", "TR", expectedEmail},

		{"phone", "GB", expectedPhoneGB},
		{"Phone", "IM", expectedPhoneGB}, // Crown dep -> GB
		{"PHONE", "JE", expectedPhoneGB}, // Crown dep -> GB
		{"phone", "TR", expectedPhoneTR},
		{"phone", "IE", expectedPhoneIE},
		{"phone", "US", expectedPhoneIE}, // International -> IE fallback
		{"phone", "", expectedPhoneGB},   // Empty -> GB fallback

		{"whatsapp", "GB", expectedWAGB},
		{"wa", "GB", expectedWAGB},
		{"WhatsApp", "TR", expectedWATR},
		{"WA", "TR", expectedWATR},
		{"whatsapp", "IE", expectedWAIE},
		{"whatsapp", "FR", expectedWAIE}, // International -> IE fallback
	}

	for _, tt := range regionalTests {
		t.Run(fmt.Sprintf("%s_%s", tt.channel, tt.region), func(t *testing.T) {
			got := ResolveContact(tt.channel, tt.region, "deanprice.com")
			if got != tt.expected {
				t.Errorf("ResolveContact(%q, %q, host) = %q, want %q", tt.channel, tt.region, got, tt.expected)
			}
		})
	}

	// 2. Alias host resolution
	aliasTests := []struct {
		channel  string
		region   string
		host     string
		expected string
	}{
		{"phone", "GB", "deanprice.uk", expectedPhoneGB},
		{"whatsapp", "GB", "deanprice.uk", expectedWAGB},
		{"phone", "GB", "www.deanprice.uk", expectedPhoneGB},
		{"phone", "TR", "deanprice.tr", expectedPhoneTR},
		{"whatsapp", "TR", "deanprice.tr", expectedWATR},
		{"phone", "TR", "www.deanprice.tr", expectedPhoneTR},
		{"phone", "IE", "deanprice.ie", expectedPhoneIE},
		{"whatsapp", "IE", "deanprice.ie", expectedWAIE},
		{"phone", "IE", "www.deanprice.ie", expectedPhoneIE},
	}
	for _, tt := range aliasTests {
		t.Run(fmt.Sprintf("%s_%s_%s", tt.channel, tt.region, tt.host), func(t *testing.T) {
			got := ResolveContact(tt.channel, tt.region, tt.host)
			if got != tt.expected {
				t.Errorf("ResolveContact(%q, %q, %q) = %q, want %q", tt.channel, tt.region, tt.host, got, tt.expected)
			}
		})
	}

	// 3. Dev and Preview host normalization
	devHosts := []string{"localhost", "127.0.0.1", "::1", "[::1]", "", "iphone.local", "laptop.lan", "preview.pages.dev", "phone.ts.net"}
	for _, host := range devHosts {
		t.Run("dev_host_"+host, func(t *testing.T) {
			got := ResolveContact("email", "", host)
			if got != expectedEmail {
				t.Errorf("Dev host %q failed email resolution: got %q, want %q", host, got, expectedEmail)
			}
		})
	}

	// 3. Security: Hostile domain protection
	gotEvil := ResolveContact("email", "", "evil-scraper.com")
	if gotEvil == expectedEmail {
		t.Errorf("Security flaw: evil-scraper.com resolved legitimate email: %q", gotEvil)
	}

	// 4. Invalid inputs
	if got := ResolveContact("unknown_channel", "GB", "deanprice.com"); got != "" {
		t.Errorf("Expected empty string for unknown channel, got %q", got)
	}
}

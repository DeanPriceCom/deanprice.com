package defaults

import "regexp"

const (
	DefaultEmail   = "hello@example.com"
	DefaultPhoneGB = "+447700900000"
	DefaultPhoneIE = "+353877000000"
	DefaultPhoneTR = "+905320000000"
)

var nonDigitRegex = regexp.MustCompile(`[^0-9]`)

// CleanDigits strips all non-numeric characters from a phone number string.
func CleanDigits(phone string) string {
	return nonDigitRegex.ReplaceAllString(phone, "")
}

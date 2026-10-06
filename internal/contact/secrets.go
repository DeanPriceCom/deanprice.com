package contact

// Package-level variables initialized to default fallbacks.
// When tools generates data.go, its init() function overwrites these with
// environment-specific or freshly salted keystreams.
// On clean checkouts where data.go has not yet been generated, these defaults
// ensure the package compiles and unit tests pass cleanly out of the box.
var (
	masterSeed uint64 = 0x123456789abcdef0

	emailBytes = []byte{143, 67, 230, 217, 102, 203, 77, 161, 182, 73, 167, 182, 30, 173, 53, 197, 217, 137, 98, 164, 121, 132, 99, 42}

	phoneMap = map[string][]byte{
		"GB": {207, 250, 161, 103, 196, 128, 245, 221, 119, 131, 164, 54, 216, 149, 210, 139, 180},
		"IE": {136, 51, 224, 181, 48, 211, 34, 111, 157, 16, 162, 57, 162, 138, 160, 70, 102},
		"TR": {65, 143, 24, 241, 168, 57, 79, 41, 144, 113, 17, 120, 40, 227, 190, 170, 151},
	}

	waMap = map[string][]byte{
		"GB": {37, 35, 151, 228, 252, 51, 134, 219, 63, 75, 254, 25, 121, 27, 208, 8, 75, 46, 90, 255, 116, 102, 217, 145, 141, 205},
		"IE": {61, 121, 10, 141, 161, 151, 34, 9, 45, 22, 130, 49, 1, 131, 186, 66, 25, 100, 61, 212, 59, 65, 203, 106, 195, 23},
		"TR": {205, 32, 99, 36, 41, 127, 223, 245, 19, 151, 51, 53, 131, 160, 54, 222, 158, 122, 149, 159, 100, 26, 134, 22, 175, 16},
	}
)

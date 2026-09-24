package cipher

const (
	KnuthMultiplier uint64 = 0x5851f42d4c957f2d
	KnuthIncrement  uint64 = 0x14057b7ef767814f
)

// SplitMix64 is a 64-bit pseudo-random number generator step.
func SplitMix64(x uint64) uint64 {
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// MixString folds a string into a 64-bit state using Knuth's LCG.
func MixString(state uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		state = (state * KnuthMultiplier) + uint64(s[i])
	}
	return state
}

// Crypt performs the symmetric XOR keystream operation on data given a seed, domain, event, and nonce.
// Since XOR is symmetric, Crypt(Crypt(data, ...), ...) == data.
func Crypt(data []byte, seed uint64, domain, event, nonce string) []byte {
	state := seed
	state = MixString(state, domain)
	state = MixString(state, event)
	state = MixString(state, nonce)

	out := make([]byte, len(data))
	for i, b := range data {
		state = (state * KnuthMultiplier) + KnuthIncrement
		mixed := SplitMix64(state)
		out[i] = b ^ byte(mixed>>56) ^ byte(i)
	}
	return out
}

package cipher

import (
	"bytes"
	"testing"
)

func TestCipherSymmetry(t *testing.T) {
	original := []byte("mailto:hello@example.com")
	seed := uint64(0x123456789abcdef0)
	domain := "deanprice.com"
	event := "touchstart"
	nonce := "email"

	encrypted := Crypt(original, seed, domain, event, nonce)
	if bytes.Equal(original, encrypted) {
		t.Fatalf("Encrypted payload should not match original")
	}

	decrypted := Crypt(encrypted, seed, domain, event, nonce)
	if !bytes.Equal(original, decrypted) {
		t.Fatalf("Decrypted payload %q does not match original %q", string(decrypted), string(original))
	}
}

func TestCipherDifferentInputs(t *testing.T) {
	data := []byte("test-data-payload")
	seed := uint64(0x123456789abcdef0)

	enc1 := Crypt(data, seed, "deanprice.com", "touchstart", "nonce")
	enc2 := Crypt(data, seed, "otherdomain.com", "touchstart", "nonce")
	enc3 := Crypt(data, seed, "deanprice.com", "mouseenter", "nonce")
	enc4 := Crypt(data, seed, "deanprice.com", "touchstart", "different_nonce")
	enc5 := Crypt(data, seed+1, "deanprice.com", "touchstart", "nonce")

	if bytes.Equal(enc1, enc2) {
		t.Errorf("Different domains produced identical ciphertexts")
	}
	if bytes.Equal(enc1, enc3) {
		t.Errorf("Different events produced identical ciphertexts")
	}
	if bytes.Equal(enc1, enc4) {
		t.Errorf("Different nonces produced identical ciphertexts")
	}
	if bytes.Equal(enc1, enc5) {
		t.Errorf("Different seeds produced identical ciphertexts")
	}
}

func TestSplitMix64Deterministic(t *testing.T) {
	val1 := SplitMix64(0x42)
	val2 := SplitMix64(0x42)
	if val1 != val2 {
		t.Errorf("SplitMix64 is non-deterministic: %x vs %x", val1, val2)
	}
	if val1 == 0 {
		t.Errorf("SplitMix64 returned 0 for non-zero input")
	}
}

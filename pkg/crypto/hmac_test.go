package crypto

import "testing"

func TestGetSHA256(t *testing.T) {
	const text = "some data to hash"
	const secret = "secret"
	const expected = "ab7bca798ad67207c5717d6a33090562d90ec29817af683c5f9f5d9da78a0f5d"
	result := GetSHA256(text, secret)
	if result != expected {
		t.Errorf("wrong sha1, expected: %s, have: %s", expected, result)
	}
}

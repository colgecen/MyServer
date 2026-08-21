package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// SignEntry returns HMAC-SHA256 of the JSON entry.
func SignEntry(key []byte, entry any) string {
	b, _ := json.Marshal(entry)
	mac := hmac.New(sha256.New, key)
	mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyEntry(key []byte, entry any, sig string) bool {
	expected := SignEntry(key, entry)
	return hmac.Equal([]byte(expected), []byte(sig))
}

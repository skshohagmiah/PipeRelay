package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func Sign(secret string, payload []byte) (signature string, timestamp int64) {
	timestamp = time.Now().Unix()
	toSign := fmt.Sprintf("%d.%s", timestamp, string(payload))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(toSign))
	sig := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("v1=%s", sig), timestamp
}

func Verify(secret string, payload []byte, timestamp int64, signature string) bool {
	toSign := fmt.Sprintf("%d.%s", timestamp, string(payload))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(toSign))
	expected := fmt.Sprintf("v1=%s", hex.EncodeToString(mac.Sum(nil)))

	return hmac.Equal([]byte(expected), []byte(signature))
}

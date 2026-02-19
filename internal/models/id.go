package models

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

func NewID(prefix string) string {
	t := time.Now()
	entropy := ulid.Monotonic(mrand.New(mrand.NewSource(t.UnixNano())), 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return fmt.Sprintf("%s_%s", prefix, id.String())
}

func NewAPIKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return fmt.Sprintf("pk_%s", string(b))
}

func NewSecret() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 40)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return fmt.Sprintf("whsec_%s", string(b))
}

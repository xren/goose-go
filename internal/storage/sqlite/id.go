package sqlite

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return hex.EncodeToString(b[:])
}

package auth

import (
	"bytes"
	"crypto/rsa"
	"testing"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	var privBuf, pubBuf bytes.Buffer
	if err := GenerateKeyPair(&privBuf, &pubBuf); err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	key, err := ParsePrivateKey(privBuf.Bytes())
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}
	return key
}

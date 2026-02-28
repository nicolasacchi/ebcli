package auth

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrivateKey_PKCS8(t *testing.T) {
	var privBuf, pubBuf bytes.Buffer
	if err := GenerateKeyPair(&privBuf, &pubBuf); err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	path := filepath.Join(t.TempDir(), "test.pem")
	if err := os.WriteFile(path, privBuf.Bytes(), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	key, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey PKCS#8: %v", err)
	}
	if key.N.BitLen() != KeySize {
		t.Errorf("key size = %d, want %d", key.N.BitLen(), KeySize)
	}
}

func TestLoadPrivateKey_PKCS1(t *testing.T) {
	key := generateTestKey(t)

	derBytes := x509.MarshalPKCS1PrivateKey(key)
	pemData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: derBytes})

	path := filepath.Join(t.TempDir(), "test.pem")
	if err := os.WriteFile(path, pemData, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey PKCS#1: %v", err)
	}
	if loaded.N.BitLen() != KeySize {
		t.Errorf("key size = %d, want %d", loaded.N.BitLen(), KeySize)
	}
}

func TestLoadPrivateKey_NotFound(t *testing.T) {
	_, err := LoadPrivateKey("nonexistent.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadPrivateKey_InvalidPEM(t *testing.T) {
	_, err := ParsePrivateKey([]byte("not a PEM"))
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestLoadPrivateKey_UnsupportedType(t *testing.T) {
	p := []byte("-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----\n")
	_, err := ParsePrivateKey(p)
	if err == nil {
		t.Fatal("expected error for unsupported PEM type")
	}
}

func TestGenerateKeyPair(t *testing.T) {
	var privBuf, pubBuf bytes.Buffer
	if err := GenerateKeyPair(&privBuf, &pubBuf); err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	if privBuf.Len() == 0 {
		t.Error("private key is empty")
	}
	if pubBuf.Len() == 0 {
		t.Error("public key is empty")
	}

	key, err := ParsePrivateKey(privBuf.Bytes())
	if err != nil {
		t.Fatalf("ParsePrivateKey on generated key: %v", err)
	}
	if key.N.BitLen() != KeySize {
		t.Errorf("generated key size = %d, want %d", key.N.BitLen(), KeySize)
	}
}

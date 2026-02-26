package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
)

const KeySize = 4096

// LoadPrivateKey reads a PEM file and parses it as an RSA private key.
// Supports both PKCS#8 ("BEGIN PRIVATE KEY") and PKCS#1 ("BEGIN RSA PRIVATE KEY") formats.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}
	return ParsePrivateKey(data)
}

// ParsePrivateKey parses PEM-encoded RSA private key data.
// Tries PKCS#8 first, falls back to PKCS#1.
func ParsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key data")
	}

	switch block.Type {
	case "PRIVATE KEY":
		// PKCS#8 (browser-generated SubtleCrypto keys)
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS#8 private key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA (got %T)", key)
		}
		return rsaKey, nil

	case "RSA PRIVATE KEY":
		// PKCS#1 (openssl genrsa)
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS#1 private key: %w", err)
		}
		return key, nil

	default:
		return nil, fmt.Errorf("unsupported PEM block type %q (expected PRIVATE KEY or RSA PRIVATE KEY)", block.Type)
	}
}

// GenerateKeyPair creates a new 4096-bit RSA keypair and writes the
// private key (PKCS#8 PEM) and public key (PKIX PEM) to the given writers.
func GenerateKeyPair(privOut, pubOut io.Writer) error {
	key, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return fmt.Errorf("generating RSA key: %w", err)
	}

	// Private key as PKCS#8
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}
	if err := pem.Encode(privOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("encoding private key PEM: %w", err)
	}

	// Public key as PKIX
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("marshaling public key: %w", err)
	}
	if err := pem.Encode(pubOut, &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}); err != nil {
		return fmt.Errorf("encoding public key PEM: %w", err)
	}

	return nil
}

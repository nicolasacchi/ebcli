package auth

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	JWTIssuer   = "enablebanking.com"
	JWTAudience = "api.enablebanking.com"
	MaxTTL      = 24 * time.Hour
	DefaultTTL  = 1 * time.Hour
)

// GenerateJWT creates an RS256-signed JWT for the Enable Banking API.
// kid is set to the appID (application UUID from registration).
// TTL is clamped to MaxTTL (24h). Zero TTL defaults to 1h.
func GenerateJWT(privateKey *rsa.PrivateKey, appID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if ttl > MaxTTL {
		ttl = MaxTTL
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    JWTIssuer,
		Audience:  jwt.ClaimStrings{JWTAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = appID

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signed, nil
}

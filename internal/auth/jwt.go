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

// ebClaims uses a string "aud" instead of an array.
// Enable Banking rejects the standard JWT array form ["api.enablebanking.com"].
type ebClaims struct {
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func (c ebClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Exp, 0)), nil
}
func (c ebClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Iat, 0)), nil
}
func (c ebClaims) GetNotBefore() (*jwt.NumericDate, error) { return nil, nil }
func (c ebClaims) GetIssuer() (string, error)              { return c.Iss, nil }
func (c ebClaims) GetSubject() (string, error)              { return "", nil }
func (c ebClaims) GetAudience() (jwt.ClaimStrings, error)   { return jwt.ClaimStrings{c.Aud}, nil }

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
	claims := ebClaims{
		Iss: JWTIssuer,
		Aud: JWTAudience,
		Iat: now.Unix(),
		Exp: now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = appID

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signed, nil
}

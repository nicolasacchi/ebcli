package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateJWT_Structure(t *testing.T) {
	key := generateTestKey(t)

	appID := "cf589be3-3755-465b-a8df-a90a16a31403"
	token, err := GenerateJWT(key, appID, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parsing JWT: %v", err)
	}

	if alg := parsed.Method.Alg(); alg != "RS256" {
		t.Errorf("alg = %q, want RS256", alg)
	}
	if kid, ok := parsed.Header["kid"]; !ok || kid != appID {
		t.Errorf("kid = %v, want %q", kid, appID)
	}

	iss, err := parsed.Claims.GetIssuer()
	if err != nil || iss != JWTIssuer {
		t.Errorf("iss = %q, want %q", iss, JWTIssuer)
	}

	aud, err := parsed.Claims.GetAudience()
	if err != nil || len(aud) != 1 || aud[0] != JWTAudience {
		t.Errorf("aud = %v, want [%q]", aud, JWTAudience)
	}

	exp, err := parsed.Claims.GetExpirationTime()
	if err != nil || exp == nil {
		t.Fatal("missing exp claim")
	}
	iat, err := parsed.Claims.GetIssuedAt()
	if err != nil || iat == nil {
		t.Fatal("missing iat claim")
	}

	diff := exp.Time.Sub(iat.Time)
	if diff < 59*time.Minute || diff > 61*time.Minute {
		t.Errorf("exp-iat = %v, want ~1h", diff)
	}
}

func TestGenerateJWT_TTLClamping(t *testing.T) {
	key := generateTestKey(t)

	tests := []struct {
		name    string
		ttl     time.Duration
		wantTTL time.Duration
	}{
		{"zero defaults to 1h", 0, DefaultTTL},
		{"negative defaults to 1h", -time.Hour, DefaultTTL},
		{"normal 2h", 2 * time.Hour, 2 * time.Hour},
		{"exceeds max clamped to 24h", 48 * time.Hour, MaxTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateJWT(key, "test-id", tt.ttl)
			if err != nil {
				t.Fatalf("GenerateJWT: %v", err)
			}

			parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				return &key.PublicKey, nil
			})
			if err != nil {
				t.Fatalf("parsing JWT: %v", err)
			}

			exp, _ := parsed.Claims.GetExpirationTime()
			iat, _ := parsed.Claims.GetIssuedAt()
			diff := exp.Time.Sub(iat.Time)

			tolerance := 2 * time.Second
			if diff < tt.wantTTL-tolerance || diff > tt.wantTTL+tolerance {
				t.Errorf("TTL = %v, want ~%v", diff, tt.wantTTL)
			}
		})
	}
}

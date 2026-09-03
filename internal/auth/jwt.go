package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Validator struct {
	key              *rsa.PublicKey
	issuer, audience string
}
type Claims struct {
	Subdomain string `json:"subdomain"`
	jwt.RegisteredClaims
}

func New(publicPEM, issuer, audience string) (*Validator, error) {
	b, _ := pem.Decode([]byte(publicPEM))
	if b == nil {
		return nil, fmt.Errorf("invalid JWT_PUBLIC_KEY PEM")
	}
	var rsaKey *rsa.PublicKey
	if key, err := x509.ParsePKIXPublicKey(b.Bytes); err == nil {
		rsaKey, _ = key.(*rsa.PublicKey)
	} else if key, err := x509.ParsePKCS1PublicKey(b.Bytes); err == nil {
		rsaKey = key
	}
	if rsaKey == nil {
		return nil, fmt.Errorf("JWT public key must be an RSA PEM key")
	}
	return &Validator{rsaKey, issuer, audience}, nil
}

func (v *Validator) Validate(header string) (Claims, error) {
	var c Claims
	if !strings.HasPrefix(header, "Bearer ") {
		return c, fmt.Errorf("missing bearer token")
	}
	t, err := jwt.ParseWithClaims(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), &c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected signing algorithm")
		}
		return v.key, nil
	}, jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !t.Valid {
		return Claims{}, fmt.Errorf("invalid token")
	}
	if c.ExpiresAt == nil || c.NotBefore == nil {
		return Claims{}, fmt.Errorf("required time claims missing")
	}
	if c.Subdomain == "" {
		return Claims{}, fmt.Errorf("subdomain claim missing")
	}
	return c, nil
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

type Target struct {
	Token     string  `json:"token"`
	RPS       float64 `json:"rps"`
	Subdomain string  `json:"subdomain"`
}

type Config struct {
	Port         string
	Timeout      time.Duration
	BodyLimit    int64
	LogLevel     string
	RestyDebug   bool
	LockboxID    string // Metadata for Yandex Lockbox; the JSON is supplied through one of the sources below.
	Targets      map[string]Target
	JWTPublicKey string
	JWTIssuer    string
	JWTAudience  string
}

func Load() (Config, error) {

	c := Config{Port: env("PORT", "8080"), LogLevel: env("LOG_LEVEL", "info"), BodyLimit: 10 << 20}
	log.Info().Msg(env("PORT", "serv"))
	var err error
	c.Timeout, err = time.ParseDuration(env("REQUEST_TIMEOUT", "1s"))
	if err != nil || c.Timeout <= 0 {
		return c, fmt.Errorf("REQUEST_TIMEOUT must be a positive duration")
	}
	if v := os.Getenv("BODY_LIMIT"); v != "" {
		c.BodyLimit, err = strconv.ParseInt(v, 10, 64)
		if err != nil || c.BodyLimit < 1 {
			return c, fmt.Errorf("BODY_LIMIT must be positive")
		}
	}

	c.RestyDebug, _ = strconv.ParseBool(env("RESTY_DEBUG", "false"))
	c.LockboxID = os.Getenv("LOCKBOX_SECRET_ID")
	if c.LockboxID == "" {
		return c, fmt.Errorf("LOCKBOX_SECRET_ID is required")
	}
	c.JWTPublicKey, c.JWTIssuer, c.JWTAudience = os.Getenv("JWT_PUBLIC_KEY"), os.Getenv("JWT_ISSUER"), os.Getenv("JWT_AUDIENCE")
	if file := os.Getenv("JWT_PUBLIC_KEY_FILE"); c.JWTPublicKey == "" && file != "" {
		b, e := os.ReadFile(file)
		if e != nil {
			return c, fmt.Errorf("read JWT_PUBLIC_KEY_FILE: %w", e)
		}
		c.JWTPublicKey = string(b)
	}
	if c.JWTPublicKey == "" || c.JWTIssuer == "" || c.JWTAudience == "" {
		return c, fmt.Errorf("JWT_PUBLIC_KEY, JWT_ISSUER and JWT_AUDIENCE are required")
	}
	raw := os.Getenv("LOCKBOX_SECRET_JSON")
	if raw == "" {
		if file := os.Getenv("LOCKBOX_SECRET_FILE"); file != "" {
			b, e := os.ReadFile(file)
			if e != nil {
				return c, fmt.Errorf("read LOCKBOX_SECRET_FILE: %w", e)
			}
			raw = string(b)
		}
	}
	if raw == "" {
		return c, fmt.Errorf("LOCKBOX_SECRET_JSON or LOCKBOX_SECRET_FILE is required")
	}
	if err := json.Unmarshal([]byte(raw), &c.Targets); err != nil || len(c.Targets) == 0 {
		return c, fmt.Errorf("invalid Lockbox JSON")
	}
	host := regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	for name, t := range c.Targets {
		// Allow the compact Lockbox format to use its key as the amoCRM subdomain.
		if t.Subdomain == "" {
			t.Subdomain = name
			c.Targets[name] = t
		}
		if t.Token == "" || t.RPS < 1 || !host.MatchString(t.Subdomain) {
			return c, fmt.Errorf("invalid Lockbox target %q", name)
		}
	}
	return c, nil
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

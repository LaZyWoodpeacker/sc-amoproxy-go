package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"proxy-api/internal/auth"
	"proxy-api/internal/client"
	"proxy-api/internal/config"
	"proxy-api/internal/httpx"
	"proxy-api/internal/limiter"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	cfg    config.Config
	auth   *auth.Validator
	client *client.Client
	lim    *limiter.Limiter
	log    zerolog.Logger
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func New(c config.Config, a *auth.Validator, cl *client.Client, l *limiter.Limiter, log zerolog.Logger) *Handler {
	return &Handler{c, a, cl, l, log}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Info().Msg(r.URL.Path)
	id := r.Header.Get("X-Request-ID")
	if id == "" {
		id = fmt.Sprintf("%x", time.Now().UnixNano())
	}
	rw := &responseWriter{ResponseWriter: w}
	defer func() {
		event := h.log.Info()
		if rw.status >= 400 {
			event = h.log.Warn().Str("error_type", "http_error")
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		subdomain := ""
		if len(parts) > 0 {
			subdomain = parts[0]
		}
		event.Str("request_id", id).Str("method", r.Method).Str("path", r.URL.Path).
			Str("subdomain", subdomain).
			Dur("duration", time.Since(start)).Int("status", rw.status).Msg("request")
	}()
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.Timeout)
	defer cancel()
	rw.Header().Set("X-Request-ID", id)
	if r.URL.Path == "/health" {
		if r.Method == "GET" {
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte("ok"))
			return
		}
		h.error(rw, 405, id, "bad_request")
		return
	}
	switch r.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		h.error(rw, 405, id, "bad_request")
		return
	}
	claims, err := h.auth.Validate(r.Header.Get("Authorization"))
	if err != nil {
		h.error(rw, 401, id, "unauthorized")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		h.error(rw, 400, id, "bad_request")
		return
	}
	sub := parts[0]
	if claims.Subdomain != sub {
		h.error(rw, 403, id, "forbidden")
		return
	}
	target, ok := h.cfg.Targets[sub]
	if !ok {
		h.error(rw, 404, id, "not_found")
		return
	}
	if !h.lim.Allow(ctx, sub, target.RPS) {
		h.error(rw, 429, id, "rate_limited")
		return
	}
	body := http.MaxBytesReader(rw, r.Body, h.cfg.BodyLimit)
	data, err := io.ReadAll(body)
	if err != nil {
		h.error(rw, 400, id, "bad_request")
		return
	}
	url := "https://" + target.Subdomain + ".amocrm.ru" + r.URL.Path[len(sub)+1:]
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	result := h.client.Do(ctx, r.Method, url, target.Token, id, r.Header, bytes.NewReader(data))
	if result.Err != nil {
		if result.Timeout || ctx.Err() != nil {
			h.error(rw, 504, id, "request_timeout")
		} else {
			h.error(rw, 502, id, "upstream_error")
		}
		return
	}
	httpx.CopySafe(rw.Header(), result.Header)
	rw.WriteHeader(result.Status)
	_, _ = rw.Write(result.Body)
}

func (h *Handler) error(w http.ResponseWriter, status int, id, kind string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": kind, "request_id": id})
}

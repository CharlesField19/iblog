package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID))
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		log.Printf("http request_id=%s method=%s path=%s status=%d duration=%s", requestID, r.Method, r.URL.Path, wrapped.status, time.Since(started).Round(time.Millisecond))
		if isAuditableMutation(r) {
			user, _ := r.Context().Value(usernameKey).(string)
			log.Printf("audit request_id=%s user=%s action=%s path=%s status=%d", requestID, userOrAnonymous(user), r.Method, r.URL.Path, wrapped.status)
		}
	})
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}
func userOrAnonymous(user string) string {
	if user == "" {
		return "anonymous"
	}
	return user
}
func isAuditableMutation(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		return false
	}
	return strings.HasPrefix(r.URL.Path, "/api/v1/admin/") || r.URL.Path == "/api/v1/auth/logout" || r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/setup"
}
func (s *server) securityHeaders(next http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, origin := range strings.Split(env("BLOG_CORS_ORIGINS", ""), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if env("BLOG_COOKIE_SECURE", "false") == "true" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if origin := r.Header.Get("Origin"); origin != "" && allowed[origin] {
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api/") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				fail(w, 500, "internal", "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

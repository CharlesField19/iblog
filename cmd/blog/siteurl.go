package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// parsePublicBase normalizes an explicitly configured public base URL.
// Only an http(s) origin is accepted: no path prefix, query, fragment or
// userinfo. A trailing slash is tolerated and removed.
func parsePublicBase(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	if strings.ContainsAny(u.Host, "\\%") {
		return "", false
	}
	return scheme + "://" + strings.ToLower(u.Host), true
}

func isLoopbackBase(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func requestUsesLoopbackHost(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := r.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func firstForwardedValue(value string) string {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			return part
		}
	}
	return ""
}

func trustProxy() bool {
	return env("BLOG_TRUST_PROXY", "false") == "true"
}

// requestBaseURL derives the public origin from the request. Forwarded headers
// are honoured only when BLOG_TRUST_PROXY=true, because they are trivially
// spoofable when the app is reachable directly.
func requestBaseURL(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if trustProxy() {
		if forwarded := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
			scheme = strings.ToLower(forwarded)
		}
		if forwarded := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
			host = forwarded
		}
	}
	return parsePublicBase(scheme + "://" + host)
}

// listenBaseURL is the last-resort local address, using the real listen port
// instead of hard-coding 11190.
func listenBaseURL() string {
	_, port, err := net.SplitHostPort(env("BLOG_ADDR", ":11190"))
	if err != nil || port == "" {
		return "http://127.0.0.1:11190"
	}
	return "http://127.0.0.1:" + port
}

// siteSettingURL reads the public address saved from the admin console.
func (s *server) siteSettingURL() string {
	var raw string
	if err := s.db.QueryRow("SELECT value_json FROM site_settings WHERE setting_key='site'").Scan(&raw); err != nil {
		return ""
	}
	var site struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(raw), &site); err != nil {
		return ""
	}
	return site.URL
}

// staticBaseURL is the configured base without looking at the request; it is
// used for the startup warning only.
func (s *server) staticBaseURL() (string, bool) {
	for _, candidate := range []string{
		s.siteSettingURL(),
		os.Getenv("BLOG_PUBLIC_URL"),
		os.Getenv("BLOG_PUBLIC_ORIGIN"),
	} {
		if base, ok := parsePublicBase(candidate); ok {
			return base, true
		}
	}
	return "", false
}

// canonicalBaseURL resolves the public origin used by robots.txt and
// sitemap.xml. Precedence: admin setting, BLOG_PUBLIC_URL, BLOG_PUBLIC_ORIGIN,
// the request itself, then BLOG_ADDR. A loopback placeholder is ignored when
// the request arrives for a real host, so the shipped compose default cannot
// mask a correctly proxied deployment.
func (s *server) canonicalBaseURL(r *http.Request) string {
	for _, candidate := range []string{
		s.siteSettingURL(),
		os.Getenv("BLOG_PUBLIC_URL"),
		os.Getenv("BLOG_PUBLIC_ORIGIN"),
	} {
		if base, ok := parsePublicBase(candidate); ok {
			if !isLoopbackBase(base) || requestUsesLoopbackHost(r) {
				return base
			}
		}
	}
	if base, ok := requestBaseURL(r); ok {
		return base
	}
	return listenBaseURL()
}

// validateProductionConfig rejects combinations that would publish insecure
// cookies or generate a public sitemap for the wrong host. Local HTTP
// preview remains valid when secure cookies and a public HTTPS origin are unset.
func validateProductionConfig(s *server) error {
	secure := env("BLOG_COOKIE_SECURE", "false") == "true"
	base, ok := s.staticBaseURL()
	if ok && strings.HasPrefix(base, "https://") && !secure {
		return fmt.Errorf("https public site URL %s requires BLOG_COOKIE_SECURE=true", base)
	}
	if secure && (!ok || isLoopbackBase(base)) {
		return fmt.Errorf("BLOG_COOKIE_SECURE=true requires a non-loopback public site URL in admin, BLOG_PUBLIC_URL, or BLOG_PUBLIC_ORIGIN")
	}
	if trustProxy() && secure && ok && strings.HasPrefix(base, "http://") {
		return fmt.Errorf("BLOG_TRUST_PROXY=true with secure cookies requires an https public site URL, got %s", base)
	}
	return nil
}

// productionConfigWarning reports deployments that start but still publish
// session cookies over an unencrypted public connection. The local preview
// default (loopback origin) stays silent; anything else is operator-facing.
func productionConfigWarning(s *server) string {
	if env("BLOG_COOKIE_SECURE", "false") == "true" || !trustProxy() {
		return ""
	}
	if base, ok := s.staticBaseURL(); ok && isLoopbackBase(base) {
		return ""
	}
	return "BLOG_TRUST_PROXY=true with BLOG_COOKIE_SECURE=false: public HTTPS deployments must set BLOG_PUBLIC_ORIGIN=https://<domain> and BLOG_COOKIE_SECURE=true"
}

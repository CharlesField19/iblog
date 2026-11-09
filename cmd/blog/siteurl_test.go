package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParsePublicBase(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://blog.example.com", "https://blog.example.com", true},
		{" https://blog.example.com/ ", "https://blog.example.com", true},
		{"http://127.0.0.1:11190", "http://127.0.0.1:11190", true},
		{"https://BLOG.example.com", "https://blog.example.com", true},
		{"HTTPS://blog.example.com", "https://blog.example.com", true},
		{"", "", false},
		{"blog.example.com", "", false},
		{"ftp://blog.example.com", "", false},
		{"https://blog.example.com/sub", "", false},
		{"https://blog.example.com/?q=1", "", false},
		{"https://user:pass@blog.example.com", "", false},
		{"javascript:alert(1)", "", false},
	}
	for _, tc := range cases {
		got, ok := parsePublicBase(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parsePublicBase(%q) = %q,%v want %q,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCanonicalBaseURLPrecedence(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:11190/robots.txt", nil)
	if got := s.canonicalBaseURL(req); got != "http://127.0.0.1:11190" {
		t.Fatalf("request fallback = %q", got)
	}

	t.Setenv("BLOG_PUBLIC_ORIGIN", "https://origin.example.com")
	req = httptest.NewRequest(http.MethodGet, "http://blog.example.com/robots.txt", nil)
	if got := s.canonicalBaseURL(req); got != "https://origin.example.com" {
		t.Fatalf("origin fallback = %q", got)
	}

	t.Setenv("BLOG_PUBLIC_URL", "https://url.example.com")
	if got := s.canonicalBaseURL(req); got != "https://url.example.com" {
		t.Fatalf("public url precedence = %q", got)
	}

	if _, err := s.db.Exec("INSERT INTO site_settings(setting_key,value_json,updated_at) VALUES('site',?,?)", `{"url":"https://admin.example.com"}`, now()); err != nil {
		t.Fatal(err)
	}
	if got := s.canonicalBaseURL(req); got != "https://admin.example.com" {
		t.Fatalf("admin precedence = %q", got)
	}
}

func TestLoopbackPlaceholderDoesNotMaskRealHost(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_PUBLIC_ORIGIN", "http://127.0.0.1:11190")
	req := httptest.NewRequest(http.MethodGet, "http://blog.example.com/robots.txt", nil)
	if got := s.canonicalBaseURL(req); got != "http://blog.example.com" {
		t.Fatalf("loopback placeholder leaked to real host: %q", got)
	}

	t.Setenv("BLOG_TRUST_PROXY", "true")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "blog.example.com")
	if got := s.canonicalBaseURL(req); got != "https://blog.example.com" {
		t.Fatalf("forwarded origin = %q", got)
	}

	local := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:11190/robots.txt", nil)
	if got := s.canonicalBaseURL(local); got != "http://127.0.0.1:11190" {
		t.Fatalf("local preview base = %q", got)
	}
}

func TestListenBaseURLUsesConfiguredPort(t *testing.T) {
	t.Setenv("BLOG_ADDR", ":10003")
	if got := listenBaseURL(); got != "http://127.0.0.1:10003" {
		t.Fatalf("listen base = %q", got)
	}
	s := testServer(t)
	req := &http.Request{Method: http.MethodGet, Host: "", Header: http.Header{}}
	if got := s.canonicalBaseURL(req); got != "http://127.0.0.1:10003" {
		t.Fatalf("canonical fallback = %q", got)
	}
}

func TestRobotsAndSitemapUseConfiguredBase(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	for _, row := range []struct{ slug, status, title string }{
		{"hello", "published", "Hello"},
		{"draft-post", "draft", "Draft"},
	} {
		if _, err := s.db.Exec(
			"INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)",
			row.slug, row.title, "", "", row.status, now(), now(),
		); err != nil {
			t.Fatal(err)
		}
	}

	robots := httptest.NewRecorder()
	s.robots(robots, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	body := robots.Body.String()
	if !strings.Contains(body, "Sitemap: https://blog.example.com/sitemap.xml") {
		t.Fatalf("robots sitemap line missing: %q", body)
	}
	if strings.Contains(body, "/articles?") {
		t.Fatalf("robots still blocks the literal query list: %q", body)
	}

	sitemap := httptest.NewRecorder()
	s.sitemap(sitemap, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	xml := sitemap.Body.String()
	for _, want := range []string{
		"<loc>https://blog.example.com/</loc>",
		"<loc>https://blog.example.com/articles</loc>",
		"<loc>https://blog.example.com/articles/hello</loc>",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("sitemap missing %q: %q", want, xml)
		}
	}
	if strings.Contains(xml, "draft-post") {
		t.Fatalf("sitemap contains a draft: %q", xml)
	}
}

func TestSaveSettingValidatesSiteURL(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)

	for _, tc := range []struct{ in, stored string }{
		{"https://blog.example.com/", "https://blog.example.com"},
		{" http://127.0.0.1:8080 ", "http://127.0.0.1:8080"},
	} {
		if code, body := call(t, h, "PUT", "/api/v1/admin/settings/site", map[string]any{"name": "iBlog", "url": tc.in}, token); code != http.StatusOK {
			t.Fatalf("save %q: status=%d body=%v", tc.in, code, body)
		}
		var raw string
		if err := s.db.QueryRow("SELECT value_json FROM site_settings WHERE setting_key='site'").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var site struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(raw), &site); err != nil {
			t.Fatal(err)
		}
		if site.URL != tc.stored {
			t.Fatalf("stored url = %q want %q", site.URL, tc.stored)
		}
	}

	for _, value := range []any{"javascript:alert(1)", "https://blog.example.com/path", "blog.example.com", 42} {
		if code, body := call(t, h, "PUT", "/api/v1/admin/settings/site", map[string]any{"url": value}, token); code != http.StatusBadRequest {
			t.Fatalf("accepted invalid url %v: status=%d body=%v", value, code, body)
		}
	}

	t.Setenv("BLOG_PUBLIC_URL", "https://env.example.com")
	if code, body := call(t, h, "PUT", "/api/v1/admin/settings/site", map[string]any{"url": ""}, token); code != http.StatusOK {
		t.Fatalf("clear url: status=%d body=%v", code, body)
	}
	robots := httptest.NewRecorder()
	s.robots(robots, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if !strings.Contains(robots.Body.String(), "Sitemap: https://env.example.com/sitemap.xml") {
		t.Fatalf("cleared url did not fall back to env: %q", robots.Body.String())
	}
}

func TestRequestOriginHonorsTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:11190/api/v1/auth/login", nil)
	req.Host = "blog.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := requestOrigin(req); got != "http://blog.example.com" {
		t.Fatalf("untrusted forwarded proto was used: %q", got)
	}
	t.Setenv("BLOG_TRUST_PROXY", "true")
	if got := requestOrigin(req); got != "https://blog.example.com" {
		t.Fatalf("trusted forwarded proto was ignored: %q", got)
	}
}

func TestExpectedRequestOriginIgnoresLoopbackPlaceholder(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:11190/api/v1/auth/login", nil)
	req.Host = "blog.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	t.Setenv("BLOG_TRUST_PROXY", "true")
	t.Setenv("BLOG_PUBLIC_ORIGIN", "http://127.0.0.1:11190")
	if got := expectedRequestOrigin(req); got != "https://blog.example.com" {
		t.Fatalf("loopback placeholder blocked the proxied origin: %q", got)
	}
	t.Setenv("BLOG_PUBLIC_ORIGIN", "https://configured.example.com")
	if got := expectedRequestOrigin(req); got != "https://configured.example.com" {
		t.Fatalf("configured origin was not preferred: %q", got)
	}
}

func TestProductionConfigRejectsContradictions(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_COOKIE_SECURE", "false")
	t.Setenv("BLOG_TRUST_PROXY", "false")
	t.Setenv("BLOG_PUBLIC_URL", "")
	t.Setenv("BLOG_PUBLIC_ORIGIN", "")
	if err := validateProductionConfig(s); err != nil {
		t.Fatalf("local preview was rejected: %v", err)
	}
	t.Setenv("BLOG_COOKIE_SECURE", "true")
	t.Setenv("BLOG_PUBLIC_ORIGIN", "http://127.0.0.1:11190")
	if err := validateProductionConfig(s); err == nil {
		t.Fatal("secure cookies with a loopback origin were accepted")
	}
	t.Setenv("BLOG_COOKIE_SECURE", "false")
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	if err := validateProductionConfig(s); err == nil {
		t.Fatal("https origin with insecure cookies was accepted")
	}
	t.Setenv("BLOG_COOKIE_SECURE", "true")
	t.Setenv("BLOG_TRUST_PROXY", "true")
	t.Setenv("BLOG_PUBLIC_URL", "http://blog.example.com")
	if err := validateProductionConfig(s); err == nil {
		t.Fatal("trusted proxy with secure cookies and http origin was accepted")
	}
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	if err := validateProductionConfig(s); err != nil {
		t.Fatalf("valid production configuration was rejected: %v", err)
	}
}

func TestProductionConfigWarningForInsecureProxyDeployment(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_COOKIE_SECURE", "false")
	t.Setenv("BLOG_TRUST_PROXY", "true")
	t.Setenv("BLOG_PUBLIC_URL", "")
	t.Setenv("BLOG_PUBLIC_ORIGIN", "http://127.0.0.1:11190")
	if warning := productionConfigWarning(s); warning != "" {
		t.Fatalf("local preview warned: %q", warning)
	}
	t.Setenv("BLOG_PUBLIC_ORIGIN", "")
	if warning := productionConfigWarning(s); warning == "" {
		t.Fatal("proxy deployment without a site URL was not warned")
	}
	t.Setenv("BLOG_PUBLIC_ORIGIN", "http://blog.example.com")
	if warning := productionConfigWarning(s); warning == "" {
		t.Fatal("public http proxy deployment was not warned")
	}
	t.Setenv("BLOG_COOKIE_SECURE", "true")
	t.Setenv("BLOG_PUBLIC_ORIGIN", "https://blog.example.com")
	if warning := productionConfigWarning(s); warning != "" {
		t.Fatalf("secure deployment warned: %q", warning)
	}
}

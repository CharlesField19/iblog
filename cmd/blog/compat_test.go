package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHttpOnlySessionCookieAndLogout(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	if code, _ := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != 201 {
		t.Fatalf("setup status=%d", code)
	}
	r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"testadmin","password":"test-password-123"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	cookies := w.Result().Cookies()
	var loginResponse map[string]any
	if err := json.NewDecoder(w.Body).Decode(&loginResponse); err != nil {
		t.Fatal(err)
	}
	if _, hasToken := loginResponse["token"]; hasToken {
		t.Fatal("login response must not expose the bearer token; the HttpOnly cookie is the session")
	}
	if w.Code != 200 || len(cookies) == 0 || cookies[0].Name != sessionCookieName || !cookies[0].HttpOnly {
		t.Fatalf("login cookie missing or not HttpOnly: status=%d cookies=%v", w.Code, w.Result().Cookies())
	}
	cookie := cookies[0]
	r = httptest.NewRequest("GET", "/api/v1/admin/articles", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("cookie authorization status=%d body=%s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	r.Header.Set("Origin", "http://example.com")
	r.Host = "example.com"
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("logout status=%d body=%s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/api/v1/admin/articles", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("revoked cookie status=%d", w.Code)
	}
}

func TestCookieAuthenticatedMutationsRejectCrossSiteOrigin(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	if code, _ := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != 201 {
		t.Fatalf("setup status=%d", code)
	}
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"testadmin","password":"test-password-123"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(login, req)
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set the session cookie")
	}

	for _, tc := range []struct {
		name   string
		origin string
		want   int
		path   string
	}{
		{name: "cross origin", origin: "https://attacker.example", want: 403},
		{name: "cross-origin logout", origin: "https://attacker.example", want: 403, path: "/api/v1/auth/logout"},
		{name: "missing origin", want: 403},
		{name: "same-origin host login", origin: "http://example.com", want: 200, path: "/api/v1/auth/login"},
		{name: "cross-origin first-run setup", origin: "https://attacker.example", want: 403, path: "/api/v1/auth/setup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path
			if path == "" {
				path = "/api/v1/admin/articles"
			}
			body := `{} `
			if path == "/api/v1/auth/login" {
				body = `{"username":"testadmin","password":"test-password-123"}`
			} else if path == "/api/v1/auth/setup" {
				body = `{"username":"newadmin","password":"test-password-123"}`
			} else if path == "/api/v1/auth/setup" {
				body = `{"username":"setup-user","password":"test-password-123"}`
			}
			r := httptest.NewRequest("POST", path, strings.NewReader(body))
			r.Host = "example.com"
			r.AddCookie(cookies[0])
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if tc.want != 0 && w.Code != tc.want {
				t.Fatalf("status=%d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestConfiguredPublicOriginBehindTLSProxy(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_PUBLIC_ORIGIN", "https://blog.example")
	h := s.routes()
	cookie := &http.Cookie{Name: sessionCookieName, Value: "test"}
	for _, tc := range []struct {
		origin string
		want   int
	}{
		{origin: "https://blog.example", want: http.StatusUnauthorized}, // same origin passes CSRF, then auth validates token
		{origin: "https://evil.example", want: http.StatusForbidden},
		{origin: "null", want: http.StatusForbidden},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/articles", strings.NewReader(`{}`))
		r.Header.Set("Origin", tc.origin)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("origin %q: status=%d, want %d", tc.origin, w.Code, tc.want)
		}
	}
}

func TestCookieMutationWithoutOriginIsRejected(t *testing.T) {
	s := testServer(t)
	if code, _ := call(t, s.routes(), http.MethodPost, "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != http.StatusCreated {
		t.Fatalf("setup status=%d", code)
	}
	token, err := makeToken("testadmin", 1, env("BLOG_JWT_SECRET", ""), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/articles", strings.NewReader(`{}`))
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing Origin status=%d, want %d", w.Code, http.StatusForbidden)
	}
	// Explicit bearer-only clients remain usable without ambient cookies/Origin.
	r = httptest.NewRequest(http.MethodPost, "/api/v1/admin/articles", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bearer request unexpectedly rejected before API validation: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestArticleCountsFiltersAndPermissionBoundaries(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	for _, in := range []map[string]any{
		{"title": "Go one", "slug": "go-one", "tags": []string{"Go"}, "status": "published"},
		{"title": "Go two", "slug": "go-two", "tags": []string{"Go"}, "status": "published"},
		{"title": "Draft secret", "slug": "draft-secret", "tags": []string{"Rust"}, "status": "draft"},
	} {
		if code, body := call(t, h, "POST", "/api/v1/admin/articles", in, token); code != 201 {
			t.Fatalf("create article status=%d body=%v", code, body)
		}
	}
	code, public := call(t, h, "GET", "/api/v1/articles?tag=Go&page=1&pageSize=1", nil, "")
	if code != 200 || public["total"].(float64) != 2 || public["page"].(float64) != 1 || public["pageSize"].(float64) != 1 || len(public["items"].([]any)) != 1 {
		t.Fatalf("filtered public pagination contract: code=%d body=%v", code, public)
	}
	if code, body := call(t, h, "GET", "/api/v1/articles?tag=Rust", nil, ""); code != 200 || body["total"].(float64) != 0 {
		t.Fatalf("draft leaked to public listing: code=%d body=%v", code, body)
	}
	if code, body := call(t, h, "GET", "/api/v1/admin/articles?status=draft", nil, ""); code != 401 || body["error"] == nil {
		t.Fatalf("unauthorized admin contract: code=%d body=%v", code, body)
	}
	if code, body := call(t, h, "GET", "/api/v1/admin/articles?status=draft", nil, token); code != 200 || body["total"].(float64) != 1 {
		t.Fatalf("admin draft filter count: code=%d body=%v", code, body)
	}
}

func TestRecoverAdminRevokesPreviousTokens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BLOG_DB_PATH", filepath.Join(dir, "blog.db"))
	t.Setenv("BLOG_JWT_SECRET", "test-only-signing-secret")
	db, err := sql.Open("sqlite", os.Getenv("BLOG_DB_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	oldToken := adminToken(t, s)
	if err = recoverAdmin("recovered-admin", strings.NewReader("recovered-password-123\n")); err != nil {
		t.Fatal(err)
	}
	if code, body := call(t, s.routes(), "GET", "/api/v1/admin/articles", nil, oldToken); code != 401 {
		t.Fatalf("old token survived recovery: %d %v", code, body)
	}
	if code, body := call(t, s.routes(), "POST", "/api/v1/auth/login", map[string]any{"username": "recovered-admin", "password": "recovered-password-123"}, ""); code != 200 {
		t.Fatalf("recovered credentials not accepted: %d %v", code, body)
	}
	_ = db.Close()
}

func adminToken(t *testing.T, s *server) string {
	t.Helper()
	h := s.routes()
	setup := httptest.NewRecorder()
	setupReq := httptest.NewRequest("POST", "/api/v1/auth/setup", strings.NewReader(`{"username":"testadmin","password":"test-password-123"}`))
	setupReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(setup, setupReq)
	if code := setup.Code; code != 201 {
		var body map[string]any
		_ = json.Unmarshal(setup.Body.Bytes(), &body)
		t.Fatalf("setup: %d %v", code, body)
	}
	login := httptest.NewRecorder()
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"testadmin","password":"test-password-123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(login, loginReq)
	if login.Code != 200 {
		var body map[string]any
		_ = json.Unmarshal(login.Body.Bytes(), &body)
		t.Fatalf("login: %d %v", login.Code, body)
	}
	for _, cookie := range login.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie.Value
		}
	}
	t.Fatal("login did not set session cookie")
	return ""
}

func TestArticleContract(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	for _, path := range []string{"/api/v1/articles?page=0", "/api/v1/articles?pageSize=101"} {
		if code, _ := call(t, h, "GET", path, nil, ""); code != 400 {
			t.Fatalf("expected invalid pagination for %s, got %d", path, code)
		}
	}
	if code, _ := call(t, h, "GET", "/api/v1/admin/articles?status=invalid", nil, token); code != 400 {
		t.Fatalf("expected invalid status 400, got %d", code)
	}
	code, created := call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": " Draft ", "slug": "unique", "summary": "old", "contentMd": "old", "status": "draft", "tags": []string{"Go", "go", "Vue"}}, token)
	if code != 201 || created["title"] != "Draft" || len(created["tags"].([]any)) != 2 {
		t.Fatalf("create draft: %d %v", code, created)
	}
	if code, _ = call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": "Duplicate", "slug": "unique"}, token); code != 409 {
		t.Fatalf("duplicate slug status=%d", code)
	}
	if code, _ = call(t, h, "GET", "/api/v1/articles/unique", nil, ""); code != 404 {
		t.Fatalf("draft visible to public status=%d", code)
	}
	code, updated := call(t, h, "PUT", "/api/v1/admin/articles/1", map[string]any{"title": "Updated", "slug": "unique", "summary": "", "contentMd": "", "status": "published", "tags": []string{"Vue"}}, token)
	if code != 200 || updated["summary"] != "" || updated["contentMd"] != "" {
		t.Fatalf("clear fields: %d %v", code, updated)
	}
	code, listing := call(t, h, "GET", "/api/v1/articles?tag=Vue&q=Updated&page=1&pageSize=1", nil, "")
	if code != 200 || listing["total"].(float64) != 1 || listing["pageSize"].(float64) != 1 {
		t.Fatalf("filter: %d %v", code, listing)
	}
	code, detail := call(t, h, "GET", "/api/v1/articles/unique", nil, "")
	if code != 200 || detail["contentMd"] != "" {
		t.Fatalf("detail empty markdown: %d %v", code, detail)
	}
	if code, _ = call(t, h, "GET", "/api/v1/admin/articles/nope", nil, token); code != 400 {
		t.Fatalf("invalid article id status=%d", code)
	}
	if code, _ = call(t, h, "DELETE", "/api/v1/admin/articles/1", nil, token); code != 204 {
		t.Fatalf("delete status=%d", code)
	}
}

func TestTagFriendSettingsAccountContract(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	code, tag := call(t, h, "POST", "/api/v1/admin/tags", map[string]any{"name": "Go"}, token)
	if code != 201 || tag["count"].(float64) != 0 {
		t.Fatalf("tag create: %d %v", code, tag)
	}
	if code, _ = call(t, h, "POST", "/api/v1/admin/tags", map[string]any{"name": "go"}, token); code != 409 {
		t.Fatalf("case duplicate status=%d", code)
	}
	code, tag = call(t, h, "PUT", "/api/v1/admin/tags/1", map[string]any{"name": "Golang"}, token)
	if code != 200 || tag["name"] != "Golang" {
		t.Fatalf("tag rename: %d %v", code, tag)
	}
	if code, _ = call(t, h, "DELETE", "/api/v1/admin/tags/1", nil, token); code != 204 {
		t.Fatalf("tag delete status=%d", code)
	}
	if code, _ = call(t, h, "POST", "/api/v1/admin/friends", map[string]any{"name": "Bad", "url": "javascript:alert(1)"}, token); code != 400 {
		t.Fatalf("unsafe link status=%d", code)
	}
	code, f := call(t, h, "POST", "/api/v1/admin/friends", map[string]any{"name": "A", "url": "https://example.org"}, token)
	if code != 201 {
		t.Fatalf("friend create %d %v", code, f)
	}
	code, f = call(t, h, "PUT", "/api/v1/admin/friends/1", map[string]any{"name": "B", "url": "https://example.net", "sortOrder": 3}, token)
	if code != 200 || f["name"] != "B" {
		t.Fatalf("friend update %d %v", code, f)
	}
	if code, _ = call(t, h, "DELETE", "/api/v1/admin/friends/1", nil, token); code != 204 {
		t.Fatalf("friend delete status=%d", code)
	}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/settings/about", map[string]any{"intro": "hi"}, token); code != 200 {
		t.Fatalf("save setting status=%d", code)
	}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/settings/unknown", map[string]any{"x": true}, token); code != 400 {
		t.Fatalf("unknown setting status=%d", code)
	}
	code, settings := call(t, h, "GET", "/api/v1/settings", nil, "")
	if code != 200 || settings["about"].(map[string]any)["intro"] != "hi" {
		t.Fatalf("settings %d %v", code, settings)
	}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/account/username", map[string]any{"username": "ab"}, token); code != 400 {
		t.Fatalf("short username status=%d", code)
	}
	code, account := call(t, h, "PUT", "/api/v1/admin/account/username", map[string]any{"username": "new_admin"}, token)
	if code != 200 || account["username"] != "new_admin" {
		t.Fatalf("account rename %d %v", code, account)
	}
	if _, hasToken := account["token"]; hasToken {
		t.Fatal("account response must not expose the bearer token")
	}
	if code, _ = call(t, h, "GET", "/api/v1/admin/articles", nil, token); code != 401 {
		t.Fatalf("stale token status=%d", code)
	}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/account/password", map[string]any{"currentPassword": "bad", "newPassword": "new-password-123"}, token); code != 401 {
		t.Fatalf("bad password status=%d", code)
	}
	accountToken, err := makeToken("new_admin", 2, env("BLOG_JWT_SECRET", ""), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/account/password", map[string]any{"currentPassword": "test-password-123", "newPassword": "new-password-123"}, accountToken); code != 200 {
		t.Fatalf("password change status=%d", code)
	}
	if code, _ = call(t, h, "POST", "/api/v1/auth/login", map[string]any{"username": "new_admin", "password": "new-password-123"}, ""); code != 200 {
		t.Fatalf("login with new password status=%d", code)
	}
}

func TestCorsSecurityAndLoginLimit(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_CORS_ORIGINS", "https://allowed.example")
	h := s.routes()
	r := httptest.NewRequest("OPTIONS", "/api/v1/admin/articles", nil)
	r.Header.Set("Origin", "https://allowed.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "https://allowed.example" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("CORS preflight: %d %v", w.Code, w.Header())
	}
	r = httptest.NewRequest("GET", "/api/v1/articles", nil)
	r.Header.Set("Origin", "https://other.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unlisted origin was allowed")
	}
	for i := 0; i < 5; i++ {
		if code, _ := call(t, h, "POST", "/api/v1/auth/login", map[string]any{"username": "testadmin", "password": "wrong"}, ""); code != 401 {
			t.Fatalf("failure %d: status=%d", i, code)
		}
	}
	if code, _ := call(t, h, "POST", "/api/v1/auth/login", map[string]any{"username": "testadmin", "password": "wrong"}, ""); code != 429 {
		t.Fatalf("rate limit status=%d", code)
	}
}

func TestCreateArticleRollbackWhenTagFails(t *testing.T) {
	s := testServer(t)
	token := adminToken(t, s)
	_, err := s.db.Exec("CREATE TRIGGER reject_tag BEFORE INSERT ON tags WHEN new.name='bad' BEGIN SELECT RAISE(ABORT,'bad tag'); END")
	if err != nil {
		t.Fatal(err)
	}
	code, _ := call(t, s.routes(), "POST", "/api/v1/admin/articles", map[string]any{"title": "Atomic", "slug": "atomic", "tags": []string{"bad"}}, token)
	if code != 500 {
		t.Fatalf("expected database failure, got %d", code)
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM articles WHERE slug='atomic'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial article inserted: count=%d err=%v", n, err)
	}
}

package main

import (
	"net/http/httptest"
	"testing"
)

func adminToken(t *testing.T, s *server) string {
	t.Helper()
	if code, body := call(t, s.routes(), "POST", "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != 201 {
		t.Fatalf("setup: %d %v", code, body)
	}
	code, body := call(t, s.routes(), "POST", "/api/v1/auth/login", map[string]any{"username": "testadmin", "password": "test-password-123"}, "")
	if code != 200 {
		t.Fatalf("login: %d %v", code, body)
	}
	return body["token"].(string)
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
	if code, _ = call(t, h, "GET", "/api/v1/admin/articles", nil, token); code != 401 {
		t.Fatalf("stale token status=%d", code)
	}
	newToken := account["token"].(string)
	if code, _ = call(t, h, "PUT", "/api/v1/admin/account/password", map[string]any{"currentPassword": "bad", "newPassword": "new-password-123"}, newToken); code != 401 {
		t.Fatalf("bad password status=%d", code)
	}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/account/password", map[string]any{"currentPassword": "test-password-123", "newPassword": "new-password-123"}, newToken); code != 200 {
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
	if code != 400 {
		t.Fatalf("expected tag validation failure, got %d", code)
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM articles WHERE slug='atomic'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial article inserted: count=%d err=%v", n, err)
	}
}

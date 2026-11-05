package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func testServer(t *testing.T) *server {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	oldUser, oldPassword, oldSecret := os.Getenv("BLOG_ADMIN_USERNAME"), os.Getenv("BLOG_ADMIN_PASSWORD"), os.Getenv("BLOG_JWT_SECRET")
	t.Setenv("BLOG_ADMIN_USERNAME", "testadmin")
	t.Setenv("BLOG_ADMIN_PASSWORD", "test-password-123")
	t.Setenv("BLOG_JWT_SECRET", "test-only-signing-secret")
	t.Cleanup(func() {
		_ = os.Setenv("BLOG_ADMIN_USERNAME", oldUser)
		_ = os.Setenv("BLOG_ADMIN_PASSWORD", oldPassword)
		_ = os.Setenv("BLOG_JWT_SECRET", oldSecret)
	})
	return &server{db: db, webDist: "../../frontend/web/dist-rebuilt", adminDist: "../../frontend/admin/dist-rebuilt", loginFailures: make(map[string]loginFailure)}
}
func call(t *testing.T, h http.Handler, method, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var v map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v
}
func TestPublicAndAdminFlow(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	if code, _ := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != 201 {
		t.Fatalf("setup status=%d", code)
	}
	if code, _ := call(t, h, "GET", "/api/v1/health", nil, ""); code != 200 {
		t.Fatalf("health status=%d", code)
	}
	if code, _ := call(t, h, "GET", "/api/v1/admin/articles", nil, ""); code != 401 {
		t.Fatalf("unauth admin status=%d", code)
	}
	code, login := call(t, h, "POST", "/api/v1/auth/login", map[string]any{"username": "testadmin", "password": "test-password-123"}, "")
	if code != 200 {
		t.Fatalf("login status=%d body=%v", code, login)
	}
	token, _ := login["token"].(string)
	if token == "" {
		t.Fatal("missing login token")
	}
	code, created := call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": "First", "contentMd": "hello", "tags": []string{"Go"}, "status": "published"}, token)
	if code != 201 {
		t.Fatalf("create status=%d body=%v", code, created)
	}
	slug, _ := created["slug"].(string)
	if slug == "" {
		t.Fatal("missing generated slug")
	}
	code, public := call(t, h, "GET", "/api/v1/articles", nil, "")
	if code != 200 || public["total"].(float64) != 1 {
		t.Fatalf("public list status=%d body=%v", code, public)
	}
	code, detail := call(t, h, "GET", "/api/v1/articles/"+slug, nil, "")
	if code != 200 || detail["contentMd"] != "hello" {
		t.Fatalf("article detail status=%d body=%v", code, detail)
	}
	code, _ = call(t, h, "GET", "/api/v1/admin/tags", nil, token)
	if code != 200 {
		t.Fatalf("admin tags status=%d", code)
	}
	code, _ = call(t, h, "DELETE", "/api/v1/admin/articles/1", nil, token)
	if code != 204 {
		t.Fatalf("delete status=%d", code)
	}
}
func TestLoginRejectsBadPassword(t *testing.T) {
	s := testServer(t)
	if code, _ := call(t, s.routes(), "POST", "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != 201 {
		t.Fatalf("setup status=%d", code)
	}
	code, _ := call(t, s.routes(), "POST", "/api/v1/auth/login", map[string]any{"username": "testadmin", "password": "wrong"}, "")
	if code != 401 {
		t.Fatalf("status=%d", code)
	}
}

func TestFirstRunSetupIsOneTime(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	if code, data := call(t, h, "GET", "/api/v1/auth/setup-status", nil, ""); code != 200 || data["setupRequired"] != true {
		t.Fatalf("initial setup status=%d data=%v", code, data)
	}
	if code, _ := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "x", "password": "short"}, ""); code != 400 {
		t.Fatalf("weak setup status=%d", code)
	}
	code, created := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "first-admin", "password": "strong-test-password"}, "")
	if code != 201 || created["token"] == "" {
		t.Fatalf("setup status=%d data=%v", code, created)
	}
	if code, data := call(t, h, "GET", "/api/v1/auth/setup-status", nil, ""); code != 200 || data["setupRequired"] != false {
		t.Fatalf("post setup status=%d data=%v", code, data)
	}
	if code, _ := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "second-admin", "password": "another-strong-password"}, ""); code != 409 {
		t.Fatalf("second setup status=%d", code)
	}
	if code, _ := call(t, h, "POST", "/api/v1/auth/login", map[string]any{"username": "first-admin", "password": "strong-test-password"}, ""); code != 200 {
		t.Fatalf("login after setup status=%d", code)
	}
}

func TestMigrationsAreIdempotentAndKeepExistingDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO articles(slug,title,created_at,updated_at) VALUES('keep-me','Existing','now','now')"); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE slug='keep-me'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing row count=%d err=%v", count, err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version='0001_init'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration ledger count=%d err=%v", count, err)
	}
}

func TestLoadOrCreateSecretPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jwt_secret")
	first, err := loadOrCreateSecret(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateSecret(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || first != second {
		t.Fatalf("secret did not persist: first length=%d equal=%v", len(first), first == second)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secret permissions=%o, want 600", info.Mode().Perm())
	}
}

func TestCleanInstallAssetsAndCRUD(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	if code, _ := call(t, h, "POST", "/api/v1/auth/setup", map[string]any{"username": "testadmin", "password": "test-password-123"}, ""); code != 201 {
		t.Fatalf("setup status=%d", code)
	}
	for _, path := range []string{"/", "/admin/", "/assets/index-D5Lrp8y2.js", "/admin/assets/index-2aPgEh4H.js", "/admin/login"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("asset %q: status=%d bytes=%d", path, w.Code, w.Body.Len())
		}
	}
	_, login := call(t, h, "POST", "/api/v1/auth/login", map[string]any{"username": "testadmin", "password": "test-password-123"}, "")
	token := login["token"].(string)
	code, f := call(t, h, "POST", "/api/v1/admin/friends", map[string]any{"name": "A", "url": "https://example.org"}, token)
	if code != 201 || f["name"] != "A" {
		t.Fatalf("friend create: %d %v", code, f)
	}
	code, f = call(t, h, "GET", "/api/v1/friends", nil, "")
	if code != 200 || len(f["items"].([]any)) != 1 {
		t.Fatalf("friend list: %d %v", code, f)
	}
	code, _ = call(t, h, "PUT", "/api/v1/admin/settings/about", map[string]any{"intro": "Welcome"}, token)
	if code != 200 {
		t.Fatalf("save setting: %d", code)
	}
	code, settings := call(t, h, "GET", "/api/v1/settings", nil, "")
	if code != 200 || settings["about"].(map[string]any)["intro"] != "Welcome" {
		t.Fatalf("settings: %d %v", code, settings)
	}
	site := map[string]any{"name": "iBlog", "titlePrefix": "A Writer", "titleAccent": "Notes", "subtitle": "Ideas", "footerName": "Author", "footerText": "Made with care"}
	if code, _ = call(t, h, "PUT", "/api/v1/admin/settings/site", site, token); code != 200 {
		t.Fatalf("save site settings: %d", code)
	}
	if code, settings = call(t, h, "GET", "/api/v1/settings", nil, ""); code != 200 || settings["site"].(map[string]any)["titlePrefix"] != "A Writer" {
		t.Fatalf("public site settings: %d %v", code, settings)
	}
	if code, settings = call(t, h, "GET", "/api/v1/admin/settings", nil, token); code != 200 || settings["settings"].(map[string]any)["site"].(map[string]any)["footerText"] != "Made with care" {
		t.Fatalf("admin site settings: %d %v", code, settings)
	}
	code, article := call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": "Draft", "status": "draft"}, token)
	if code != 201 {
		t.Fatalf("draft create: %d %v", code, article)
	}
	code, listing := call(t, h, "GET", "/api/v1/articles", nil, "")
	if code != 200 || listing["total"].(float64) != 0 {
		t.Fatalf("draft publicly visible: %d %v", code, listing)
	}
	code, _ = call(t, h, "PUT", "/api/v1/admin/articles/1", map[string]any{"title": "Published", "slug": article["slug"], "status": "published"}, token)
	if code != 200 {
		t.Fatalf("publish status=%d", code)
	}
	code, listing = call(t, h, "GET", "/api/v1/articles", nil, "")
	if code != 200 || listing["total"].(float64) != 1 {
		t.Fatalf("publish list: %d %v", code, listing)
	}
}

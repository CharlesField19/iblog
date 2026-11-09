package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	return &server{db: db, webDist: "../../frontend/web/dist", adminDist: "../../frontend/admin/dist", loginFailures: make(map[string]loginFailure)}
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

func TestErrorResponsesExposeOnlyStableCode(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	status, body := call(t, h, "GET", "/api/v1/articles?page=0", nil, "")
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%v", status, body)
	}
	errBody, ok := body["error"].(map[string]any)
	if !ok || errBody["code"] != "bad_request" {
		t.Fatalf("error contract=%v", body["error"])
	}
	if _, ok := errBody["message"]; ok {
		t.Fatalf("error response must not expose message: %v", errBody)
	}
}
func TestPublicAndAdminFlow(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	if code, _ := call(t, h, "GET", "/api/v1/health", nil, ""); code != 200 {
		t.Fatalf("health status=%d", code)
	}
	if code, _ := call(t, h, "GET", "/api/v1/admin/articles", nil, ""); code != 401 {
		t.Fatalf("unauth admin status=%d", code)
	}
	token := adminToken(t, s)
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

func TestNewArticleWithoutSlugUsesSemanticSlug(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	code, created := call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Hello, Go & SQLite!", "contentMd": "body", "status": "draft",
	}, token)
	if code != http.StatusCreated {
		t.Fatalf("create status=%d body=%v", code, created)
	}
	if got := created["slug"]; got != "hello-go-sqlite" {
		t.Fatalf("semantic slug=%v", got)
	}
	code, created = call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Hello, Go & SQLite!", "contentMd": "body", "status": "draft",
	}, token)
	if code != http.StatusCreated || created["slug"] != "hello-go-sqlite-2" {
		t.Fatalf("duplicate semantic slug: status=%d body=%v", code, created)
	}
	code, _ = call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Bad slug", "slug": "not a valid/slug", "contentMd": "body", "status": "draft",
	}, token)
	if code != http.StatusBadRequest {
		t.Fatalf("invalid slug status=%d", code)
	}
}

func TestArticleSlugsDoNotReuseAliases(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	_, original := call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Original", "slug": "legacy-title", "contentMd": "body", "status": "published",
	}, token)
	id := int(original["id"].(float64))
	if _, err := s.db.Exec("INSERT INTO article_slug_aliases(slug,article_id,created_at) VALUES(?,?,?)", "hello-world", id, now()); err != nil {
		t.Fatal(err)
	}

	code, created := call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Hello World", "contentMd": "body", "status": "draft",
	}, token)
	if code != http.StatusCreated || created["slug"] != "hello-world-2" {
		t.Fatalf("generated slug reused alias: status=%d body=%v", code, created)
	}

	code, _ = call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Explicit alias", "slug": "hello-world", "contentMd": "body", "status": "draft",
	}, token)
	if code != http.StatusConflict {
		t.Fatalf("explicit alias status=%d, want %d", code, http.StatusConflict)
	}

	code, _ = call(t, h, http.MethodPut, "/api/v1/admin/articles/"+fmt.Sprint(id), map[string]any{
		"title": "Original", "slug": "hello-world", "contentMd": "body", "status": "published",
	}, token)
	if code != http.StatusConflict {
		t.Fatalf("update to alias status=%d, want %d", code, http.StatusConflict)
	}
}

func TestArticleSlugChangeKeepsOldLink(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	_, created := call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": "Link preservation", "slug": "old-link", "contentMd": "body", "status": "published"}, token)
	id := int(created["id"].(float64))
	code, updated := call(t, h, "PUT", fmt.Sprintf("/api/v1/admin/articles/%d", id), map[string]any{"title": "Link preservation", "slug": "new-link", "contentMd": "body", "status": "published"}, token)
	if code != 200 {
		t.Fatalf("update status=%d body=%v", code, updated)
	}
	code, legacy := call(t, h, "GET", "/api/v1/articles/old-link", nil, "")
	if code != 200 || legacy["slug"] != "new-link" {
		t.Fatalf("legacy link status=%d body=%v", code, legacy)
	}
}

func TestBatchArticleActions(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	_, first := call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": "Draft one", "slug": "draft-one", "summary": "summary", "contentMd": "preview body", "tags": []string{"Go"}, "status": "draft"}, token)
	_, second := call(t, h, "POST", "/api/v1/admin/articles", map[string]any{"title": "Draft two", "slug": "draft-two", "contentMd": "another body", "tags": []string{"Go"}, "status": "draft"}, token)
	id1, id2 := int64(first["id"].(float64)), int64(second["id"].(float64))
	code, filtered := call(t, h, "GET", "/api/v1/admin/articles?q=Go", nil, token)
	if code != 200 || filtered["total"].(float64) != 2 {
		t.Fatalf("tag search status=%d body=%v", code, filtered)
	}
	code, _ = call(t, h, "POST", "/api/v1/admin/articles/batch", map[string]any{"ids": []int64{id1}, "action": "archive"}, token)
	if code != 200 {
		t.Fatalf("batch archive status=%d", code)
	}
	code, _ = call(t, h, "POST", "/api/v1/admin/articles/batch", map[string]any{"ids": []int64{id2}, "action": "publish"}, token)
	if code != 200 {
		t.Fatalf("batch publish status=%d", code)
	}
	code, _ = call(t, h, "POST", "/api/v1/admin/articles/batch", map[string]any{"ids": []int64{id2}, "action": "delete"}, token)
	if code != 200 {
		t.Fatalf("batch delete status=%d", code)
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

func TestRequestCorrelationAndAuditLogsDoNotIncludeRequestBodies(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	h := requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/articles/42", bytes.NewBufferString(`{"password":"do-not-log","contentMd":"private body"}`))
	r = r.WithContext(context.WithValue(r.Context(), usernameKey, "audit-user"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" || !strings.Contains(output.String(), "request_id="+requestID) || !strings.Contains(output.String(), "audit request_id="+requestID) || !strings.Contains(output.String(), "user=audit-user") {
		t.Fatalf("missing correlation/audit log: header=%q logs=%q", requestID, output.String())
	}
	if strings.Contains(output.String(), "do-not-log") || strings.Contains(output.String(), "private body") {
		t.Fatalf("sensitive request body was logged: %s", output.String())
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
	if code != 201 || created["expiresAt"] == nil {
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("secret permissions=%o, want 600", info.Mode().Perm())
	}
}

func TestCleanInstallAssetsAndCRUD(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	webAssets, err := filepath.Glob(filepath.Join(s.webDist, "assets", "*.js"))
	if err != nil || len(webAssets) == 0 {
		t.Fatalf("find built web assets: files=%v err=%v", webAssets, err)
	}
	adminAssets, err := filepath.Glob(filepath.Join(s.adminDist, "assets", "*.js"))
	if err != nil || len(adminAssets) == 0 {
		t.Fatalf("find built admin assets: files=%v err=%v", adminAssets, err)
	}
	paths := []string{"/", "/admin/", "/admin/login"}
	for _, asset := range webAssets {
		paths = append(paths, "/assets/"+filepath.Base(asset))
	}
	for _, asset := range adminAssets {
		paths = append(paths, "/admin/assets/"+filepath.Base(asset))
	}
	for _, path := range paths {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("asset %q: status=%d bytes=%d", path, w.Code, w.Body.Len())
		}
	}
	token := adminToken(t, s)
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

func TestFTSSearchRanksTitleAndTracksArticleTags(t *testing.T) {
	s := testServer(t)
	for _, statement := range []string{
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('title-hit','SQLite in the title','','unrelated body','published','2026-01-01','2026-01-01','2026-01-01')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('body-hit','Different title','','SQLite in the body','published','2026-01-02','2026-01-02','2026-01-02')`,
		`INSERT INTO tags(name) VALUES('SearchableTag')`,
		`INSERT INTO article_tags(article_id,tag_id) SELECT id,(SELECT id FROM tags WHERE name='SearchableTag') FROM articles WHERE slug='body-hit'`,
	} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	code, result := call(t, s.routes(), http.MethodGet, "/api/v1/articles?q=SQLite&page=1&pageSize=10", nil, "")
	if code != http.StatusOK || result["total"] != float64(2) {
		t.Fatalf("FTS result: status=%d response=%v", code, result)
	}
	items := result["items"].([]any)
	if got := items[0].(map[string]any)["slug"]; got != "title-hit" {
		t.Fatalf("title match should rank first, got %v", got)
	}
	code, result = call(t, s.routes(), http.MethodGet, "/api/v1/articles?q=SearchableTag", nil, "")
	if code != http.StatusOK || result["total"] != float64(1) {
		t.Fatalf("tag was not indexed: status=%d response=%v", code, result)
	}
	if _, err := s.db.Exec(`UPDATE tags SET name='RenamedSearchTag' WHERE name='SearchableTag'`); err != nil {
		t.Fatal(err)
	}
	code, result = call(t, s.routes(), http.MethodGet, "/api/v1/articles?q=RenamedSearchTag", nil, "")
	if code != http.StatusOK || result["total"] != float64(1) {
		t.Fatalf("renamed tag was not reindexed: status=%d response=%v", code, result)
	}
}

func TestSecurityAndStaticCacheHeaders(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	page := httptest.NewRecorder()
	h.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || page.Header().Get("Cache-Control") != "no-cache" || page.Header().Get("Content-Security-Policy") == "" || page.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("page security/cache headers: status=%d headers=%v", page.Code, page.Header())
	}
	assets, err := filepath.Glob(filepath.Join(s.webDist, "assets", "*.js"))
	if err != nil || len(assets) == 0 {
		t.Fatalf("find asset: %v", err)
	}
	asset := httptest.NewRecorder()
	h.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/"+filepath.Base(assets[0]), nil))
	if asset.Code != http.StatusOK || asset.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset cache headers: status=%d headers=%v", asset.Code, asset.Header())
	}
	t.Setenv("BLOG_COOKIE_SECURE", "true")
	secure := httptest.NewRecorder()
	h.ServeHTTP(secure, httptest.NewRequest(http.MethodGet, "/", nil))
	if secure.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("secure deployment did not set HSTS")
	}
}

func TestViewCountsAreBufferedAndFlushedTogether(t *testing.T) {
	s := testServer(t)
	token := adminToken(t, s)
	if code, body := call(t, s.routes(), http.MethodPost, "/api/v1/admin/articles", map[string]any{"title": "Views", "slug": "views-test", "status": "published"}, token); code != http.StatusCreated {
		t.Fatalf("create article: %d %v", code, body)
	}
	for i := 0; i < 3; i++ {
		if code, body := call(t, s.routes(), http.MethodGet, "/api/v1/articles/views-test", nil, ""); code != http.StatusOK {
			t.Fatalf("read article: %d %v", code, body)
		}
		if code, body := call(t, s.routes(), http.MethodPost, "/api/v1/articles/views-test/view", nil, ""); code != http.StatusNoContent {
			t.Fatalf("record view: %d %v", code, body)
		}
	}
	var before int
	if err := s.db.QueryRow("SELECT views FROM articles WHERE slug='views-test'").Scan(&before); err != nil || before != 0 {
		t.Fatalf("views were not buffered: value=%d err=%v", before, err)
	}
	if err := s.flushViews(context.Background()); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := s.db.QueryRow("SELECT views FROM articles WHERE slug='views-test'").Scan(&after); err != nil || after != 3 {
		t.Fatalf("view batch not persisted: value=%d err=%v", after, err)
	}
}

func TestArticleStorageErrorsReturnServerError(t *testing.T) {
	s := testServer(t)
	if _, err := s.db.Exec(`INSERT INTO articles(slug,title,content_md,status,created_at,updated_at) VALUES('query-error','Query error','body','published','now','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("DROP TABLE article_tags"); err != nil {
		t.Fatal(err)
	}
	code, body := call(t, s.routes(), http.MethodGet, "/api/v1/articles/query-error", nil, "")
	if code != http.StatusInternalServerError || body["error"] == nil {
		t.Fatalf("storage error should not be reported as not-found: status=%d body=%v", code, body)
	}
}

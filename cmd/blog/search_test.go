package main

import (
	"database/sql"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestFTSMigrationBackfillsExistingDataAndTriggers(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"0001_init", "0002_query_indexes", "0003_article_slug_aliases", "0004_auth_token_version"} {
		script, err := schemaFS.ReadFile("migrations/" + name + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(script)); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?, '2026-01-01')", name); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`PRAGMA foreign_keys=ON`,
		`INSERT INTO articles(id,slug,title,summary,content_md,status,created_at,updated_at) VALUES(1,'legacy','Legacy searchable','','preserved body','published','2026-01-01','2026-01-01')`,
		`INSERT INTO tags(id,name) VALUES(1,'LegacyTag')`,
		`INSERT INTO article_tags(article_id,tag_id) VALUES(1,1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
	assertMatches := func(term string, want int) {
		t.Helper()
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM article_search WHERE article_search MATCH ?", ftsQuery(term)).Scan(&count); err != nil || count != want {
			t.Fatalf("%q matches=%d want=%d err=%v", term, count, want, err)
		}
	}
	assertMatches("LegacyTag", 1)
	assertMatches("preserved", 1)
	if _, err = db.Exec(`UPDATE articles SET content_md='updated body' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	assertMatches("preserved", 0)
	assertMatches("updated", 1)
	if _, err = db.Exec(`DELETE FROM tags WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	assertMatches("LegacyTag", 0)
	if _, err = db.Exec(`DELETE FROM articles WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	assertMatches("updated", 0)
}

func TestSearchCacheKeysAndGeneration(t *testing.T) {
	s := testServer(t)
	key := publicSearchCacheKey(url.Values{"q": {"hello  world"}})
	for _, q := range []url.Values{
		{"q": {"hello world"}}, {"q": {"HELLO  WORLD"}},
		{"q": {"hello  world"}, "sort": {"oldest"}},
		{"q": {"hello  world"}, "tag": {"other"}},
		{"q": {"hello  world"}, "page": {"2"}},
	} {
		if publicSearchCacheKey(q) == key {
			t.Fatalf("different queries share cache key: %v", q)
		}
	}
	_, generation := s.getSearchCache(key)
	s.setSearchCache(key, []byte("original"), generation)
	body, _ := s.getSearchCache(key)
	body[0] = 'X'
	if cached, _ := s.getSearchCache(key); string(cached) != "original" {
		t.Fatal("cache body was not copied")
	}
	s.invalidateSearchCache()
	s.setSearchCache(key, []byte("stale in-flight query"), generation)
	if cached, _ := s.getSearchCache(key); cached != nil {
		t.Fatal("in-flight query repopulated invalidated cache")
	}
	s.searchCache[key] = searchCacheEntry{body: []byte("expired"), expiresAt: time.Now().Add(-time.Second)}
	if cached, _ := s.getSearchCache(key); cached != nil {
		t.Fatal("expired result was returned")
	}
}

func TestFTSWeightsRankSummaryBeforeBody(t *testing.T) {
	s := testServer(t)
	for _, statement := range []string{
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('summary-hit','Neutral title','quartz','plain','published','2026-01-01','2026-01-01','2026-01-01')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('body-hit','Neutral title','plain','quartz','published','2026-01-02','2026-01-02','2026-01-02')`,
	} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	code, result := call(t, s.routes(), http.MethodGet, "/api/v1/articles?q=quartz", nil, "")
	if code != http.StatusOK || result["total"] != float64(2) {
		t.Fatalf("weighted search: %d %v", code, result)
	}
	if first := result["items"].([]any)[0].(map[string]any)["slug"]; first != "summary-hit" {
		t.Fatalf("summary must outrank newer body match; first=%v", first)
	}
}

func TestSearchParametersAndLiteralQueries(t *testing.T) {
	s := testServer(t)
	for _, statement := range []string{
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('first','SQLite first','','literal 100% and a_b and 中文','published','2026-01-01','2026-01-01','2026-01-01')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('second','SQLite second','','hello  world','published','2026-01-02','2026-01-02','2026-01-02')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at) VALUES('hidden','SQLite draft','','private','draft','2026-01-03','2026-01-03')`,
	} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	h := s.routes()
	for _, tc := range []struct{ query, slug string }{
		{"q=SQLite&pageSize=1", "second"},
		{"q=SQLite&pageSize=1&page=2", "first"},
		{"q=SQLite&pageSize=1&sort=oldest", "first"},
		{"q=%25", "first"}, {"q=_", "first"},
		{"q=" + url.QueryEscape("中文"), "first"},
		{"q=" + url.QueryEscape("100%"), "first"},
		{"q=" + url.QueryEscape("hello  world"), "second"},
	} {
		code, result := call(t, h, http.MethodGet, "/api/v1/articles?"+tc.query, nil, "")
		if code != http.StatusOK {
			t.Fatalf("%s: %d %v", tc.query, code, result)
		}
		items := result["items"].([]any)
		if len(items) == 0 || items[0].(map[string]any)["slug"] != tc.slug {
			t.Fatalf("%s: expected %s, got %v", tc.query, tc.slug, result)
		}
	}
	for _, query := range []string{"page=0", "page=oops", "pageSize=101", "pageSize=oops", "sort=invalid"} {
		if code, result := call(t, h, http.MethodGet, "/api/v1/articles?"+query, nil, ""); code != http.StatusBadRequest {
			t.Fatalf("invalid %s: %d %v", query, code, result)
		}
	}
	for _, query := range []string{`" OR *`, `a_b`, `100%`, `\abc`, `NEAR(foo)`} {
		if code, result := call(t, h, http.MethodGet, "/api/v1/articles?q="+url.QueryEscape(query), nil, ""); code != http.StatusOK {
			t.Fatalf("literal %q: %d %v", query, code, result)
		}
	}
}

func TestAdminUpdatedSortUsesUpdatedAt(t *testing.T) {
	s := testServer(t)
	for _, statement := range []string{
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('old-published','Older published','','','published','2026-01-01','2026-03-10','2026-01-01')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('recent-published','Recently published','','','published','2026-03-01','2026-03-02','2026-03-01')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at) VALUES('recent-draft','Recently updated draft','','','draft','2026-02-01','2026-03-20')`,
	} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	token := adminToken(t, s)
	code, result := call(t, s.routes(), http.MethodGet, "/api/v1/admin/articles?sort=updated", nil, token)
	if code != http.StatusOK {
		t.Fatalf("updated sort: %d %v", code, result)
	}
	items := result["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected all articles, got %v", result)
	}
	want := []string{"recent-draft", "old-published", "recent-published"}
	for i, item := range items {
		if got := item.(map[string]any)["slug"]; got != want[i] {
			t.Fatalf("sort=updated item %d = %v, want %s; response=%v", i, got, want[i], result)
		}
	}
}

func TestAdminSearchKeepsRelevanceWithDefaultUpdatedSort(t *testing.T) {
	s := testServer(t)
	for _, statement := range []string{
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('relevant','rareterm in title','','','published','2026-01-01','2026-01-01','2026-01-01')`,
		`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES('recent-weak','Common title','','rareterm in body','published','2026-01-02','2026-03-20','2026-01-02')`,
	} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	token := adminToken(t, s)
	for _, sort := range []string{"", "updated"} {
		path := "/api/v1/admin/articles?q=rareterm&sort=" + sort
		code, result := call(t, s.routes(), http.MethodGet, path, nil, token)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %v", path, code, result)
		}
		items := result["items"].([]any)
		if got := items[0].(map[string]any)["slug"]; got != "relevant" {
			t.Fatalf("%s ordered by update time instead of FTS relevance: %v", path, result)
		}
	}
}

func TestPublicAPIRejectsAdminOnlyUpdatedSort(t *testing.T) {
	s := testServer(t)
	if code, result := call(t, s.routes(), http.MethodGet, "/api/v1/articles?sort=updated", nil, ""); code != http.StatusBadRequest {
		t.Fatalf("public sort=updated status=%d response=%v", code, result)
	}
}

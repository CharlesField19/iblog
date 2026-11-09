package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func insertSSRArticle(t *testing.T, db *sql.DB, slug, title, status string) int64 {
	t.Helper()
	result, err := db.Exec("INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES(?,?,?,?,?,?,?,?)", slug, title, "Summary for "+title, "# "+title+"\n\nReadable body.", status, now(), now(), now())
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSSRArticleAndPublicStatusCodes(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	id := insertSSRArticle(t, s.db, "hello-world", "Hello <World>", "published")
	insertSSRArticle(t, s.db, "draft-only", "Draft", "draft")
	if _, err := s.db.Exec("INSERT INTO article_slug_aliases(slug,article_id,created_at) VALUES(?,?,?)", "old-hello", id, now()); err != nil {
		t.Fatal(err)
	}

	h := s.routes()
	article := httptest.NewRecorder()
	h.ServeHTTP(article, httptest.NewRequest(http.MethodGet, "/articles/hello-world", nil))
	body := article.Body.String()
	if article.Code != http.StatusOK {
		t.Fatalf("article status=%d body=%s", article.Code, body)
	}
	for _, want := range []string{
		"<title>Hello &lt;World&gt; · iBlog</title>",
		`<link rel="canonical" href="https://blog.example.com/articles/hello-world">`,
		"<h1>Hello &lt;World&gt;</h1>",
		"Readable body.",
		`application/ld+json`,
		`https://schema.org`,
		`__IBLOG_INITIAL__`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("article HTML missing %q: %s", want, body)
		}
	}
	var views int
	if err := s.db.QueryRow("SELECT views FROM articles WHERE id=?", id).Scan(&views); err != nil {
		t.Fatal(err)
	}
	if views != 0 {
		t.Fatalf("SSR request counted as a view: %d", views)
	}

	alias := httptest.NewRecorder()
	h.ServeHTTP(alias, httptest.NewRequest(http.MethodGet, "/articles/old-hello", nil))
	if alias.Code != http.StatusMovedPermanently || alias.Header().Get("Location") != "/articles/hello-world" {
		t.Fatalf("alias redirect: status=%d location=%q", alias.Code, alias.Header().Get("Location"))
	}

	for _, path := range []string{"/articles/draft-only", "/articles/missing", "/unknown-page"} {
		missing := httptest.NewRecorder()
		h.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, path, nil))
		if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `content="noindex"`) {
			t.Fatalf("%s: status=%d body=%s", path, missing.Code, missing.Body.String())
		}
	}
}

func TestSSRListMetadataAndInitialData(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	for i := 0; i < 2; i++ {
		insertSSRArticle(t, s.db, "list-"+string(rune('a'+i)), "List article", "published")
	}
	h := s.routes()
	list := httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/articles?page=2", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d", list.Code)
	}
	if !strings.Contains(list.Body.String(), `rel="canonical" href="https://blog.example.com/articles?page=2"`) {
		t.Fatalf("page 2 canonical missing: %s", list.Body.String())
	}
	search := httptest.NewRecorder()
	h.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/articles?q=secret", nil))
	if !strings.Contains(search.Body.String(), `name="robots" content="noindex,follow"`) {
		t.Fatalf("search page is indexable: %s", search.Body.String())
	}
}

func TestFeedContainsPublishedArticlesOnly(t *testing.T) {
	s := testServer(t)
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	insertSSRArticle(t, s.db, "feed-post", "Feed & Post", "published")
	insertSSRArticle(t, s.db, "feed-draft", "Hidden Draft", "draft")
	recorder := httptest.NewRecorder()
	s.feedXML(recorder, httptest.NewRequest(http.MethodGet, "/feed.xml", nil))
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/rss+xml; charset=utf-8" {
		t.Fatalf("feed response: status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, "Feed &amp; Post") || strings.Contains(body, "Hidden Draft") {
		t.Fatalf("feed content incorrect: %s", body)
	}
}

func TestRecordArticleViewEndpoint(t *testing.T) {
	s := testServer(t)
	insertSSRArticle(t, s.db, "view-endpoint", "View endpoint", "published")
	recorder := httptest.NewRecorder()
	s.routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/articles/view-endpoint/view", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("view status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if err := s.flushViews(context.Background()); err != nil {
		t.Fatal(err)
	}
	var views int
	if err := s.db.QueryRow("SELECT views FROM articles WHERE slug='view-endpoint'").Scan(&views); err != nil || views != 1 {
		t.Fatalf("views=%d err=%v", views, err)
	}
}

func TestRecordArticleViewDeduplicatesClientWindow(t *testing.T) {
	s := testServer(t)
	insertSSRArticle(t, s.db, "view-dedup", "View dedup", "published")
	h := s.routes()
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/articles/view-dedup/view", nil)
		r.Header.Set("User-Agent", "dedup-test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent {
			t.Fatalf("view status=%d", w.Code)
		}
	}
	if err := s.flushViews(context.Background()); err != nil {
		t.Fatal(err)
	}
	var views int
	if err := s.db.QueryRow("SELECT views FROM articles WHERE slug='view-dedup'").Scan(&views); err != nil || views != 1 {
		t.Fatalf("views=%d err=%v", views, err)
	}
}

func TestSemanticSlugKeepsAliasRedirectCanonical(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	token := adminToken(t, s)
	_, created := call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{
		"title": "Semantic title", "contentMd": "body", "status": "published",
	}, token)
	id := int(created["id"].(float64))
	if created["slug"] != "semantic-title" {
		t.Fatalf("slug=%v", created["slug"])
	}
	code, _ := call(t, h, http.MethodPut, "/api/v1/admin/articles/"+strconv.Itoa(id), map[string]any{
		"title": "Semantic title", "slug": "canonical-title", "contentMd": "body", "status": "published",
	}, token)
	if code != http.StatusOK {
		t.Fatalf("update status=%d", code)
	}
	redirect := httptest.NewRecorder()
	h.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/articles/semantic-title", nil))
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/articles/canonical-title" {
		t.Fatalf("redirect status=%d location=%q", redirect.Code, redirect.Header().Get("Location"))
	}
}

package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		writeJSON(w, 503, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "version": "1.1.6"})
}
func (s *server) robots(w http.ResponseWriter, r *http.Request) {
	base := s.canonicalBaseURL(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /api/\nSitemap: %s/sitemap.xml\n", base)
}
func (s *server) sitemap(w http.ResponseWriter, r *http.Request) {
	base := s.canonicalBaseURL(r)
	key := "sitemap:" + base
	if body, ok := s.getPublicDocCache(key); ok {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	generation := s.publicDocCacheVersion()
	rows, err := s.db.QueryContext(r.Context(), "SELECT slug,updated_at FROM articles WHERE status='published' ORDER BY updated_at DESC")
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer rows.Close()
	type entry struct{ slug, updated string }
	var entries []entry
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.slug, &item.updated); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	var output bytes.Buffer
	fmt.Fprint(&output, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	for _, path := range []string{"/", "/articles", "/about", "/friends"} {
		fmt.Fprintf(&output, "<url><loc>%s</loc></url>", html.EscapeString(base+path))
	}
	for _, item := range entries {
		fmt.Fprintf(&output, "<url><loc>%s/articles/%s</loc><lastmod>%s</lastmod></url>", html.EscapeString(base), html.EscapeString(url.PathEscape(item.slug)), html.EscapeString(item.updated))
	}
	fmt.Fprint(&output, "</urlset>")
	body := output.Bytes()
	s.setPublicDocCache(key, body, generation)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *server) adminHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/api/v1/admin/markdown/preview" && r.Method == http.MethodPost {
		var input struct {
			Content string `json:"contentMd"`
		}
		if err := decode(r, &input); err != nil || len(input.Content) > maxContentLen {
			fail(w, http.StatusBadRequest, "bad_request", "invalid Markdown")
			return
		}
		html, err := renderMarkdown(normalizedMarkdown(input.Content))
		if err != nil {
			fail(w, http.StatusBadRequest, "bad_request", "invalid Markdown")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"html": string(html)})
		return
	}
	if p == "/api/v1/admin/articles/batch" && r.Method == "POST" {
		s.batchArticles(w, r)
		return
	}
	for _, kind := range []string{"articles", "tags", "friends"} {
		prefix := "/api/v1/admin/" + kind + "/"
		if strings.HasPrefix(p, prefix) {
			id, err := strconv.ParseInt(strings.TrimPrefix(p, prefix), 10, 64)
			if err != nil || id < 1 {
				fail(w, 400, "bad_request", "id must be a positive integer")
				return
			}
		}
	}
	if strings.HasPrefix(p, "/api/v1/admin/articles/") {
		r.SetPathValue("id", strings.TrimPrefix(p, "/api/v1/admin/articles/"))
	}
	if strings.HasPrefix(p, "/api/v1/admin/tags/") {
		r.SetPathValue("id", strings.TrimPrefix(p, "/api/v1/admin/tags/"))
	}
	if strings.HasPrefix(p, "/api/v1/admin/friends/") {
		r.SetPathValue("id", strings.TrimPrefix(p, "/api/v1/admin/friends/"))
	}
	if strings.HasPrefix(p, "/api/v1/admin/settings/") {
		r.SetPathValue("key", strings.TrimPrefix(p, "/api/v1/admin/settings/"))
	}
	switch {
	case p == "/api/v1/admin/articles" && r.Method == "GET":
		s.listArticles(w, r)
	case p == "/api/v1/admin/articles" && r.Method == "POST":
		s.createArticle(w, r)
	case strings.HasPrefix(p, "/api/v1/admin/articles/"):
		if r.Method == "GET" {
			a, e := s.articleBy(r, r.PathValue("id"), false)
			if e != nil {
				if errors.Is(e, sql.ErrNoRows) {
					fail(w, 404, "not_found", "文章不存在")
				} else {
					fail(w, 500, "internal", "服务器内部错误")
				}
			} else {
				writeJSON(w, 200, a)
			}
		} else if r.Method == "PUT" {
			s.updateArticle(w, r)
		} else if r.Method == "DELETE" {
			s.deleteArticle(w, r)
		} else {
			http.NotFound(w, r)
		}
	case p == "/api/v1/admin/tags":
		if r.Method == "GET" {
			s.tags(w, r)
		} else if r.Method == "POST" {
			s.saveTag(w, r)
		} else {
			fail(w, 404, "not_found", "unknown API endpoint")
		}
	case strings.HasPrefix(p, "/api/v1/admin/tags/"):
		if r.Method == "PUT" {
			s.saveTag(w, r)
		} else if r.Method == "DELETE" {
			s.deleteTag(w, r)
		} else {
			http.NotFound(w, r)
		}
	case p == "/api/v1/admin/friends":
		if r.Method == "GET" {
			s.friends(w, r)
		} else if r.Method == "POST" {
			s.saveFriend(w, r)
		} else {
			fail(w, 404, "not_found", "unknown API endpoint")
		}
	case strings.HasPrefix(p, "/api/v1/admin/friends/"):
		if r.Method == "PUT" {
			s.saveFriend(w, r)
		} else if r.Method == "DELETE" {
			s.deleteFriend(w, r)
		} else {
			http.NotFound(w, r)
		}
	case p == "/api/v1/admin/settings":
		if r.Method == "GET" {
			s.settings(w, r)
		} else {
			fail(w, 404, "not_found", "unknown API endpoint")
		}
	case strings.HasPrefix(p, "/api/v1/admin/settings/"):
		if r.Method == "PUT" {
			s.saveSetting(w, r)
		} else {
			fail(w, 404, "not_found", "unknown API endpoint")
		}
	case strings.HasPrefix(p, "/api/v1/admin/account/"):
		if r.Method == "PUT" {
			s.changeAccount(w, r)
		} else {
			fail(w, 404, "not_found", "unknown API endpoint")
		}
	default:
		http.NotFound(w, r)
	}
}
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /robots.txt", s.robots)
	mux.HandleFunc("GET /sitemap.xml", s.sitemap)
	mux.HandleFunc("GET /feed.xml", s.feedXML)
	mux.HandleFunc("GET /api/v1/articles", s.listArticles)
	mux.HandleFunc("GET /api/v1/articles/{slug}", s.getArticle)
	mux.HandleFunc("POST /api/v1/articles/{slug}/view", s.recordArticleView)
	mux.HandleFunc("GET /api/v1/tags", s.tags)
	mux.HandleFunc("GET /api/v1/friends", s.friends)
	mux.HandleFunc("GET /api/v1/settings", s.settings)
	mux.HandleFunc("GET /api/v1/auth/setup-status", s.setupStatus)
	mux.HandleFunc("POST /api/v1/auth/setup", s.setupAdmin)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("POST /api/v1/auth/logout", s.authorize(http.HandlerFunc(s.logout)))
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.adminHandler(w, r)
	})
	mux.Handle("/api/v1/admin/", s.authorize(admin))
	mux.HandleFunc("GET /articles", s.renderArticlesPage)
	mux.HandleFunc("GET /articles/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/articles", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /articles/{slug}", s.renderArticlePage)
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) { s.renderStaticPage(w, r, "about") })
	mux.HandleFunc("GET /friends", func(w http.ResponseWriter, r *http.Request) { s.renderStaticPage(w, r, "friends") })
	mux.HandleFunc("GET /assets/", func(w http.ResponseWriter, r *http.Request) {
		serveSPA(w, r, s.webDist, "/index.html")
	})
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		serveSPA(w, r, s.webDist, "/favicon.ico")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			fail(w, 404, "not_found", "unknown API endpoint")
			return
		}
		if r.URL.Path == "/admin" {
			http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/admin/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/admin")
			serveSPA(w, r, s.adminDist, "/index.html")
			return
		}
		if r.URL.Path == "/" {
			s.renderHomePage(w, r)
			return
		}
		s.renderNotFound(w, r)
	})
	return recoverer(requestLogger(s.securityHeaders(mux)))
}

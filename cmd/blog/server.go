package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type server struct {
	db                    *sql.DB
	webDist, adminDist    string
	loginMu               sync.Mutex
	loginFailures         map[string]loginFailure
	viewMu                sync.Mutex
	pendingViews          map[int64]int64
	searchCacheMu         sync.Mutex
	searchCache           map[string]searchCacheEntry
	searchCacheGeneration uint64
}
type searchCacheEntry struct {
	body      []byte
	expiresAt time.Time
}
type loginFailure struct {
	count   int
	expires time.Time
}
type apiError struct {
	Code string `json:"code"`
}

type requestContextKey string

const (
	requestIDKey requestContextKey = "request-id"
	usernameKey  requestContextKey = "username"
)

const sessionCookieName = "iblog_session"

const (
	maxLoginBodyBytes   = 4 << 10
	maxArticleBodyBytes = 1 << 20
	maxTitleLen         = 200
	maxSlugLen          = 200
	maxSummaryLen       = 1000
	maxContentLen       = 200000
	maxTagLen           = 24
	maxArticleTags      = 10
)

type article struct {
	ID          int64         `json:"id"`
	Slug        string        `json:"slug"`
	Title       string        `json:"title"`
	Summary     string        `json:"summary"`
	Tags        []string      `json:"tags"`
	Status      string        `json:"status"`
	Views       int64         `json:"views"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
	PublishedAt *string       `json:"publishedAt"`
	ContentMD   *string       `json:"contentMd,omitempty"`
	ContentHTML template.HTML `json:"contentHtml,omitempty"`
	Previous    *articleLink  `json:"previous,omitempty"`
	Next        *articleLink  `json:"next,omitempty"`
}
type articleLink struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}
type friend struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": apiError{Code: code}})
}
func decode(r *http.Request, target any) error {
	return decodeLimit(r, target, 2<<20)
}

func decodeLimit(r *http.Request, target any, limit int64) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > limit {
		return errors.New("request body too large")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("unexpected content after JSON body")
	}
	return nil
}

func validArticleFields(title, slug, summary, content string, tags []string) bool {
	if title == "" || len([]rune(title)) > maxTitleLen || len([]rune(slug)) > maxSlugLen ||
		len([]rune(summary)) > maxSummaryLen || len([]rune(content)) > maxContentLen || len(tags) > maxArticleTags {
		return false
	}
	for _, tag := range tags {
		if len([]rune(strings.TrimSpace(tag))) > maxTagLen {
			return false
		}
	}
	return true
}
func validArticleSlug(slug string) bool {
	return slug == "" || regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(slug)
}
func now() string { return time.Now().UTC().Format(time.RFC3339) }
func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		writeJSON(w, 503, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "version": "1.1.2"})
}
func (s *server) robots(w http.ResponseWriter, r *http.Request) {
	base := s.canonicalBaseURL(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /api/\nSitemap: %s/sitemap.xml\n", base)
}
func (s *server) sitemap(w http.ResponseWriter, r *http.Request) {
	base := s.canonicalBaseURL(r)
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
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprint(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	for _, path := range []string{"/", "/articles", "/about", "/friends"} {
		fmt.Fprintf(w, "<url><loc>%s</loc></url>", html.EscapeString(base+path))
	}
	for _, item := range entries {
		fmt.Fprintf(w, "<url><loc>%s/articles/%s</loc><lastmod>%s</lastmod></url>", html.EscapeString(base), html.EscapeString(url.PathEscape(item.slug)), html.EscapeString(item.updated))
	}
	fmt.Fprint(w, "</urlset>")
}

func (s *server) listArticles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	admin := strings.HasPrefix(r.URL.Path, "/api/v1/admin/")
	cacheKey := ""
	var cacheGeneration uint64
	page, size, valid := pageParams(q)
	if !valid {
		fail(w, 400, "bad_request", "page must be >= 1 and pageSize must be between 1 and 100")
		return
	}
	if st := q.Get("status"); admin && st != "" && st != "draft" && st != "published" {
		fail(w, 400, "bad_request", "status 必须为 draft 或 published")
		return
	}
	sortBy := q.Get("sort")
	if !admin && sortBy == "updated" {
		fail(w, 400, "bad_request", "sort 参数无效")
		return
	}
	if sortBy != "" && sortBy != "updated" && sortBy != "oldest" && sortBy != "title" && sortBy != "views" {
		fail(w, 400, "bad_request", "sort 参数无效")
		return
	}
	if !admin {
		cacheKey = publicSearchCacheKey(q)
		cached, generation := s.getSearchCache(cacheKey)
		cacheGeneration = generation
		if cached != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
	}
	where := []string{}
	args := []any{}
	from := "articles a"
	if !admin {
		where = append(where, "a.status='published'")
	} else if st := q.Get("status"); st != "" {
		where = append(where, "a.status=?")
		args = append(args, st)
	}
	if text := strings.TrimSpace(q.Get("q")); text != "" {
		if utf8.RuneCountInString(text) < 3 {
			v := "%" + escapeLike(text) + "%"
			where = append(where, "(a.title LIKE ? ESCAPE '\\' OR a.summary LIKE ? ESCAPE '\\' OR a.content_md LIKE ? ESCAPE '\\' OR EXISTS(SELECT 1 FROM article_tags sx JOIN tags st ON st.id=sx.tag_id WHERE sx.article_id=a.id AND st.name LIKE ? ESCAPE '\\'))")
			args = append(args, v, v, v, v)
		} else {
			from = "articles a JOIN article_search ON article_search.rowid=a.id"
			where = append(where, "article_search MATCH ?")
			args = append(args, ftsQuery(text))
		}
	}
	if tag := strings.TrimSpace(q.Get("tag")); tag != "" {
		where = append(where, "EXISTS(SELECT 1 FROM article_tags x JOIN tags z ON z.id=x.tag_id WHERE x.article_id=a.id AND z.name=?)")
		args = append(args, tag)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+from+clause, args...).Scan(&total); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	queryArgs := append([]any{}, args...)
	orderBy := "COALESCE(a.published_at,a.updated_at) DESC,a.id DESC"
	if text := strings.TrimSpace(q.Get("q")); utf8.RuneCountInString(text) >= 3 {
		orderBy = "bm25(article_search, 5.0, 2.0, 1.0, 3.0) ASC, COALESCE(a.published_at,a.updated_at) DESC,a.id DESC"
		if sortBy == "" || (admin && sortBy == "updated") {
			orderBy = "CASE WHEN lower(a.title) LIKE lower(?) ESCAPE '\\' THEN 0 ELSE 1 END, " + orderBy
			queryArgs = append(queryArgs, "%"+escapeLike(text)+"%")
		}
	}
	switch sortBy {
	case "updated":
		if text := strings.TrimSpace(q.Get("q")); text == "" || utf8.RuneCountInString(text) < 3 {
			orderBy = "a.updated_at DESC,a.id DESC"
		}
	case "oldest":
		orderBy = "COALESCE(a.published_at,a.updated_at) ASC,a.id ASC"
	case "title":
		orderBy = "a.title COLLATE NOCASE ASC,a.id ASC"
	case "views":
		orderBy = "a.views DESC,a.id DESC"
	}
	queryArgs = append(queryArgs, size, (page-1)*size)
	rows, err := s.db.QueryContext(r.Context(), "SELECT a.id,a.slug,a.title,a.summary,a.status,a.views,a.created_at,a.updated_at,a.published_at FROM "+from+clause+" ORDER BY "+orderBy+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer rows.Close()
	items := []article{}
	for rows.Next() {
		var a article
		var pub sql.NullString
		if err = rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &a.Status, &a.Views, &a.CreatedAt, &a.UpdatedAt, &pub); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		if pub.Valid {
			a.PublishedAt = &pub.String
		}
		items = append(items, a)
	}
	if err = rows.Err(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if err = rows.Close(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	tagsByArticle, tagsErr := s.tagsForMany(r.Context(), items)
	if tagsErr != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	for i := range items {
		items[i].Tags = tagsByArticle[items[i].ID]
		if items[i].Tags == nil {
			items[i].Tags = []string{}
		}
	}
	response := map[string]any{"items": items, "total": total, "page": page, "pageSize": size}
	if cacheKey != "" {
		if body, marshalErr := json.Marshal(response); marshalErr == nil {
			s.setSearchCache(cacheKey, body, cacheGeneration)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}
	}
	writeJSON(w, 200, response)
}
func escapeLike(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return r.Replace(s)
}

func ftsQuery(input string) string {
	part := strings.ReplaceAll(strings.TrimSpace(input), `"`, `""`)
	return `"` + part + `"`
}

func publicSearchCacheKey(q url.Values) string {
	canonical, _ := json.Marshal([]string{
		strings.TrimSpace(q.Get("q")),
		strings.TrimSpace(q.Get("tag")),
		q.Get("page"),
		q.Get("pageSize"),
		q.Get("sort"),
	})
	return fmt.Sprintf("%x", sha256.Sum256(canonical))
}

func (s *server) getSearchCache(key string) ([]byte, uint64) {
	s.searchCacheMu.Lock()
	defer s.searchCacheMu.Unlock()
	entry, exists := s.searchCache[key]
	if !exists || time.Now().After(entry.expiresAt) {
		delete(s.searchCache, key)
		return nil, s.searchCacheGeneration
	}
	return append([]byte(nil), entry.body...), s.searchCacheGeneration
}

func (s *server) setSearchCache(key string, body []byte, generation uint64) {
	s.searchCacheMu.Lock()
	defer s.searchCacheMu.Unlock()
	if generation != s.searchCacheGeneration {
		return
	}
	if s.searchCache == nil {
		s.searchCache = make(map[string]searchCacheEntry)
	}
	if len(s.searchCache) >= 128 {
		for staleKey, entry := range s.searchCache {
			if time.Now().After(entry.expiresAt) || len(s.searchCache) >= 96 {
				delete(s.searchCache, staleKey)
			}
		}
	}
	s.searchCache[key] = searchCacheEntry{body: append([]byte(nil), body...), expiresAt: time.Now().Add(15 * time.Second)}
}

func (s *server) invalidateSearchCache() {
	s.searchCacheMu.Lock()
	s.searchCacheGeneration++
	s.searchCache = make(map[string]searchCacheEntry)
	s.searchCacheMu.Unlock()
}
func pageParams(q url.Values) (int, int, bool) {
	page, size := 1, 10
	if value := q.Get("page"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			page = n
		} else {
			return 0, 0, false
		}
	}
	if value := q.Get("pageSize"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			size = n
		} else {
			return 0, 0, false
		}
	}
	return page, size, page >= 1 && size >= 1 && size <= 100 && page <= 1000000
}
func (s *server) tagsFor(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT t.name FROM tags t JOIN article_tags x ON x.tag_id=t.id WHERE x.article_id=? ORDER BY t.name", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// tagsForMany loads tags for a whole page in one query. The previous
// per-article query made the admin list increasingly slow as article count
// grew, even though each individual query was fast.
func (s *server) tagsForMany(ctx context.Context, items []article) (map[int64][]string, error) {
	result := make(map[int64][]string, len(items))
	if len(items) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(items))
	args := make([]any, len(items))
	for i, item := range items {
		placeholders[i] = "?"
		args[i] = item.ID
	}
	rows, err := s.db.QueryContext(ctx, "SELECT x.article_id,t.name FROM article_tags x JOIN tags t ON t.id=x.tag_id WHERE x.article_id IN ("+strings.Join(placeholders, ",")+") ORDER BY t.name", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var articleID int64
		var name string
		if err := rows.Scan(&articleID, &name); err != nil {
			return nil, err
		}
		result[articleID] = append(result[articleID], name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *server) getArticle(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	a, err := s.articleBy(r, slug, true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "not_found", "文章不存在")
		} else {
			fail(w, 500, "internal", "服务器内部错误")
		}
		return
	}
	if a.Status != "published" {
		fail(w, 404, "not_found", "文章不存在")
		return
	}
	var previous, next articleLink
	stamp := a.UpdatedAt
	if a.PublishedAt != nil {
		stamp = *a.PublishedAt
	}
	if err := s.db.QueryRowContext(r.Context(), "SELECT slug,title FROM articles WHERE status='published' AND (COALESCE(published_at,updated_at) < ? OR (COALESCE(published_at,updated_at) = ? AND id < ?)) ORDER BY COALESCE(published_at,updated_at) DESC,id DESC LIMIT 1", stamp, stamp, a.ID).Scan(&previous.Slug, &previous.Title); err == nil {
		a.Previous = &previous
	} else if !errors.Is(err, sql.ErrNoRows) {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if err := s.db.QueryRowContext(r.Context(), "SELECT slug,title FROM articles WHERE status='published' AND (COALESCE(published_at,updated_at) > ? OR (COALESCE(published_at,updated_at) = ? AND id > ?)) ORDER BY COALESCE(published_at,updated_at) ASC,id ASC LIMIT 1", stamp, stamp, a.ID).Scan(&next.Slug, &next.Title); err == nil {
		a.Next = &next
	} else if !errors.Is(err, sql.ErrNoRows) {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 200, a)
}
func (s *server) recordArticleView(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	a, err := s.articleBy(r, slug, true)
	if err != nil || a.Status != "published" {
		fail(w, http.StatusNotFound, "not_found", "文章不存在")
		return
	}
	s.recordView(a.ID)
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) recordView(articleID int64) {
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.pendingViews == nil {
		s.pendingViews = make(map[int64]int64)
	}
	s.pendingViews[articleID]++
}

func (s *server) flushViews(ctx context.Context) error {
	s.viewMu.Lock()
	if len(s.pendingViews) == 0 {
		s.viewMu.Unlock()
		return nil
	}
	pending := s.pendingViews
	s.pendingViews = make(map[int64]int64)
	s.viewMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err == nil {
		for id, count := range pending {
			if _, err = tx.ExecContext(ctx, "UPDATE articles SET views=views+? WHERE id=?", count, id); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	if err != nil {
		s.viewMu.Lock()
		for id, count := range pending {
			s.pendingViews[id] += count
		}
		s.viewMu.Unlock()
		return err
	}
	return nil
}
func (s *server) articleBy(r *http.Request, key string, bySlug bool) (article, error) {
	var a article
	var pub sql.NullString
	v, err := strconv.ParseInt(key, 10, 64)
	var row *sql.Row
	if bySlug {
		row = s.db.QueryRowContext(r.Context(), "SELECT id,slug,title,summary,content_md,status,views,created_at,updated_at,published_at FROM articles WHERE slug=?", key)
	} else {
		if err != nil || v < 1 {
			return a, sql.ErrNoRows
		}
		row = s.db.QueryRowContext(r.Context(), "SELECT id,slug,title,summary,content_md,status,views,created_at,updated_at,published_at FROM articles WHERE id=?", v)
	}
	var content string
	if e := row.Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &content, &a.Status, &a.Views, &a.CreatedAt, &a.UpdatedAt, &pub); e != nil {
		if !bySlug || !errors.Is(e, sql.ErrNoRows) {
			return a, e
		}
		row = s.db.QueryRowContext(r.Context(), "SELECT a.id,a.slug,a.title,a.summary,a.content_md,a.status,a.views,a.created_at,a.updated_at,a.published_at FROM article_slug_aliases x JOIN articles a ON a.id=x.article_id WHERE x.slug=?", key)
		if e = row.Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &content, &a.Status, &a.Views, &a.CreatedAt, &a.UpdatedAt, &pub); e != nil {
			return a, e
		}
	}
	a.ContentMD = &content
	if a.Status == "published" || strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		a.ContentHTML, err = renderMarkdown(normalizedMarkdown(content))
		if err != nil {
			return a, err
		}
	}
	if pub.Valid {
		a.PublishedAt = &pub.String
	}
	a.Tags, err = s.tagsFor(r.Context(), a.ID)
	if err != nil {
		return a, err
	}
	return a, nil
}
func (s *server) tags(w http.ResponseWriter, r *http.Request) {
	admin := strings.HasPrefix(r.URL.Path, "/api/v1/admin/")
	query := "SELECT t.id,t.name,COUNT(x.article_id) FROM tags t LEFT JOIN article_tags x ON x.tag_id=t.id GROUP BY t.id ORDER BY t.name"
	if !admin {
		query = "SELECT t.id,t.name,COUNT(a.id) FROM tags t LEFT JOIN article_tags x ON x.tag_id=t.id LEFT JOIN articles a ON a.id=x.article_id AND a.status='published' GROUP BY t.id HAVING COUNT(a.id)>0 ORDER BY t.name"
	}
	rows, err := s.db.QueryContext(r.Context(), query)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer rows.Close()
	type item struct {
		ID    int64  `json:"id,omitempty"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	items := []item{}
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.ID, &v.Name, &v.Count); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		if !admin {
			v.ID = 0
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if admin {
		writeJSON(w, 200, map[string]any{"items": items})
	} else {
		writeJSON(w, 200, items)
	}
}
func (s *server) friends(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT id,name,url,avatar,description,sort_order,created_at,updated_at FROM friend_links ORDER BY sort_order,id")
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer rows.Close()
	items := []friend{}
	for rows.Next() {
		var f friend
		if err := rows.Scan(&f.ID, &f.Name, &f.URL, &f.Avatar, &f.Description, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (s *server) settings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT setting_key,value_json FROM site_settings")
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer rows.Close()
	m := map[string]json.RawMessage{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil || !json.Valid([]byte(v)) {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		m[k] = json.RawMessage(v)
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		writeJSON(w, 200, map[string]any{"settings": map[string]any{"about": m["about"], "contacts": m["contacts"], "site": m["site"]}})
		return
	}
	writeJSON(w, 200, m)
}

func (s *server) createArticle(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title   string   `json:"title"`
		Slug    string   `json:"slug"`
		Summary string   `json:"summary"`
		Content string   `json:"contentMd"`
		Status  string   `json:"status"`
		Tags    []string `json:"tags"`
	}
	if decodeLimit(r, &in, maxArticleBodyBytes) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Slug = strings.TrimSpace(in.Slug)
	if !validArticleFields(in.Title, in.Slug, in.Summary, in.Content, in.Tags) || !validArticleSlug(in.Slug) {
		fail(w, 400, "bad_request", "文章字段超出允许范围")
		return
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.Status != "draft" && in.Status != "published" {
		fail(w, 400, "bad_request", "status 必须为 draft 或 published")
		return
	}
	if in.Slug == "" {
		var err error
		in.Slug, err = makeArticleSlug(s.db, in.Title)
		if err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
	}
	t := now()
	var pub any
	if in.Status == "published" {
		pub = t
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(r.Context(), "INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES(?,?,?,?,?,?,?,?)", in.Slug, in.Title, in.Summary, in.Content, in.Status, t, t, pub)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			fail(w, 409, "conflict", "slug 已存在")
		} else {
			fail(w, 500, "internal", "服务器内部错误")
		}
		return
	}
	id, _ := res.LastInsertId()
	if err = setArticleTags(r, tx, id, in.Tags); err != nil {
		fail(w, 400, "bad_request", "标签无效")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	s.invalidateSearchCache()
	a, err := s.articleBy(r, strconv.FormatInt(id, 10), false)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 201, a)
}

func makeSlug() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return "post-" + time.Now().UTC().Format("20060102") + "-" + string(b), nil
}

func makeArticleSlug(db *sql.DB, title string) (string, error) {
	base := slugifyTitle(title)
	if base == "" {
		return makeSlug()
	}
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM articles WHERE slug=? OR EXISTS (SELECT 1 FROM article_slug_aliases WHERE slug=?)", candidate, candidate).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return makeSlug()
}

func slugifyTitle(title string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(strings.TrimSuffix(b.String(), "-"), "-")
}
func setArticleTags(r *http.Request, tx *sql.Tx, id int64, names []string) error {
	var err error
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM article_tags WHERE article_id=?", id); err != nil {
		return err
	}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		_, err = tx.ExecContext(r.Context(), "INSERT OR IGNORE INTO tags(name) VALUES(?)", n)
		if err != nil {
			return err
		}
		var tid int64
		if err = tx.QueryRowContext(r.Context(), "SELECT id FROM tags WHERE name=? COLLATE NOCASE", n).Scan(&tid); err != nil {
			return err
		}
		if _, err = tx.ExecContext(r.Context(), "INSERT OR IGNORE INTO article_tags(article_id,tag_id) VALUES(?,?)", id, tid); err != nil {
			return err
		}
	}
	return nil
}
func (s *server) updateArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	old, err := s.articleBy(r, id, false)
	if err != nil {
		fail(w, 404, "not_found", "文章不存在")
		return
	}
	var in struct {
		Title   string   `json:"title"`
		Slug    string   `json:"slug"`
		Summary string   `json:"summary"`
		Content string   `json:"contentMd"`
		Status  string   `json:"status"`
		Tags    []string `json:"tags"`
	}
	if decodeLimit(r, &in, maxArticleBodyBytes) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		fail(w, 400, "bad_request", "标题不能为空")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Slug = strings.TrimSpace(in.Slug)
	if !validArticleFields(in.Title, in.Slug, in.Summary, in.Content, in.Tags) || (in.Slug != old.Slug && !validArticleSlug(in.Slug)) {
		fail(w, 400, "bad_request", "文章字段超出允许范围")
		return
	}
	if in.Slug == "" {
		fail(w, 400, "bad_request", "更新时必须填写 slug")
		return
	}
	if in.Status != "draft" && in.Status != "published" {
		fail(w, 400, "bad_request", "status 必须为 draft 或 published")
		return
	}
	t := now()
	pub := old.PublishedAt
	if in.Status == "published" && pub == nil {
		pub = &t
	}
	var pv any
	if pub != nil {
		pv = *pub
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	defer tx.Rollback()
	if old.Slug != in.Slug {
		if _, err = tx.ExecContext(r.Context(), "INSERT OR IGNORE INTO article_slug_aliases(slug,article_id,created_at) VALUES(?,?,?)", old.Slug, old.ID, t); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
	}
	res, err := tx.ExecContext(r.Context(), "UPDATE articles SET slug=?,title=?,summary=?,content_md=?,status=?,updated_at=?,published_at=? WHERE id=?", in.Slug, in.Title, in.Summary, in.Content, in.Status, t, pv, old.ID)
	if err != nil {
		if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
			fail(w, 409, "conflict", "slug 已存在")
		} else {
			fail(w, 500, "internal", "服务器内部错误")
		}
		return
	}
	if changed, rowsErr := res.RowsAffected(); rowsErr != nil || changed == 0 {
		if rowsErr != nil {
			fail(w, 500, "internal", "服务器内部错误")
		} else {
			fail(w, 404, "not_found", "文章不存在")
		}
		return
	}
	if err = setArticleTags(r, tx, old.ID, in.Tags); err != nil {
		fail(w, 400, "bad_request", "标签无效")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	s.invalidateSearchCache()
	a, err := s.articleBy(r, id, false)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 200, a)
}
func (s *server) deleteArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := s.db.ExecContext(r.Context(), "DELETE FROM articles WHERE id=?", id)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "not_found", "文章不存在")
		return
	}
	s.invalidateSearchCache()
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) batchArticles(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if decodeLimit(r, &in, 64<<10) != nil || len(in.IDs) == 0 || len(in.IDs) > 100 || (in.Action != "publish" && in.Action != "archive" && in.Action != "delete") {
		fail(w, 400, "bad_request", "批量操作参数无效")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(in.IDs)), ",")
	args := make([]any, len(in.IDs))
	for i, id := range in.IDs {
		args[i] = id
	}
	var result sql.Result
	switch in.Action {
	case "publish":
		result, err = tx.ExecContext(r.Context(), "UPDATE articles SET status='published', published_at=COALESCE(published_at,?), updated_at=? WHERE id IN ("+placeholders+")", append([]any{now(), now()}, args...)...)
	case "archive":
		result, err = tx.ExecContext(r.Context(), "UPDATE articles SET status='draft', updated_at=? WHERE id IN ("+placeholders+")", append([]any{now()}, args...)...)
	case "delete":
		result, err = tx.ExecContext(r.Context(), "DELETE FROM articles WHERE id IN ("+placeholders+")", args...)
	}
	if err != nil || tx.Commit() != nil {
		_ = tx.Rollback()
		fail(w, 500, "internal", "批量操作失败")
		return
	}
	n, _ := result.RowsAffected()
	s.invalidateSearchCache()
	writeJSON(w, 200, map[string]any{"affected": n})
}

func (s *server) saveTag(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if decodeLimit(r, &in, maxLoginBodyBytes) != nil || strings.TrimSpace(in.Name) == "" || len([]rune(strings.TrimSpace(in.Name))) > maxTagLen {
		fail(w, 400, "bad_request", "标签名不能为空")
		return
	}
	if r.Method == "POST" {
		res, err := s.db.ExecContext(r.Context(), "INSERT INTO tags(name) VALUES(?)", strings.TrimSpace(in.Name))
		if err != nil {
			fail(w, 409, "conflict", "标签已存在")
			return
		}
		id, _ := res.LastInsertId()
		s.invalidateSearchCache()
		writeJSON(w, 201, map[string]any{"id": id, "name": strings.TrimSpace(in.Name), "count": 0})
		return
	}
	res, err := s.db.ExecContext(r.Context(), "UPDATE tags SET name=? WHERE id=?", strings.TrimSpace(in.Name), r.PathValue("id"))
	if err != nil {
		fail(w, 409, "conflict", "标签已存在")
		return
	}
	changed, _ := res.RowsAffected()
	if changed == 0 {
		fail(w, 404, "not_found", "标签不存在")
		return
	}
	var count int
	if err = s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM article_tags WHERE tag_id=?", r.PathValue("id")).Scan(&count); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	s.invalidateSearchCache()
	writeJSON(w, 200, map[string]any{"id": id, "name": strings.TrimSpace(in.Name), "count": count})
}
func (s *server) deleteTag(w http.ResponseWriter, r *http.Request) {
	res, err := s.db.ExecContext(r.Context(), "DELETE FROM tags WHERE id=?", r.PathValue("id"))
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "not_found", "标签不存在")
		return
	}
	s.invalidateSearchCache()
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) saveFriend(w http.ResponseWriter, r *http.Request) {
	var f friend
	if decodeLimit(r, &f, 64<<10) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	f.Name = strings.TrimSpace(f.Name)
	f.URL = strings.TrimSpace(f.URL)
	f.Avatar = strings.TrimSpace(f.Avatar)
	if f.Name == "" || f.URL == "" || len([]rune(f.Name)) > 100 || len([]rune(f.URL)) > 2048 || len([]rune(f.Avatar)) > 2048 || len([]rune(f.Description)) > 1000 {
		fail(w, 400, "bad_request", "名称和链接不能为空")
		return
	}
	if !strings.HasPrefix(f.URL, "http://") && !strings.HasPrefix(f.URL, "https://") {
		fail(w, 400, "bad_request", "url 必须以 http:// 或 https:// 开头")
		return
	}
	u, err := url.Parse(f.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fail(w, 400, "bad_request", "url 必须以 http:// 或 https:// 开头")
		return
	}
	t := now()
	if r.Method == "POST" {
		res, err := s.db.ExecContext(r.Context(), "INSERT INTO friend_links(name,url,avatar,description,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", f.Name, f.URL, f.Avatar, f.Description, f.SortOrder, t, t)
		if err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		f.ID, _ = res.LastInsertId()
		f.CreatedAt = t
		f.UpdatedAt = t
		writeJSON(w, 201, f)
		return
	}
	id := r.PathValue("id")
	res, err := s.db.ExecContext(r.Context(), "UPDATE friend_links SET name=?,url=?,avatar=?,description=?,sort_order=?,updated_at=? WHERE id=?", f.Name, f.URL, f.Avatar, f.Description, f.SortOrder, t, id)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "not_found", "友链不存在")
		return
	}
	f.ID, _ = strconv.ParseInt(id, 10, 64)
	f.UpdatedAt = t
	writeJSON(w, 200, f)
}
func (s *server) deleteFriend(w http.ResponseWriter, r *http.Request) {
	res, err := s.db.ExecContext(r.Context(), "DELETE FROM friend_links WHERE id=?", r.PathValue("id"))
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "not_found", "友链不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) saveSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key != "about" && key != "contacts" && key != "site" {
		fail(w, 400, "invalid_key", "unknown setting key")
		return
	}
	var raw json.RawMessage
	if decodeLimit(r, &raw, 256<<10) != nil || len(raw) == 0 || !json.Valid(raw) {
		fail(w, 400, "bad_request", "setting body must be valid JSON")
		return
	}
	if key == "site" {
		normalized, ok := normalizeSiteSetting(raw)
		if !ok {
			fail(w, 400, "bad_request", "site url must be an http(s) origin without a path")
			return
		}
		raw = normalized
	}
	_, err := s.db.ExecContext(r.Context(), "INSERT INTO site_settings(setting_key,value_json,updated_at) VALUES(?,?,?) ON CONFLICT(setting_key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at", key, string(raw), now())
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// normalizeSiteSetting keeps the site object extensible while validating the
// public address used to build robots.txt and sitemap.xml. An empty url clears
// the setting so the environment fallback applies again.
func normalizeSiteSetting(raw json.RawMessage) (json.RawMessage, bool) {
	var site map[string]json.RawMessage
	if err := json.Unmarshal(raw, &site); err != nil {
		return nil, false
	}
	value, present := site["url"]
	if !present {
		return raw, true
	}
	var siteURL string
	if err := json.Unmarshal(value, &siteURL); err != nil {
		return nil, false
	}
	if strings.TrimSpace(siteURL) == "" {
		delete(site, "url")
	} else {
		base, ok := parsePublicBase(siteURL)
		if !ok {
			return nil, false
		}
		normalized, err := json.Marshal(base)
		if err != nil {
			return nil, false
		}
		site["url"] = normalized
	}
	encoded, err := json.Marshal(site)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

// Password hashes use a salted PBKDF2-HMAC-SHA256 encoding for clean installs.
func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	h := pbkdf2([]byte(password), salt, 120000, 32)
	return fmt.Sprintf("pbkdf2$120000$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}
func verifyPassword(password, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 4 || p[0] != "pbkdf2" {
		return false
	}
	i, e := strconv.Atoi(p[1])
	if e != nil || i < 10000 || i > 1000000 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[2])
	if e != nil {
		return false
	}
	want, e := base64.RawStdEncoding.DecodeString(p[3])
	if e != nil {
		return false
	}
	return hmac.Equal(want, pbkdf2([]byte(password), salt, i, len(want)))
}
func pbkdf2(password, salt []byte, iter, n int) []byte {
	out := make([]byte, 0, n)
	for block := uint32(1); len(out) < n; block++ {
		m := hmac.New(sha256.New, password)
		m.Write(salt)
		m.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := m.Sum(nil)
		t := append([]byte{}, u...)
		for i := 1; i < iter; i++ {
			m = hmac.New(sha256.New, password)
			m.Write(u)
			u = m.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:n]
}
func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeLimit(r, &in, maxLoginBodyBytes) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	if strings.TrimSpace(in.Username) == "" || in.Password == "" {
		fail(w, 400, "bad_request", "用户名和密码不能为空")
		return
	}
	ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
	if splitErr != nil {
		ip = r.RemoteAddr
	}
	if s.loginBlocked(ip) {
		fail(w, 429, "rate_limited", "登录尝试过于频繁，请稍后再试")
		return
	}
	var id int64
	var username, hash string
	if s.db.QueryRowContext(r.Context(), "SELECT id,username,password_hash FROM users ORDER BY id LIMIT 1").Scan(&id, &username, &hash) != nil || username != in.Username || !verifyPassword(in.Password, hash) {
		s.recordLoginFailure(ip)
		fail(w, 401, "unauthorized", "用户名或密码错误")
		return
	}
	s.clearLoginFailures(ip)
	ttl := tokenTTLHours()
	exp := time.Now().UTC().Add(time.Duration(ttl) * time.Hour)
	var version int
	if err := s.db.QueryRowContext(r.Context(), "SELECT token_version FROM users WHERE id=?", id).Scan(&version); err != nil {
		fail(w, 500, "internal", "登录服务暂不可用")
		return
	}
	token, err := makeToken(username, version, env("BLOG_JWT_SECRET", ""), exp)
	if err != nil {
		fail(w, 500, "internal", "JWT secret 未配置")
		return
	}
	setSessionCookie(w, token, exp)
	writeJSON(w, 200, map[string]string{"expiresAt": exp.Format(time.RFC3339)})
}
func (s *server) loginBlocked(ip string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	f := s.loginFailures[ip]
	return time.Now().Before(f.expires) && f.count >= 5
}
func (s *server) recordLoginFailure(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginFailures == nil {
		s.loginFailures = make(map[string]loginFailure)
	}
	if len(s.loginFailures) >= 1024 {
		for key, failure := range s.loginFailures {
			if time.Now().After(failure.expires) {
				delete(s.loginFailures, key)
			}
		}
		if len(s.loginFailures) >= 1024 && s.loginFailures[ip].expires.IsZero() {
			oldest := ""
			for key, failure := range s.loginFailures {
				if oldest == "" || failure.expires.Before(s.loginFailures[oldest].expires) {
					oldest = key
				}
			}
			delete(s.loginFailures, oldest)
		}
	}
	f := s.loginFailures[ip]
	if time.Now().After(f.expires) {
		f.count = 0
		f.expires = time.Now().Add(10 * time.Minute)
	}
	f.count++
	s.loginFailures[ip] = f
}
func (s *server) clearLoginFailures(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	delete(s.loginFailures, ip)
}
func makeToken(username string, version int, secret string, exp time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("missing signing secret")
	}
	enc := base64.RawURLEncoding
	head, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	body, _ := json.Marshal(map[string]any{"username": username, "iss": "blog", "sub": username, "ver": version, "iat": time.Now().Unix(), "exp": exp.Unix()})
	input := enc.EncodeToString(head) + "." + enc.EncodeToString(body)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(input))
	return input + "." + enc.EncodeToString(m.Sum(nil)), nil
}
func (s *server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			if cookie, err := r.Cookie(sessionCookieName); err == nil {
				authorization = "Bearer " + cookie.Value
			}
		}
		if !strings.HasPrefix(authorization, "Bearer ") {
			fail(w, 401, "unauthorized", "missing bearer token")
			return
		}
		parts := strings.Split(strings.TrimPrefix(authorization, "Bearer "), ".")
		if len(parts) != 3 {
			fail(w, 401, "unauthorized", "missing bearer token")
			return
		}
		header, headErr := base64.RawURLEncoding.DecodeString(parts[0])
		var hdr struct {
			Alg string `json:"alg"`
			Typ string `json:"typ"`
		}
		if headErr != nil || json.Unmarshal(header, &hdr) != nil || hdr.Alg != "HS256" || hdr.Typ != "JWT" {
			fail(w, 401, "unauthorized", "invalid token")
			return
		}
		m := hmac.New(sha256.New, []byte(env("BLOG_JWT_SECRET", "")))
		m.Write([]byte(parts[0] + "." + parts[1]))
		sig, e := base64.RawURLEncoding.DecodeString(parts[2])
		if e != nil || !hmac.Equal(sig, m.Sum(nil)) {
			fail(w, 401, "unauthorized", "invalid token")
			return
		}
		b, e := base64.RawURLEncoding.DecodeString(parts[1])
		var c struct {
			Username string `json:"username"`
			Exp      int64  `json:"exp"`
			IssuedAt int64  `json:"iat"`
			Issuer   string `json:"iss"`
			Subject  string `json:"sub"`
			Version  int    `json:"ver"`
		}
		nowUnix := time.Now().Unix()
		if e != nil || json.Unmarshal(b, &c) != nil || c.Username == "" || c.Issuer != "blog" || c.Subject != c.Username || c.IssuedAt <= 0 || c.IssuedAt > nowUnix+60 || c.Exp <= nowUnix || c.Exp <= c.IssuedAt {
			fail(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		var stored string
		var version int
		if s.db.QueryRowContext(r.Context(), "SELECT username,token_version FROM users ORDER BY id LIMIT 1").Scan(&stored, &version) != nil || stored != c.Username || c.Version < 1 || version != c.Version {
			fail(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), usernameKey, c.Username)))
	})
}
func (s *server) changeAccount(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/admin/account/username":
		var in struct {
			Username string `json:"username"`
		}
		if decodeLimit(r, &in, maxLoginBodyBytes) != nil || !regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`).MatchString(in.Username) {
			fail(w, 400, "bad_request", "用户名需为3-32位字母数字-_")
			return
		}
		if _, err := s.db.ExecContext(r.Context(), "UPDATE users SET username=?,token_version=token_version+1 WHERE id=(SELECT id FROM users ORDER BY id LIMIT 1)", in.Username); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		s.loginFromAccount(w, r, in.Username, true)
	case "/api/v1/admin/account/password":
		var in struct {
			Current string `json:"currentPassword"`
			New     string `json:"newPassword"`
		}
		if decodeLimit(r, &in, maxLoginBodyBytes) != nil || len([]rune(in.New)) < 8 || len([]rune(in.New)) > 128 {
			fail(w, 400, "bad_request", "新密码至少需要 8 位")
			return
		}
		var id int64
		var u, h string
		if s.db.QueryRowContext(r.Context(), "SELECT id,username,password_hash FROM users ORDER BY id LIMIT 1").Scan(&id, &u, &h) != nil || !verifyPassword(in.Current, h) {
			fail(w, 401, "bad_credentials", "当前密码不正确")
			return
		}
		hash, err := passwordHash(in.New)
		if err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		if _, err = s.db.ExecContext(r.Context(), "UPDATE users SET password_hash=?,token_version=token_version+1 WHERE id=?", hash, id); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		s.loginFromAccount(w, r, u, false)
	}
}
func (s *server) loginFromAccount(w http.ResponseWriter, r *http.Request, u string, includeUsername bool) {
	ttl := tokenTTLHours()
	exp := time.Now().Add(time.Duration(ttl) * time.Hour)
	var version int
	if e := s.db.QueryRowContext(r.Context(), "SELECT token_version FROM users WHERE username=?", u).Scan(&version); e != nil {
		fail(w, 500, "internal", "登录服务暂不可用")
		return
	}
	token, e := makeToken(u, version, env("BLOG_JWT_SECRET", ""), exp)
	if e != nil {
		fail(w, 500, "internal", "JWT secret 未配置")
		return
	}
	response := map[string]string{"expiresAt": exp.UTC().Format(time.RFC3339)}
	setSessionCookie(w, token, exp)
	if includeUsername {
		response["username"] = u
	}
	writeJSON(w, 200, response)
}

func tokenTTLHours() int {
	ttl, _ := strconv.Atoi(env("BLOG_TOKEN_TTL_HOURS", "72"))
	if ttl < 1 || ttl > 720 {
		return 72
	}
	return ttl
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: env("BLOG_COOKIE_SECURE", "false") == "true", SameSite: http.SameSiteLaxMode})
}
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: env("BLOG_COOKIE_SECURE", "false") == "true", SameSite: http.SameSiteLaxMode})
}
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	_, err := s.db.ExecContext(r.Context(), "UPDATE users SET token_version=token_version+1")
	if err != nil {
		fail(w, 500, "internal", "退出登录失败")
		return
	}
	clearSessionCookie(w)
	writeJSON(w, 200, map[string]bool{"ok": true})
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

func (s *server) setupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 200, map[string]bool{"setupRequired": count == 0})
}

func (s *server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeLimit(r, &in, maxLoginBodyBytes) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`).MatchString(in.Username) {
		fail(w, 400, "bad_request", "用户名需为 3-32 位字母、数字、- 或 _")
		return
	}
	if len([]rune(in.Password)) < 8 || len([]rune(in.Password)) > 128 {
		fail(w, 400, "bad_request", "密码长度需为 8-128 个字符")
		return
	}
	hash, err := passwordHash(in.Password)
	if err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	if count != 0 {
		fail(w, 409, "already_initialized", "管理员账号已初始化")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO users(username,password_hash,created_at) VALUES(?,?,?)", in.Username, hash, now()); err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 409, "already_initialized", "管理员账号已初始化")
		return
	}
	ttl := tokenTTLHours()
	exp := time.Now().UTC().Add(time.Duration(ttl) * time.Hour)
	token, err := makeToken(in.Username, 1, env("BLOG_JWT_SECRET", ""), exp)
	if err != nil {
		fail(w, 500, "internal", "登录服务暂不可用")
		return
	}
	setSessionCookie(w, token, exp)
	writeJSON(w, 201, map[string]string{"expiresAt": exp.Format(time.RFC3339)})
}

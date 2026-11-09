package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

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
	var taken bool
	if err = tx.QueryRowContext(r.Context(), "SELECT EXISTS (SELECT 1 FROM articles WHERE slug=? UNION ALL SELECT 1 FROM article_slug_aliases WHERE slug=?)", in.Slug, in.Slug).Scan(&taken); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if taken {
		fail(w, 409, "conflict", "slug 已存在")
		return
	}
	res, err := tx.ExecContext(r.Context(), "INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES(?,?,?,?,?,?,?,?)", in.Slug, in.Title, in.Summary, in.Content, in.Status, t, t, pub)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			fail(w, 409, "conflict", "slug 已存在")
		} else {
			fail(w, 500, "internal", "服务器内部错误")
		}
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if err = setArticleTags(r, tx, id, in.Tags); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
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
		var taken bool
		if err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM articles WHERE slug=? UNION ALL SELECT 1 FROM article_slug_aliases WHERE slug=?)", candidate, candidate).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
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
		var taken bool
		if err = tx.QueryRowContext(r.Context(), "SELECT EXISTS (SELECT 1 FROM articles WHERE slug=? AND id<>? UNION ALL SELECT 1 FROM article_slug_aliases WHERE slug=?)", in.Slug, old.ID, in.Slug).Scan(&taken); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		if taken {
			fail(w, 409, "conflict", "slug 已存在")
			return
		}
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
		fail(w, 500, "internal", "服务器内部错误")
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
	n, err := res.RowsAffected()
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
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
	n, err := result.RowsAffected()
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	s.invalidateSearchCache()
	writeJSON(w, 200, map[string]any{"affected": n})
}

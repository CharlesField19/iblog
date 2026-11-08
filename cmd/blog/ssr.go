package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type ssrPage struct {
	Kind        string
	RequestPath string
	Title       string
	Description string
	Canonical   string
	Robots      string
	OGType      string
	SiteName    string
	Site        ssrSite
	Contacts    map[string]string
	About       ssrAbout
	Articles    []article
	Article     *article
	Friends     []friend
	Tags        []string
	Query       string
	Tag         string
	Page        int
	PageSize    int
	Total       int
	PrevURL     string
	NextURL     string
	NotFound    bool
	JSONLD      template.JS
	InitialJSON template.JS
	CSSPath     string
	JSPath      string
	RSSPath     string
}

type ssrSite struct {
	Name        string
	TitlePrefix string
	TitleAccent string
	Subtitle    string
	FooterName  string
	FooterText  string
	Author      string
}

type ssrAbout struct {
	Intro       string
	Bio         string
	Skills      []string
	Timeline    []ssrTimeline
	ContactNote string
}

type ssrTimeline struct {
	Year        string
	Title       string
	Description string
}

type ssrTemplateData struct {
	Page ssrPage
}

var ssrTemplate = template.Must(template.New("ssr").Funcs(template.FuncMap{
	"slugURL": func(slug string) string { return "/articles/" + url.PathEscape(slug) },
}).Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="theme-color" content="#f5f5f7">
  <meta name="description" content="{{.Page.Description}}">
  <meta name="robots" content="{{.Page.Robots}}">
  <meta property="og:site_name" content="{{.Page.SiteName}}">
  <meta property="og:title" content="{{.Page.Title}}">
  <meta property="og:description" content="{{.Page.Description}}">
  <meta property="og:type" content="{{.Page.OGType}}">
  <meta property="og:url" content="{{.Page.Canonical}}">
  <meta property="og:image" content="{{.Page.CanonicalBase}}/favicon.ico">
  <meta name="twitter:card" content="summary">
  <meta name="twitter:title" content="{{.Page.Title}}">
  <meta name="twitter:description" content="{{.Page.Description}}">
  {{if not .Page.NotFound}}<link rel="canonical" href="{{.Page.Canonical}}">{{end}}
  <link rel="alternate" type="application/rss+xml" title="{{.Page.SiteName}} RSS" href="{{.Page.RSSPath}}">
  <link rel="icon" href="/favicon.ico">
  {{if .Page.CSSPath}}<link rel="stylesheet" href="{{.Page.CSSPath}}">{{end}}
  {{if .Page.JSONLD}}<script type="application/ld+json">{{.Page.JSONLD}}</script>{{end}}
  <title>{{.Page.Title}}</title>
</head>
<body>
  <div id="app">
  <header class="ssr-header"><a href="/">{{.Page.SiteName}}</a><nav><a href="/">首页</a><a href="/articles">文章</a><a href="/friends">友链</a><a href="/about">关于</a></nav></header>
  <main class="ssr-main">
    {{if .Page.NotFound}}
      <article class="ssr-card"><h1>页面不存在</h1><p>你访问的页面不存在或已被移除。</p><p><a href="/">返回首页</a></p></article>
    {{else if eq .Page.Kind "home"}}
      <section class="ssr-hero"><h1>{{.Page.Site.TitlePrefix}} <em>{{.Page.Site.TitleAccent}}</em></h1><p>{{.Page.Site.Subtitle}}</p></section>
      <section class="ssr-section"><h2>最新文章</h2>{{template "article-list" .Page}}</section>
    {{else if eq .Page.Kind "articles"}}
      <section class="ssr-section"><h1>文章</h1><p>{{if .Page.Query}}搜索：{{.Page.Query}}{{else if .Page.Tag}}标签：{{.Page.Tag}}{{else}}记录技术、生活与想法。{{end}}</p>{{template "article-list" .Page}}{{if or .Page.PrevURL .Page.NextURL}}<nav class="ssr-pager" aria-label="文章分页">{{if .Page.PrevURL}}<a href="{{.Page.PrevURL}}">上一页</a>{{end}}<span>第 {{.Page.Page}} 页</span>{{if .Page.NextURL}}<a href="{{.Page.NextURL}}">下一页</a>{{end}}</nav>{{end}}</section>
    {{else if eq .Page.Kind "article"}}
      {{with .Page.Article}}<article class="ssr-article"><p class="ssr-meta">{{if .PublishedAt}}{{.PublishedAt}}{{else}}{{.UpdatedAt}}{{end}}</p><h1>{{.Title}}</h1>{{if .Summary}}<p class="ssr-summary">{{.Summary}}</p>{{end}}<div class="ssr-content">{{.ContentHTML}}</div><p class="ssr-meta">标签：{{range $i, $tag := .Tags}}{{if $i}}, {{end}}{{$tag}}{{end}}</p></article>{{end}}
    {{else if eq .Page.Kind "about"}}
      <section class="ssr-section"><h1>关于</h1><p>{{.Page.About.Intro}}</p><h2>我是谁</h2><p>{{.Page.About.Bio}}</p>{{if .Page.About.Skills}}<h2>技能</h2><ul>{{range .Page.About.Skills}}<li>{{.}}</li>{{end}}</ul>{{end}}</section>
    {{else if eq .Page.Kind "friends"}}
      <section class="ssr-section"><h1>友链</h1><ul>{{range .Page.Friends}}<li><a href="{{.URL}}">{{.Name}}</a>{{if .Description}}：{{.Description}}{{end}}</li>{{end}}</ul></section>
    {{end}}
  </main>
  <footer class="ssr-footer">{{.Page.Site.FooterText}}</footer>
  </div>
  <script type="application/json" id="__IBLOG_INITIAL__">{{.Page.InitialJSON}}</script>
  {{if and .Page.JSPath (not .Page.NotFound)}}<script type="module" src="{{.Page.JSPath}}"></script>{{end}}
</body>
</html>
{{define "article-list"}}<div class="ssr-list">{{range .Articles}}<article class="ssr-card"><p class="ssr-meta">{{if .PublishedAt}}{{.PublishedAt}}{{else}}{{.UpdatedAt}}{{end}}</p><h2><a href="{{slugURL .Slug}}">{{.Title}}</a></h2>{{if .Summary}}<p>{{.Summary}}</p>{{end}}</article>{{else}}<p>暂无文章。</p>{{end}}</div>{{end}}`))

func (s *server) renderPage(w http.ResponseWriter, r *http.Request, page ssrPage, status int) {
	page.CSSPath = s.webAssetPath("*.css")
	page.JSPath = s.webAssetPath("*.js")
	page.RSSPath = page.CanonicalBase() + "/feed.xml"
	if page.InitialJSON == "" {
		if raw, err := json.Marshal(page.initialValue()); err == nil {
			page.InitialJSON = template.JS(string(raw))
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := ssrTemplate.Execute(w, ssrTemplateData{Page: page}); err != nil {
		return
	}
}

func (p ssrPage) CanonicalBase() string {
	base := p.Canonical
	if parsed, err := url.Parse(base); err == nil {
		return parsed.Scheme + "://" + parsed.Host
	}
	return base
}

func (p ssrPage) initialValue() any {
	metadata := map[string]any{"__kind": p.Kind, "__path": p.RequestPath}
	switch p.Kind {
	case "article":
		metadata["article"], metadata["settings"] = p.Article, p.settingsValue()
	case "home", "articles":
		metadata["items"], metadata["page"], metadata["pageSize"], metadata["total"], metadata["settings"] = p.Articles, p.Page, p.PageSize, p.Total, p.settingsValue()
	case "friends":
		metadata["items"], metadata["settings"] = p.Friends, p.settingsValue()
	default:
		metadata["settings"] = p.settingsValue()
	}
	return metadata
}

func (p ssrPage) settingsValue() map[string]any {
	return map[string]any{
		"site":     map[string]any{"name": p.Site.Name, "titlePrefix": p.Site.TitlePrefix, "titleAccent": p.Site.TitleAccent, "subtitle": p.Site.Subtitle, "footerName": p.Site.FooterName, "footerText": p.Site.FooterText},
		"contacts": p.Contacts,
		"about":    p.About,
	}
}

func (s *server) webAssetPath(pattern string) string {
	matches, _ := filepath.Glob(filepath.Join(s.webDist, "assets", pattern))
	if len(matches) == 0 {
		return ""
	}
	return "/assets/" + filepath.Base(matches[0])
}

func (s *server) ssrSettings(r *http.Request) (ssrSite, map[string]string, ssrAbout, error) {
	site := ssrSite{Name: "iBlog", TitlePrefix: "Chenghao's", TitleAccent: "Blog", Subtitle: "Code · Life · Ideas", FooterName: "Chenghao", FooterText: "用代码与热爱搭建"}
	contacts := map[string]string{"email": "", "github": "", "x": "", "telegram": ""}
	about := ssrAbout{Intro: "记录技术、生活与想法。", Bio: "用代码与热爱搭建。", Skills: []string{"Coding", "Photography", "Writing"}, Timeline: []ssrTimeline{{Year: "2024", Title: "持续记录", Description: "整理经验并分享有用的发现。"}, {Year: "2020", Title: "开始写作", Description: "从技术、生活与想法开始记录。"}}, ContactNote: "欢迎通过社交媒体联系我。"}
	rows, err := s.db.QueryContext(r.Context(), "SELECT setting_key,value_json FROM site_settings")
	if err != nil {
		return site, contacts, about, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return site, contacts, about, err
		}
		switch key {
		case "site":
			var value struct {
				Name, TitlePrefix, TitleAccent, Subtitle, FooterName, FooterText, Author string
			}
			if json.Unmarshal([]byte(raw), &value) == nil {
				if value.Name != "" {
					site.Name = value.Name
				}
				if value.TitlePrefix != "" {
					site.TitlePrefix = value.TitlePrefix
				}
				if value.TitleAccent != "" {
					site.TitleAccent = value.TitleAccent
				}
				if value.Subtitle != "" {
					site.Subtitle = value.Subtitle
				}
				if value.FooterName != "" {
					site.FooterName = value.FooterName
				}
				if value.FooterText != "" {
					site.FooterText = value.FooterText
				}
				if value.Author != "" {
					site.Author = value.Author
				}
			}
		case "contacts":
			var value map[string]string
			if json.Unmarshal([]byte(raw), &value) == nil {
				for key, item := range value {
					contacts[key] = item
				}
			}
		case "about":
			var value ssrAbout
			if json.Unmarshal([]byte(raw), &value) == nil {
				if value.Intro != "" {
					about.Intro = value.Intro
				}
				if value.Bio != "" {
					about.Bio = value.Bio
				}
				if len(value.Skills) > 0 {
					about.Skills = value.Skills
				}
				if len(value.Timeline) > 0 {
					about.Timeline = value.Timeline
				}
				if value.ContactNote != "" {
					about.ContactNote = value.ContactNote
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return site, contacts, about, err
	}
	if strings.TrimSpace(site.Author) == "" {
		site.Author = site.FooterName
	}
	return site, contacts, about, nil
}

func (s *server) basePage(r *http.Request, kind, title, description, path string) (ssrPage, error) {
	site, contacts, about, err := s.ssrSettings(r)
	if err != nil {
		return ssrPage{}, err
	}
	base := s.canonicalBaseURL(r)
	page := ssrPage{Kind: kind, RequestPath: r.URL.RequestURI(), Title: title, Description: description, Canonical: base + path, Robots: "index,follow", OGType: "website", SiteName: site.Name, Site: site, Contacts: contacts, About: about}
	return page, nil
}

func truncateDescription(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 160 {
		return string(runes[:157]) + "..."
	}
	return value
}

func (s *server) renderHomePage(w http.ResponseWriter, r *http.Request) {
	page, err := s.basePage(r, "home", "iBlog · Code · Life · Ideas", "记录技术、生活与想法。", "/")
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	page.Title = page.Site.Name + " · " + page.Site.Subtitle
	page.Description = truncateDescription(page.Site.Subtitle)
	page.Articles, err = s.publicArticles(r, 1, 3, "", "")
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	s.renderPage(w, r, page, http.StatusOK)
}

func (s *server) renderArticlesPage(w http.ResponseWriter, r *http.Request) {
	pageNumber := 1
	if value := r.URL.Query().Get("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1000000 {
			fail(w, http.StatusBadRequest, "bad_request", "page must be between 1 and 1000000")
			return
		}
		pageNumber = parsed
	}
	pageSize := 12
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	values := url.Values{}
	if pageNumber > 1 {
		values.Set("page", strconv.Itoa(pageNumber))
	}
	if query != "" {
		values.Set("q", query)
	}
	if tag != "" {
		values.Set("tag", tag)
	}
	path := "/articles"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	title := "文章 · iBlog"
	if query != "" {
		title = "搜索：" + query + " · iBlog"
	}
	if tag != "" {
		title = "标签：" + tag + " · iBlog"
	}
	page, err := s.basePage(r, "articles", title, "记录技术、生活与想法。", path)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if query != "" {
		page.Canonical = s.canonicalBaseURL(r) + "/articles"
		page.Title = "搜索：" + query + " · " + page.Site.Name
		page.Description = truncateDescription("搜索文章：" + query)
	} else if tag != "" {
		page.Title = "标签：" + tag + " · " + page.Site.Name
	} else if pageNumber > 1 {
		page.Title = "文章 · 第 " + strconv.Itoa(pageNumber) + " 页 · " + page.Site.Name
	} else {
		page.Title = "文章 · " + page.Site.Name
	}
	page.Query, page.Tag, page.Page, page.PageSize = query, tag, pageNumber, pageSize
	page.Articles, page.Total, err = s.publicArticlePage(r, pageNumber, pageSize, query, tag)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if query != "" {
		page.Robots = "noindex,follow"
	}
	if pageNumber > 1 {
		values.Set("page", strconv.Itoa(pageNumber-1))
		page.PrevURL = "/articles?" + values.Encode()
	}
	if pageNumber*pageSize < page.Total {
		values.Set("page", strconv.Itoa(pageNumber+1))
		page.NextURL = "/articles?" + values.Encode()
	}
	s.renderPage(w, r, page, http.StatusOK)
}

func (s *server) renderArticlePage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	a, err := s.articleBy(r, slug, true)
	if err != nil || a.Status != "published" {
		s.renderNotFound(w, r)
		return
	}
	if a.Slug != slug {
		http.Redirect(w, r, "/articles/"+url.PathEscape(a.Slug), http.StatusMovedPermanently)
		return
	}
	page, err := s.basePage(r, "article", a.Title+" · iBlog", truncateDescription(a.Summary), "/articles/"+url.PathEscape(a.Slug))
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	page.OGType, page.Article = "article", &a
	page.JSONLD = s.articleJSONLD(page)
	s.renderPage(w, r, page, http.StatusOK)
}

func (s *server) renderStaticPage(w http.ResponseWriter, r *http.Request, kind string) {
	titles := map[string][2]string{"about": {"关于 · iBlog", "了解 iBlog 的作者、经历与联系方式。"}, "friends": {"友链 · iBlog", "发现值得访问的朋友和站点。"}}
	meta := titles[kind]
	page, err := s.basePage(r, kind, meta[0], meta[1], "/"+kind)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if kind == "friends" {
		page.Friends, err = s.publicFriends(r)
	}
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	s.renderPage(w, r, page, http.StatusOK)
}

func (s *server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	page, err := s.basePage(r, "not-found", "页面不存在 · iBlog", "你访问的页面不存在或已被移除。", "")
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	page.NotFound, page.Robots = true, "noindex"
	s.renderPage(w, r, page, http.StatusNotFound)
}

func (s *server) publicArticles(r *http.Request, page, size int, query, tag string) ([]article, error) {
	items, _, err := s.publicArticlePage(r, page, size, query, tag)
	return items, err
}

func (s *server) publicArticlePage(r *http.Request, page, size int, query, tag string) ([]article, int, error) {
	where := []string{"a.status='published'"}
	args := []any{}
	from := "articles a"
	if query != "" {
		if utf8.RuneCountInString(query) >= 3 {
			from = "articles a JOIN article_search ON article_search.rowid=a.id"
			where = append(where, "article_search MATCH ?")
			args = append(args, ftsQuery(query))
		} else {
			like := "%" + escapeLike(query) + "%"
			where = append(where, "(a.title LIKE ? ESCAPE '\\' OR a.summary LIKE ? ESCAPE '\\' OR a.content_md LIKE ? ESCAPE '\\' OR EXISTS (SELECT 1 FROM article_tags aq JOIN tags tq ON tq.id=aq.tag_id WHERE aq.article_id=a.id AND tq.name LIKE ? ESCAPE '\\'))")
			args = append(args, like, like, like, like)
		}
	}
	if tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM article_tags atq JOIN tags ttq ON ttq.id=atq.tag_id WHERE atq.article_id=a.id AND ttq.name=?)")
		args = append(args, tag)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+from+" WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	orderBy := "COALESCE(a.published_at,a.updated_at) DESC,a.id DESC"
	if query != "" && utf8.RuneCountInString(query) >= 3 {
		orderBy = "bm25(article_search, 5.0, 2.0, 1.0, 3.0) ASC, " + orderBy
	}
	rows, err := s.db.QueryContext(r.Context(), "SELECT a.id,a.slug,a.title,a.summary,a.status,a.views,a.created_at,a.updated_at,a.published_at FROM "+from+" WHERE "+whereSQL+" ORDER BY "+orderBy+" LIMIT ? OFFSET ?", append(args, size, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []article{}
	for rows.Next() {
		var a article
		var pub sql.NullString
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &a.Status, &a.Views, &a.CreatedAt, &a.UpdatedAt, &pub); err != nil {
			return nil, 0, err
		}
		if pub.Valid {
			a.PublishedAt = &pub.String
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	tags, err := s.tagsForMany(r.Context(), items)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].Tags = tags[items[i].ID]
	}
	return items, total, nil
}

func (s *server) publicFriends(r *http.Request) ([]friend, error) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT id,name,url,avatar,description,sort_order,created_at,updated_at FROM friend_links ORDER BY sort_order,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	friends := []friend{}
	for rows.Next() {
		var f friend
		if err := rows.Scan(&f.ID, &f.Name, &f.URL, &f.Avatar, &f.Description, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		friends = append(friends, f)
	}
	return friends, rows.Err()
}

func (s *server) articleJSONLD(page ssrPage) template.JS {
	if page.Article == nil {
		return ""
	}
	a := page.Article
	published := a.UpdatedAt
	if a.PublishedAt != nil {
		published = *a.PublishedAt
	}
	data := map[string]any{"@context": "https://schema.org", "@type": "BlogPosting", "headline": a.Title, "description": truncateDescription(a.Summary), "datePublished": published, "dateModified": a.UpdatedAt, "author": map[string]string{"@type": "Person", "name": page.Site.Author}, "mainEntityOfPage": map[string]string{"@type": "WebPage", "@id": page.Canonical}, "inLanguage": "zh-CN"}
	breadcrumb := map[string]any{"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": []any{map[string]any{"@type": "ListItem", "position": 1, "name": "首页", "item": page.CanonicalBase() + "/"}, map[string]any{"@type": "ListItem", "position": 2, "name": "文章", "item": page.CanonicalBase() + "/articles"}, map[string]any{"@type": "ListItem", "position": 3, "name": a.Title, "item": page.Canonical}}}
	encoded, _ := json.Marshal([]any{data, breadcrumb})
	return template.JS(string(encoded))
}

func (s *server) feedXML(w http.ResponseWriter, r *http.Request) {
	base := s.canonicalBaseURL(r)
	site, _, _, err := s.ssrSettings(r)
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	articles, _, err := s.publicArticlePage(r, 1, 100, "", "")
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>%s</title><link>%s/</link><description>%s</description>`, template.HTMLEscapeString(site.Name), template.HTMLEscapeString(base), template.HTMLEscapeString(site.Subtitle))
	for _, a := range articles {
		stamp := a.UpdatedAt
		if a.PublishedAt != nil {
			stamp = *a.PublishedAt
		}
		if parsed, parseErr := time.Parse(time.RFC3339, stamp); parseErr == nil {
			stamp = parsed.UTC().Format(time.RFC1123Z)
		}
		link := base + "/articles/" + url.PathEscape(a.Slug)
		fmt.Fprintf(w, `<item><title>%s</title><link>%s</link><guid>%s</guid><pubDate>%s</pubDate><description>%s</description></item>`, template.HTMLEscapeString(a.Title), template.HTMLEscapeString(link), template.HTMLEscapeString(link), template.HTMLEscapeString(stamp), template.HTMLEscapeString(a.Summary))
	}
	fmt.Fprint(w, "</channel></rss>")
}

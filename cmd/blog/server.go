package main

import (
	"database/sql"
	"html/template"
	"sync"
	"time"
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
	viewDedupMu           sync.Mutex
	viewDedup             map[string]time.Time
	settingsCacheMu       sync.Mutex
	settingsCache         ssrSettingsCache
	publicDocCacheMu      sync.Mutex
	publicDocCache        map[string]publicDocCacheEntry
	publicDocGeneration   uint64
}
type searchCacheEntry struct {
	body      []byte
	expiresAt time.Time
}
type loginFailure struct {
	count   int
	expires time.Time
}
type publicDocCacheEntry struct {
	body      []byte
	expiresAt time.Time
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
	viewDedupWindow     = 30 * time.Minute
	viewDedupLimit      = 10000
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

package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func benchmarkServer(b *testing.B) *server {
	b.Helper()
	db, err := sql.Open("sqlite", filepath.Join(b.TempDir(), "benchmark.db"))
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	b.Cleanup(func() { _ = db.Close() })
	if err := migrate(db); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if _, err := db.Exec(`INSERT INTO articles(slug,title,summary,content_md,status,created_at,updated_at,published_at) VALUES(?,?,?,?, 'published', '2026-01-01', '2026-01-01', '2026-01-01')`,
			fmt.Sprintf("benchmark-%03d", i), fmt.Sprintf("Benchmark article %03d", i), "searchable summary", "benchmark searchable content"); err != nil {
			b.Fatal(err)
		}
	}
	return &server{db: db, webDist: "../../frontend/web/dist", adminDist: "../../frontend/admin/dist", loginFailures: make(map[string]loginFailure)}
}

func BenchmarkPublicArticleList(b *testing.B) {
	s := benchmarkServer(b)
	h := s.routes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/articles?page=1", nil))
	}
}

func BenchmarkFTSSearch(b *testing.B) {
	s := benchmarkServer(b)
	h := s.routes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/articles?q=searchable", nil))
	}
}

func BenchmarkSSRHome(b *testing.B) {
	s := benchmarkServer(b)
	h := s.routes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}
}

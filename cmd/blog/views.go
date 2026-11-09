package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *server) recordArticleView(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	a, err := s.articleBy(r, slug, true)
	if err != nil || a.Status != "published" {
		fail(w, http.StatusNotFound, "not_found", "文章不存在")
		return
	}
	if s.allowArticleView(r, a.ID) {
		s.recordView(a.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func clientIP(r *http.Request) string {
	if trustProxy() {
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	return r.RemoteAddr
}

func (s *server) allowArticleView(r *http.Request, articleID int64) bool {
	key := fmt.Sprintf("%d|%s|%s", articleID, clientIP(r), r.UserAgent())
	now := time.Now()
	s.viewDedupMu.Lock()
	defer s.viewDedupMu.Unlock()
	if s.viewDedup == nil {
		s.viewDedup = make(map[string]time.Time)
	}
	if seen, ok := s.viewDedup[key]; ok && now.Sub(seen) < viewDedupWindow {
		return false
	}
	if len(s.viewDedup) >= viewDedupLimit {
		for oldKey, seen := range s.viewDedup {
			if now.Sub(seen) >= viewDedupWindow {
				delete(s.viewDedup, oldKey)
			}
		}
	}
	if len(s.viewDedup) >= viewDedupLimit {
		var oldestKey string
		var oldest time.Time
		for candidate, seen := range s.viewDedup {
			if oldestKey == "" || seen.Before(oldest) {
				oldestKey, oldest = candidate, seen
			}
		}
		delete(s.viewDedup, oldestKey)
	}
	s.viewDedup[key] = now
	return true
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

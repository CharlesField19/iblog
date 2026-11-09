package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const publicDocCacheTTL = 5 * time.Minute

func (s *server) getPublicDocCache(key string) ([]byte, bool) {
	s.publicDocCacheMu.Lock()
	defer s.publicDocCacheMu.Unlock()
	entry, ok := s.publicDocCache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(s.publicDocCache, key)
		}
		return nil, false
	}
	return append([]byte(nil), entry.body...), true
}

func (s *server) publicDocCacheVersion() uint64 {
	s.publicDocCacheMu.Lock()
	defer s.publicDocCacheMu.Unlock()
	return s.publicDocGeneration
}

func (s *server) setPublicDocCache(key string, body []byte, generation uint64) {
	s.publicDocCacheMu.Lock()
	defer s.publicDocCacheMu.Unlock()
	if generation != s.publicDocGeneration {
		return
	}
	if s.publicDocCache == nil {
		s.publicDocCache = make(map[string]publicDocCacheEntry)
	}
	if len(s.publicDocCache) >= 16 {
		for cachedKey, entry := range s.publicDocCache {
			if time.Now().After(entry.expiresAt) || len(s.publicDocCache) >= 12 {
				delete(s.publicDocCache, cachedKey)
			}
		}
	}
	s.publicDocCache[key] = publicDocCacheEntry{body: append([]byte(nil), body...), expiresAt: time.Now().Add(publicDocCacheTTL)}
}

func (s *server) invalidatePublicDocCache() {
	s.publicDocCacheMu.Lock()
	s.publicDocGeneration++
	s.publicDocCache = make(map[string]publicDocCacheEntry)
	s.publicDocCacheMu.Unlock()
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
	s.invalidatePublicDocCache()
}

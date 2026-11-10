package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *server) publicSiteSetting(raw json.RawMessage) json.RawMessage {
	var site map[string]json.RawMessage
	if json.Unmarshal(raw, &site) != nil {
		return raw
	}
	delete(site, "favicon")
	publicSite, err := json.Marshal(site)
	if err != nil {
		return raw
	}
	return publicSite
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
		writeJSON(w, 200, map[string]any{"settings": map[string]any{"about": m["about"], "contacts": m["contacts"], "site": s.publicSiteSetting(m["site"])}})
		return
	}
	if raw, ok := m["site"]; ok {
		m["site"] = s.publicSiteSetting(raw)
	}
	writeJSON(w, 200, m)
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
	s.invalidateSSRSettingsCache()
	s.invalidatePublicDocCache()
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *server) favicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	if r.URL.Path == "/default-favicon.ico" {
		r.URL.Path = "/default-favicon.ico"
		serveSPA(w, r, s.webDist, "/default-favicon.ico")
		return
	}
	http.ServeFile(w, r, s.activeFaviconPath())
}

// normalizeSiteSetting keeps the site object extensible while validating the
// public address used to build robots.txt and sitemap.xml. An empty url clears
// the setting so the environment fallback applies again.
func normalizeSiteSetting(raw json.RawMessage) (json.RawMessage, bool) {
	var site map[string]json.RawMessage
	if err := json.Unmarshal(raw, &site); err != nil {
		return nil, false
	}
	delete(site, "favicon")
	value, present := site["url"]
	if !present {
		encoded, err := json.Marshal(site)
		return encoded, err == nil
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

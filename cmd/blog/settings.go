package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

const maxFaviconBytes = 128 << 10

func parseFaviconDataURL(value string) ([]byte, string, bool) {
	value = strings.TrimSpace(value)
	comma := strings.IndexByte(value, ',')
	if !strings.HasPrefix(value, "data:") || comma < 6 || !strings.Contains(value[:comma], ";base64") {
		return nil, "", false
	}
	mime := strings.TrimSuffix(strings.TrimPrefix(value[:comma], "data:"), ";base64")
	if mime != "image/x-icon" && mime != "image/vnd.microsoft.icon" && mime != "image/png" {
		return nil, "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(value[comma+1:])
	if err != nil || len(decoded) == 0 || len(decoded) > maxFaviconBytes {
		return nil, "", false
	}
	switch mime {
	case "image/x-icon", "image/vnd.microsoft.icon":
		if len(decoded) < 6 || !bytes.Equal(decoded[:4], []byte{0, 0, 1, 0}) || decoded[4] == 0 && decoded[5] == 0 {
			return nil, "", false
		}
	case "image/png":
		if len(decoded) < 8 || !bytes.Equal(decoded[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			return nil, "", false
		}
	}
	return decoded, mime, true
}

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
		writeJSON(w, 200, map[string]any{"settings": map[string]any{"about": m["about"], "contacts": m["contacts"], "site": m["site"]}})
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
	var raw string
	err := s.db.QueryRowContext(r.Context(), "SELECT value_json FROM site_settings WHERE setting_key='site'").Scan(&raw)
	if err == nil {
		var site map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &site) == nil {
			var value string
			if json.Unmarshal(site["favicon"], &value) == nil {
				if data, mime, ok := parseFaviconDataURL(value); ok {
					w.Header().Set("Cache-Control", "no-cache")
					w.Header().Set("Content-Type", mime)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
			}
		}
	}
	root := s.webDist
	if r.URL.Path == "/admin/favicon.ico" {
		root = s.adminDist
	}
	serveSPA(w, r, root, "/favicon.ico")
}

// normalizeSiteSetting keeps the site object extensible while validating the
// public address used to build robots.txt and sitemap.xml. An empty url clears
// the setting so the environment fallback applies again.
func normalizeSiteSetting(raw json.RawMessage) (json.RawMessage, bool) {
	var site map[string]json.RawMessage
	if err := json.Unmarshal(raw, &site); err != nil {
		return nil, false
	}
	if value, present := site["favicon"]; present {
		var favicon string
		if err := json.Unmarshal(value, &favicon); err != nil {
			return nil, false
		}
		if strings.TrimSpace(favicon) == "" {
			delete(site, "favicon")
		} else if _, _, ok := parseFaviconDataURL(favicon); !ok {
			return nil, false
		}
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

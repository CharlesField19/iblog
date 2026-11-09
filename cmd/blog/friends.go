package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

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

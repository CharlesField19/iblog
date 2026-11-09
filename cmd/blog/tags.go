package main

import (
	"net/http"
	"strconv"
	"strings"
)

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
			if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
				fail(w, 409, "conflict", "标签已存在")
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
		s.invalidateSearchCache()
		writeJSON(w, 201, map[string]any{"id": id, "name": strings.TrimSpace(in.Name), "count": 0})
		return
	}
	res, err := s.db.ExecContext(r.Context(), "UPDATE tags SET name=? WHERE id=?", strings.TrimSpace(in.Name), r.PathValue("id"))
	if err != nil {
		if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
			fail(w, 409, "conflict", "标签已存在")
		} else {
			fail(w, 500, "internal", "服务器内部错误")
		}
		return
	}
	changed, err := res.RowsAffected()
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
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
	n, err := res.RowsAffected()
	if err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	if n == 0 {
		fail(w, 404, "not_found", "标签不存在")
		return
	}
	s.invalidateSearchCache()
	w.WriteHeader(http.StatusNoContent)
}

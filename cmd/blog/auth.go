package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Password hashes use a salted PBKDF2-HMAC-SHA256 encoding for clean installs.
func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	h := pbkdf2([]byte(password), salt, 120000, 32)
	return fmt.Sprintf("pbkdf2$120000$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}
func verifyPassword(password, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 4 || p[0] != "pbkdf2" {
		return false
	}
	i, e := strconv.Atoi(p[1])
	if e != nil || i < 10000 || i > 1000000 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[2])
	if e != nil {
		return false
	}
	want, e := base64.RawStdEncoding.DecodeString(p[3])
	if e != nil {
		return false
	}
	return hmac.Equal(want, pbkdf2([]byte(password), salt, i, len(want)))
}
func pbkdf2(password, salt []byte, iter, n int) []byte {
	out := make([]byte, 0, n)
	for block := uint32(1); len(out) < n; block++ {
		m := hmac.New(sha256.New, password)
		m.Write(salt)
		m.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := m.Sum(nil)
		t := append([]byte{}, u...)
		for i := 1; i < iter; i++ {
			m = hmac.New(sha256.New, password)
			m.Write(u)
			u = m.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:n]
}
func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeLimit(r, &in, maxLoginBodyBytes) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	if strings.TrimSpace(in.Username) == "" || in.Password == "" {
		fail(w, 400, "bad_request", "用户名和密码不能为空")
		return
	}
	ip := clientIP(r)
	if s.loginBlocked(ip) {
		fail(w, 429, "rate_limited", "登录尝试过于频繁，请稍后再试")
		return
	}
	var id int64
	var username, hash string
	if s.db.QueryRowContext(r.Context(), "SELECT id,username,password_hash FROM users ORDER BY id LIMIT 1").Scan(&id, &username, &hash) != nil || username != in.Username || !verifyPassword(in.Password, hash) {
		s.recordLoginFailure(ip)
		fail(w, 401, "unauthorized", "用户名或密码错误")
		return
	}
	s.clearLoginFailures(ip)
	ttl := tokenTTLHours()
	exp := time.Now().UTC().Add(time.Duration(ttl) * time.Hour)
	var version int
	if err := s.db.QueryRowContext(r.Context(), "SELECT token_version FROM users WHERE id=?", id).Scan(&version); err != nil {
		fail(w, 500, "internal", "登录服务暂不可用")
		return
	}
	token, err := makeToken(username, version, env("BLOG_JWT_SECRET", ""), exp)
	if err != nil {
		fail(w, 500, "internal", "JWT secret 未配置")
		return
	}
	setSessionCookie(w, token, exp)
	writeJSON(w, 200, map[string]string{"expiresAt": exp.Format(time.RFC3339)})
}
func (s *server) loginBlocked(ip string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	f := s.loginFailures[ip]
	return time.Now().Before(f.expires) && f.count >= 5
}
func (s *server) recordLoginFailure(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginFailures == nil {
		s.loginFailures = make(map[string]loginFailure)
	}
	if len(s.loginFailures) >= 1024 {
		for key, failure := range s.loginFailures {
			if time.Now().After(failure.expires) {
				delete(s.loginFailures, key)
			}
		}
		if len(s.loginFailures) >= 1024 && s.loginFailures[ip].expires.IsZero() {
			oldest := ""
			for key, failure := range s.loginFailures {
				if oldest == "" || failure.expires.Before(s.loginFailures[oldest].expires) {
					oldest = key
				}
			}
			delete(s.loginFailures, oldest)
		}
	}
	f := s.loginFailures[ip]
	if time.Now().After(f.expires) {
		f.count = 0
		f.expires = time.Now().Add(10 * time.Minute)
	}
	f.count++
	s.loginFailures[ip] = f
}
func (s *server) clearLoginFailures(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	delete(s.loginFailures, ip)
}
func makeToken(username string, version int, secret string, exp time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("missing signing secret")
	}
	enc := base64.RawURLEncoding
	head, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	body, _ := json.Marshal(map[string]any{"username": username, "iss": "blog", "sub": username, "ver": version, "iat": time.Now().Unix(), "exp": exp.Unix()})
	input := enc.EncodeToString(head) + "." + enc.EncodeToString(body)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(input))
	return input + "." + enc.EncodeToString(m.Sum(nil)), nil
}
func (s *server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			if cookie, err := r.Cookie(sessionCookieName); err == nil {
				authorization = "Bearer " + cookie.Value
			}
		}
		if !strings.HasPrefix(authorization, "Bearer ") {
			fail(w, 401, "unauthorized", "missing bearer token")
			return
		}
		parts := strings.Split(strings.TrimPrefix(authorization, "Bearer "), ".")
		if len(parts) != 3 {
			fail(w, 401, "unauthorized", "missing bearer token")
			return
		}
		header, headErr := base64.RawURLEncoding.DecodeString(parts[0])
		var hdr struct {
			Alg string `json:"alg"`
			Typ string `json:"typ"`
		}
		if headErr != nil || json.Unmarshal(header, &hdr) != nil || hdr.Alg != "HS256" || hdr.Typ != "JWT" {
			fail(w, 401, "unauthorized", "invalid token")
			return
		}
		m := hmac.New(sha256.New, []byte(env("BLOG_JWT_SECRET", "")))
		m.Write([]byte(parts[0] + "." + parts[1]))
		sig, e := base64.RawURLEncoding.DecodeString(parts[2])
		if e != nil || !hmac.Equal(sig, m.Sum(nil)) {
			fail(w, 401, "unauthorized", "invalid token")
			return
		}
		b, e := base64.RawURLEncoding.DecodeString(parts[1])
		var c struct {
			Username string `json:"username"`
			Exp      int64  `json:"exp"`
			IssuedAt int64  `json:"iat"`
			Issuer   string `json:"iss"`
			Subject  string `json:"sub"`
			Version  int    `json:"ver"`
		}
		nowUnix := time.Now().Unix()
		if e != nil || json.Unmarshal(b, &c) != nil || c.Username == "" || c.Issuer != "blog" || c.Subject != c.Username || c.IssuedAt <= 0 || c.IssuedAt > nowUnix+60 || c.Exp <= nowUnix || c.Exp <= c.IssuedAt {
			fail(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		var stored string
		var version int
		if s.db.QueryRowContext(r.Context(), "SELECT username,token_version FROM users ORDER BY id LIMIT 1").Scan(&stored, &version) != nil || stored != c.Username || c.Version < 1 || version != c.Version {
			fail(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), usernameKey, c.Username)))
	})
}
func (s *server) changeAccount(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/admin/account/username":
		var in struct {
			Username string `json:"username"`
		}
		if decodeLimit(r, &in, maxLoginBodyBytes) != nil || !regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`).MatchString(in.Username) {
			fail(w, 400, "bad_request", "用户名需为3-32位字母数字-_")
			return
		}
		if _, err := s.db.ExecContext(r.Context(), "UPDATE users SET username=?,token_version=token_version+1 WHERE id=(SELECT id FROM users ORDER BY id LIMIT 1)", in.Username); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		s.loginFromAccount(w, r, in.Username, true)
	case "/api/v1/admin/account/password":
		var in struct {
			Current string `json:"currentPassword"`
			New     string `json:"newPassword"`
		}
		if decodeLimit(r, &in, maxLoginBodyBytes) != nil || len([]rune(in.New)) < 8 || len([]rune(in.New)) > 128 {
			fail(w, 400, "bad_request", "新密码至少需要 8 位")
			return
		}
		var id int64
		var u, h string
		if s.db.QueryRowContext(r.Context(), "SELECT id,username,password_hash FROM users ORDER BY id LIMIT 1").Scan(&id, &u, &h) != nil || !verifyPassword(in.Current, h) {
			fail(w, 401, "bad_credentials", "当前密码不正确")
			return
		}
		hash, err := passwordHash(in.New)
		if err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		if _, err = s.db.ExecContext(r.Context(), "UPDATE users SET password_hash=?,token_version=token_version+1 WHERE id=?", hash, id); err != nil {
			fail(w, 500, "internal", "服务器内部错误")
			return
		}
		s.loginFromAccount(w, r, u, false)
	}
}
func (s *server) loginFromAccount(w http.ResponseWriter, r *http.Request, u string, includeUsername bool) {
	ttl := tokenTTLHours()
	exp := time.Now().Add(time.Duration(ttl) * time.Hour)
	var version int
	if e := s.db.QueryRowContext(r.Context(), "SELECT token_version FROM users WHERE username=?", u).Scan(&version); e != nil {
		fail(w, 500, "internal", "登录服务暂不可用")
		return
	}
	token, e := makeToken(u, version, env("BLOG_JWT_SECRET", ""), exp)
	if e != nil {
		fail(w, 500, "internal", "JWT secret 未配置")
		return
	}
	response := map[string]string{"expiresAt": exp.UTC().Format(time.RFC3339)}
	setSessionCookie(w, token, exp)
	if includeUsername {
		response["username"] = u
	}
	writeJSON(w, 200, response)
}

func tokenTTLHours() int {
	ttl, _ := strconv.Atoi(env("BLOG_TOKEN_TTL_HOURS", "72"))
	if ttl < 1 || ttl > 720 {
		return 72
	}
	return ttl
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: env("BLOG_COOKIE_SECURE", "false") == "true", SameSite: http.SameSiteLaxMode})
}
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: env("BLOG_COOKIE_SECURE", "false") == "true", SameSite: http.SameSiteLaxMode})
}
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	_, err := s.db.ExecContext(r.Context(), "UPDATE users SET token_version=token_version+1")
	if err != nil {
		fail(w, 500, "internal", "退出登录失败")
		return
	}
	clearSessionCookie(w)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *server) setupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		fail(w, 500, "internal", "服务器内部错误")
		return
	}
	writeJSON(w, 200, map[string]bool{"setupRequired": count == 0})
}

func (s *server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeLimit(r, &in, maxLoginBodyBytes) != nil {
		fail(w, 400, "bad_request", "请求数据无效")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`).MatchString(in.Username) {
		fail(w, 400, "bad_request", "用户名需为 3-32 位字母、数字、- 或 _")
		return
	}
	if len([]rune(in.Password)) < 8 || len([]rune(in.Password)) > 128 {
		fail(w, 400, "bad_request", "密码长度需为 8-128 个字符")
		return
	}
	hash, err := passwordHash(in.Password)
	if err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	if count != 0 {
		fail(w, 409, "already_initialized", "管理员账号已初始化")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO users(username,password_hash,created_at) VALUES(?,?,?)", in.Username, hash, now()); err != nil {
		fail(w, 500, "internal", "创建管理员失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 409, "already_initialized", "管理员账号已初始化")
		return
	}
	ttl := tokenTTLHours()
	exp := time.Now().UTC().Add(time.Duration(ttl) * time.Hour)
	token, err := makeToken(in.Username, 1, env("BLOG_JWT_SECRET", ""), exp)
	if err != nil {
		fail(w, 500, "internal", "登录服务暂不可用")
		return
	}
	setSessionCookie(w, token, exp)
	writeJSON(w, 201, map[string]string{"expiresAt": exp.Format(time.RFC3339)})
}

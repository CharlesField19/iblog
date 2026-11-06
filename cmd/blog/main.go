package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "recover-admin" {
		if len(os.Args) != 3 {
			log.Fatal("usage: blog recover-admin USERNAME (reads the new password from stdin)")
		}
		if err := recoverAdmin(os.Args[2], os.Stdin); err != nil {
			log.Fatal(err)
		}
		return
	}
	dbPath := env("BLOG_DB_PATH", "./data/blog.db")
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatal(err)
	}
	if os.Getenv("BLOG_JWT_SECRET") == "" {
		secret, err := loadOrCreateSecret(filepath.Join(dataDir, "jwt_secret"))
		if err != nil {
			log.Fatal(err)
		}
		if err = os.Setenv("BLOG_JWT_SECRET", secret); err != nil {
			log.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	if err = migrate(db); err != nil {
		log.Fatal(err)
	}
	app := &server{db: db, webDist: env("BLOG_WEB_DIST", "./frontend/web/dist"), adminDist: env("BLOG_ADMIN_DIST", "./frontend/admin/dist")}
	// Bound every phase of an HTTP exchange. ReadHeaderTimeout alone does not
	// protect the server from slow request bodies or clients that never finish
	// reading a response.
	httpServer := &http.Server{
		Addr:              env("BLOG_ADDR", ":11190"),
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := app.flushViews(context.Background()); err != nil {
					log.Printf("view flush failed: %v", err)
				}
			case <-ctx.Done():
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := app.flushViews(flushCtx); err != nil {
					log.Printf("final view flush failed: %v", err)
				}
				cancel()
				return
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("blog server listening on %s", httpServer.Addr)
	err = httpServer.ListenAndServe()
	stop()
	<-flushDone
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func recoverAdmin(username string, input io.Reader) error {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`).MatchString(username) {
		return fmt.Errorf("username must be 3-32 letters, digits, _ or -")
	}
	password, err := bufio.NewReader(io.LimitReader(input, maxLoginBodyBytes)).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("read password: %w", err)
	}
	password = strings.TrimSuffix(strings.TrimSuffix(password, "\n"), "\r")
	if len([]rune(password)) < 8 || len([]rune(password)) > 128 {
		return fmt.Errorf("password must be 8-128 characters")
	}
	dbPath := env("BLOG_DB_PATH", "./data/blog.db")
	if err = os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		return err
	}
	if err = migrate(db); err != nil {
		return err
	}
	hash, err := passwordHash(password)
	if err != nil {
		return err
	}
	result, err := db.Exec("UPDATE users SET username=?,password_hash=?,token_version=token_version+1 WHERE id=(SELECT id FROM users ORDER BY id LIMIT 1)", username, hash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("no administrator account found; use the first-run setup page")
	}
	fmt.Println("Administrator credentials updated; all existing sessions have been revoked.")
	return nil
}

func loadOrCreateSecret(path string) (string, error) {
	secret, err := os.ReadFile(path)
	if err == nil {
		value := strings.TrimSpace(string(secret))
		if len(value) < 64 {
			return "", fmt.Errorf("stored JWT secret %s is invalid", path)
		}
		return value, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return "", err
	}
	value := hex.EncodeToString(bytes)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return loadOrCreateSecret(path)
		}
		return "", err
	}
	if _, err = file.WriteString(value + "\n"); err != nil {
		_ = file.Close()
		return "", err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	return value, nil
}

func serveSPA(w http.ResponseWriter, r *http.Request, root, index string) {
	path := filepath.Clean("." + string(filepath.Separator) + r.URL.Path)
	if strings.Contains(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if path == "." || path == string(filepath.Separator) {
		http.ServeFile(w, r, filepath.Join(root, index))
		return
	}
	if info, err := os.Stat(filepath.Join(root, path)); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filepath.Join(root, path))
		return
	}
	// SPA deep links fall back to index; missing asset paths remain 404.
	if filepath.Ext(path) != "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(root, index))
}

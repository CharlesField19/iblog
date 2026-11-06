CREATE TABLE IF NOT EXISTS article_slug_aliases (slug TEXT PRIMARY KEY, article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE, created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_article_slug_aliases_article ON article_slug_aliases(article_id);

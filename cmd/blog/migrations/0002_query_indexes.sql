-- Keep common admin/public list queries index-backed as the database grows.
CREATE INDEX IF NOT EXISTS idx_articles_updated ON articles(updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_article_tags_article ON article_tags(article_id);

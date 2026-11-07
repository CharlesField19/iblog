CREATE INDEX IF NOT EXISTS idx_articles_status_published ON articles(status, published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_articles_updated ON articles(updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_article_tags_tag_article ON article_tags(tag_id, article_id);
CREATE INDEX IF NOT EXISTS idx_article_tags_article_tag ON article_tags(article_id, tag_id);

CREATE VIRTUAL TABLE IF NOT EXISTS article_search USING fts5(
  title,
  summary,
  body,
  tags,
  tokenize='trigram'
);

DELETE FROM article_search;
INSERT INTO article_search(rowid, title, summary, body, tags)
SELECT a.id, a.title, a.summary, a.content_md,
       COALESCE((SELECT group_concat(t.name, ' ') FROM tags t JOIN article_tags x ON x.tag_id=t.id WHERE x.article_id=a.id), '')
FROM articles a;

CREATE TRIGGER IF NOT EXISTS articles_search_insert AFTER INSERT ON articles BEGIN
  INSERT INTO article_search(rowid, title, summary, body, tags) VALUES (new.id, new.title, new.summary, new.content_md, '');
END;

CREATE TRIGGER IF NOT EXISTS articles_search_delete AFTER DELETE ON articles BEGIN
  DELETE FROM article_search WHERE rowid=old.id;
END;

CREATE TRIGGER IF NOT EXISTS articles_search_update AFTER UPDATE OF title, summary, content_md ON articles BEGIN
  DELETE FROM article_search WHERE rowid=old.id;
  INSERT INTO article_search(rowid, title, summary, body, tags)
    SELECT new.id, new.title, new.summary, new.content_md,
      COALESCE((SELECT group_concat(t.name, ' ') FROM tags t JOIN article_tags x ON x.tag_id=t.id WHERE x.article_id=new.id), '');
END;

CREATE TRIGGER IF NOT EXISTS article_tags_search_insert AFTER INSERT ON article_tags BEGIN
  DELETE FROM article_search WHERE rowid=new.article_id;
  INSERT INTO article_search(rowid, title, summary, body, tags)
    SELECT a.id, a.title, a.summary, a.content_md,
      COALESCE((SELECT group_concat(t.name, ' ') FROM tags t JOIN article_tags x ON x.tag_id=t.id WHERE x.article_id=a.id), '')
    FROM articles a WHERE a.id=new.article_id;
END;

CREATE TRIGGER IF NOT EXISTS article_tags_search_delete AFTER DELETE ON article_tags BEGIN
  DELETE FROM article_search WHERE rowid=old.article_id;
  INSERT INTO article_search(rowid, title, summary, body, tags)
    SELECT a.id, a.title, a.summary, a.content_md,
      COALESCE((SELECT group_concat(t.name, ' ') FROM tags t JOIN article_tags x ON x.tag_id=t.id WHERE x.article_id=a.id), '')
    FROM articles a WHERE a.id=old.article_id;
END;

CREATE TRIGGER IF NOT EXISTS tags_search_update AFTER UPDATE OF name ON tags BEGIN
  DELETE FROM article_search WHERE rowid IN (SELECT a.id FROM articles a JOIN article_tags x ON x.article_id=a.id WHERE x.tag_id=new.id);
  INSERT INTO article_search(rowid, title, summary, body, tags)
    SELECT a.id, a.title, a.summary, a.content_md,
      COALESCE((SELECT group_concat(t.name, ' ') FROM tags t JOIN article_tags x ON x.tag_id=t.id WHERE x.article_id=a.id), '')
    FROM articles a JOIN article_tags x ON x.article_id=a.id WHERE x.tag_id=new.id;
END;

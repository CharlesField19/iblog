# SEO and SSR verification

This checklist is for iBlog v1.1.2. It verifies the Go-rendered public document, not only the Vue application after JavaScript starts.

## Local or staging checks

Set the public origin before testing canonical URLs:

```sh
export BLOG_PUBLIC_URL=https://example.com
curl -fsS -D /tmp/home.headers https://example.com/ -o /tmp/home.html
curl -fsS -D /tmp/feed.headers https://example.com/feed.xml -o /tmp/feed.xml
```

For a published article, confirm the response is `200`, the body contains the title, summary and rendered content, exactly one canonical URL points to the current slug, JSON-LD contains `BlogPosting` and `BreadcrumbList`, `__IBLOG_INITIAL__` is present, and RSS contains the absolute article URL.

For a draft, deleted article, old alias and unknown path, confirm:

```sh
curl -sS -o /dev/null -w '%{http_code}\n' https://example.com/articles/draft-slug   # 404
curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' https://example.com/articles/old-slug # 301
curl -sS -o /dev/null -w '%{http_code}\n' https://example.com/not-a-route             # 404
```

Drafts and unknown paths must not appear in the HTML, sitemap or RSS. Search pages (`/articles?q=...`) use `noindex,follow`; paginated unfiltered lists retain a canonical URL for their page.

## Cloudflare

1. Keep the origin's `robots.txt` and `sitemap.xml` reachable through the public hostname.
2. In **Rules → Transform Rules → Managed Transforms**, inspect managed robots/AI crawler content. A Cloudflare-injected `User-agent: Baiduspider` with `Disallow: /` prevents Baidu crawling even when the origin allows it.
3. If Baidu indexing is desired, disable or change that managed block, then purge the cached `/robots.txt` response.
4. Keep `/admin` and `/api/` disallowed, but do not challenge or cache HTML pages in a way that changes their status codes.
5. Verify the final edge response, not only the origin response.

```sh
curl -fsS https://example.com/robots.txt
curl -fsS https://example.com/sitemap.xml
curl -sS -D - https://example.com/articles/known-slug -o /tmp/article.html
```

Image cover, upload, processing and WebP/AVIF features remain deferred from this release.

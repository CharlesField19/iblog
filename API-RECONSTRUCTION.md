# Recovered API inventory (inferred from the executable and frontend bundles)

This is an inventory, not a complete contract. Request/response details and validation rules must be verified against the running service or recovered from disassembly.

## Public

| Method | Path | Evidence |
|---|---|---|
| GET | `/api/v1/health` | Docker healthcheck and binary strings |
| GET | `/api/v1/articles` | Binary strings and frontend bundle |
| GET | `/api/v1/articles/{slug}` | Binary strings |
| GET | `/api/v1/tags` | Binary strings and frontend bundle |
| GET | `/api/v1/friends` | Binary strings and frontend bundle |
| GET | `/api/v1/settings` | Binary strings and frontend bundle |
| POST | `/api/v1/auth/login` | Binary strings and frontend bundle |

## Administrator (authenticated)

| Methods | Path |
|---|---|
| GET, POST | `/api/v1/admin/articles` |
| GET, PUT, DELETE | `/api/v1/admin/articles/{id}` |
| GET, POST | `/api/v1/admin/tags` |
| PUT, DELETE | `/api/v1/admin/tags/{id}` |
| GET, POST | `/api/v1/admin/friends` |
| PUT, DELETE | `/api/v1/admin/friends/{id}` |
| GET | `/api/v1/admin/settings` |
| PUT | `/api/v1/admin/settings/{key}` |
| PUT | `/api/v1/admin/account/username` |
| PUT | `/api/v1/admin/account/password` |

Other recovered clues: JWT bearer authentication; bcrypt password hashing; configurable token TTL; CORS configuration; article search supports matching title, summary, and Markdown content; article status is `draft` or `published`; view counts are incremented by article retrieval. Exact authorization and input semantics are not yet verified.

## Configuration names observed in the image/binary

`BLOG_ADDR`, `BLOG_DB_PATH`, `BLOG_WEB_DIST`, `BLOG_ADMIN_DIST`, `BLOG_JWT_SECRET`, `BLOG_TOKEN_TTL_HOURS`, `BLOG_ADMIN_USERNAME`, `BLOG_ADMIN_PASSWORD`, `BLOG_CORS_ORIGINS`.

Never place production credentials or JWT secrets in this reconstruction workspace.

FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run typecheck && npm run test:i18n && npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY --from=frontend /src/frontend/web/dist/ ./frontend/web/dist/
COPY --from=frontend /src/frontend/admin/dist/ ./frontend/admin/dist/
RUN CGO_ENABLED=0 go test -count=1 ./... \
    && CGO_ENABLED=0 go vet ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/blog ./cmd/blog

FROM alpine:3.20
LABEL org.opencontainers.image.version="1.1.2"
RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -H -u 10001 blog \
    && mkdir -p /data /app/static/web /app/static/admin \
    && chown -R blog:nogroup /data /app
WORKDIR /app
COPY --from=backend --chown=blog:nogroup /out/blog /usr/local/bin/blog
COPY --from=frontend --chown=blog:nogroup /src/frontend/web/dist/ /app/static/web/
COPY --from=frontend --chown=blog:nogroup /src/frontend/admin/dist/ /app/static/admin/
ENV BLOG_ADDR=:11190 BLOG_DB_PATH=/data/blog.db BLOG_WEB_DIST=/app/static/web BLOG_ADMIN_DIST=/app/static/admin
USER blog
EXPOSE 11190
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD wget -qO- http://127.0.0.1:11190/api/v1/health || exit 1
ENTRYPOINT ["blog"]

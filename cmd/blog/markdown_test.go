package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestRenderMarkdownSupportsCommonSyntax(t *testing.T) {
	html, err := renderMarkdown("# Title\n\n**bold** and [link](https://example.com)\n\n- one\n- two\n\n```go\nfmt.Println(1)\n```")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<h1>Title</h1>", "<strong>bold</strong>", `href="https://example.com"`, "<ul>", "<code>"} {
		if !strings.Contains(string(html), want) {
			t.Fatalf("rendered Markdown missing %q: %s", want, html)
		}
	}
}

func TestRenderMarkdownHighlightsAndWrapsCodeSafely(t *testing.T) {
	html, err := renderMarkdown("```go\nfunc main() { fmt.Println(\\\"a very long line\\\") }\n```")
	if err != nil {
		t.Fatal(err)
	}
	output := string(html)
	if !strings.Contains(output, `class="`) {
		t.Fatalf("code block was not highlighted: %s", output)
	}
	if strings.Contains(strings.ToLower(output), "javascript:") || strings.Contains(output, "<script") {
		t.Fatalf("unsafe content survived: %s", output)
	}
}

func TestMarkdownGFMAndDangerousLinkSyntax(t *testing.T) {
	html, err := renderMarkdown("| A | B |\n|---|---|\n| one | two |\n\n~~removed~~\n\n[bad](javascript:alert%281%29)\n\n![bad](data:text/html;base64,PHNjcmlwdD4=)")
	if err != nil {
		t.Fatal(err)
	}
	output := string(html)
	for _, want := range []string{"<table>", "<th>A</th>", "<td>one</td>", "<del>removed</del>"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q: %s", want, output)
		}
	}
	for _, forbidden := range []string{"javascript:", "data:text", "<script"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("unsafe syntax survived: %s", output)
		}
	}
}

func TestMarkdownPreviewMatchesPublishedHTMLAndRequiresAuth(t *testing.T) {
	s := testServer(t)
	h := s.routes()
	source := "## Shared renderer\n\n**bold**\n\n[unsafe](javascript:alert%281%29)"
	previewPath := "/api/v1/admin/markdown/preview"
	if code, _ := call(t, h, http.MethodPost, previewPath, map[string]any{"contentMd": source}, ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated preview status=%d", code)
	}
	token := adminToken(t, s)
	code, preview := call(t, h, http.MethodPost, previewPath, map[string]any{"contentMd": source}, token)
	if code != http.StatusOK {
		t.Fatalf("preview: %d %v", code, preview)
	}
	if code, _ = call(t, h, http.MethodPost, previewPath, map[string]any{"contentMd": 42}, token); code != http.StatusBadRequest {
		t.Fatalf("invalid preview status=%d", code)
	}
	code, created := call(t, h, http.MethodPost, "/api/v1/admin/articles", map[string]any{"title": "Shared", "slug": "shared-renderer", "status": "published", "contentMd": source}, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, created)
	}
	code, public := call(t, h, http.MethodGet, "/api/v1/articles/shared-renderer", nil, "")
	if code != http.StatusOK || public["contentHtml"] != preview["html"] {
		t.Fatalf("public and preview differ: %d %v versus %v", code, public, preview)
	}
	if created["contentMd"] != source {
		t.Fatal("admin editor lost the Markdown source")
	}
}

func TestRenderMarkdownRemovesUnsafeHTMLAndURLs(t *testing.T) {
	html, err := renderMarkdown(`<script>alert(1)</script><img src="javascript:alert(1)" onerror="alert(2)"><a href="javascript:alert(3)">bad</a><b>raw</b>`)
	if err != nil {
		t.Fatal(err)
	}
	output := string(html)
	for _, forbidden := range []string{"script", "javascript:", "onerror", "<b>"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("unsafe Markdown survived (%q): %s", forbidden, output)
		}
	}
}

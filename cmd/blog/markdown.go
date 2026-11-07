package main

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

var markdownParser = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

var markdownPolicy = func() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "hr", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "code", "ul", "ol", "li", "dl", "dt", "dd", "table", "thead", "tbody", "tr", "th", "td", "em", "strong", "del", "s", "a", "img")
	policy.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	policy.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	policy.AllowURLSchemes("http", "https", "mailto")
	policy.RequireNoFollowOnLinks(true)
	return policy
}()

// renderMarkdown is the only Markdown-to-HTML boundary in the application.
// Raw HTML is omitted by goldmark's safe default, then the result is reduced
// again by an explicit allowlist before it is returned to a browser.
func renderMarkdown(source string) (template.HTML, error) {
	var rendered bytes.Buffer
	if err := markdownParser.Convert([]byte(source), &rendered); err != nil {
		return "", err
	}
	return template.HTML(markdownPolicy.Sanitize(rendered.String())), nil
}

func normalizedMarkdown(source string) string {
	return strings.TrimSpace(source)
}

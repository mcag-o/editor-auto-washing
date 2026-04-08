package formatter

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"content-hub/domain"
)

type WechatHtmlFormatter struct {
	templateRoot string
	templates    map[string]*template.Template
}

func NewWechatHtmlFormatter(templateRoot string) *WechatHtmlFormatter {
	f := &WechatHtmlFormatter{
		templateRoot: templateRoot,
		templates:    make(map[string]*template.Template),
	}
	return f
}

func (f *WechatHtmlFormatter) Format(article *domain.ArticleDraft, templateName string) (string, error) {
	tmpl, err := f.getTemplate(templateName)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", templateName, err)
	}

	data := buildTemplateData(article)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", templateName, err)
	}
	return buf.String(), nil
}

func (f *WechatHtmlFormatter) getTemplate(name string) (*template.Template, error) {
	if tmpl, ok := f.templates[name]; ok {
		return tmpl, nil
	}

	tmpl, err := template.New(name).Parse(buildDefaultTemplate(name))
	if err != nil {
		return nil, fmt.Errorf("parse default template %s: %w", name, err)
	}
	f.templates[name] = tmpl
	return tmpl, nil
}

func buildTemplateData(article *domain.ArticleDraft) map[string]any {
	return map[string]any{
		"title":           getMapStr(article.Meta, "title", ""),
		"digest":          getMapStr(article.Meta, "digest", ""),
		"author":          getMapStr(article.Meta, "author", ""),
		"date_cn":         time.Now().Format("2006年01月02日"),
		"date_short":      time.Now().Format("2006.01.02"),
		"source_count":    len(article.TargetPlatforms),
		"news_count":      len(article.Sections),
		"headline_title":  getMapStr(article.Headline, "title", ""),
		"headline_body":   getMapStr(article.Headline, "body", ""),
		"headline_source": getMapStr(article.Headline, "source", ""),
		"conclusion":      article.Conclusion,
		"cta":             article.CTA,
		"sections":        article.Sections,
	}
}

func getMapStr(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func buildDefaultTemplate(name string) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>{{.title}}</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;line-height:1.8;color:#333;max-width:677px;margin:0 auto;padding:20px;}
h1{font-size:24px;font-weight:700;margin:20px 0;}
.meta{color:#888;font-size:14px;margin-bottom:20px;}
.section{margin:16px 0;padding:12px;background:#f9f9f9;border-radius:8px;}
.conclusion{margin-top:24px;padding:16px;background:#e8f5e9;border-left:4px solid #4caf50;}
.cta{margin-top:16px;padding:12px;background:#fff3e0;text-align:center;border-radius:8px;}
</style>
</head>
<body>
<h1>{{.title}}</h1>
<div class="meta">
<span>{{.date_cn}}</span>
{{if .author}}<span>作者: {{.author}}</span>{{end}}
{{if .digest}}<p>{{.digest}}</p>{{end}}
</div>

{{if .headline_title}}
<section class="section">
<h2>{{.headline_title}}</h2>
{{if .headline_body}}<p>{{.headline_body}}</p>{{end}}
{{if .headline_source}}<small>来源: {{.headline_source}}</small>{{end}}
</section>
{{end}}

{{range $i, $sec := .sections}}
<section class="section">
{{printf "%v" $sec}}
</section>
{{end}}

{{if .conclusion}}
<div class="conclusion">
<p>{{.conclusion}}</p>
</div>
{{end}}

{{if .cta}}
<div class="cta">
<p>{{.cta}}</p>
</div>
{{end}}
</body>
</html>`
}

func (f *WechatHtmlFormatter) Validate(article *domain.ArticleDraft) []string {
	var warnings []string

	if article == nil {
		return append(warnings, "article is nil")
	}

	if article.Meta == nil {
		warnings = append(warnings, "meta is nil")
	}

	if getMapStr(article.Meta, "title", "") == "" {
		warnings = append(warnings, "title is empty")
	}

	if getMapStr(article.Meta, "digest", "") == "" {
		warnings = append(warnings, "digest is empty")
	}

	if len(article.Sections) == 0 {
		warnings = append(warnings, "no sections")
	}

	return warnings
}

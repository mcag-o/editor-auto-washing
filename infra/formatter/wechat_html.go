package formatter

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"content-hub/domain"
)

var wechatImageHostPatterns = []string{"mmbiz.qpic.cn", "mmbiz.qlogo.cn", "wx.qlogo.cn"}

type WechatHtmlFormatter struct {
	catalog *TemplateCatalog
}

func NewWechatHtmlFormatter(templateRoots []string) *WechatHtmlFormatter {
	return &WechatHtmlFormatter{catalog: NewTemplateCatalog(templateRoots)}
}

func (f *WechatHtmlFormatter) Render(draft *domain.ArticleDraft, templateName string) (string, error) {
	if draft == nil {
		return "", domain.NewValidationErr("draft is required", nil)
	}
	validation := f.ValidateDraft(draft, templateName)
	if len(validation.Errors) > 0 {
		return "", domain.NewValidationErr(strings.Join(validation.Errors, "; "), nil)
	}
	resolvedTemplate := strings.TrimSpace(templateName)
	if resolvedTemplate == "" {
		resolvedTemplate = strings.TrimSpace(draft.Template)
	}
	templateText, err := f.catalog.ReadTemplate(resolvedTemplate)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", resolvedTemplate, err)
	}
	rendered := f.applyReplacements(templateText, f.replacements(draft))
	outputErrors := f.ValidateRenderedOutput(rendered)
	if len(outputErrors) > 0 {
		return "", domain.NewValidationErr(strings.Join(outputErrors, "; "), nil)
	}
	return rendered, nil
}

func (f *WechatHtmlFormatter) ValidateDraft(draft *domain.ArticleDraft, templateName string) domain.DraftValidationResult {
	result := domain.DraftValidationResult{}
	if draft == nil {
		result.Errors = append(result.Errors, "draft is required")
		return result
	}
	resolvedTemplate := strings.TrimSpace(templateName)
	if resolvedTemplate == "" {
		resolvedTemplate = strings.TrimSpace(draft.Template)
	}
	if resolvedTemplate == "" {
		result.Errors = append(result.Errors, "template is required")
	} else if templates, err := f.catalog.ListTemplates(); err == nil && len(templates) > 0 {
		found := false
		for _, candidate := range templates {
			if candidate == resolvedTemplate {
				found = true
				break
			}
		}
		if !found {
			result.Errors = append(result.Errors, fmt.Sprintf("template must be one of: %s", strings.Join(templates, ", ")))
		}
	}
	if domain.DraftString(draft.Meta["title"]) == "" {
		result.Errors = append(result.Errors, "meta.title is required")
	} else if len([]rune(domain.DraftString(draft.Meta["title"]))) > 32 {
		result.Errors = append(result.Errors, fmt.Sprintf("meta.title must be <= 32 characters, got %d", len([]rune(domain.DraftString(draft.Meta["title"])))))
	}
	if domain.DraftString(draft.Meta["digest"]) == "" {
		result.Errors = append(result.Errors, "meta.digest is required")
	} else if len([]rune(domain.DraftString(draft.Meta["digest"]))) > 128 {
		result.Errors = append(result.Errors, fmt.Sprintf("meta.digest must be <= 128 characters, got %d", len([]rune(domain.DraftString(draft.Meta["digest"])))))
	}
	if domain.DraftString(draft.Meta["author"]) == "" {
		result.Errors = append(result.Errors, "meta.author is required")
	}
	if domain.DraftString(draft.Meta["thumb_media_id"]) == "" && domain.DraftString(draft.Meta["cover_media_id"]) == "" {
		result.Warnings = append(result.Warnings, "meta.thumb_media_id is missing")
	}
	if domain.DraftString(draft.Headline["title"]) == "" {
		result.Errors = append(result.Errors, "headline.title is required")
	}
	if len(domain.DraftParagraphs(draft.Headline["body"])) == 0 {
		result.Errors = append(result.Errors, "headline.body is required")
	}
	if len(draft.Sections) == 0 {
		result.Errors = append(result.Errors, "sections must be a non-empty array")
	} else {
		for sectionIndex, rawSection := range draft.Sections {
			section, ok := rawSection.(map[string]any)
			if !ok {
				result.Errors = append(result.Errors, fmt.Sprintf("sections[%d] must be an object", sectionIndex))
				continue
			}
			if domain.DraftString(section["cn"]) == "" && domain.DraftString(section["title"]) == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("sections[%d] is missing cn/title", sectionIndex))
			}
			blocks, ok := section["blocks"].([]map[string]any)
			if !ok {
				rawBlocks, okAny := section["blocks"].([]any)
				if okAny {
					blocks = make([]map[string]any, 0, len(rawBlocks))
					for _, block := range rawBlocks {
						if mapped, okMap := block.(map[string]any); okMap {
							blocks = append(blocks, mapped)
						}
					}
				}
			}
			for blockIndex, block := range blocks {
				blockType := domain.DraftString(block["type"])
				if blockType == "" {
					blockType = "card"
				}
				switch blockType {
				case "card", "opinion":
					if domain.DraftString(block["title"]) == "" {
						result.Errors = append(result.Errors, fmt.Sprintf("sections[%d].blocks[%d] is missing title", sectionIndex, blockIndex))
					}
				case "week-ahead":
					if len(asAnySlice(block["days"])) == 0 {
						result.Errors = append(result.Errors, fmt.Sprintf("sections[%d].blocks[%d] needs days", sectionIndex, blockIndex))
					}
				case "image":
					if urlValue := domain.DraftString(block["url"]); urlValue != "" && !isWechatImageURL(urlValue) {
						result.Warnings = append(result.Warnings, fmt.Sprintf("sections[%d].blocks[%d] image URL does not look like WeChat CDN", sectionIndex, blockIndex))
					}
				}
			}
		}
	}
	return result
}

func (f *WechatHtmlFormatter) ValidateRenderedOutput(htmlText string) []string {
	issues := []string{}
	stripped := stripHTMLComments(htmlText)
	if strings.Contains(stripped, "{{") || strings.Contains(stripped, "}}") {
		issues = append(issues, "rendered HTML still contains unresolved placeholders")
	}
	trimmed := strings.TrimSpace(htmlText)
	if trimmed == "" {
		issues = append(issues, "rendered HTML is empty")
	}
	if !strings.Contains(strings.ToLower(trimmed), "<html") {
		issues = append(issues, "rendered HTML must contain <html")
	}
	if !strings.Contains(strings.ToLower(trimmed), "<body") {
		issues = append(issues, "rendered HTML must contain <body")
	}
	if len([]byte(htmlText)) > 1024*1024 {
		issues = append(issues, fmt.Sprintf("rendered HTML exceeds 1MB limit: %d bytes", len([]byte(htmlText))))
	}
	return issues
}

func (f *WechatHtmlFormatter) applyReplacements(templateText string, replacements map[string]string) string {
	rendered := templateText
	for key, value := range replacements {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}
	return rendered
}

func (f *WechatHtmlFormatter) replacements(draft *domain.ArticleDraft) map[string]string {
	return map[string]string{
		"DATE_CN":         html.EscapeString(domain.DraftString(draft.Meta["date_cn"])),
		"DATE_SHORT":      html.EscapeString(domain.DraftString(draft.Meta["date_short"])),
		"SOURCE_COUNT":    html.EscapeString(fmt.Sprint(len(draft.SourceRefs))),
		"NEWS_COUNT":      html.EscapeString(fmt.Sprint(len(draft.Sections))),
		"TITLE":           html.EscapeString(domain.DraftString(draft.Meta["title"])),
		"SUBTITLE":        html.EscapeString(domain.DraftString(draft.Meta["subtitle"])),
		"AUTHOR":          html.EscapeString(domain.DraftString(draft.Meta["author"])),
		"DIGEST":          renderParagraphs(domain.DraftParagraphs(draft.Meta["digest"])),
		"HEADLINE_TITLE":  html.EscapeString(domain.DraftString(draft.Headline["title"])),
		"HEADLINE_BODY":   renderParagraphs(domain.DraftParagraphs(draft.Headline["body"])),
		"HEADLINE_SOURCE": html.EscapeString(domain.DraftString(draft.Headline["source"])),
		"HEADLINE_IMAGE":  renderImageBlock(draft.Headline["image"]),
		"BODY_SECTIONS":   renderSections(draft.Sections),
		"CONCLUSION":      renderParagraphs(domain.DraftParagraphs(draft.Conclusion)),
		"CTA":             renderParagraphs(defaultCTA(draft.CTA)),
	}
}

func defaultCTA(value string) []string {
	paragraphs := domain.DraftParagraphs(value)
	if len(paragraphs) > 0 {
		return paragraphs
	}
	return []string{"你最关注哪一点？欢迎留言讨论。"}
}

func renderSections(sections []any) string {
	parts := make([]string, 0, len(sections))
	for _, rawSection := range sections {
		section, ok := rawSection.(map[string]any)
		if !ok {
			continue
		}
		sectionTitle := html.EscapeString(firstNonEmpty(section["cn"], section["title"], "分区"))
		sectionEnglish := html.EscapeString(firstNonEmpty(section["en"], section["title_en"], "SECTION"))
		parts = append(parts, `<section class="section"><p class="section-en">`+sectionEnglish+`</p><h2>`+sectionTitle+`</h2>`)
		parts = append(parts, renderImageBlock(section["image"]))
		for _, rawBlock := range asAnySlice(section["blocks"]) {
			block, ok := rawBlock.(map[string]any)
			if !ok {
				continue
			}
			parts = append(parts, renderBlock(block))
		}
		parts = append(parts, `</section>`)
	}
	return strings.Join(parts, "")
}

func renderBlock(block map[string]any) string {
	blockType := firstNonEmpty(block["type"], "card")
	switch blockType {
	case "paragraph":
		return `<section class="block paragraph">` + renderParagraphs(domain.DraftParagraphs(block["text"])) + `</section>`
	case "quote":
		return `<blockquote class="block quote"><p>` + html.EscapeString(domain.DraftString(block["text"])) + `</p><footer>` + html.EscapeString(domain.DraftString(block["attribution"])) + `</footer></blockquote>`
	case "takeaways":
		items := make([]string, 0, len(asAnySlice(block["items"])))
		for _, item := range asAnySlice(block["items"]) {
			items = append(items, `<li>`+html.EscapeString(domain.DraftString(item))+`</li>`)
		}
		return `<section class="block takeaways"><h3>` + html.EscapeString(firstNonEmpty(block["title"], "核心结论")) + `</h3><ul>` + strings.Join(items, "") + `</ul></section>`
	case "week-ahead":
		rows := make([]string, 0, len(asAnySlice(block["days"])))
		for _, item := range asAnySlice(block["days"]) {
			day, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, `<li><strong>`+html.EscapeString(domain.DraftString(day["label"]))+`</strong> `+html.EscapeString(domain.DraftString(day["events"]))+`</li>`)
		}
		return `<section class="block week-ahead"><h3>` + html.EscapeString(firstNonEmpty(block["title"], "下周前瞻")) + `</h3><ul>` + strings.Join(rows, "") + `</ul></section>`
	case "image":
		return renderImageBlock(block)
	default:
		return `<section class="block card"><h3>` + html.EscapeString(domain.DraftString(block["title"])) + `</h3>` + renderParagraphs(domain.DraftParagraphs(block["body"])) + `<p class="source">` + html.EscapeString(domain.DraftString(block["source"])) + `</p></section>`
	}
}

func renderImageBlock(value any) string {
	image, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	imageURL := html.EscapeString(domain.DraftString(image["url"]))
	if imageURL == "" {
		return ""
	}
	caption := html.EscapeString(firstNonEmpty(image["caption"], "配图"))
	return `<figure class="image"><img src="` + imageURL + `" alt="` + caption + `" /><figcaption>` + caption + `</figcaption></figure>`
}

func renderParagraphs(paragraphs []string) string {
	parts := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		parts = append(parts, `<p>`+html.EscapeString(paragraph)+`</p>`)
	}
	return strings.Join(parts, "")
}

func stripHTMLComments(value string) string {
	rendered := value
	for strings.Contains(rendered, "<!--") && strings.Contains(rendered, "-->") {
		start := strings.Index(rendered, "<!--")
		end := strings.Index(rendered[start:], "-->")
		if end < 0 {
			break
		}
		end += start
		rendered = rendered[:start] + rendered[end+3:]
	}
	return rendered
}

func isWechatImageURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	for _, pattern := range wechatImageHostPatterns {
		if strings.Contains(host, pattern) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...any) string {
	for _, value := range values {
		if trimmed := domain.DraftString(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func asAnySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

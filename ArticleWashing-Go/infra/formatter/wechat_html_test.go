package formatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"content-hub/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWechatHtmlFormatterRendersTemplateFromConfiguredRoots(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))

	f := NewWechatHtmlFormatter([]string{templateDir})

	html, err := f.Render(mustBuildValidDraft(), "daily-intelligence")
	require.NoError(t, err)
	assert.Contains(t, html, "<h1>市场快讯</h1>")
	assert.Contains(t, html, "继续关注后续变化")
	assert.NotContains(t, html, "{{TITLE}}")
}

func TestWechatHtmlFormatterValidateDraftReportsMissingRequiredFields(t *testing.T) {
	f := NewWechatHtmlFormatter(nil)

	result := f.ValidateDraft(&domain.ArticleDraft{Template: "daily-intelligence"}, "daily-intelligence")

	assert.Contains(t, result.Errors, "meta.title is required")
	assert.Contains(t, result.Errors, "meta.digest is required")
	assert.Contains(t, result.Errors, "meta.author is required")
	assert.Contains(t, result.Errors, "headline.title is required")
	assert.Contains(t, result.Errors, "headline.body is required")
	assert.Contains(t, result.Errors, "sections must be a non-empty array")
}

func TestWechatHtmlFormatterValidateRenderedOutputDetectsUnresolvedPlaceholders(t *testing.T) {
	f := NewWechatHtmlFormatter(nil)

	errs := f.ValidateRenderedOutput("<html><body>{{TITLE}}</body></html>")

	assert.Contains(t, errs, "rendered HTML still contains unresolved placeholders")
}

func TestWechatHtmlFormatterRenderFailsForMissingTemplate(t *testing.T) {
	f := NewWechatHtmlFormatter([]string{t.TempDir()})

	_, err := f.Render(mustBuildValidDraft(), "missing-template")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load template missing-template")
}

func TestWechatHtmlFormatterRenderFailsWhenTemplateLeavesPlaceholderOutput(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "broken.html"), []byte(`<html><body>{{TITLE}}{{UNUSED_PLACEHOLDER}}</body></html>`), 0o644))

	f := NewWechatHtmlFormatter([]string{templateDir})

	_, err := f.Render(mustBuildValidDraft(), "broken")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rendered HTML still contains unresolved placeholders")
}

func TestWechatHtmlFormatterRendersStructuredSections(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body>{{BODY_SECTIONS}}</body></html>`), 0o644))

	f := NewWechatHtmlFormatter([]string{templateDir})

	html, err := f.Render(mustBuildValidDraft(), "daily-intelligence")
	require.NoError(t, err)
	assert.True(t, strings.Contains(html, "政策动向") || strings.Contains(html, "市场焦点"))
	assert.Contains(t, html, "板块延续轮动")
}

func mustBuildValidDraft() *domain.ArticleDraft {
	draft := domain.NewArticleDraft("daily-intelligence")
	draft.Meta["title"] = "市场快讯"
	draft.Meta["digest"] = "盘前关注政策与板块轮动。"
	draft.Meta["author"] = "研究编辑部"
	draft.Headline["title"] = "政策窗口期开启"
	draft.Headline["body"] = []string{"市场仍在等待增量信号。", "资金围绕高景气赛道轮动。"}
	draft.Headline["source"] = "综合公开信息"
	draft.Sections = []any{
		map[string]any{
			"en": "POLICY",
			"cn": "政策动向",
			"blocks": []map[string]any{{
				"type":   "card",
				"number": "1",
				"title":  "板块延续轮动",
				"body":   []string{"关注高股息与设备更新主线。"},
				"source": "券商晨会",
			}},
		},
	}
	draft.Conclusion = "关注量能变化与政策节奏。"
	draft.CTA = "继续关注后续变化"
	return draft
}

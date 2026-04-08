package formatter

import (
	"strings"
	"testing"

	"content-hub/domain"
)

func TestNewWechatHtmlFormatter(t *testing.T) {
	f := NewWechatHtmlFormatter("./templates")

	if f == nil {
		t.Fatal("NewWechatHtmlFormatter returned nil")
	}
	if f.templateRoot != "./templates" {
		t.Errorf("expected templateRoot %q, got %q", "./templates", f.templateRoot)
	}
}

func TestFormatWithValidArticle(t *testing.T) {
	f := NewWechatHtmlFormatter("./templates")

	article := &domain.ArticleDraft{
		Meta: map[string]any{
			"title":  "Test Article",
			"digest": "A test digest",
			"author": "Test Author",
		},
		Headline: map[string]any{
			"title":  "Headline Title",
			"body":   "Headline Body",
			"source": "Test Source",
		},
		Sections:   []any{"Section 1", "Section 2"},
		Conclusion: "Test Conclusion",
		CTA:        "Test CTA",
	}

	html, err := f.Format(article, "wechat")
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	if html == "" {
		t.Fatal("Format returned empty string")
	}

	if !strings.Contains(html, "Test Article") {
		t.Error("output should contain title")
	}
	if !strings.Contains(html, "Headline Title") {
		t.Error("output should contain headline title")
	}
	if !strings.Contains(html, "Test Conclusion") {
		t.Error("output should contain conclusion")
	}
	if !strings.Contains(html, "Test CTA") {
		t.Error("output should contain CTA")
	}
}

func TestValidateMissingFields(t *testing.T) {
	f := NewWechatHtmlFormatter("./templates")

	article := &domain.ArticleDraft{
		Meta:     map[string]any{},
		Sections: []any{},
	}

	warnings := f.Validate(article)

	if len(warnings) == 0 {
		t.Fatal("expected warnings for incomplete article")
	}

	expectedWarnings := []string{"title is empty", "digest is empty", "no sections"}
	for _, expected := range expectedWarnings {
		found := false
		for _, w := range warnings {
			if w == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning %q not found in %v", expected, warnings)
		}
	}
}

func TestValidateNilArticle(t *testing.T) {
	f := NewWechatHtmlFormatter("./templates")

	warnings := f.Validate(nil)

	if len(warnings) == 0 {
		t.Fatal("expected warnings for nil article")
	}

	if warnings[0] != "article is nil" {
		t.Errorf("expected first warning %q, got %q", "article is nil", warnings[0])
	}
}

func TestValidateValidArticle(t *testing.T) {
	f := NewWechatHtmlFormatter("./templates")

	article := &domain.ArticleDraft{
		Meta: map[string]any{
			"title":  "Complete Article",
			"digest": "A complete digest",
		},
		Headline: map[string]any{
			"title": "Headline",
		},
		Sections:   []any{"Section 1"},
		Conclusion: "Conclusion text",
		CTA:        "Call to action",
	}

	warnings := f.Validate(article)

	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for valid article, got %d: %v", len(warnings), warnings)
	}
}

func TestFormatArticleWithAllFields(t *testing.T) {
	f := NewWechatHtmlFormatter("./templates")

	article := domain.NewArticleDraft("default")
	article.Meta["title"] = "Full Article"
	article.Meta["digest"] = "Full digest"
	article.Meta["author"] = "Author Name"
	article.Headline["title"] = "Breaking News"
	article.Headline["body"] = "Details here"
	article.Headline["source"] = "Source A"
	article.Sections = append(article.Sections, "First section")
	article.Sections = append(article.Sections, "Second section")
	article.Conclusion = "In summary..."
	article.CTA = "Subscribe now!"

	html, err := f.Format(article, "wechat")
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	checks := []string{
		"Full Article",
		"Full digest",
		"Author Name",
		"Breaking News",
		"Details here",
		"Source A",
		"First section",
		"Second section",
		"In summary...",
		"Subscribe now!",
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("output should contain %q", check)
		}
	}
}

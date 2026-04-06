from pathlib import Path
import tempfile
import unittest

from content_hub.domain.formatting.models import ArticleDraft, FormatTarget
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog
from content_hub.infrastructure.formatters.wechat_html_formatter import WechatHtmlFormatter


class WechatValidationParityTestCase(unittest.TestCase):
    def test_validate_warns_for_non_wechat_image_urls_and_missing_cover_media_id(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            template_root = Path(tmp_dir)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))
            article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报测试", "digest": "这里是一段摘要", "date": "2026-03-31"},
                headline={"title": "头条标题", "body": ["正文"], "source": "来源"},
                sections=[
                    {
                        "cn": "要闻",
                        "en": "BRIEFING",
                        "blocks": [
                            {
                                "type": "image",
                                "url": "https://example.com/not-wechat.jpg",
                                "caption": "非微信图片",
                            }
                        ],
                    }
                ],
                conclusion="",
                cta="",
                source_refs=[],
                target_platforms=["wechat"],
            )

            errors = formatter.validate(article, FormatTarget(platform="wechat", template="daily-intelligence", output_format="html"))

            self.assertEqual(errors, [])
            self.assertTrue(any("thumb_media_id" in warning for warning in formatter.last_warnings))
            self.assertTrue(any("does not look like WeChat CDN" in warning for warning in formatter.last_warnings))

    def test_validate_reports_unresolved_placeholders_for_rendered_html(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            template_root = Path(tmp_dir)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}{{UNRESOLVED}}</html>",
                encoding="utf-8",
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))
            article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报测试", "digest": "这里是一段摘要", "date": "2026-03-31"},
                headline={"title": "头条标题", "body": ["正文"], "source": "来源"},
                sections=[{"cn": "要闻", "en": "BRIEFING", "blocks": [{"type": "card", "title": "新闻标题", "body": ["正文"], "source": "来源"}]}],
                conclusion="",
                cta="",
                source_refs=[],
                target_platforms=["wechat"],
            )

            asset = formatter.render(article, FormatTarget(platform="wechat", template="daily-intelligence", output_format="html"))

            self.assertTrue(any("unresolved placeholders" in error for error in formatter.validate_rendered_output(asset.content)))


if __name__ == "__main__":
    unittest.main()
